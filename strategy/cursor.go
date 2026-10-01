package strategy

import (
	"fmt"
	"slices"

	"github.com/exalynt/turn"
	"github.com/exalynt/turn/codec"
	"github.com/exalynt/turn/internal/paging"
)

// Direction is the direction a cursor request reads relative to the listing's
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

// CursorRequest asks for a batch of items next to a cursor. The zero
// CursorRequest asks for the first batch, reading forward, at the policy's
// default size.
type CursorRequest struct {
	// Direction is the direction to read from Cursor. Values other than
	// Forward and Backward are rejected with turn.ErrInvalidDirection.
	Direction Direction

	// Size is the number of items per page. Zero selects the policy's default,
	// and negative or above-maximum sizes are rejected with
	// turn.ErrInvalidSize.
	Size int

	// Cursor is the exclusive boundary to read from. When empty, a Forward
	// request starts at the beginning of the listing and a Backward request
	// at the end.
	Cursor codec.Cursor
}

// CursorPlan is a prepared keyset query over positions of type P.
//
// For a Forward plan, the consumer selects items strictly after Boundary in
// canonical order; for a Backward plan, items strictly before Boundary in
// reverse canonical order. A nil Boundary selects from the start or end of the
// listing respectively. Either way the consumer fetches at most FetchLimit
// items.
//
// Create plans with [CursorPaginator.Prepare] and pass them to Finish
// unchanged; a plan carries the scope it was prepared for.
type CursorPlan[P any] struct {
	turn.Window

	// Direction is the direction the query reads.
	Direction Direction

	// Boundary is the decoded request cursor, or nil if the request had none.
	Boundary *P

	scope string
}

// CursorInfo describes a cursor page.
//
// To continue reading forward, request EndCursor with Forward while
// [turn.Page.HasMore] is true on a Forward page; to continue backward, request
// StartCursor with Backward while HasMore is true on a Backward page. Whether
// items exist in the opposite direction is not reported.
type CursorInfo struct {
	// Direction is the direction the page was read in.
	Direction Direction

	// StartCursor is the position of the first item in canonical order, or
	// empty if the page has no items.
	StartCursor codec.Cursor

	// EndCursor is the position of the last item in canonical order, or empty
	// if the page has no items.
	EndCursor codec.Cursor
}

// CursorOptions configures a [CursorPaginator].
type CursorOptions[T, P any] struct {
	// Policy bounds the requested page size.
	Policy turn.Policy

	// Codec converts positions to cursors and back. It is required.
	Codec codec.Codec[P]

	// Position returns an item's position in the canonical order. It must
	// identify the item uniquely, typically by ending with a unique
	// tie-breaker such as an ID, and be safe for concurrent use. It is
	// required.
	Position func(T) P
}

// CursorPaginator prepares and finishes keyset queries for items of type T
// ordered by positions of type P. Construct one per listing with [NewCursor]
// and reuse it; it is safe for concurrent use if its codec and position
// function are.
type CursorPaginator[T, P any] struct {
	policy   turn.Policy
	codec    codec.Codec[P]
	position func(T) P
}

// NewCursor returns a [CursorPaginator] configured by options. It returns an
// error wrapping [turn.ErrInvalidOptions] if the policy is invalid or the codec
// or position function is missing.
func NewCursor[T, P any](options CursorOptions[T, P]) (*CursorPaginator[T, P], error) {
	policy, err := paging.ResolvePolicy(options.Policy)
	if err != nil {
		return nil, err
	}
	if options.Codec == nil {
		return nil, fmt.Errorf("%w: codec is required", turn.ErrInvalidOptions)
	}
	if options.Position == nil {
		return nil, fmt.Errorf("%w: position function is required", turn.ErrInvalidOptions)
	}
	return &CursorPaginator[T, P]{policy: policy, codec: options.Codec, position: options.Position}, nil
}

// Prepare validates request, decodes its cursor for scope, and returns the
// plan for the consumer's query.
//
// It returns an error wrapping [turn.ErrInvalidDirection] or
// [turn.ErrInvalidSize] for an invalid request, and [turn.ErrInvalidCursor],
// together with the codec's error, if the cursor does not decode for scope.
func (p *CursorPaginator[T, P]) Prepare(request CursorRequest, scope string) (CursorPlan[P], error) {
	if !request.Direction.valid() {
		return CursorPlan[P]{}, fmt.Errorf("%w: %d", turn.ErrInvalidDirection, request.Direction)
	}
	window, err := paging.NewWindow(p.policy, request.Size)
	if err != nil {
		return CursorPlan[P]{}, err
	}
	plan := CursorPlan[P]{Window: window, Direction: request.Direction, scope: scope}
	if request.Cursor != "" {
		boundary, err := p.codec.Decode(request.Cursor, scope)
		if err != nil {
			return CursorPlan[P]{}, fmt.Errorf("%w: %w", turn.ErrInvalidCursor, err)
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
// It returns an error wrapping [turn.ErrInvalidPlan] if this paginator could
// not have prepared plan, [turn.ErrInvalidBatch] if items holds more than
// plan.FetchLimit() items, and the codec's error if encoding a cursor fails.
func (p *CursorPaginator[T, P]) Finish(plan CursorPlan[P], items []T) (turn.Page[T, CursorInfo], error) {
	if !paging.Allows(p.policy, plan.Window) || !plan.Direction.valid() {
		return turn.Page[T, CursorInfo]{}, fmt.Errorf("%w: direction %d at size %d", turn.ErrInvalidPlan, plan.Direction, plan.Size)
	}
	kept, more, err := paging.Trim(plan.Window, items)
	if err != nil {
		return turn.Page[T, CursorInfo]{}, err
	}
	if plan.Direction == Backward {
		kept = slices.Clone(kept)
		slices.Reverse(kept)
	}
	info := CursorInfo{Direction: plan.Direction}
	if len(kept) > 0 {
		if info.StartCursor, err = p.encode(kept[0], plan.scope); err != nil {
			return turn.Page[T, CursorInfo]{}, err
		}
		if info.EndCursor, err = p.encode(kept[len(kept)-1], plan.scope); err != nil {
			return turn.Page[T, CursorInfo]{}, err
		}
	}
	return turn.Page[T, CursorInfo]{Items: kept, Info: info, HasMore: more}, nil
}

// encode returns the cursor for item's position.
func (p *CursorPaginator[T, P]) encode(item T, scope string) (codec.Cursor, error) {
	cursor, err := p.codec.Encode(p.position(item), scope)
	if err != nil {
		return "", fmt.Errorf("turn: encode cursor: %w", err)
	}
	return cursor, nil
}
