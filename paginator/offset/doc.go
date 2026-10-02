// Package offset serves one-based numbered pages. It follows the plan, fetch,
// page flow described in [github.com/exalynt/turn/paginator] and returns a
// [Page].
//
// [Paginator.Plan] turns a [Selector] into a [Plan] whose Offset and
// Limit the consumer applies to its canonically ordered query.
package offset
