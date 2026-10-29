//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	turnhttp "github.com/exalynt/turn/http"
	"github.com/exalynt/turn/paginator"
	"github.com/exalynt/turn/paginator/offset"
	"github.com/exalynt/turn/store"
	turnsql "github.com/exalynt/turn/store/sql"
)

// serve writes items as the response body, with links in a Link
// header, or reports err the way the README's handlers do.
func serve(w http.ResponseWriter, items []user, links []turnhttp.Link, err error) {
	switch {
	case paginator.IsSelectorError(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if header := turnhttp.FormatLinks(links...); header != "" {
		w.Header().Set("Link", header)
	}
	w.Header().Set("Content-Type", "application/json")
	// A body that fails to encode fails the client's decode.
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

// follow requests path from server, then each successive link with the
// relation rel, until a response has none. It returns the IDs on each page
// and the path of the last one.
func follow(t *testing.T, server *httptest.Server, path, rel string) (pages [][]string, last string) {
	t.Helper()
	for path != "" {
		if len(pages) > len(users) {
			t.Fatal("paging did not end")
		}
		res, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var body struct{ Items []user }
		err = json.NewDecoder(res.Body).Decode(&body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK || err != nil {
			t.Fatalf("GET %s: %s, %v", path, res.Status, err)
		}
		pages = append(pages, ids(body.Items))
		last, path = path, links(res.Header.Get("Link"))[rel]
	}
	return pages, last
}

// links parses a Link header into each relation's target.
func links(header string) map[string]string {
	out := map[string]string{}
	if header == "" {
		return out
	}
	for _, link := range strings.Split(header, ", ") {
		target, params, _ := strings.Cut(link, ">; ")
		rel := strings.TrimSuffix(strings.TrimPrefix(params, `rel="`), `"`)
		out[rel] = strings.TrimPrefix(target, "<")
	}
	return out
}

// An HTTP API with numbered pages on MySQL: turnhttp reads the page from the
// URL and links to the next and previous ones, which the client follows.
func TestHTTPNumberedPages(t *testing.T) {
	db, table := mysqlDB(t)
	pages, err := offset.New[user](offset.Options{})
	if err != nil {
		t.Fatal(err)
	}
	listing, err := store.NewOffset(pages,
		store.Sort{Field: "created_at", Desc: true},
		store.Sort{Field: "id", Desc: true},
	)
	if err != nil {
		t.Fatal(err)
	}

	var query turnhttp.OffsetQuery
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		selector, err := query.Parse(r.URL.Query())
		if err != nil {
			serve(w, nil, nil, err)
			return
		}
		page, err := listing.List(r.Context(), selector, func(ctx context.Context, q store.Query) ([]user, error) {
			c := turnsql.Render(turnsql.MySQL, q, 1)
			rows, err := db.QueryContext(ctx, "SELECT id, name, status, created_at FROM "+table+c.Tail(), c.Args...)
			if err != nil {
				return nil, err
			}
			return scanUsers(rows)
		})
		serve(w, page.Items, turnhttp.OffsetLinks(page, query.URL(r.URL)), err)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	next, last := follow(t, server, "/users?size=4", turnhttp.RelNext)
	prev, _ := follow(t, server, last, turnhttp.RelPrev)

	assertPages(t, "next", next, [][]string{
		{"u09", "u08", "u07", "u06"},
		{"u05", "u04", "u03", "u02"},
		{"u01"},
	})
	assertPages(t, "prev", prev, [][]string{
		{"u01"},
		{"u05", "u04", "u03", "u02"},
		{"u09", "u08", "u07", "u06"},
	})
}

// An HTTP API with cursors on PostgreSQL: turnhttp reads the cursor from the
// URL and links to the pages either side, which the client follows.
func TestHTTPCursors(t *testing.T) {
	db, table := postgresDB(t)
	list := listActiveSQL(t, db, table, turnsql.Postgres, "status = $1")

	var query turnhttp.CursorQuery
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		selector, err := query.Parse(r.URL.Query())
		if err != nil {
			serve(w, nil, nil, err)
			return
		}
		page, err := list(selector)
		serve(w, page.Items, turnhttp.CursorLinks(page, query.URL(r.URL)), err)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	next, last := follow(t, server, "/users?size=4", turnhttp.RelNext)
	prev, _ := follow(t, server, last, turnhttp.RelPrev)

	assertPages(t, "next", next, activeForward)
	assertPages(t, "prev", prev, [][]string{
		{"u03", "u02", "u01"},
		{"u09", "u08", "u06", "u05"},
	})
}
