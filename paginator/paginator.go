// Package paginator holds what every paginator shares: the size [Policy], the
// prepared query [Window], the result [Page], and the errors. The paginators
// themselves live in its subpackages:
// [github.com/exalynt/turn/paginator/offset] serves numbered pages and
// [github.com/exalynt/turn/paginator/cursor] serves keyset pagination.
//
// Every paginator follows the same flow: Prepare a plan from a request, run the
// consumer's own query using the plan, then Finish the plan with the fetched
// items to get a [Page].
//
//	plan, err := p.Prepare(request)
//	// handle err
//	rows, err := fetch(ctx, plan) // at most plan.FetchLimit() rows
//	// handle err
//	page, err := p.Finish(plan, rows)
//
// Each plan asks for one item more than the page size. Finish drops that
// lookahead item and reports its presence as [Page.HasMore], so no paginator
// needs a count query. Finish expects the complete result of the query: fewer
// than FetchLimit items means the query was exhausted, so a failed or partial
// fetch must be handled before calling it. Plans must reach Finish unchanged;
// Finish rejects plans its paginator could not have prepared.
//
// Every paginator needs a deterministic canonical order, ideally ending with a
// unique tie-breaker. None promises a consistent snapshot across requests.
// Construct a paginator once per listing and reuse it; paginators are safe for
// concurrent use.
//
// # Errors
//
// Invalid requests are reported with [ErrInvalidSize], [ErrInvalidPage],
// [ErrOffsetTooLarge], [ErrInvalidDirection], and [ErrInvalidCursor], which a
// handler can map to a client error with [errors.Is]. [ErrInvalidOptions],
// [ErrInvalidPlan], and [ErrInvalidBatch] report misuse by the consuming code.
package paginator

// Page is one page of results. T is the item type and I is the paginator's
// metadata, such as cursor.Info or offset.Info.
type Page[T, I any] struct {
	// Items holds at most the plan's Size items, in canonical order. It is
	// never nil, and appending to it never overwrites the fetched batch.
	Items []T

	// Info describes the page for navigating to adjacent pages.
	Info I

	// HasMore reports whether the query found an item beyond this page: in
	// the requested direction for cursor pages, and after this page for
	// numbered pages.
	HasMore bool
}
