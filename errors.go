package turn

import "errors"

// Errors reported for invalid requests. A handler can map them to a client
// error, such as HTTP 400, with [errors.Is]. The returned error wraps the
// sentinel with detail about the rejected value.
var (
	// ErrInvalidSize reports a negative page size or one above the policy's
	// MaxSize.
	ErrInvalidSize = errors.New("turn: invalid page size")

	// ErrInvalidPage reports a negative page number.
	ErrInvalidPage = errors.New("turn: invalid page number")

	// ErrOffsetTooLarge reports a numbered page whose offset exceeds the
	// configured MaxOffset or the int64 range.
	ErrOffsetTooLarge = errors.New("turn: offset too large")

	// ErrInvalidDirection reports a cursor request direction other than
	// Forward or Backward.
	ErrInvalidDirection = errors.New("turn: invalid direction")

	// ErrInvalidCursor reports a cursor the codec could not decode for the
	// requested scope. The returned error also wraps the codec's error.
	ErrInvalidCursor = errors.New("turn: invalid cursor")
)

// Errors reported for misuse by the consuming code rather than by the request.
// They indicate a bug and usually map to a server error.
var (
	// ErrInvalidOptions reports paginator options that cannot be used, such as
	// a negative size policy or a missing codec.
	ErrInvalidOptions = errors.New("turn: invalid options")

	// ErrInvalidPlan reports a plan passed to Finish that the paginator could
	// not have prepared, such as a zero plan or one with altered fields.
	ErrInvalidPlan = errors.New("turn: invalid plan")

	// ErrInvalidBatch reports a fetched batch with more items than the plan's
	// FetchLimit.
	ErrInvalidBatch = errors.New("turn: invalid batch")
)
