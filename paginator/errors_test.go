package paginator_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/exalynt/turn/paginator"
)

func TestIsSelectorError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{paginator.ErrInvalidSize, true},
		{paginator.ErrInvalidPage, true},
		{paginator.ErrOffsetTooLarge, true},
		{paginator.ErrInvalidDirection, true},
		{paginator.ErrInvalidCursor, true},
		{fmt.Errorf("%w: -1 is negative", paginator.ErrInvalidSize), true},
		{fmt.Errorf("%w: %w", paginator.ErrInvalidCursor, errors.New("bad digest")), true},
		{paginator.ErrInvalidOptions, false},
		{paginator.ErrInvalidPlan, false},
		{paginator.ErrInvalidBatch, false},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := paginator.IsSelectorError(tt.err); got != tt.want {
			t.Errorf("IsSelectorError(%v) = %t, want %t", tt.err, got, tt.want)
		}
	}
}
