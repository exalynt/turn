// Package turn provides page-based and cursor-based pagination primitives.
//
// turn handles the parts of pagination every listing repeats: bounding the
// requested page size, turning a request into what a query needs, and shaping
// the result. It never builds or runs queries; the consumer owns query
// construction, filtering, authorization, and record mapping.
//
// This package holds what every strategy shares: the size [Policy], the
// prepared query [Window], the result [Page], and the errors. The paginators
// themselves live in [github.com/exalynt/turn/strategy].
//
// Every strategy follows the same flow: Prepare a plan from a request, run the
// consumer's own query using the plan, then Finish the plan with the fetched
// items to get a [Page].
//
//	plan, err := paginator.Prepare(request)
//	// handle err
//	rows, err := fetch(ctx, plan) // at most plan.FetchLimit() rows
//	// handle err
//	page, err := paginator.Finish(plan, rows)
//
// Each plan asks for one item more than the page size. Finish drops that
// lookahead item and reports its presence as [Page.HasMore], so no strategy
// needs a count query. Finish expects the complete result of the query: fewer
// than FetchLimit items means the query was exhausted, so a failed or partial
// fetch must be handled before calling it.
//
// Every strategy needs a deterministic canonical order, ideally ending with a
// unique tie-breaker. None promises a consistent snapshot across requests.
//
// # Errors
//
// Invalid requests are reported with [ErrInvalidSize], [ErrInvalidPage],
// [ErrOffsetTooLarge], [ErrInvalidDirection], and [ErrInvalidCursor], which a
// handler can map to a client error with [errors.Is]. [ErrInvalidOptions],
// [ErrInvalidPlan], and [ErrInvalidBatch] report misuse by the consuming code.
//
// turn is in Alpha: it is still taking shape, so expect frequent breaking
// changes. See https://readme.exalynt.com/how-it-works/stability-levels.
package turn
