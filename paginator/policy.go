package paginator

// Policy bounds the page sizes a request may ask for. Every paginator uses it.
//
// A zero MaxSize selects 100, and a zero DefaultSize selects 25 or MaxSize,
// whichever is smaller, so the zero Policy defaults to 25 and allows up to 100.
// Paginator constructors reject a policy with negative sizes, a DefaultSize
// above MaxSize, or a MaxSize of [math.MaxInt], with [ErrInvalidOptions].
type Policy struct {
	// DefaultSize is the size used when a request leaves Size zero.
	DefaultSize int

	// MaxSize is the largest size a request may ask for. Larger sizes are
	// rejected with ErrInvalidSize rather than clamped.
	MaxSize int
}

// Window is the size of a prepared query. Every paginator's plan embeds it.
type Window struct {
	// Size is the most items the page will hold. It is at least 1 in a
	// prepared plan.
	Size int
}

// FetchLimit returns how many items the consumer's query should fetch: Size
// plus one lookahead item, which tells Finish whether more items exist without
// a count query.
func (w Window) FetchLimit() int {
	return w.Size + 1
}
