package offset

import (
	"fmt"
	"math"

	"github.com/exalynt/turn/internal/paging"
	"github.com/exalynt/turn/paginator"
)

// Selector selects a numbered page. The zero Selector selects page 1
// at the policy's default size.
type Selector struct {
	// Number is the one-based page number. Zero selects page 1, and negative
	// numbers are rejected with paginator.ErrInvalidPage.
	Number int64

	// Size is the number of items per page. Zero selects the policy's default,
	// and negative or above-maximum sizes are rejected with
	// paginator.ErrInvalidSize.
	Size int
}

// Plan is a prepared numbered-page query. The consumer skips Offset
// items of its canonically ordered query and fetches at most Limit items.
//
// Create plans with [Paginator.Prepare] and pass them to Finish
// unchanged.
type Plan struct {
	paginator.Window

	// Number is the one-based page number, with zero already resolved to 1.
	Number int64

	// Offset is the number of items before this page: (Number - 1) * Size.
	Offset int64
}

// Page is one numbered page of items of type T.
//
// When HasMore is true, page Number + 1 continues the listing. A Number above
// 1 permits navigating to an earlier page, but does not establish that earlier
// pages still hold items. Selecting a different Size changes which items each
// page number covers.
type Page[T any] struct {
	// Items holds at most Size items, in canonical order. It is never nil,
	// and appending to it never overwrites the fetched batch.
	Items []T

	// HasMore reports whether the query found an item after this page.
	HasMore bool

	// Number is the one-based page number.
	Number int64

	// Size is the page size the page was prepared with.
	Size int
}

// Options configures a [Paginator].
type Options struct {
	// Policy bounds the requested page size.
	Policy paginator.Policy

	// MaxOffset, when not nil, caps the offset a selector may reach; deeper
	// pages are rejected with paginator.ErrOffsetTooLarge. It must not be
	// negative. When nil, only the int64 range limits the offset.
	MaxOffset *int64
}

// Paginator prepares and finishes numbered-page queries for items of
// type T. Construct one per listing with [New] and reuse it; it is safe
// for concurrent use.
type Paginator[T any] struct {
	policy    paginator.Policy
	maxOffset int64
}

// New returns a [Paginator] configured by options. It returns an
// error wrapping [paginator.ErrInvalidOptions] if the policy or MaxOffset is
// invalid.
func New[T any](options Options) (*Paginator[T], error) {
	policy, err := paging.ResolvePolicy(options.Policy)
	if err != nil {
		return nil, err
	}
	maxOffset := int64(math.MaxInt64)
	if options.MaxOffset != nil {
		if *options.MaxOffset < 0 {
			return nil, fmt.Errorf("%w: maximum offset %d is negative", paginator.ErrInvalidOptions, *options.MaxOffset)
		}
		maxOffset = *options.MaxOffset
	}
	return &Paginator[T]{policy: policy, maxOffset: maxOffset}, nil
}

// Prepare validates selector and returns the plan for the consumer's query.
//
// It returns an error wrapping [paginator.ErrInvalidSize] or
// [paginator.ErrInvalidPage] for an invalid selector, and
// [paginator.ErrOffsetTooLarge] if the page's offset exceeds MaxOffset or the
// int64 range. A page past the end of the data is not an error; it finishes as
// an empty page.
func (p *Paginator[T]) Prepare(selector Selector) (Plan, error) {
	window, err := paging.NewWindow(p.policy, selector.Size)
	if err != nil {
		return Plan{}, err
	}
	number := selector.Number
	switch {
	case number == 0:
		number = 1
	case number < 0:
		return Plan{}, fmt.Errorf("%w: %d is negative", paginator.ErrInvalidPage, number)
	}
	offset, err := p.offset(number, window.Size)
	if err != nil {
		return Plan{}, err
	}
	return Plan{Window: window, Number: number, Offset: offset}, nil
}

// Finish builds the page from items, the result of running plan's query. It
// trims the lookahead item and sets HasMore if it was present. Finish never
// reorders or modifies items.
//
// It returns an error wrapping [paginator.ErrInvalidPlan] if this paginator
// could not have prepared plan, and [paginator.ErrInvalidBatch] if items holds
// more than plan.Limit() items.
func (p *Paginator[T]) Finish(plan Plan, items []T) (Page[T], error) {
	if err := p.check(plan); err != nil {
		return Page[T]{}, err
	}
	kept, more, err := paging.Trim(plan.Window, items)
	if err != nil {
		return Page[T]{}, err
	}
	return Page[T]{Items: kept, HasMore: more, Number: plan.Number, Size: plan.Size}, nil
}

// offset returns (number - 1) * size, or an error if it exceeds maxOffset.
// number and size must be positive.
func (p *Paginator[T]) offset(number int64, size int) (int64, error) {
	if number-1 > p.maxOffset/int64(size) {
		return 0, fmt.Errorf("%w: page %d at size %d", paginator.ErrOffsetTooLarge, number, size)
	}
	return (number - 1) * int64(size), nil
}

// check reports whether p could have prepared plan.
func (p *Paginator[T]) check(plan Plan) error {
	if !paging.Allows(p.policy, plan.Window) || plan.Number < 1 {
		return fmt.Errorf("%w: page %d at size %d", paginator.ErrInvalidPlan, plan.Number, plan.Size)
	}
	if offset, err := p.offset(plan.Number, plan.Size); err != nil || offset != plan.Offset {
		return fmt.Errorf("%w: offset %d does not match page %d at size %d", paginator.ErrInvalidPlan, plan.Offset, plan.Number, plan.Size)
	}
	return nil
}
