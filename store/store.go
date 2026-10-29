package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"github.com/exalynt/turn/codec"
	"github.com/exalynt/turn/paginator"
	"github.com/exalynt/turn/paginator/cursor"
	"github.com/exalynt/turn/paginator/offset"
)

// Fetch runs q against the consumer's store and returns the complete result,
// in the order q asks for. It returns an error rather than a partial result if
// the query fails.
type Fetch[T any] func(ctx context.Context, q Query) ([]T, error)

// Key is a keyset sort key of a listing of items of type T: a [Sort] plus how
// to read the key's value from an item. Build keys with [Asc] and [Desc].
type Key[T any] struct {
	Sort

	value  func(T) any
	decode func(json.RawMessage) (any, error)
}

// Asc returns an ascending key on field, whose value in an item is read by
// value.
//
// The value must be one the store can compare with the field, and it must not
// be nil: keyset pagination cannot order around nulls. Cursors hold it as
// JSON, so V must round-trip through encoding/json exactly. time.Time does,
// at nanosecond precision.
func Asc[T, V any](field string, value func(T) V) Key[T] {
	return newKey(Sort{Field: field}, value)
}

// Desc returns a descending key on field, whose value in an item is read by
// value. The value is subject to the same rules as for [Asc].
func Desc[T, V any](field string, value func(T) V) Key[T] {
	return newKey(Sort{Field: field, Desc: true}, value)
}

// newKey returns the key sorting by s whose values, of type V, are read by
// value.
func newKey[T, V any](s Sort, value func(T) V) Key[T] {
	k := Key[T]{Sort: s}
	if value == nil {
		return k
	}
	k.value = func(item T) any { return value(item) }
	k.decode = func(data json.RawMessage) (any, error) {
		var v V
		err := json.Unmarshal(data, &v)
		return v, err
	}
	return k
}

// Position is an item's place in a [Cursor] listing: its value for each key,
// in order. It encodes through encoding/json as a JSON array, so a codec for
// positions must encode them with encoding/json, as
// [github.com/exalynt/turn/codec/plain] does.
type Position struct {
	values []any             // the item's key values, when encoding
	raw    []json.RawMessage // the key values, once decoded
}

// MarshalJSON encodes p as a JSON array of its key values.
func (p Position) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.values)
}

// UnmarshalJSON decodes a JSON array of key values into p. Each value is
// decoded into its key's type when the cursor is used.
func (p *Position) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &p.raw)
}

// CursorOptions configures a [Cursor].
type CursorOptions struct {
	// Policy bounds the requested page size.
	Policy paginator.Policy

	// Codec converts positions to cursors and back. It is required, and must
	// be safe for concurrent use.
	Codec codec.Codec[Position]
}

// Cursor paginates a listing of items of type T by keyset, in the order of
// its keys. Construct one per listing with [NewCursor] and reuse it; it is
// safe for concurrent use if the codec and the keys' value functions are.
type Cursor[T any] struct {
	paginator *cursor.Paginator[T, Position]
	keys      []Key[T]
	order     []Sort
}

// NewCursor returns a [Cursor] ordering its listing by keys, built with [Asc]
// and [Desc]. The keys' values are an item's position, so together they must
// identify it uniquely, typically by ending with a unique key such as an ID.
//
// It returns an error wrapping [paginator.ErrInvalidOptions] if the policy is
// invalid, the codec is missing, keys is empty, or a key has an empty or
// repeated field or no value function.
func NewCursor[T any](options CursorOptions, keys ...Key[T]) (*Cursor[T], error) {
	order := sorts(keys)
	if err := checkOrder(order); err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k.value == nil {
			return nil, fmt.Errorf("%w: key %q has no value function", paginator.ErrInvalidOptions, k.Field)
		}
	}
	keys = slices.Clone(keys)
	p, err := cursor.New(cursor.Options[T, Position]{
		Policy: options.Policy,
		Codec:  options.Codec,
		Position: func(item T) Position {
			values := make([]any, len(keys))
			for i, k := range keys {
				values[i] = k.value(item)
			}
			return Position{values: values}
		},
	})
	if err != nil {
		return nil, err
	}
	return &Cursor[T]{paginator: p, keys: keys, order: order}, nil
}

// Reverse returns a Cursor over the same listing in the opposite order, with
// every key's direction flipped. Each scopes its cursors to its own order, so
// each rejects the other's.
func (c *Cursor[T]) Reverse() *Cursor[T] {
	keys := slices.Clone(c.keys)
	for i := range keys {
		keys[i].Desc = !keys[i].Desc
	}
	return &Cursor[T]{paginator: c.paginator, keys: keys, order: sorts(keys)}
}

// List plans selector in scope, fetches the page's items with fetch, and
// builds the page.
//
// The scope values are passed to the paginator's Plan method, which describes
// them, together with the listing's keys, so a cursor issued for another
// order, including the [Cursor.Reverse] of this one, or before the keys
// changed is rejected. Pass a name for the listing and the filters its query
// is built from; the scope needn't name the order.
//
// It returns the errors of the paginator's Plan and Page methods, an error
// wrapping [paginator.ErrInvalidCursor] if the cursor's values don't fit the
// keys or one of them is null, and fetch's error unchanged.
func (c *Cursor[T]) List(ctx context.Context, selector cursor.Selector, fetch Fetch[T], scope ...any) (cursor.Page[T], error) {
	plan, err := c.paginator.Plan(selector, append([]any{c.order}, scope...)...)
	if err != nil {
		return cursor.Page[T]{}, err
	}
	q, err := c.query(plan)
	if err != nil {
		return cursor.Page[T]{}, err
	}
	items, err := fetch(ctx, q)
	if err != nil {
		return cursor.Page[T]{}, err
	}
	return c.paginator.Page(plan, items)
}

// query returns the query for plan. A Backward plan reverses every key and
// keeps the boundary, so the query always selects items after [Query.After].
func (c *Cursor[T]) query(plan cursor.Plan[Position]) (Query, error) {
	backward := plan.Direction == cursor.Backward
	q := Query{Sort: make([]Sort, len(c.order)), Limit: plan.Limit()}
	for i, s := range c.order {
		q.Sort[i] = Sort{Field: s.Field, Desc: s.Desc != backward}
	}
	if plan.Boundary == nil {
		return q, nil
	}
	raw := plan.Boundary.raw
	if len(raw) != len(c.keys) {
		return Query{}, fmt.Errorf("%w: %d values for %d keys", paginator.ErrInvalidCursor, len(raw), len(c.keys))
	}
	q.After = make([]any, len(c.keys))
	for i, k := range c.keys {
		v, err := k.decode(raw[i])
		if err != nil {
			return Query{}, fmt.Errorf("%w: %s: %w", paginator.ErrInvalidCursor, k.Field, err)
		}
		if isNil(v) {
			return Query{}, fmt.Errorf("%w: %s is null", paginator.ErrInvalidCursor, k.Field)
		}
		q.After[i] = v
	}
	return q, nil
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
// Plan method.
func (o *Offset[T]) Query(plan offset.Plan) Query {
	return Query{Sort: slices.Clone(o.order), Limit: plan.Limit(), Offset: plan.Offset}
}

// List plans selector, fetches the page's items with fetch, and builds the
// page. It returns the errors of the paginator's Plan and Page methods, and
// fetch's error unchanged.
func (o *Offset[T]) List(ctx context.Context, selector offset.Selector, fetch Fetch[T]) (offset.Page[T], error) {
	plan, err := o.paginator.Plan(selector)
	if err != nil {
		return offset.Page[T]{}, err
	}
	items, err := fetch(ctx, o.Query(plan))
	if err != nil {
		return offset.Page[T]{}, err
	}
	return o.paginator.Page(plan, items)
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

// sorts returns the sort order of keys.
func sorts[T any](keys []Key[T]) []Sort {
	order := make([]Sort, len(keys))
	for i, k := range keys {
		order[i] = k.Sort
	}
	return order
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
