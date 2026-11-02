package obdpid

import (
	"testing"
	"time"
)

func TestReadingAge(t *testing.T) {
	now := time.Unix(100, 0)
	r := Reading{At: now.Add(-1500 * time.Millisecond)}
	if got := r.AgeMs(now); got != 1500 {
		t.Fatalf("AgeMs = %d, want 1500", got)
	}
}
