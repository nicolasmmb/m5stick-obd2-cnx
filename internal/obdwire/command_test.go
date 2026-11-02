package obdwire

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		line string
		kind Kind
	}{
		{"A", KindAck},
		{"C,REPLAY", KindReplay},
		{"C,LIVE", KindLive},
		{"C,CLEAR", KindClear},
	}
	for _, tt := range tests {
		c, err := ParseCommand(tt.line)
		if err != nil {
			t.Fatalf("%q: %v", tt.line, err)
		}
		if c.Kind != tt.kind {
			t.Fatalf("%q kind=%v want %v", tt.line, c.Kind, tt.kind)
		}
	}
}

func TestParseReplayPID(t *testing.T) {
	c, err := ParseCommand("R,P,241,0C,1AF8,138,OK")
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != KindRecord || c.Record.Type != RecordPID {
		t.Fatalf("unexpected command: %+v", c)
	}
	if c.Record.PID != "0C" || c.Record.Raw != "1AF8" || c.Record.Ms != 241 {
		t.Fatalf("unexpected record: %+v", c.Record)
	}
}

func TestParseInvalid(t *testing.T) {
	for _, line := range []string{"", "X", "R,P,1", "C,FOO"} {
		if _, err := ParseCommand(line); err == nil {
			t.Fatalf("expected error for %q", line)
		}
	}
}
