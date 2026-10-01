package paging

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/exalynt/turn"
)

func TestResolvePolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  turn.Policy
		want    turn.Policy
		wantErr error
	}{
		{name: "zero policy", policy: turn.Policy{}, want: turn.Policy{DefaultSize: 25, MaxSize: 100}},
		{name: "explicit sizes", policy: turn.Policy{DefaultSize: 10, MaxSize: 50}, want: turn.Policy{DefaultSize: 10, MaxSize: 50}},
		{name: "default only", policy: turn.Policy{DefaultSize: 40}, want: turn.Policy{DefaultSize: 40, MaxSize: 100}},
		{name: "max only above default", policy: turn.Policy{MaxSize: 500}, want: turn.Policy{DefaultSize: 25, MaxSize: 500}},
		{name: "max only below default", policy: turn.Policy{MaxSize: 10}, want: turn.Policy{DefaultSize: 10, MaxSize: 10}},
		{name: "default equals max", policy: turn.Policy{DefaultSize: 50, MaxSize: 50}, want: turn.Policy{DefaultSize: 50, MaxSize: 50}},
		{name: "max of one", policy: turn.Policy{MaxSize: 1}, want: turn.Policy{DefaultSize: 1, MaxSize: 1}},
		{name: "largest max", policy: turn.Policy{MaxSize: math.MaxInt - 1}, want: turn.Policy{DefaultSize: 25, MaxSize: math.MaxInt - 1}},
		{name: "negative default", policy: turn.Policy{DefaultSize: -1}, wantErr: turn.ErrInvalidOptions},
		{name: "negative max", policy: turn.Policy{MaxSize: -1}, wantErr: turn.ErrInvalidOptions},
		{name: "default above max", policy: turn.Policy{DefaultSize: 51, MaxSize: 50}, wantErr: turn.ErrInvalidOptions},
		{name: "default above implied max", policy: turn.Policy{DefaultSize: 101}, wantErr: turn.ErrInvalidOptions},
		{name: "max leaves no room for lookahead", policy: turn.Policy{MaxSize: math.MaxInt}, wantErr: turn.ErrInvalidOptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePolicy(tt.policy)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ResolvePolicy(%+v) error = %v, want %v", tt.policy, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ResolvePolicy(%+v) = %+v, want %+v", tt.policy, got, tt.want)
			}
		})
	}
}

func TestNewWindow(t *testing.T) {
	policy := turn.Policy{DefaultSize: 10, MaxSize: 50}
	tests := []struct {
		name    string
		size    int
		want    turn.Window
		wantErr error
	}{
		{name: "zero selects default", size: 0, want: turn.Window{Size: 10}},
		{name: "one", size: 1, want: turn.Window{Size: 1}},
		{name: "max", size: 50, want: turn.Window{Size: 50}},
		{name: "above max", size: 51, wantErr: turn.ErrInvalidSize},
		{name: "negative", size: -1, wantErr: turn.ErrInvalidSize},
		{name: "min int", size: math.MinInt, wantErr: turn.ErrInvalidSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewWindow(policy, tt.size)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewWindow(%d) error = %v, want %v", tt.size, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NewWindow(%d) = %+v, want %+v", tt.size, got, tt.want)
			}
		})
	}
}

func TestAllows(t *testing.T) {
	policy := turn.Policy{DefaultSize: 10, MaxSize: 50}
	tests := []struct {
		size int
		want bool
	}{
		{size: -1, want: false},
		{size: 0, want: false},
		{size: 1, want: true},
		{size: 50, want: true},
		{size: 51, want: false},
	}
	for _, tt := range tests {
		if got := Allows(policy, turn.Window{Size: tt.size}); got != tt.want {
			t.Errorf("Allows(Window{Size: %d}) = %t, want %t", tt.size, got, tt.want)
		}
	}
}

func TestTrim(t *testing.T) {
	window := turn.Window{Size: 3}
	tests := []struct {
		name     string
		items    []int
		want     []int
		wantMore bool
		wantErr  error
	}{
		{name: "nil", items: nil, want: []int{}},
		{name: "empty", items: []int{}, want: []int{}},
		{name: "partial", items: []int{1, 2}, want: []int{1, 2}},
		{name: "exactly size", items: []int{1, 2, 3}, want: []int{1, 2, 3}},
		{name: "with lookahead", items: []int{1, 2, 3, 4}, want: []int{1, 2, 3}, wantMore: true},
		{name: "above fetch limit", items: []int{1, 2, 3, 4, 5}, wantErr: turn.ErrInvalidBatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, more, err := Trim(window, tt.items)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Trim(%v) error = %v, want %v", tt.items, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got == nil {
				t.Errorf("Trim(%v) returned nil items", tt.items)
			}
			if !slices.Equal(got, tt.want) || more != tt.wantMore {
				t.Errorf("Trim(%v) = %v, %t, want %v, %t", tt.items, got, more, tt.want, tt.wantMore)
			}
		})
	}
}

func TestTrimAppendKeepsBatch(t *testing.T) {
	items := []int{1, 2, 3, 4}
	kept, _, err := Trim(turn.Window{Size: 3}, items)
	if err != nil {
		t.Fatal(err)
	}
	_ = append(kept, 99)
	if items[3] != 4 {
		t.Errorf("appending to trimmed items overwrote the batch: %v", items)
	}
}
