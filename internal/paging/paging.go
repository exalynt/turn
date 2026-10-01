// Package paging holds the sizing and lookahead mechanics shared by the
// pagination strategies.
package paging

import (
	"fmt"
	"math"

	"github.com/exalynt/turn"
)

const (
	defaultSize    = 25
	defaultMaxSize = 100
)

// ResolvePolicy fills in the defaults of p and validates the result.
func ResolvePolicy(p turn.Policy) (turn.Policy, error) {
	if p.DefaultSize < 0 || p.MaxSize < 0 {
		return turn.Policy{}, fmt.Errorf("%w: policy sizes must not be negative", turn.ErrInvalidOptions)
	}
	if p.MaxSize == 0 {
		p.MaxSize = defaultMaxSize
	}
	if p.DefaultSize == 0 {
		p.DefaultSize = min(defaultSize, p.MaxSize)
	}
	if p.DefaultSize > p.MaxSize {
		return turn.Policy{}, fmt.Errorf("%w: default size %d exceeds maximum size %d", turn.ErrInvalidOptions, p.DefaultSize, p.MaxSize)
	}
	if p.MaxSize == math.MaxInt {
		return turn.Policy{}, fmt.Errorf("%w: maximum size leaves no room for the lookahead item", turn.ErrInvalidOptions)
	}
	return p, nil
}

// NewWindow returns the window for a requested size under the resolved
// policy p.
func NewWindow(p turn.Policy, size int) (turn.Window, error) {
	switch {
	case size == 0:
		return turn.Window{Size: p.DefaultSize}, nil
	case size < 0:
		return turn.Window{}, fmt.Errorf("%w: %d is negative", turn.ErrInvalidSize, size)
	case size > p.MaxSize:
		return turn.Window{}, fmt.Errorf("%w: %d exceeds the maximum of %d", turn.ErrInvalidSize, size, p.MaxSize)
	}
	return turn.Window{Size: size}, nil
}

// Allows reports whether the resolved policy p could have produced w.
func Allows(p turn.Policy, w turn.Window) bool {
	return w.Size >= 1 && w.Size <= p.MaxSize
}

// Trim removes the lookahead item from a fetched batch and reports whether
// it was present.
func Trim[T any](w turn.Window, items []T) ([]T, bool, error) {
	if len(items) > w.FetchLimit() {
		return nil, false, fmt.Errorf("%w: got %d items, more than the fetch limit of %d", turn.ErrInvalidBatch, len(items), w.FetchLimit())
	}
	if items == nil {
		return []T{}, false, nil
	}
	n := min(len(items), w.Size)
	return items[:n:n], len(items) > w.Size, nil
}
