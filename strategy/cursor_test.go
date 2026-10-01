package strategy_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/exalynt/turn"
	"github.com/exalynt/turn/codec"
	"github.com/exalynt/turn/codec/plain"
	"github.com/exalynt/turn/strategy"
)

const scope = "items:id_asc"

// item is a listed record whose canonical order is ascending ID.
type item struct {
	ID int
}

func itemsOf(ids ...int) []item {
	items := make([]item, len(ids))
	for i, id := range ids {
		items[i] = item{ID: id}
	}
	return items
}

func idsOf(items []item) []int {
	ids := make([]int, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

func itemPosition(it item) int {
	return it.ID
}

func intPtr(v int) *int {
	return &v
}

func newCursor(t *testing.T, policy turn.Policy) *strategy.CursorPaginator[item, int] {
	t.Helper()
	p, err := strategy.NewCursor(strategy.CursorOptions[item, int]{
		Policy:   policy,
		Codec:    plain.Codec[int]{},
		Position: itemPosition,
	})
	if err != nil {
		t.Fatalf("NewCursor error = %v", err)
	}
	return p
}

func prepareCursor(t *testing.T, p *strategy.CursorPaginator[item, int], request strategy.CursorRequest, scope string) strategy.CursorPlan[int] {
	t.Helper()
	plan, err := p.Prepare(request, scope)
	if err != nil {
		t.Fatalf("Prepare(%+v) error = %v", request, err)
	}
	return plan
}

func encode(t *testing.T, id int, scope string) codec.Cursor {
	t.Helper()
	cursor, err := plain.Codec[int]{}.Encode(id, scope)
	if err != nil {
		t.Fatal(err)
	}
	return cursor
}

func decode(t *testing.T, cursor codec.Cursor, scope string) int {
	t.Helper()
	id, err := plain.Codec[int]{}.Decode(cursor, scope)
	if err != nil {
		t.Fatalf("Decode(%q) error = %v", cursor, err)
	}
	return id
}

// failingCodec decodes nothing and fails every Encode with errEncode.
type failingCodec struct{}

var errEncode = errors.New("encode failed")

func (failingCodec) Encode(int, string) (codec.Cursor, error) { return "", errEncode }

func (failingCodec) Decode(codec.Cursor, string) (int, error) { return 0, errors.New("decode failed") }

func TestNewCursor(t *testing.T) {
	tests := []struct {
		name    string
		options strategy.CursorOptions[item, int]
		wantErr error
	}{
		{
			name:    "valid",
			options: strategy.CursorOptions[item, int]{Codec: plain.Codec[int]{}, Position: itemPosition},
		},
		{
			name:    "invalid policy",
			options: strategy.CursorOptions[item, int]{Policy: turn.Policy{MaxSize: -1}, Codec: plain.Codec[int]{}, Position: itemPosition},
			wantErr: turn.ErrInvalidOptions,
		},
		{
			name:    "missing codec",
			options: strategy.CursorOptions[item, int]{Position: itemPosition},
			wantErr: turn.ErrInvalidOptions,
		},
		{
			name:    "missing position",
			options: strategy.CursorOptions[item, int]{Codec: plain.Codec[int]{}},
			wantErr: turn.ErrInvalidOptions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := strategy.NewCursor(tt.options)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewCursor error = %v, want %v", err, tt.wantErr)
			}
			if (p == nil) != (err != nil) {
				t.Errorf("NewCursor = %v, %v, want exactly one of paginator and error", p, err)
			}
		})
	}
}

func TestCursorPrepare(t *testing.T) {
	p := newCursor(t, turn.Policy{DefaultSize: 10, MaxSize: 50})
	tests := []struct {
		name          string
		request       strategy.CursorRequest
		wantSize      int
		wantDirection strategy.Direction
		wantBoundary  *int
	}{
		{name: "zero request", request: strategy.CursorRequest{}, wantSize: 10, wantDirection: strategy.Forward},
		{
			name:          "backward from end",
			request:       strategy.CursorRequest{Direction: strategy.Backward, Size: 5},
			wantSize:      5,
			wantDirection: strategy.Backward,
		},
		{
			name:          "forward from cursor",
			request:       strategy.CursorRequest{Cursor: encode(t, 7, scope), Size: 50},
			wantSize:      50,
			wantDirection: strategy.Forward,
			wantBoundary:  intPtr(7),
		},
		{
			name:          "backward from cursor",
			request:       strategy.CursorRequest{Direction: strategy.Backward, Cursor: encode(t, 0, scope)},
			wantSize:      10,
			wantDirection: strategy.Backward,
			wantBoundary:  intPtr(0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := prepareCursor(t, p, tt.request, scope)
			if plan.Size != tt.wantSize || plan.Direction != tt.wantDirection {
				t.Errorf("Prepare(%+v) = size %d, direction %d, want size %d, direction %d",
					tt.request, plan.Size, plan.Direction, tt.wantSize, tt.wantDirection)
			}
			switch {
			case tt.wantBoundary == nil && plan.Boundary != nil:
				t.Errorf("Prepare(%+v) Boundary = %d, want nil", tt.request, *plan.Boundary)
			case tt.wantBoundary != nil && plan.Boundary == nil:
				t.Errorf("Prepare(%+v) Boundary = nil, want %d", tt.request, *tt.wantBoundary)
			case tt.wantBoundary != nil && *plan.Boundary != *tt.wantBoundary:
				t.Errorf("Prepare(%+v) Boundary = %d, want %d", tt.request, *plan.Boundary, *tt.wantBoundary)
			}
		})
	}
}

func TestCursorPrepareErrors(t *testing.T) {
	p := newCursor(t, turn.Policy{DefaultSize: 10, MaxSize: 50})
	tests := []struct {
		name       string
		request    strategy.CursorRequest
		wantErr    error
		wantReason error
	}{
		{name: "invalid direction", request: strategy.CursorRequest{Direction: strategy.Backward + 1}, wantErr: turn.ErrInvalidDirection},
		{name: "negative size", request: strategy.CursorRequest{Size: -1}, wantErr: turn.ErrInvalidSize},
		{name: "size above max", request: strategy.CursorRequest{Size: 51}, wantErr: turn.ErrInvalidSize},
		{
			name:       "malformed cursor",
			request:    strategy.CursorRequest{Cursor: "not a cursor"},
			wantErr:    turn.ErrInvalidCursor,
			wantReason: plain.ErrMalformed,
		},
		{
			name:       "cursor from another scope",
			request:    strategy.CursorRequest{Cursor: encode(t, 7, "other")},
			wantErr:    turn.ErrInvalidCursor,
			wantReason: plain.ErrScopeMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Prepare(tt.request, scope)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Prepare(%+v) error = %v, want %v", tt.request, err, tt.wantErr)
			}
			if tt.wantReason != nil && !errors.Is(err, tt.wantReason) {
				t.Errorf("Prepare(%+v) error = %v, want it to wrap %v", tt.request, err, tt.wantReason)
			}
		})
	}
}

func TestCursorFinish(t *testing.T) {
	p := newCursor(t, turn.Policy{})
	tests := []struct {
		name      string
		direction strategy.Direction
		fetched   []int
		want      []int
		wantMore  bool
	}{
		{name: "forward, empty", direction: strategy.Forward, fetched: nil, want: []int{}},
		{name: "forward, one item", direction: strategy.Forward, fetched: []int{4}, want: []int{4}},
		{name: "forward, exactly size", direction: strategy.Forward, fetched: []int{4, 5, 6}, want: []int{4, 5, 6}},
		{name: "forward, with lookahead", direction: strategy.Forward, fetched: []int{4, 5, 6, 7}, want: []int{4, 5, 6}, wantMore: true},
		{name: "backward, empty", direction: strategy.Backward, fetched: []int{}, want: []int{}},
		{name: "backward, one item", direction: strategy.Backward, fetched: []int{4}, want: []int{4}},
		{name: "backward, exactly size", direction: strategy.Backward, fetched: []int{6, 5, 4}, want: []int{4, 5, 6}},
		{name: "backward, with lookahead", direction: strategy.Backward, fetched: []int{6, 5, 4, 3}, want: []int{4, 5, 6}, wantMore: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := prepareCursor(t, p, strategy.CursorRequest{Direction: tt.direction, Size: 3}, scope)
			var fetched []item
			if tt.fetched != nil {
				fetched = itemsOf(tt.fetched...)
			}
			page, err := p.Finish(plan, fetched)
			if err != nil {
				t.Fatalf("Finish(%v) error = %v", tt.fetched, err)
			}
			if page.Items == nil {
				t.Error("Finish returned nil Items")
			}
			if got := idsOf(page.Items); !slices.Equal(got, tt.want) || page.HasMore != tt.wantMore {
				t.Errorf("Finish(%v) = %v, HasMore %t, want %v, HasMore %t", tt.fetched, got, page.HasMore, tt.want, tt.wantMore)
			}
			if page.Info.Direction != tt.direction {
				t.Errorf("Finish Info.Direction = %d, want %d", page.Info.Direction, tt.direction)
			}
			if len(tt.want) == 0 {
				if page.Info.StartCursor != "" || page.Info.EndCursor != "" {
					t.Errorf("Finish on empty page cursors = %q, %q, want empty", page.Info.StartCursor, page.Info.EndCursor)
				}
				return
			}
			if got := decode(t, page.Info.StartCursor, scope); got != tt.want[0] {
				t.Errorf("StartCursor holds %d, want %d", got, tt.want[0])
			}
			if got := decode(t, page.Info.EndCursor, scope); got != tt.want[len(tt.want)-1] {
				t.Errorf("EndCursor holds %d, want %d", got, tt.want[len(tt.want)-1])
			}
		})
	}
}

func TestCursorFinishKeepsBatch(t *testing.T) {
	p := newCursor(t, turn.Policy{})
	for _, direction := range []strategy.Direction{strategy.Forward, strategy.Backward} {
		plan := prepareCursor(t, p, strategy.CursorRequest{Direction: direction, Size: 3}, scope)
		fetched := itemsOf(9, 8, 7, 6)
		page, err := p.Finish(plan, fetched)
		if err != nil {
			t.Fatal(err)
		}
		_ = append(page.Items, item{ID: 99})
		if got := idsOf(fetched); !slices.Equal(got, []int{9, 8, 7, 6}) {
			t.Errorf("direction %d: batch changed to %v", direction, got)
		}
	}
}

func TestCursorFinishScopesCursors(t *testing.T) {
	p := newCursor(t, turn.Policy{})
	plan := prepareCursor(t, p, strategy.CursorRequest{}, scope)
	page, err := p.Finish(plan, itemsOf(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Prepare(strategy.CursorRequest{Cursor: page.Info.EndCursor}, scope); err != nil {
		t.Errorf("Prepare with EndCursor in its scope error = %v", err)
	}
	if _, err := p.Prepare(strategy.CursorRequest{Cursor: page.Info.EndCursor}, "other"); !errors.Is(err, turn.ErrInvalidCursor) {
		t.Errorf("Prepare with EndCursor in another scope error = %v, want %v", err, turn.ErrInvalidCursor)
	}
}

func TestCursorFinishEncodeError(t *testing.T) {
	p, err := strategy.NewCursor(strategy.CursorOptions[item, int]{Codec: failingCodec{}, Position: itemPosition})
	if err != nil {
		t.Fatal(err)
	}
	plan := prepareCursor(t, p, strategy.CursorRequest{}, scope)

	if _, err := p.Finish(plan, itemsOf(1)); !errors.Is(err, errEncode) {
		t.Errorf("Finish error = %v, want %v", err, errEncode)
	}
	// An empty page has no cursors to encode.
	if _, err := p.Finish(plan, nil); err != nil {
		t.Errorf("Finish on empty page error = %v", err)
	}
}

func TestCursorFinishInvalidBatch(t *testing.T) {
	p := newCursor(t, turn.Policy{})
	plan := prepareCursor(t, p, strategy.CursorRequest{Size: 2}, scope)
	if _, err := p.Finish(plan, itemsOf(1, 2, 3, 4)); !errors.Is(err, turn.ErrInvalidBatch) {
		t.Errorf("Finish error = %v, want %v", err, turn.ErrInvalidBatch)
	}
}

func TestCursorFinishInvalidPlan(t *testing.T) {
	p := newCursor(t, turn.Policy{DefaultSize: 10, MaxSize: 50})
	valid := prepareCursor(t, p, strategy.CursorRequest{}, scope)

	sizeAboveMax := valid
	sizeAboveMax.Size = 51
	invalidDirection := valid
	invalidDirection.Direction = strategy.Backward + 1

	tests := []struct {
		name string
		plan strategy.CursorPlan[int]
	}{
		{name: "zero plan", plan: strategy.CursorPlan[int]{}},
		{name: "size above max", plan: sizeAboveMax},
		{name: "invalid direction", plan: invalidDirection},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := p.Finish(tt.plan, nil); !errors.Is(err, turn.ErrInvalidPlan) {
				t.Errorf("Finish error = %v, want %v", err, turn.ErrInvalidPlan)
			}
		})
	}
}

// fetchCursor runs plan against data, which is in canonical order, the way a
// consumer's keyset query would.
func fetchCursor(data []item, plan strategy.CursorPlan[int]) []item {
	var rows []item
	if plan.Direction == strategy.Forward {
		for _, it := range data {
			if plan.Boundary == nil || it.ID > *plan.Boundary {
				rows = append(rows, it)
			}
		}
	} else {
		for _, it := range slices.Backward(data) {
			if plan.Boundary == nil || it.ID < *plan.Boundary {
				rows = append(rows, it)
			}
		}
	}
	return rows[:min(len(rows), plan.FetchLimit())]
}

// TestCursorWalk pages through a whole listing forward and then backward,
// following each page's cursors the way a client would.
func TestCursorWalk(t *testing.T) {
	data := itemsOf(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	p := newCursor(t, turn.Policy{})

	walk := func(direction strategy.Direction) [][]int {
		var pages [][]int
		request := strategy.CursorRequest{Direction: direction, Size: 3}
		for {
			plan := prepareCursor(t, p, request, scope)
			page, err := p.Finish(plan, fetchCursor(data, plan))
			if err != nil {
				t.Fatal(err)
			}
			pages = append(pages, idsOf(page.Items))
			if !page.HasMore {
				return pages
			}
			if direction == strategy.Forward {
				request.Cursor = page.Info.EndCursor
			} else {
				request.Cursor = page.Info.StartCursor
			}
		}
	}

	forward := walk(strategy.Forward)
	if want := [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10}}; !slices.EqualFunc(forward, want, slices.Equal) {
		t.Errorf("forward pages = %v, want %v", forward, want)
	}
	backward := walk(strategy.Backward)
	if want := [][]int{{8, 9, 10}, {5, 6, 7}, {2, 3, 4}, {1}}; !slices.EqualFunc(backward, want, slices.Equal) {
		t.Errorf("backward pages = %v, want %v", backward, want)
	}
}
