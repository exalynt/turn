package paginator

import (
	"errors"
	"fmt"
)

// Errors reported for an invalid selector: a page selection the paginator's
// caller supplied, such as from an HTTP request or command-line flags, that it
// cannot serve. [IsSelectorError] reports whether an error wraps one of them.
// The returned error wraps the sentinel with detail about the rejected value.
var (
	// ErrInvalidSize reports a page size that is not an integer, negative, or
	// above the policy's MaxSize. A paginator's Plan method reports a size
	// outside the policy's bounds with a [SizeError], which wraps it.
	ErrInvalidSize = errors.New("turn: invalid page size")

	// ErrInvalidPage reports a negative page number.
	ErrInvalidPage = errors.New("turn: invalid page number")

	// ErrOffsetTooLarge reports a numbered page whose offset exceeds the
	// configured MaxOffset or the int64 range. A paginator's Plan method
	// reports it with an [OffsetError], which wraps it.
	ErrOffsetTooLarge = errors.New("turn: offset too large")

	// ErrInvalidDirection reports a cursor selector direction other than
	// Forward or Backward.
	ErrInvalidDirection = errors.New("turn: invalid direction")

	// ErrInvalidCursor reports a cursor the codec could not decode for the
	// requested scope. The returned error also wraps the codec's error.
	ErrInvalidCursor = errors.New("turn: invalid cursor")
)

// SizeError reports a requested page size outside a policy's bounds: below
// zero, or above MaxSize. It wraps [ErrInvalidSize], and holds the bounds so a
// caller can tell whoever asked which sizes are allowed without repeating the
// policy: from 1 to MaxSize, with zero selecting the policy's default.
type SizeError struct {
	// Size is the requested size.
	Size int

	// MaxSize is the largest size the policy allows.
	MaxSize int
}

func (e *SizeError) Error() string {
	if e.Size < 0 {
		return fmt.Sprintf("%v: %d is negative", ErrInvalidSize, e.Size)
	}
	return fmt.Sprintf("%v: %d exceeds the maximum of %d", ErrInvalidSize, e.Size, e.MaxSize)
}

// Unwrap returns [ErrInvalidSize].
func (e *SizeError) Unwrap() error {
	return ErrInvalidSize
}

// OffsetError reports a numbered page deeper than a paginator allows: one
// whose offset exceeds its MaxOffset or the int64 range. It wraps
// [ErrOffsetTooLarge], and holds the deepest page allowed at the requested
// size, so a caller can tell whoever asked without repeating the paginator's
// options.
type OffsetError struct {
	// Number is the requested page number.
	Number int64

	// Size is the requested page size, with zero already resolved to the
	// policy's default.
	Size int

	// MaxNumber is the deepest page number allowed at Size.
	MaxNumber int64
}

func (e *OffsetError) Error() string {
	return fmt.Sprintf("%v: page %d at size %d", ErrOffsetTooLarge, e.Number, e.Size)
}

// Unwrap returns [ErrOffsetTooLarge].
func (e *OffsetError) Unwrap() error {
	return ErrOffsetTooLarge
}

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
