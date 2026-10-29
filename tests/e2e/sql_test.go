//go:build e2e

package e2e

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/exalynt/turn/codec/plain"
	"github.com/exalynt/turn/paginator/cursor"
	"github.com/exalynt/turn/paginator/offset"
	"github.com/exalynt/turn/store"
	turnsql "github.com/exalynt/turn/store/sql"
)

// postgresDB connects to the test Postgres and returns a users table filled
// with the fixture users.
func postgresDB(t *testing.T) (db *sql.DB, table string) {
	t.Helper()
	db, err := sql.Open("pgx", env("TURN_POSTGRES_URL", "postgres://turn:turn@localhost:5432/turn?sslmode=disable"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return db, sqlUsers(t, db, "TIMESTAMPTZ", "$1, $2, $3, $4")
}

// mysqlDB connects to the test MySQL and returns a users table filled with the
// fixture users.
func mysqlDB(t *testing.T) (db *sql.DB, table string) {
	t.Helper()
	cfg, err := mysql.ParseDSN(env("TURN_MYSQL_DSN", "turn:turn@tcp(localhost:3306)/turn"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ParseTime = true
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	db = sql.OpenDB(connector)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return db, sqlUsers(t, db, "DATETIME(6)", "?, ?, ?, ?")
}

// sqlUsers creates a users table only this test uses, fills it with the
// fixture users, and drops it when the test ends. timestamp is the column type
// for created_at, and values the insert's placeholders.
func sqlUsers(t *testing.T, db *sql.DB, timestamp, values string) string {
	t.Helper()
	ctx := context.Background()
	table := uniqueName(t, "users")
	create := fmt.Sprintf(`CREATE TABLE %s (
		id VARCHAR(16) PRIMARY KEY,
		name VARCHAR(64) NOT NULL,
		status VARCHAR(16) NOT NULL,
		created_at %s NOT NULL)`, table, timestamp)
	if _, err := db.ExecContext(ctx, create); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), "DROP TABLE "+table); err != nil {
			t.Errorf("drop %s: %v", table, err)
		}
	})
	insert := fmt.Sprintf("INSERT INTO %s (id, name, status, created_at) VALUES (%s)", table, values)
	for _, u := range users {
		if _, err := db.ExecContext(ctx, insert, u.ID, u.Name, u.Status, u.CreatedAt); err != nil {
			t.Fatal(err)
		}
	}
	return table
}

// scanUsers reads every row of rows and closes it.
func scanUsers(rows *sql.Rows) (out []user, err error) {
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var u user
		if err := rows.Scan(&u.ID, &u.Name, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Numbered pages with only the core: plan, run your own LIMIT/OFFSET query,
// and page the rows, asking for the next page number while HasMore is true.
func TestNumberedPagesWithOwnQuery(t *testing.T) {
	ctx := context.Background()
	db, table := postgresDB(t)
	pages, err := offset.New[user](offset.Options{})
	if err != nil {
		t.Fatal(err)
	}

	list := func(selector offset.Selector) (offset.Page[user], error) {
		plan, err := pages.Plan(selector)
		if err != nil {
			return offset.Page[user]{}, err
		}
		rows, err := db.QueryContext(ctx,
			"SELECT id, name, status, created_at FROM "+table+
				" ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2",
			plan.Limit(), plan.Offset)
		if err != nil {
			return offset.Page[user]{}, err
		}
		items, err := scanUsers(rows)
		if err != nil {
			return offset.Page[user]{}, err
		}
		return pages.Page(plan, items)
	}

	var got [][]string
	selector := offset.Selector{Size: pageSize}
	for {
		page, err := list(selector)
		if err != nil {
			t.Fatal(err)
		}
		if page.Number != int64(len(got)+1) || page.Size != pageSize {
			t.Errorf("page %d: got number %d, size %d", len(got)+1, page.Number, page.Size)
		}
		got = append(got, ids(page.Items))
		if !page.HasMore || len(got) > len(users) {
			break
		}
		selector.Number = page.Number + 1
	}

	assertPages(t, "numbered", got, [][]string{
		{"u09", "u08", "u07", "u06"},
		{"u05", "u04", "u03", "u02"},
		{"u01"},
	})
}

// Cursors with only the core: a position type, the plain codec, and a keyset
// query written by hand for each direction.
func TestCursorsWithOwnQuery(t *testing.T) {
	ctx := context.Background()
	db, table := postgresDB(t)

	type position struct {
		CreatedAt time.Time
		ID        string
	}
	cursors, err := cursor.New(cursor.Options[user, position]{
		Codec:    plain.Codec[position]{},
		Position: func(u user) position { return position{CreatedAt: u.CreatedAt, ID: u.ID} },
	})
	if err != nil {
		t.Fatal(err)
	}

	forward, backward := readCursors(t, func(selector cursor.Selector) (cursor.Page[user], error) {
		plan, err := cursors.Plan(selector, "users:created_at_desc")
		if err != nil {
			return cursor.Page[user]{}, err
		}
		order, compare := "DESC", "<"
		if plan.Direction == cursor.Backward {
			order, compare = "ASC", ">"
		}
		query := "SELECT id, name, status, created_at FROM " + table
		var args []any
		if b := plan.Boundary; b != nil {
			query += fmt.Sprintf(" WHERE (created_at, id) %s ($1, $2)", compare)
			args = append(args, b.CreatedAt, b.ID)
		}
		query += fmt.Sprintf(" ORDER BY created_at %s, id %s LIMIT $%d", order, order, len(args)+1)
		args = append(args, plan.Limit())

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return cursor.Page[user]{}, err
		}
		items, err := scanUsers(rows)
		if err != nil {
			return cursor.Page[user]{}, err
		}
		return cursors.Page(plan, items)
	})

	assertPages(t, "forward", forward, [][]string{
		{"u09", "u08", "u07", "u06"},
		{"u05", "u04", "u03", "u02"},
		{"u01"},
	})
	assertPages(t, "backward", backward, [][]string{
		{"u05", "u04", "u03", "u02"},
		{"u09", "u08", "u07", "u06"},
	})
}

// newUserCursor returns the store.Cursor for users, newest first. idField is
// the name the store holds the ID under.
func newUserCursor(t *testing.T, idField string) *store.Cursor[user] {
	t.Helper()
	c, err := store.NewCursor(store.CursorOptions{Codec: plain.Codec[store.Position]{}},
		store.Desc("created_at", func(u user) time.Time { return u.CreatedAt }),
		store.Desc(idField, func(u user) string { return u.ID }),
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// listActiveSQL returns a cursor listing of the active users in table. where
// is the query's own filter, holding the status as its first placeholder;
// turnsql renders the pagination parts after it in dialect.
func listActiveSQL(t *testing.T, db *sql.DB, table string, dialect turnsql.Dialect, where string) func(cursor.Selector) (cursor.Page[user], error) {
	listing := newUserCursor(t, "id")
	return func(selector cursor.Selector) (cursor.Page[user], error) {
		return listing.List(context.Background(), selector, func(ctx context.Context, q store.Query) ([]user, error) {
			c := turnsql.Render(dialect, q, 2)
			rows, err := db.QueryContext(ctx,
				"SELECT id, name, status, created_at FROM "+table+" WHERE "+where+c.And()+c.Tail(),
				append([]any{"active"}, c.Args...)...)
			if err != nil {
				return nil, err
			}
			return scanUsers(rows)
		}, "users", "active")
	}
}

// activeForward and activeBackward are the pages of active users that
// readCursors reads.
var (
	activeForward = [][]string{
		{"u09", "u08", "u06", "u05"},
		{"u03", "u02", "u01"},
	}
	activeBackward = [][]string{
		{"u09", "u08", "u06", "u05"},
	}
)

// Cursors on PostgreSQL with store.Cursor and turnsql, under a filter.
func TestStoreCursorPostgres(t *testing.T) {
	db, table := postgresDB(t)
	forward, backward := readCursors(t, listActiveSQL(t, db, table, turnsql.Postgres, "status = $1"))
	assertPages(t, "forward", forward, activeForward)
	assertPages(t, "backward", backward, activeBackward)
}

// Cursors on MySQL with store.Cursor and turnsql, under a filter.
func TestStoreCursorMySQL(t *testing.T) {
	db, table := mysqlDB(t)
	forward, backward := readCursors(t, listActiveSQL(t, db, table, turnsql.MySQL, "status = ?"))
	assertPages(t, "forward", forward, activeForward)
	assertPages(t, "backward", backward, activeBackward)
}
