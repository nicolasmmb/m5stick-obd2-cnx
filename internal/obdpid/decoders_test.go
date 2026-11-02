package obdpid

import "testing"

func TestDecoders(t *testing.T) {
	table := Table()
	tests := []struct {
		pid  string
		raw  []byte
		want float64
		unit string
	}{
		{"0C", []byte{0x1A, 0xF8}, 1726, "rpm"},
		{"0D", []byte{0x0A}, 10, "km/h"},
		{"05", []byte{0x73}, 75, "C"},
		{"11", []byte{0x50}, 31.372549019607842, "%"},
		{"42", []byte{0x30, 0xFB}, 12.539, "V"},
		{"3C", []byte{0x15, 0x54}, 506, "C"},
		{"44", []byte{0x6D, 0x00}, 0.8515625, "lambda"},
		{"06", []byte{0x80}, 0, "%"},
	}
	for _, tt := range tests {
		d, ok := table[tt.pid]
		if !ok {
			t.Fatalf("missing pid %s", tt.pid)
		}
		got, err := d.Decode(tt.raw)
		if err != nil {
			t.Fatalf("%s: %v", tt.pid, err)
		}
		diff := got - tt.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.000001 || d.Unit() != tt.unit {
			t.Fatalf("%s = %v %q, want %v %q", tt.pid, got, d.Unit(), tt.want, tt.unit)
		}
	}
}

func TestInvalidDecoder(t *testing.T) {
	if _, err := (U16{ID: "0C"}).Decode([]byte{1}); err == nil {
		t.Fatal("expected invalid data error")
	}
}
