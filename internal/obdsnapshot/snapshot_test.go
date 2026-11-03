package obdsnapshot

import (
	"testing"
	"unsafe"

	"m5stick-obd2-cnx/internal/obdpid"
)

func TestSlotSize(t *testing.T) {
	if got := unsafe.Sizeof(Slot{}); got != 16 {
		t.Fatalf("Slot size = %d, want 16 (memory budget)", got)
	}
	if got := unsafe.Sizeof([Slots]Slot{}); got != 1024 {
		t.Fatalf("snapshot buffer = %d, want 1024", got)
	}
	if got := unsafe.Sizeof(Store{}); got > 3*1024+128 {
		t.Fatalf("Store = %d bytes, over budget", got)
	}
}

func TestPutGet(t *testing.T) {
	var s Snapshot
	s.Put(0x3043, 1726, obdpid.QualityOK, 100)

	slot, ok := s.Get(0x3043)
	if !ok || slot.Value != 1726 || slot.AtMs != 100 || slot.Quality != obdpid.QualityOK {
		t.Fatalf("Get = %+v,%v", slot, ok)
	}
	if _, ok := s.Get(0); ok {
		t.Fatal("zero key must not match")
	}
}

func TestPublishCarriesForward(t *testing.T) {
	st := New()
	buf := st.Begin()
	buf.Put(0x3043, 1000, obdpid.QualityOK, 10)
	st.Publish(10)

	slot, ok := st.Current().Get(0x3043)
	if !ok || slot.Value != 1000 {
		t.Fatalf("first publish = %+v,%v", slot, ok)
	}

	buf = st.Begin()
	buf.Put(0x3044, 50, obdpid.QualityStale, 20)
	st.Publish(20)

	cur := st.Current()
	if slot, ok := cur.Get(0x3043); !ok || slot.Value != 1000 {
		t.Fatalf("carry-forward lost 0C: %+v,%v", slot, ok)
	}
	if slot, ok := cur.Get(0x3044); !ok || slot.Value != 50 || slot.Quality != obdpid.QualityStale {
		t.Fatalf("new slot = %+v,%v", slot, ok)
	}
	if cur.AtMs != 20 {
		t.Fatalf("AtMs = %d", cur.AtMs)
	}
}

func TestQualityPreserved(t *testing.T) {
	qualities := []obdpid.Quality{
		obdpid.QualityOK,
		obdpid.QualityNoData,
		obdpid.QualityTimeout,
		obdpid.QualityStale,
		obdpid.QualityInvalid,
		obdpid.QualityUnsupported,
	}
	for _, q := range qualities {
		var s Snapshot
		s.Put(0x3043, 1, q, 1)
		slot, ok := s.Get(0x3043)
		if !ok || slot.Quality != q {
			t.Fatalf("quality %d lost: %+v,%v", q, slot, ok)
		}
	}
}

func TestClear(t *testing.T) {
	st := New()
	buf := st.Begin()
	buf.Put(0x3043, 1000, obdpid.QualityOK, 10)
	st.Publish(10)
	st.Clear()
	if _, ok := st.Current().Get(0x3043); ok {
		t.Fatal("Clear kept a slot")
	}
}

func TestFullSnapshotDropsExtra(t *testing.T) {
	var s Snapshot
	for i := 0; i < Slots; i++ {
		s.Put(uint16(0x1000+i), float64(i), obdpid.QualityOK, 1)
	}
	s.Put(0xFFFF, 99, obdpid.QualityOK, 1)
	if _, ok := s.Get(0xFFFF); ok {
		t.Fatal("snapshot exceeded Slots")
	}
}

func BenchmarkPut(b *testing.B) {
	var s Snapshot
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.Put(0x3043, float64(i), obdpid.QualityOK, uint32(i))
	}
}

func BenchmarkGet(b *testing.B) {
	var s Snapshot
	for i := 0; i < 32; i++ {
		s.Put(uint16(0x3000+i), float64(i), obdpid.QualityOK, uint32(i))
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := s.Get(0x3000); !ok {
			b.Fatal("missing")
		}
	}
}

func BenchmarkPublish(b *testing.B) {
	st := New()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := st.Begin()
		buf.Put(0x3043, float64(i), obdpid.QualityOK, uint32(i))
		st.Publish(uint32(i))
	}
}
