package wire

import (
	"bytes"
	"testing"
)

func TestChecksum(t *testing.T) {
	// 0x01 (PID) + 0x00 0x03 (length) + 0x03 (cmd) = 0x07.
	got := Checksum([]byte{0x01, 0x00, 0x03, 0x03})
	if got != 0x0007 {
		t.Fatalf("checksum = %#04x, want 0x0007", got)
	}
}

func TestBuildCommand(t *testing.T) {
	got := BuildCommand(0xFFFFFFFF, 0x03, nil)
	want := []byte{
		0xEF, 0x01, 0xFF, 0xFF, 0xFF, 0xFF,
		0x01, 0x00, 0x03, 0x03, 0x00, 0x07,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("BuildCommand = % X, want % X", got, want)
	}
}

func TestBuildCommandInto(t *testing.T) {
	buf := make([]byte, 32)
	got := BuildCommandInto(buf, 0xFFFFFFFF, 0x03, nil)
	want := BuildCommand(0xFFFFFFFF, 0x03, nil)
	if !bytes.Equal(got, want) {
		t.Fatalf("BuildCommandInto = % X, want % X", got, want)
	}
}

func TestBuildCommandIntoShortBuffer(t *testing.T) {
	if got := BuildCommandInto(make([]byte, 11), 0xFFFFFFFF, 0x03, nil); got != nil {
		t.Fatalf("short buffer returned % X, want nil", got)
	}
	if got := BuildCommandInto(make([]byte, 12), 0xFFFFFFFF, 0x02, []byte{0x01}); got != nil {
		t.Fatalf("short payload buffer returned % X, want nil", got)
	}
}

func TestBuildCommandWithPayload(t *testing.T) {
	got := BuildCommand(0xFFFFFFFF, 0x02, []byte{0x01})
	if len(got) != 13 {
		t.Fatalf("len = %d, want 13", len(got))
	}
	if got[7] != 0x00 || got[8] != 0x04 {
		t.Fatalf("length = %#02x%#02x, want 0x0004", got[7], got[8])
	}
	if got[10] != 0x01 {
		t.Fatalf("payload = %#02x, want 0x01", got[10])
	}
	// Checksum covers PID, length, cmd and payload.
	want := Checksum(got[6 : len(got)-2])
	if got[len(got)-2] != byte(want>>8) || got[len(got)-1] != byte(want) {
		t.Fatalf("checksum = % X, want % X", got[len(got)-2:], []byte{byte(want >> 8), byte(want)})
	}
}

func TestAckData(t *testing.T) {
	pkt := []byte{
		0xEF, 0x01, 0xFF, 0xFF, 0xFF, 0xFF,
		0x07, 0x00, 0x05, 0x00, 0x12, 0x34, 0x00, 0x00,
	}
	confirm, data := AckData(pkt)
	if confirm != 0x00 {
		t.Fatalf("confirm = %#02x, want 0x00", confirm)
	}
	if !bytes.Equal(data, []byte{0x12, 0x34}) {
		t.Fatalf("data = % X, want 12 34", data)
	}
	if c, d := AckData(pkt[:8]); c != 0xFF || d != nil {
		t.Fatalf("short ACK = (%#02x, % X), want (0xFF, nil)", c, d)
	}
}

func TestIsWakeup(t *testing.T) {
	if !IsWakeup(WakeupPattern[:]) {
		t.Fatal("WakeupPattern not recognized")
	}
	other := WakeupPattern
	other[9] = 0x00
	if IsWakeup(other[:]) {
		t.Fatal("modified packet recognized as wakeup")
	}
	if IsWakeup(WakeupPattern[:11]) {
		t.Fatal("short packet recognized as wakeup")
	}
}
