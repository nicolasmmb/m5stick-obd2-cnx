package obdstore

import (
	"testing"
	"time"

	"m5stick-obd2-cnx/internal/obdpid"
)

func TestCachePutGet(t *testing.T) {
	c := NewCache()
	r := obdpid.Reading{PID: "0C", Value: 1000, Unit: "rpm", Quality: obdpid.QualityOK, At: time.Now()}
	c.Put(r)
	got, ok := c.Get("0C")
	if !ok || got.Value != 1000 {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestSnapshotStale(t *testing.T) {
	now := time.Now()
	s := NewSnapshot(map[string]obdpid.Reading{
		"0C": {PID: "0C", Value: 1000, Quality: obdpid.QualityOK, At: now.Add(-4 * time.Second)},
	})
	r, ok := s.Get("0C")
	if !ok || r.Quality != obdpid.QualityStale {
		t.Fatalf("expected stale, got %+v", r)
	}
}
