package paginator

import "errors"

// Errors reported for an invalid selector: a page selection the paginator's
// caller supplied, such as from an HTTP request or command-line flags, that it
// cannot serve. [IsSelectorError] reports whether an error wraps one of them.
// The returned error wraps the sentinel with detail about the rejected value.
var (
	// ErrInvalidSize reports a negative page size or one above the policy's
	// MaxSize.
	ErrInvalidSize = errors.New("turn: invalid page size")

	// ErrInvalidPage reports a negative page number.
	ErrInvalidPage = errors.New("turn: invalid page number")

	// ErrOffsetTooLarge reports a numbered page whose offset exceeds the
	// configured MaxOffset or the int64 range.
	ErrOffsetTooLarge = errors.New("turn: offset too large")

	// ErrInvalidDirection reports a cursor selector direction other than
	// Forward or Backward.
	ErrInvalidDirection = errors.New("turn: invalid direction")

	// ErrInvalidCursor reports a cursor the codec could not decode for the
	// requested scope. The returned error also wraps the codec's error.
	ErrInvalidCursor = errors.New("turn: invalid cursor")
)

// selectorErrors are the errors [IsSelectorError] reports.
var selectorErrors = []error{
	ErrInvalidSize,
	ErrInvalidPage,
	ErrOffsetTooLarge,
	ErrInvalidDirection,
	ErrInvalidCursor,
}

// IsSelectorError reports whether err wraps one of the errors reported for an
// invalid selector, so the caller can report it as bad input, such as an HTTP
// 400 or a command-line usage error, and treat every other error as a failure
// of its own. It reports false for nil and for the errors reported for misuse
// by the consuming code.
func IsSelectorError(err error) bool {
	for _, target := range selectorErrors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// Errors reported for misuse by the consuming code rather than by the selector.
// They indicate a bug, not bad input.
var (
	// ErrInvalidOptions reports paginator options that cannot be used, such as
	// a negative size policy or a missing codec.
	ErrInvalidOptions = errors.New("turn: invalid options")

	// ErrInvalidPlan reports a plan passed to Page that the paginator could
	// not have produced, such as a zero plan or one with altered fields.
	ErrInvalidPlan = errors.New("turn: invalid plan")

	// ErrInvalidBatch reports a fetched batch with more items than the plan's
	// Limit.
	ErrInvalidBatch = errors.New("turn: invalid batch")
)
