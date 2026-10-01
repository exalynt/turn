// Package strategy provides turn's pagination strategies: numbered pages and
// cursors. Each follows the prepare, query, finish flow described in
// [github.com/exalynt/turn] and returns a [turn.Page].
//
// # Numbered pages
//
// [OffsetPaginator] serves one-based numbered pages. [OffsetPaginator.Prepare]
// turns a [PageRequest] into an [OffsetPlan] whose Offset and FetchLimit the
// consumer applies to its canonically ordered query.
//
// # Cursors
//
// [CursorPaginator] serves keyset pagination. The consumer defines a position
// type holding the ordered values that identify an item, such as a creation
// time and ID, and supplies a [codec.Codec] that turns positions into opaque
// [codec.Cursor] values and back. [CursorPaginator.Prepare] decodes the
// request's cursor into [CursorPlan.Boundary], which the consumer's query reads
// from in the plan's [Direction]. Finish restores canonical order and encodes
// the cursors of the first and last items.
//
// The scope passed to Prepare identifies what a cursor is valid for, such as
// the resource, filters, authorization scope, and ordering. Cursor checks never
// replace application authorization.
package strategy
