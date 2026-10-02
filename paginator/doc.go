// Package paginator holds what every paginator shares: the size [Policy], the
// planned query [Window], and the errors. The paginators themselves, each
// with its own page type, live in its subpackages:
// [github.com/exalynt/turn/paginator/offset] serves numbered pages and
// [github.com/exalynt/turn/paginator/cursor] serves keyset pagination.
//
// Every paginator follows the same three steps: plan, fetch, page. Plan turns
// a selector into a plan, the consumer fetches items with its own query using
// the plan, and Page builds a page from the plan and the fetched items.
//
//	plan, err := p.Plan(selector)
//	// handle err
//	rows, err := fetch(ctx, plan) // at most plan.Limit() rows
//	// handle err
//	page, err := p.Page(plan, rows)
//
// Each plan asks for one item more than the page size. Page drops that
// lookahead item and reports its presence as the page's HasMore, so no
// paginator needs a count query. Page expects the complete result of the
// fetch: fewer than Limit items means the query was exhausted, so a failed
// or partial fetch must be handled before calling it. Plans must reach Page
// unchanged; Page rejects plans its paginator could not have produced.
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
