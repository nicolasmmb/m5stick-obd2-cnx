package obdline

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
		pid  string
		raw  []byte
		ms   int64
	}{
		{"A\r", KindAck, "", nil, 0},
		{"C,REPLAY", KindReplay, "", nil, 0},
		{"C,LIVE", KindLive, "", nil, 0},
		{"C,CLEAR", KindClear, "", nil, 0},
		{"C,STOP\r", KindStop, "", nil, 0},
		{"C,SIM", KindSim, "", nil, 0},
		{"C,MEM", KindMem, "", nil, 0},
		{"R,P,16199,0c,0cfe,161,OK", KindRecord, "0C", []byte{0x0c, 0xfe}, 16199},
		{"R,P,1,0D,00,140,OK", KindRecord, "0D", []byte{0x00}, 1},
		{"R,P,2,44,8000AAAAAA,180,OK", KindRecord, "44", []byte{0x80, 0x00, 0xAA, 0xAA, 0xAA}, 2},
		{"R,I,0,0.1,,440", KindIgnore, "", nil, 0},
		{"R,E,10,NO_DATA,0C", KindIgnore, "", nil, 0},
		{"R,X,10,end", KindIgnore, "", nil, 0},
	}
	for _, c := range cases {
		line, ok := Parse([]byte(c.in))
		if !ok {
			t.Fatalf("%q: parse failed", c.in)
		}
		if line.Kind != c.kind {
			t.Fatalf("%q: kind = %d, want %d", c.in, line.Kind, c.kind)
		}
		if c.kind != KindRecord {
			continue
		}
		gotPID := string(line.Rec.PID[:line.Rec.PIDLen])
		if gotPID != c.pid {
			t.Fatalf("%q: pid = %q, want %q", c.in, gotPID, c.pid)
		}
		if line.Rec.Ms != c.ms {
			t.Fatalf("%q: ms = %d, want %d", c.in, line.Rec.Ms, c.ms)
		}
		if line.Rec.RawLen != uint8(len(c.raw)) {
			t.Fatalf("%q: rawlen = %d, want %d", c.in, line.Rec.RawLen, len(c.raw))
		}
		for i := range c.raw {
			if line.Rec.Raw[i] != c.raw[i] {
				t.Fatalf("%q: raw[%d] = %02X, want %02X", c.in, i, line.Rec.Raw[i], c.raw[i])
			}
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{
		"",
		"Z",
		"R,P",
		"R,P,x,0C,00,140,OK",
		"R,P,1,0C,0,140,OK",
		"R,P,1,0C,0G,140,OK",
		"R,P,1,0C,001122334455667788,140,OK",
	} {
		if _, ok := Parse([]byte(in)); ok {
			t.Fatalf("%q: expected failure", in)
		}
	}
}

func BenchmarkParseRecord(b *testing.B) {
	line := []byte("R,P,16199,0C,0CFEAAAAAA,161,OK")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := Parse(line); !ok {
			b.Fatal("parse failed")
		}
	}
}
