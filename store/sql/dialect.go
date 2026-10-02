package sql

import "strconv"

// Dialect is how a SQL database spells the parts of a query that differ
// between databases. Implement it to support a database other than [Postgres]
// and [MySQL]. A Dialect must be safe for concurrent use.
type Dialect interface {
	// Placeholder returns the placeholder for the n-th argument, counting
	// from 1, such as "$3" or "?".
	Placeholder(n int) string

	// RowValues reports whether to compare a keyset boundary as one row
	// value, such as (a, b) > ($1, $2), when every key sorts the same
	// direction. Return false if the database lacks row value comparison, or
	// does not use an index for it.
	RowValues() bool

	// Limit returns the clause that skips offset rows and returns at most
	// limit rows, given the placeholders of both. offset is empty when no
	// rows are skipped.
	Limit(limit, offset string) string
}

// Postgres is the PostgreSQL dialect: numbered placeholders ($1, $2, …), row
// value comparison, and LIMIT … OFFSET.
var Postgres Dialect = postgres{}

// MySQL is the MySQL dialect: ? placeholders and LIMIT … OFFSET. It compares
// keyset boundaries one key at a time, since MySQL does not reliably use an
// index for row value comparison.
var MySQL Dialect = mysql{}

type postgres struct{}

func (postgres) Placeholder(n int) string          { return "$" + strconv.Itoa(n) }
func (postgres) RowValues() bool                   { return true }
func (postgres) Limit(limit, offset string) string { return limitOffset(limit, offset) }

type mysql struct{}

func (mysql) Placeholder(int) string            { return "?" }
func (mysql) RowValues() bool                   { return false }
func (mysql) Limit(limit, offset string) string { return limitOffset(limit, offset) }

// limitOffset returns a LIMIT clause, with an OFFSET if offset is not empty.
func limitOffset(limit, offset string) string {
	if offset == "" {
		return "LIMIT " + limit
	}
	return "LIMIT " + limit + " OFFSET " + offset
}
