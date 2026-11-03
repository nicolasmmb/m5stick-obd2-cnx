package main

import "testing"

func TestDTCBytes(t *testing.T) {
	cases := []struct {
		code string
		a, b byte
		ok   bool
	}{
		{"P0133", 0x01, 0x33, true},
		{"U0123", 0xC1, 0x23, true},
		{"C0100", 0x41, 0x00, true},
		{"B0234", 0x82, 0x34, true},
		{"X0133", 0, 0, false},
		{"P013", 0, 0, false},
	}
	for _, c := range cases {
		a, b, ok := dtcBytes(c.code)
		if ok != c.ok || a != c.a || b != c.b {
			t.Fatalf("dtcBytes(%q) = %02X,%02X,%v", c.code, a, b, ok)
		}
	}
}

func TestDTCCResponse(t *testing.T) {
	if got := dtcResponse("43", nil); got != "4300AA||>" {
		t.Fatalf("empty response = %q", got)
	}
	got := dtcResponse("43", []string{"P0133", "U0123"})
	if got != "43020133C123AA||>" {
		t.Fatalf("response = %q", got)
	}
}

func TestSimSnapshotAnswer(t *testing.T) {
	snap := &simSnapshot{
		vin: "9BGEA48A0LG185881",
		raw: map[string]string{"0C": "1AF8", "0D": "1E"},
	}
	opts := simOptions{mil: true, dtcs: []string{"P0133"}, readiness: "07E100"}

	cases := map[string]string{
		"ATZ":   "ELM327 v2.1",
		"ATSP0": "OK",
		"0902":  "4902013942474541343841304C47313835383831||>",
		"010C":  "410C1AF8AA||>",
		"010D":  "410D1EAA||>",
		"0101":  "41018107E100AA||>",
		"03":    "43010133AA||>",
		"07":    "4700AA||>",
		"0A":    "4A00AA||>",
		"0105":  "NO DATA||>",
	}
	for cmd, want := range cases {
		if got := snap.answer(cmd, opts); got != want {
			t.Fatalf("%s = %q, want %q", cmd, got, want)
		}
	}
}

func TestLoadSnapshot(t *testing.T) {
	snap, err := loadSnapshot("../../testdata/obd-logger-sample-compact.txt")
	if err != nil {
		t.Fatal(err)
	}
	if snap.vin != "9BGEA48A0LG185881" {
		t.Fatalf("vin = %q", snap.vin)
	}
	if snap.raw["0C"] == "" || snap.raw["0D"] == "" {
		t.Fatalf("raw map = %v", snap.raw)
	}
}

func TestParseDTCList(t *testing.T) {
	got := parseDTCList(" p0133, U0123 ,, ")
	if len(got) != 2 || got[0] != "P0133" || got[1] != "U0123" {
		t.Fatalf("list = %v", got)
	}
}
