package cursor

import (
	"fmt"
	"slices"

	"github.com/exalynt/turn/codec"
	"github.com/exalynt/turn/internal/paging"
	"github.com/exalynt/turn/paginator"
)

// Direction is the direction a cursor selector reads relative to the listing's
// canonical order. The zero Direction is Forward.
type Direction uint8

const (
	// Forward selects items after the boundary in canonical order.
	Forward Direction = iota

	// Backward selects items before the boundary in canonical order.
	Backward
)

// valid reports whether d is Forward or Backward.
func (d Direction) valid() bool {
	return d <= Backward
}

// Selector selects a batch of items next to a cursor. The zero
// Selector selects the first batch, reading forward, at the policy's
// default size.
type Selector struct {
	// Direction is the direction to read from Cursor. Values other than
	// Forward and Backward are rejected with paginator.ErrInvalidDirection.
	Direction Direction

	// Size is the number of items per page. Zero selects the policy's default,
	// and negative or above-maximum sizes are rejected with
	// paginator.ErrInvalidSize.
	Size int

	// Cursor is the exclusive boundary to read from. When empty, a Forward
	// selector starts at the beginning of the listing and a Backward selector
	// at the end.
	Cursor codec.Cursor
}

// Plan is a prepared keyset query over positions of type P.
//
// For a Forward plan, the consumer selects items strictly after Boundary in
// canonical order; for a Backward plan, items strictly before Boundary in
// reverse canonical order. A nil Boundary selects from the start or end of the
// listing respectively. Either way the consumer fetches at most FetchLimit
// items.
//
// Create plans with [Paginator.Prepare] and pass them to Finish
// unchanged; a plan carries the scope and cursor it was prepared for.
type Plan[P any] struct {
	paginator.Window

	// Direction is the direction the query reads.
	Direction Direction

	// Boundary is the decoded selector cursor, or nil if the selector had none.
	Boundary *P

	scope  string
	cursor codec.Cursor
}

// Page is one cursor page of items of type T.
//
// To continue reading forward, select EndCursor with Forward while HasMore is
// true on a Forward page; to continue backward, select StartCursor with
// Backward while HasMore is true on a Backward page. Whether items exist in
// the opposite direction is not reported, but a non-empty Cursor means the
// page was read from a boundary, so items existed on its other side when the
// cursor was issued.
type Page[T any] struct {
	// Items holds at most Size items, in canonical order. It is never nil,
	// and appending to it never overwrites the fetched batch.
	Items []T

	// HasMore reports whether the query found an item beyond this page in
	// the direction it was read.
	HasMore bool

	// Direction is the direction the page was read in.
	Direction Direction

	// Size is the page size the page was prepared with.
	Size int

	// Cursor is the selector's cursor the page was read from, or empty if the
	// selector had none.
	Cursor codec.Cursor

	// StartCursor is the position of the first item in canonical order, or
	// empty if the page has no items.
	StartCursor codec.Cursor

	// EndCursor is the position of the last item in canonical order, or empty
	// if the page has no items.
	EndCursor codec.Cursor
}

// Options configures a [Paginator].
type Options[T, P any] struct {
	// Policy bounds the requested page size.
	Policy paginator.Policy

	// Codec converts positions to cursors and back. It is required.
	Codec codec.Codec[P]

	// Position returns an item's position in the canonical order. It must
	// identify the item uniquely, typically by ending with a unique
	// tie-breaker such as an ID, and be safe for concurrent use. It is
	// required.
	Position func(T) P
}

// Paginator prepares and finishes keyset queries for items of type T
// ordered by positions of type P. Construct one per listing with [New]
// and reuse it; it is safe for concurrent use if its codec and position
// function are.
type Paginator[T, P any] struct {
	policy   paginator.Policy
	codec    codec.Codec[P]
	position func(T) P
}

// New returns a [Paginator] configured by options. It returns an error wrapping
// [paginator.ErrInvalidOptions] if the policy is invalid or the codec or
// position function is missing.
func New[T, P any](options Options[T, P]) (*Paginator[T, P], error) {
	policy, err := paging.ResolvePolicy(options.Policy)
	if err != nil {
		return nil, err
	}
	if options.Codec == nil {
		return nil, fmt.Errorf("%w: codec is required", paginator.ErrInvalidOptions)
	}
	if options.Position == nil {
		return nil, fmt.Errorf("%w: position function is required", paginator.ErrInvalidOptions)
	}
	return &Paginator[T, P]{policy: policy, codec: options.Codec, position: options.Position}, nil
}

// Prepare validates selector, decodes its cursor for scope, and returns the
// plan for the consumer's query.
//
// It returns an error wrapping [paginator.ErrInvalidDirection] or
// [paginator.ErrInvalidSize] for an invalid selector, and
// [paginator.ErrInvalidCursor], together with the codec's error, if the cursor
// does not decode for scope.
func (p *Paginator[T, P]) Prepare(selector Selector, scope string) (Plan[P], error) {
	if !selector.Direction.valid() {
		return Plan[P]{}, fmt.Errorf("%w: %d", paginator.ErrInvalidDirection, selector.Direction)
	}
	window, err := paging.NewWindow(p.policy, selector.Size)
	if err != nil {
		return Plan[P]{}, err
	}
	plan := Plan[P]{Window: window, Direction: selector.Direction, scope: scope, cursor: selector.Cursor}
	if selector.Cursor != "" {
		boundary, err := p.codec.Decode(selector.Cursor, scope)
		if err != nil {
			return Plan[P]{}, fmt.Errorf("%w: %w", paginator.ErrInvalidCursor, err)
		}
		plan.Boundary = &boundary
	}
	return plan, nil
}

// Finish builds the page from items, the result of running plan's query in
// the plan's direction. It trims the lookahead item, sets HasMore if it was
// present, returns Backward items in canonical order, and encodes the cursors
// of the first and last items. Finish never reorders or modifies items.
//
// It returns an error wrapping [paginator.ErrInvalidPlan] if this paginator
// could not have prepared plan, [paginator.ErrInvalidBatch] if items holds more
// than plan.FetchLimit() items, and the codec's error if encoding a cursor
// fails.
func (p *Paginator[T, P]) Finish(plan Plan[P], items []T) (Page[T], error) {
	if !paging.Allows(p.policy, plan.Window) || !plan.Direction.valid() {
		return Page[T]{}, fmt.Errorf("%w: direction %d at size %d", paginator.ErrInvalidPlan, plan.Direction, plan.Size)
	}
	kept, more, err := paging.Trim(plan.Window, items)
	if err != nil {
		return Page[T]{}, err
	}
	if plan.Direction == Backward {
		kept = slices.Clone(kept)
		slices.Reverse(kept)
	}
	page := Page[T]{Items: kept, HasMore: more, Direction: plan.Direction, Size: plan.Size, Cursor: plan.cursor}
	if len(kept) > 0 {
		if page.StartCursor, err = p.encode(kept[0], plan.scope); err != nil {
			return Page[T]{}, err
		}
		if page.EndCursor, err = p.encode(kept[len(kept)-1], plan.scope); err != nil {
			return Page[T]{}, err
		}
	}
	return page, nil
}

// encode returns the cursor for item's position.
func (p *Paginator[T, P]) encode(item T, scope string) (codec.Cursor, error) {
	cursor, err := p.codec.Encode(p.position(item), scope)
	if err != nil {
		return "", fmt.Errorf("turn: encode cursor: %w", err)
	}
	return cursor, nil
}
