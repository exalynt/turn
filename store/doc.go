// Package store describes what a paginated query asks of a store, without
// depending on any database. It sits between the paginators and the adapters
// that speak to a particular database.
//
// A [Query] says how to sort, where a keyset boundary lies, and how many items
// to fetch. Adapters, such as [github.com/exalynt/turn/store/sql] for SQL
// databases and github.com/exalynt/turn/store/mongo for MongoDB, render a
// Query into the clauses and arguments their database needs. They never run
// queries; the consumer still owns the base query, filters, execution, and
// scanning. To support another store, render a Query yourself.
//
// [Cursor] and [Offset] pair a paginator with the listing's canonical order.
// Build one per listing, alongside its paginator p, and reuse it:
//
//	users, err := store.NewCursor(p,
//		store.Desc("created_at", func(p UserPosition) any { return p.CreatedAt }),
//		store.Desc("id", func(p UserPosition) any { return p.ID }),
//	)
//
// List then runs the prepare, query, finish flow around a [Fetch] function
// that renders the Query, runs it, and returns the items:
//
//	page, err := users.List(ctx, selector, scope, func(ctx context.Context, q store.Query) ([]User, error) {
//		// render q with an adapter, run the query, and scan the rows
//	})
//
// A Query never carries a direction. For a backward read, the sort is already
// reversed, so every adapter selects items strictly after [Query.After] in
// [Query.Sort] order, and the cursor paginator restores canonical order.
//
// Sort fields are emitted verbatim by adapters. They must come from the
// consumer's code, never from request input; map a client's choice of sort
// through an allowlist.
package store
