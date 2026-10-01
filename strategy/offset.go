package strategy

import (
	"fmt"
	"math"

	"github.com/exalynt/turn"
	"github.com/exalynt/turn/internal/paging"
)

// PageRequest asks for a numbered page. The zero PageRequest asks for page 1
// at the policy's default size.
type PageRequest struct {
	// Number is the one-based page number. Zero selects page 1, and negative
	// numbers are rejected with turn.ErrInvalidPage.
	Number int64

	// Size is the number of items per page. Zero selects the policy's default,
	// and negative or above-maximum sizes are rejected with
	// turn.ErrInvalidSize.
	Size int
}

// OffsetPlan is a prepared numbered-page query. The consumer skips Offset
// items of its canonically ordered query and fetches at most FetchLimit items.
//
// Create plans with [OffsetPaginator.Prepare] and pass them to Finish
// unchanged.
type OffsetPlan struct {
	turn.Window

	// Number is the one-based page number, with zero already resolved to 1.
	Number int64

	// Offset is the number of items before this page: (Number - 1) * Size.
	Offset int64
}

// OffsetInfo describes a numbered page.
//
// When [turn.Page.HasMore] is true, page Number + 1 continues the listing. A
// Number above 1 permits navigating to an earlier page, but does not establish
// that earlier pages still hold items. Requesting a different Size changes
// which items each page number covers.
type OffsetInfo struct {
	// Number is the one-based page number.
	Number int64

	// Size is the page size the page was prepared with.
	Size int
}

// OffsetOptions configures an [OffsetPaginator].
type OffsetOptions struct {
	// Policy bounds the requested page size.
	Policy turn.Policy

	// MaxOffset, when not nil, caps the offset a request may reach; deeper
	// pages are rejected with turn.ErrOffsetTooLarge. It must not be negative.
	// When nil, only the int64 range limits the offset.
	MaxOffset *int64
}

// OffsetPaginator prepares and finishes numbered-page queries for items of
// type T. Construct one per listing with [NewOffset] and reuse it; it is safe
// for concurrent use.
type OffsetPaginator[T any] struct {
	policy    turn.Policy
	maxOffset int64
}

// NewOffset returns an [OffsetPaginator] configured by options. It returns an
// error wrapping [turn.ErrInvalidOptions] if the policy or MaxOffset is
// invalid.
func NewOffset[T any](options OffsetOptions) (*OffsetPaginator[T], error) {
	policy, err := paging.ResolvePolicy(options.Policy)
	if err != nil {
		return nil, err
	}
	maxOffset := int64(math.MaxInt64)
	if options.MaxOffset != nil {
		if *options.MaxOffset < 0 {
			return nil, fmt.Errorf("%w: maximum offset %d is negative", turn.ErrInvalidOptions, *options.MaxOffset)
		}
		maxOffset = *options.MaxOffset
	}
	return &OffsetPaginator[T]{policy: policy, maxOffset: maxOffset}, nil
}

// Prepare validates request and returns the plan for the consumer's query.
//
// It returns an error wrapping [turn.ErrInvalidSize] or [turn.ErrInvalidPage]
// for an invalid request, and [turn.ErrOffsetTooLarge] if the page's offset
// exceeds MaxOffset or the int64 range. A page past the end of the data is not
// an error; it finishes as an empty page.
func (p *OffsetPaginator[T]) Prepare(request PageRequest) (OffsetPlan, error) {
	window, err := paging.NewWindow(p.policy, request.Size)
	if err != nil {
		return OffsetPlan{}, err
	}
	number := request.Number
	switch {
	case number == 0:
		number = 1
	case number < 0:
		return OffsetPlan{}, fmt.Errorf("%w: %d is negative", turn.ErrInvalidPage, number)
	}
	offset, err := p.offset(number, window.Size)
	if err != nil {
		return OffsetPlan{}, err
	}
	return OffsetPlan{Window: window, Number: number, Offset: offset}, nil
}

// Finish builds the page from items, the result of running plan's query. It
// trims the lookahead item and sets HasMore if it was present. Finish never
// reorders or modifies items.
//
// It returns an error wrapping [turn.ErrInvalidPlan] if this paginator could
// not have prepared plan, and [turn.ErrInvalidBatch] if items holds more than
// plan.FetchLimit() items.
func (p *OffsetPaginator[T]) Finish(plan OffsetPlan, items []T) (turn.Page[T, OffsetInfo], error) {
	if err := p.check(plan); err != nil {
		return turn.Page[T, OffsetInfo]{}, err
	}
	kept, more, err := paging.Trim(plan.Window, items)
	if err != nil {
		return turn.Page[T, OffsetInfo]{}, err
	}
	return turn.Page[T, OffsetInfo]{
		Items:   kept,
		Info:    OffsetInfo{Number: plan.Number, Size: plan.Size},
		HasMore: more,
	}, nil
}

// offset returns (number - 1) * size, or an error if it exceeds maxOffset.
// number and size must be positive.
func (p *OffsetPaginator[T]) offset(number int64, size int) (int64, error) {
	if number-1 > p.maxOffset/int64(size) {
		return 0, fmt.Errorf("%w: page %d at size %d", turn.ErrOffsetTooLarge, number, size)
	}
	return (number - 1) * int64(size), nil
}

// check reports whether p could have prepared plan.
func (p *OffsetPaginator[T]) check(plan OffsetPlan) error {
	if !paging.Allows(p.policy, plan.Window) || plan.Number < 1 {
		return fmt.Errorf("%w: page %d at size %d", turn.ErrInvalidPlan, plan.Number, plan.Size)
	}
	if offset, err := p.offset(plan.Number, plan.Size); err != nil || offset != plan.Offset {
		return fmt.Errorf("%w: offset %d does not match page %d at size %d", turn.ErrInvalidPlan, plan.Offset, plan.Number, plan.Size)
	}
	return nil
}
