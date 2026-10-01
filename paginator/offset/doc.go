// Package offset serves one-based numbered pages. It follows the prepare,
// query, finish flow described in [github.com/exalynt/turn/paginator] and
// returns a [paginator.Page].
//
// [Paginator.Prepare] turns a [Request] into a [Plan] whose Offset and
// FetchLimit the consumer applies to its canonically ordered query.
package offset
