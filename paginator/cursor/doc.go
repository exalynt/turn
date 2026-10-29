// Package cursor serves keyset pagination. It follows the plan, fetch, page
// flow described in [github.com/exalynt/turn/paginator] and returns a [Page].
//
// The consumer defines a position type holding the ordered values that
// identify an item, such as a creation time and ID, and supplies a
// [codec.Codec] that turns positions into opaque [codec.Cursor] values and
// back. [Paginator.Plan] decodes the selector's cursor into
// [Plan.Boundary], which the consumer's query reads from in the plan's
// [Direction]. [Paginator.Page] restores canonical order and encodes the cursors of the
// first and last items.
//
// The scope values passed to Plan identify what a cursor is valid for: a
// name for the listing and its ordering, and the filters its query is built
// from. Plan encodes them, so a filter added later is covered without
// changing the call. Cursor checks never replace application authorization.
package cursor
