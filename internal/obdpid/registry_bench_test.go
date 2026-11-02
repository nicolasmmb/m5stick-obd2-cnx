package obdpid

import "testing"

func BenchmarkDecodeBytes(b *testing.B) {
	reg := DefaultRegistry()
	raw := []byte{0x1A, 0xF8}
	pid := [2]byte{'0', 'C'}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := reg.DecodeBytes(pid, raw); err != nil {
			b.Fatal(err)
		}
	}
}
