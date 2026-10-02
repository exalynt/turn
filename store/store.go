package store

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/exalynt/turn/paginator"
	"github.com/exalynt/turn/paginator/cursor"
	"github.com/exalynt/turn/paginator/offset"
)

// Fetch runs q against the consumer's store and returns the complete result,
// in the order q asks for. It returns an error rather than a partial result if
// the query fails.
type Fetch[T any] func(ctx context.Context, q Query) ([]T, error)

// Key is a keyset sort key: a [Sort] plus how to read the key's value from a
// cursor position of type P.
type Key[P any] struct {
	Sort

	// Value returns the key's value in a position. The value must be one the
	// store can compare with the field, and it must not be nil: keyset
	// pagination cannot order around nulls.
	Value func(P) any
}

// Asc returns an ascending key on field, whose value in a position is read by
// value.
func Asc[P any](field string, value func(P) any) Key[P] {
	return Key[P]{Sort: Sort{Field: field}, Value: value}
}

// Desc returns a descending key on field, whose value in a position is read by
// value.
func Desc[P any](field string, value func(P) any) Key[P] {
	return Key[P]{Sort: Sort{Field: field, Desc: true}, Value: value}
}

// Cursor pairs a cursor paginator with its listing's keys. Construct one per
// listing with [NewCursor] and reuse it; it is safe for concurrent use if the
// paginator and the keys' value functions are.
type Cursor[T, P any] struct {
	paginator *cursor.Paginator[T, P]
	keys      []Key[P]
}

// NewCursor returns a [Cursor] reading p's listing in the order of keys, which
// must match the order p's positions describe.
//
// It returns an error wrapping [paginator.ErrInvalidOptions] if p is nil, keys
// is empty, a key has an empty or repeated field, or a key has no value
// function.
func NewCursor[T, P any](p *cursor.Paginator[T, P], keys ...Key[P]) (*Cursor[T, P], error) {
	if p == nil {
		return nil, fmt.Errorf("%w: paginator is required", paginator.ErrInvalidOptions)
	}
	order := make([]Sort, len(keys))
	for i, k := range keys {
		order[i] = k.Sort
	}
	if err := checkOrder(order); err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k.Value == nil {
			return nil, fmt.Errorf("%w: key %q has no value function", paginator.ErrInvalidOptions, k.Field)
		}
	}
	return &Cursor[T, P]{paginator: p, keys: slices.Clone(keys)}, nil
}

// Query returns the query for plan, which must come from the paginator's
// Prepare. A Backward plan reverses every key and keeps the boundary, so the
// query always selects items after [Query.After].
//
// It returns an error wrapping [paginator.ErrInvalidCursor] if a key's value
// in the plan's boundary is nil.
func (c *Cursor[T, P]) Query(plan cursor.Plan[P]) (Query, error) {
	backward := plan.Direction == cursor.Backward
	q := Query{Sort: make([]Sort, len(c.keys)), Limit: plan.FetchLimit()}
	for i, k := range c.keys {
		q.Sort[i] = Sort{Field: k.Field, Desc: k.Desc != backward}
	}
	if plan.Boundary != nil {
		q.After = make([]any, len(c.keys))
		for i, k := range c.keys {
			v := k.Value(*plan.Boundary)
			if isNil(v) {
				return Query{}, fmt.Errorf("%w: %s is null", paginator.ErrInvalidCursor, k.Field)
			}
			q.After[i] = v
		}
	}
	return q, nil
}

// List prepares selector in scope, fetches the page's items with fetch, and
// finishes the page. It returns the errors of the paginator's Prepare and
// Finish, the error of [Cursor.Query], and fetch's error unchanged.
func (c *Cursor[T, P]) List(ctx context.Context, selector cursor.Selector, scope string, fetch Fetch[T]) (paginator.Page[T, cursor.Info], error) {
	plan, err := c.paginator.Prepare(selector, scope)
	if err != nil {
		return paginator.Page[T, cursor.Info]{}, err
	}
	q, err := c.Query(plan)
	if err != nil {
		return paginator.Page[T, cursor.Info]{}, err
	}
	items, err := fetch(ctx, q)
	if err != nil {
		return paginator.Page[T, cursor.Info]{}, err
	}
	return c.paginator.Finish(plan, items)
}

// Offset pairs a numbered-page paginator with its listing's sort order.
// Construct one per listing with [NewOffset] and reuse it; it is safe for
// concurrent use.
type Offset[T any] struct {
	paginator *offset.Paginator[T]
	order     []Sort
}

// NewOffset returns an [Offset] reading p's listing in order, which should
// end with a unique key so that pages do not overlap.
//
// It returns an error wrapping [paginator.ErrInvalidOptions] if p is nil,
// order is empty, or a key has an empty or repeated field.
func NewOffset[T any](p *offset.Paginator[T], order ...Sort) (*Offset[T], error) {
	if p == nil {
		return nil, fmt.Errorf("%w: paginator is required", paginator.ErrInvalidOptions)
	}
	if err := checkOrder(order); err != nil {
		return nil, err
	}
	return &Offset[T]{paginator: p, order: slices.Clone(order)}, nil
}

// Query returns the query for plan, which must come from the paginator's
// Prepare.
func (o *Offset[T]) Query(plan offset.Plan) Query {
	return Query{Sort: slices.Clone(o.order), Limit: plan.FetchLimit(), Offset: plan.Offset}
}

// List prepares selector, fetches the page's items with fetch, and finishes
// the page. It returns the errors of the paginator's Prepare and Finish, and
// fetch's error unchanged.
func (o *Offset[T]) List(ctx context.Context, selector offset.Selector, fetch Fetch[T]) (paginator.Page[T, offset.Info], error) {
	plan, err := o.paginator.Prepare(selector)
	if err != nil {
		return paginator.Page[T, offset.Info]{}, err
	}
	items, err := fetch(ctx, o.Query(plan))
	if err != nil {
		return paginator.Page[T, offset.Info]{}, err
	}
	return o.paginator.Finish(plan, items)
}

// checkOrder reports whether order is a usable sort order.
func checkOrder(order []Sort) error {
	if len(order) == 0 {
		return fmt.Errorf("%w: at least one sort key is required", paginator.ErrInvalidOptions)
	}
	seen := make(map[string]bool, len(order))
	for _, s := range order {
		if s.Field == "" {
			return fmt.Errorf("%w: sort key has an empty field", paginator.ErrInvalidOptions)
		}
		if seen[s.Field] {
			return fmt.Errorf("%w: sort key %q is repeated", paginator.ErrInvalidOptions, s.Field)
		}
		seen[s.Field] = true
	}
	return nil
}

// isNil reports whether v is nil or a nil pointer, which a store would treat
// as null.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}
