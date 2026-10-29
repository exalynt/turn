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
// [Cursor] and [Offset] hold a listing's paginator and its canonical order.
// Build one per listing and reuse it. A Cursor's keys read their values from
// an item, and those values are the item's position in its cursors:
//
//	users, err := store.NewCursor(store.CursorOptions{Codec: plain.Codec[store.Position]{}},
//		store.Desc("created_at", func(u User) time.Time { return u.CreatedAt }),
//		store.Desc("id", func(u User) string { return u.ID }),
//	)
//
// List then runs the plan, fetch, page flow around a [Fetch] function
// that renders the Query, runs it, and returns the items:
//
//	page, err := users.List(ctx, selector, func(ctx context.Context, q store.Query) ([]User, error) {
//		// render q with an adapter, apply filter, run the query, and scan the rows
//	}, "users", filter)
//
// A Cursor's cursors are scoped to its keys as well as to the scope values
// passed to List, so a cursor is rejected by a listing in another order, or
// once the keys change. [Cursor.Reverse] returns the same listing in the
// opposite order.
//
// A Query never carries a direction. For a backward read, the sort is already
// reversed, so every adapter selects items strictly after [Query.After] in
// [Query.Sort] order, and the cursor paginator restores canonical order.
//
// Sort fields are emitted verbatim by adapters. They must come from the
// consumer's code, never from request input; map a client's choice of sort
// through an allowlist, such as a map from the client's choice to a Cursor.
package store
