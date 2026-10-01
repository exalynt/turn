package turn

// Page is one page of results. T is the item type and I is the strategy's
// metadata, such as strategy.CursorInfo or strategy.OffsetInfo.
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
