// Package sql renders a [store.Query] as SQL fragments: a keyset predicate,
// an ORDER BY clause, a limit clause, and their arguments. The consumer splices
// them into its own query and runs it, so the package works with
// database/sql, any driver, and any query builder. It never runs queries and
// imports no driver.
//
//	c := turnsql.Render(turnsql.Postgres, q, 2)
//	rows, err := db.QueryContext(ctx,
//		"SELECT id, name, created_at FROM users WHERE status = $1"+c.And()+c.Tail(),
//		append([]any{status}, c.Args...)...)
//
// A [Dialect] says how a database spells placeholders and limits. [Postgres]
// and [MySQL] are provided; implement Dialect for another database.
//
// Sort fields are emitted verbatim, so they may be qualified names or
// expressions, such as u.created_at. They must come from the consumer's code,
// never from request input. Keyset values must not be null.
//
// The package shares its name with database/sql, so import it under another
// name, such as turnsql, in files that use both.
package sql
