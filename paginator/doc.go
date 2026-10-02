// Package paginator holds what every paginator shares: the size [Policy], the
// prepared query [Window], and the errors. The paginators themselves, each
// with its own page type, live in its subpackages:
// [github.com/exalynt/turn/paginator/offset] serves numbered pages and
// [github.com/exalynt/turn/paginator/cursor] serves keyset pagination.
//
// Every paginator follows the same flow: Prepare a plan from a selector, run
// the consumer's own query using the plan, then Finish the plan with the
// fetched items to get a page.
//
//	plan, err := p.Prepare(selector)
//	// handle err
//	rows, err := fetch(ctx, plan) // at most plan.Limit() rows
//	// handle err
//	page, err := p.Finish(plan, rows)
//
// Each plan asks for one item more than the page size. Finish drops that
// lookahead item and reports its presence as the page's HasMore, so no
// paginator needs a count query. Finish expects the complete result of the
// query: fewer than Limit items means the query was exhausted, so a failed
// or partial fetch must be handled before calling it. Plans must reach Finish
// unchanged; Finish rejects plans its paginator could not have prepared.
//
// Every paginator needs a deterministic canonical order, ideally ending with a
// unique tie-breaker. None promises a consistent snapshot across requests.
// Construct a paginator once per listing and reuse it; paginators are safe for
// concurrent use.
//
// # Errors
//
// Invalid selectors are reported with [ErrInvalidSize], [ErrInvalidPage],
// [ErrOffsetTooLarge], [ErrInvalidDirection], and [ErrInvalidCursor].
// [IsSelectorError] reports whether an error wraps any of them, so the caller
// can report it as bad input. [ErrInvalidOptions], [ErrInvalidPlan], and
// [ErrInvalidBatch] report misuse by the consuming code.
package paginator
