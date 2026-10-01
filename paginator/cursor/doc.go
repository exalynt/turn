// Package cursor serves keyset pagination. It follows the prepare, query,
// finish flow described in [github.com/exalynt/turn/paginator] and returns a
// [paginator.Page].
//
// The consumer defines a position type holding the ordered values that
// identify an item, such as a creation time and ID, and supplies a
// [codec.Codec] that turns positions into opaque [codec.Cursor] values and
// back. [Paginator.Prepare] decodes the request's cursor into
// [Plan.Boundary], which the consumer's query reads from in the plan's
// [Direction]. Finish restores canonical order and encodes the cursors of the
// first and last items.
//
// The scope passed to Prepare identifies what a cursor is valid for, such as
// the resource, filters, authorization scope, and ordering. Cursor checks never
// replace application authorization.
package cursor
