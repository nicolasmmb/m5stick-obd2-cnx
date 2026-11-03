// Package obdline parses the compact serial protocol used by the device.
//
// Unlike obdwire, which is convenient for the host, this parser works on a
// fixed byte slice and never allocates: the firmware runs it on every replay
// record at 60 fps.
package obdline

const (
	// MaxRawBytes is the number of decoded payload bytes supported per record.
	MaxRawBytes = 8
)

type Kind uint8

const (
	KindIgnore Kind = iota
	KindAck
	KindReplay
	KindLive
	KindClear
	KindStop
	KindSim
	KindMem
	KindRecord
)

// Record is a decoded P record. PID and Raw are fixed-size to avoid retaining
// heap objects per sample.
type Record struct {
	PID     [2]byte
	PIDLen  uint8
	Raw     [MaxRawBytes]byte
	RawLen  uint8
	Ms      int64
	Latency int64
}

type Line struct {
	Kind Kind
	Rec  Record
}

// Parse decodes one line without the trailing newline. The returned Line does
// not reference the input buffer.
func Parse(line []byte) (Line, bool) {
	line = trimSpace(line)
	switch {
	case len(line) == 1 && line[0] == 'A':
		return Line{Kind: KindAck}, true
	case equal(line, "C,REPLAY"):
		return Line{Kind: KindReplay}, true
	case equal(line, "C,LIVE"):
		return Line{Kind: KindLive}, true
	case equal(line, "C,CLEAR"):
		return Line{Kind: KindClear}, true
	case equal(line, "C,STOP"):
		return Line{Kind: KindStop}, true
	case equal(line, "C,SIM"):
		return Line{Kind: KindSim}, true
	case equal(line, "C,MEM"):
		return Line{Kind: KindMem}, true
	}
	if len(line) >= 2 && line[0] == 'R' && line[1] == ',' {
		rec, ok := parseRecord(line[2:])
		if !ok {
			return Line{}, false
		}
		if rec.PIDLen == 0 {
			return Line{Kind: KindIgnore}, true
		}
		return Line{Kind: KindRecord, Rec: rec}, true
	}
	return Line{}, false
}

// parseRecord handles "P,ms,pid,raw,latency,quality". Other record types are
// valid but carry no data for the UI, so they are reported as ignored.
func parseRecord(b []byte) (Record, bool) {
	if len(b) == 0 {
		return Record{}, false
	}
	if b[0] != 'P' {
		return Record{}, true
	}
	if len(b) < 2 || b[1] != ',' {
		return Record{}, false
	}
	b = b[2:]

	var rec Record
	var ok bool
	rec.Ms, b, ok = parseInt(b)
	if !ok || len(b) == 0 || b[0] != ',' {
		return Record{}, false
	}
	b = b[1:]

	pidStart := b
	for len(b) > 0 && b[0] != ',' {
		b = b[1:]
	}
	pid := pidStart[:len(pidStart)-len(b)]
	if len(pid) == 0 || len(pid) > 2 {
		return Record{}, false
	}
	for i := 0; i < len(pid); i++ {
		rec.PID[i] = upper(pid[i])
	}
	rec.PIDLen = uint8(len(pid))
	if len(b) == 0 {
		return rec, true
	}
	b = b[1:]

	rawStart := b
	for len(b) > 0 && b[0] != ',' {
		b = b[1:]
	}
	raw := rawStart[:len(rawStart)-len(b)]
	if len(raw)%2 != 0 || len(raw)/2 > MaxRawBytes {
		return Record{}, false
	}
	for i := 0; i < len(raw); i += 2 {
		hi, ok1 := hexValue(raw[i])
		lo, ok2 := hexValue(raw[i+1])
		if !ok1 || !ok2 {
			return Record{}, false
		}
		rec.Raw[i/2] = hi<<4 | lo
	}
	rec.RawLen = uint8(len(raw) / 2)
	if len(b) == 0 {
		return rec, true
	}
	b = b[1:]

	rec.Latency, _, _ = parseInt(b)
	return rec, true
}

func parseInt(b []byte) (int64, []byte, bool) {
	if len(b) == 0 {
		return 0, b, false
	}
	neg := false
	if b[0] == '-' {
		neg = true
		b = b[1:]
	}
	var v int64
	seen := false
	for len(b) > 0 && b[0] >= '0' && b[0] <= '9' {
		v = v*10 + int64(b[0]-'0')
		b = b[1:]
		seen = true
	}
	if !seen {
		return 0, b, false
	}
	if neg {
		v = -v
	}
	return v, b, true
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && isSpace(b[0]) {
		b = b[1:]
	}
	for len(b) > 0 && isSpace(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return b
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func equal(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := 0; i < len(b); i++ {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}
