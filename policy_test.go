package turn_test

import (
	"testing"

	"github.com/exalynt/turn"
)

func TestWindowFetchLimit(t *testing.T) {
	for _, size := range []int{1, 25, 100} {
		if got := (turn.Window{Size: size}).FetchLimit(); got != size+1 {
			t.Errorf("Window{Size: %d}.FetchLimit() = %d, want %d", size, got, size+1)
		}
	}
}
