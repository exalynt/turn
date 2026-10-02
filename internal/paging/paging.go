// Package paging holds the sizing and lookahead mechanics shared by
// the paginators.
package paging

import (
	"fmt"
	"math"

	"github.com/exalynt/turn/paginator"
)

const (
	defaultSize    = 25
	defaultMaxSize = 100
)

// ResolvePolicy fills in the defaults of p and validates the result.
func ResolvePolicy(p paginator.Policy) (paginator.Policy, error) {
	if p.DefaultSize < 0 || p.MaxSize < 0 {
		return paginator.Policy{}, fmt.Errorf("%w: policy sizes must not be negative", paginator.ErrInvalidOptions)
	}
	if p.MaxSize == 0 {
		p.MaxSize = defaultMaxSize
	}
	if p.DefaultSize == 0 {
		p.DefaultSize = min(defaultSize, p.MaxSize)
	}
	if p.DefaultSize > p.MaxSize {
		return paginator.Policy{}, fmt.Errorf("%w: default size %d exceeds maximum size %d", paginator.ErrInvalidOptions, p.DefaultSize, p.MaxSize)
	}
	if p.MaxSize == math.MaxInt {
		return paginator.Policy{}, fmt.Errorf("%w: maximum size leaves no room for the lookahead item", paginator.ErrInvalidOptions)
	}
	return p, nil
}

// NewWindow returns the window for a requested size under the resolved
// policy p.
func NewWindow(p paginator.Policy, size int) (paginator.Window, error) {
	switch {
	case size == 0:
		return paginator.Window{Size: p.DefaultSize}, nil
	case size < 0:
		return paginator.Window{}, fmt.Errorf("%w: %d is negative", paginator.ErrInvalidSize, size)
	case size > p.MaxSize:
		return paginator.Window{}, fmt.Errorf("%w: %d exceeds the maximum of %d", paginator.ErrInvalidSize, size, p.MaxSize)
	}
	return paginator.Window{Size: size}, nil
}

// Allows reports whether the resolved policy p could have produced w.
func Allows(p paginator.Policy, w paginator.Window) bool {
	return w.Size >= 1 && w.Size <= p.MaxSize
}

// Trim removes the lookahead item from a fetched batch and reports whether
// it was present.
func Trim[T any](w paginator.Window, items []T) ([]T, bool, error) {
	if len(items) > w.Limit() {
		return nil, false, fmt.Errorf("%w: got %d items, more than the limit of %d", paginator.ErrInvalidBatch, len(items), w.Limit())
	}
	if items == nil {
		return []T{}, false, nil
	}
	n := min(len(items), w.Size)
	return items[:n:n], len(items) > w.Size, nil
}
