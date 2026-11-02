package obddtc

import (
	"reflect"
	"testing"
)

func TestFormat(t *testing.T) {
	cases := []struct {
		a, b byte
		want string
		ok   bool
	}{
		{0x01, 0x33, "P0133", true},
		{0xC1, 0x23, "U0123", true},
		{0x41, 0x00, "C0100", true},
		{0x82, 0x34, "B0234", true},
		{0x00, 0x00, "", false},
		{0xAA, 0xAA, "", false},
	}
	for _, c := range cases {
		got, ok := Format(c.a, c.b)
		if ok != c.ok || got != c.want {
			t.Fatalf("Format(%02X,%02X) = %q,%v; want %q,%v", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
}

func TestParseStatus(t *testing.T) {
	// Real response from the car: 41 01 00 07 E1 00.
	got, ok := ParseStatus([]byte{0x00, 0x07, 0xE1, 0x00})
	if !ok {
		t.Fatal("ParseStatus failed")
	}
	if got.MIL || got.Count != 0 {
		t.Fatalf("MIL=%v count=%d, want false/0", got.MIL, got.Count)
	}
	if got.Readiness != [3]byte{0x07, 0xE1, 0x00} {
		t.Fatalf("readiness = % X", got.Readiness)
	}

	got, ok = ParseStatus([]byte{0x83, 0x07, 0xE1, 0x00})
	if !ok || !got.MIL || got.Count != 3 {
		t.Fatalf("MIL=%v count=%d, want true/3", got.MIL, got.Count)
	}

	if _, ok := ParseStatus([]byte{0x00, 0x07}); ok {
		t.Fatal("short payload accepted")
	}
}

func TestCodes(t *testing.T) {
	// Real response from the car: 43 00 AA AA AA AA AA (no stored DTCs).
	if got := Codes([]byte{0x00, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA}); len(got) != 0 {
		t.Fatalf("empty payload decoded as %v", got)
	}

	got := Codes([]byte{0x02, 0x01, 0x33, 0xC1, 0x23})
	want := []string{"P0133", "U0123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Codes = %v, want %v", got, want)
	}

	// Count larger than the payload is clamped.
	got = Codes([]byte{0x05, 0x01, 0x33})
	if !reflect.DeepEqual(got, []string{"P0133"}) {
		t.Fatalf("Codes = %v", got)
	}

	if got := Codes(nil); got != nil {
		t.Fatalf("Codes(nil) = %v", got)
	}
}
