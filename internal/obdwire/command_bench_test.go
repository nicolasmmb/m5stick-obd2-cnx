package obdwire

import "testing"

func BenchmarkParseCommand(b *testing.B) {
	line := "R,P,140,0C,0CB4AAAAAA,140,OK"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ParseCommand(line); err != nil {
			b.Fatal(err)
		}
	}
}
