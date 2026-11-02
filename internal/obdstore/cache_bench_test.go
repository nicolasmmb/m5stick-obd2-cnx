package obdstore

import (
	"testing"
	"time"

	"m5stick-obd2-cnx/internal/obdpid"
)

var benchPIDs = []string{
	"0C", "0D", "11", "04", "0B", "10", "44", "06", "07", "05",
	"0F", "2F", "3C", "42", "52", "46", "45", "49", "4A", "4C", "33",
}

func BenchmarkCachePutGet(b *testing.B) {
	c := NewCache()
	now := time.Now()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pid := benchPIDs[i%len(benchPIDs)]
		c.Put(obdpid.Reading{PID: pid, Value: float64(i), Quality: obdpid.QualityOK, At: now})
		if _, ok := c.Get(pid); !ok {
			b.Fatal("missing")
		}
	}
}
