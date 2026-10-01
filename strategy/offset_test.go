package strategy_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/exalynt/turn"
	"github.com/exalynt/turn/strategy"
)

func newOffset(t *testing.T, options strategy.OffsetOptions) *strategy.OffsetPaginator[int] {
	t.Helper()
	p, err := strategy.NewOffset[int](options)
	if err != nil {
		t.Fatalf("NewOffset(%+v) error = %v", options, err)
	}
	return p
}

func prepareOffset(t *testing.T, p *strategy.OffsetPaginator[int], request strategy.PageRequest) strategy.OffsetPlan {
	t.Helper()
	plan, err := p.Prepare(request)
	if err != nil {
		t.Fatalf("Prepare(%+v) error = %v", request, err)
	}
	return plan
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestNewOffset(t *testing.T) {
	tests := []struct {
		name    string
		options strategy.OffsetOptions
		wantErr error
	}{
		{name: "zero options", options: strategy.OffsetOptions{}},
		{name: "zero max offset", options: strategy.OffsetOptions{MaxOffset: int64Ptr(0)}},
		{name: "invalid policy", options: strategy.OffsetOptions{Policy: turn.Policy{DefaultSize: 20, MaxSize: 10}}, wantErr: turn.ErrInvalidOptions},
		{name: "negative max offset", options: strategy.OffsetOptions{MaxOffset: int64Ptr(-1)}, wantErr: turn.ErrInvalidOptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := strategy.NewOffset[int](tt.options)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewOffset error = %v, want %v", err, tt.wantErr)
			}
			if (p == nil) != (err != nil) {
				t.Errorf("NewOffset = %v, %v, want exactly one of paginator and error", p, err)
			}
		})
	}
}

func TestOffsetPrepare(t *testing.T) {
	p := newOffset(t, strategy.OffsetOptions{Policy: turn.Policy{DefaultSize: 10, MaxSize: 50}})
	lastNumber := int64(math.MaxInt64/50 + 1)
	tests := []struct {
		name    string
		request strategy.PageRequest
		want    strategy.OffsetPlan
		wantErr error
	}{
		{
			name:    "zero request",
			request: strategy.PageRequest{},
			want:    strategy.OffsetPlan{Window: turn.Window{Size: 10}, Number: 1, Offset: 0},
		},
		{
			name:    "first page",
			request: strategy.PageRequest{Number: 1, Size: 5},
			want:    strategy.OffsetPlan{Window: turn.Window{Size: 5}, Number: 1, Offset: 0},
		},
		{
			name:    "third page",
			request: strategy.PageRequest{Number: 3, Size: 25},
			want:    strategy.OffsetPlan{Window: turn.Window{Size: 25}, Number: 3, Offset: 50},
		},
		{
			name:    "default size on later page",
			request: strategy.PageRequest{Number: 4},
			want:    strategy.OffsetPlan{Window: turn.Window{Size: 10}, Number: 4, Offset: 30},
		},
		{
			name:    "deepest page in int64 range",
			request: strategy.PageRequest{Number: lastNumber, Size: 50},
			want:    strategy.OffsetPlan{Window: turn.Window{Size: 50}, Number: lastNumber, Offset: (lastNumber - 1) * 50},
		},
		{
			name:    "max page number at size one",
			request: strategy.PageRequest{Number: math.MaxInt64, Size: 1},
			want:    strategy.OffsetPlan{Window: turn.Window{Size: 1}, Number: math.MaxInt64, Offset: math.MaxInt64 - 1},
		},
		{name: "offset overflows int64", request: strategy.PageRequest{Number: lastNumber + 1, Size: 50}, wantErr: turn.ErrOffsetTooLarge},
		{name: "max page number overflows", request: strategy.PageRequest{Number: math.MaxInt64, Size: 2}, wantErr: turn.ErrOffsetTooLarge},
		{name: "negative page", request: strategy.PageRequest{Number: -1}, wantErr: turn.ErrInvalidPage},
		{name: "min page", request: strategy.PageRequest{Number: math.MinInt64}, wantErr: turn.ErrInvalidPage},
		{name: "negative size", request: strategy.PageRequest{Size: -1}, wantErr: turn.ErrInvalidSize},
		{name: "size above max", request: strategy.PageRequest{Size: 51}, wantErr: turn.ErrInvalidSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.Prepare(tt.request)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Prepare(%+v) error = %v, want %v", tt.request, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Prepare(%+v) = %+v, want %+v", tt.request, got, tt.want)
			}
		})
	}
}

func TestOffsetPrepareMaxOffset(t *testing.T) {
	tests := []struct {
		name      string
		maxOffset int64
		request   strategy.PageRequest
		wantErr   error
	}{
		{name: "at max offset", maxOffset: 100, request: strategy.PageRequest{Number: 11, Size: 10}},
		{name: "past max offset", maxOffset: 100, request: strategy.PageRequest{Number: 12, Size: 10}, wantErr: turn.ErrOffsetTooLarge},
		{name: "below max offset, uneven size", maxOffset: 100, request: strategy.PageRequest{Number: 4, Size: 30}},
		{name: "past max offset, uneven size", maxOffset: 100, request: strategy.PageRequest{Number: 5, Size: 30}, wantErr: turn.ErrOffsetTooLarge},
		{name: "zero max offset, first page", maxOffset: 0, request: strategy.PageRequest{Number: 1}},
		{name: "zero max offset, second page", maxOffset: 0, request: strategy.PageRequest{Number: 2, Size: 1}, wantErr: turn.ErrOffsetTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newOffset(t, strategy.OffsetOptions{MaxOffset: int64Ptr(tt.maxOffset)})
			if _, err := p.Prepare(tt.request); !errors.Is(err, tt.wantErr) {
				t.Errorf("Prepare(%+v) error = %v, want %v", tt.request, err, tt.wantErr)
			}
		})
	}
}

func TestOffsetFinish(t *testing.T) {
	p := newOffset(t, strategy.OffsetOptions{})
	plan := prepareOffset(t, p, strategy.PageRequest{Number: 2, Size: 3})
	tests := []struct {
		name     string
		items    []int
		want     []int
		wantMore bool
		wantErr  error
	}{
		{name: "nil", items: nil, want: []int{}},
		{name: "partial", items: []int{4, 5}, want: []int{4, 5}},
		{name: "exactly size", items: []int{4, 5, 6}, want: []int{4, 5, 6}},
		{name: "with lookahead", items: []int{4, 5, 6, 7}, want: []int{4, 5, 6}, wantMore: true},
		{name: "above fetch limit", items: []int{4, 5, 6, 7, 8}, wantErr: turn.ErrInvalidBatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := p.Finish(plan, tt.items)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Finish(%v) error = %v, want %v", tt.items, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if page.Items == nil {
				t.Error("Finish returned nil Items")
			}
			if !slices.Equal(page.Items, tt.want) || page.HasMore != tt.wantMore {
				t.Errorf("Finish(%v) = %v, HasMore %t, want %v, HasMore %t", tt.items, page.Items, page.HasMore, tt.want, tt.wantMore)
			}
			if want := (strategy.OffsetInfo{Number: 2, Size: 3}); page.Info != want {
				t.Errorf("Finish Info = %+v, want %+v", page.Info, want)
			}
		})
	}
}

func TestOffsetFinishKeepsBatch(t *testing.T) {
	p := newOffset(t, strategy.OffsetOptions{})
	plan := prepareOffset(t, p, strategy.PageRequest{Size: 3})
	items := []int{1, 2, 3, 4}
	page, err := p.Finish(plan, items)
	if err != nil {
		t.Fatal(err)
	}
	_ = append(page.Items, 99)
	if !slices.Equal(items, []int{1, 2, 3, 4}) {
		t.Errorf("batch changed to %v", items)
	}
}

func TestOffsetFinishInvalidPlan(t *testing.T) {
	p := newOffset(t, strategy.OffsetOptions{
		Policy:    turn.Policy{DefaultSize: 10, MaxSize: 50},
		MaxOffset: int64Ptr(100),
	})
	other := newOffset(t, strategy.OffsetOptions{Policy: turn.Policy{MaxSize: 200}})
	tests := []struct {
		name string
		plan strategy.OffsetPlan
	}{
		{name: "zero plan", plan: strategy.OffsetPlan{}},
		{name: "zero number", plan: strategy.OffsetPlan{Window: turn.Window{Size: 10}, Number: 0, Offset: 0}},
		{name: "negative number", plan: strategy.OffsetPlan{Window: turn.Window{Size: 10}, Number: -1, Offset: -20}},
		{name: "altered offset", plan: strategy.OffsetPlan{Window: turn.Window{Size: 10}, Number: 2, Offset: 0}},
		{name: "altered size", plan: strategy.OffsetPlan{Window: turn.Window{Size: 20}, Number: 2, Offset: 10}},
		{name: "size above max", plan: strategy.OffsetPlan{Window: turn.Window{Size: 51}, Number: 1, Offset: 0}},
		{name: "past max offset", plan: strategy.OffsetPlan{Window: turn.Window{Size: 10}, Number: 12, Offset: 110}},
		{name: "from another paginator", plan: prepareOffset(t, other, strategy.PageRequest{Size: 150})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := p.Finish(tt.plan, nil); !errors.Is(err, turn.ErrInvalidPlan) {
				t.Errorf("Finish(%+v) error = %v, want %v", tt.plan, err, turn.ErrInvalidPlan)
			}
		})
	}
}

// TestOffsetWalk pages through a listing the way a consumer would, applying
// each plan's Offset and FetchLimit to an in-memory query.
func TestOffsetWalk(t *testing.T) {
	data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p := newOffset(t, strategy.OffsetOptions{})

	var pages [][]int
	for number := int64(1); ; number++ {
		plan := prepareOffset(t, p, strategy.PageRequest{Number: number, Size: 3})
		start := min(int(plan.Offset), len(data))
		end := min(start+plan.FetchLimit(), len(data))
		page, err := p.Finish(plan, data[start:end])
		if err != nil {
			t.Fatal(err)
		}
		pages = append(pages, page.Items)
		if !page.HasMore {
			break
		}
	}

	want := [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10}}
	if !slices.EqualFunc(pages, want, slices.Equal) {
		t.Errorf("pages = %v, want %v", pages, want)
	}
}
