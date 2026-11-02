// Package obddtc decodes the diagnostic status (Mode 01 PID 01) and the DTC
// payloads returned by Modes 03, 07 and 0A. The firmware runs this once per
// connection, so small allocations here are acceptable.
package obddtc

// Status is the decoded payload of Mode 01 PID 01 (bytes A..D).
type Status struct {
	MIL       bool
	Count     int
	Readiness [3]byte
}

// ParseStatus decodes the four data bytes of PID 01:
//
//	A7 = MIL, A6..A0 = confirmed DTC count, B/C/D = readiness bitfields.
func ParseStatus(raw []byte) (Status, bool) {
	if len(raw) < 4 {
		return Status{}, false
	}
	return Status{
		MIL:       raw[0]&0x80 != 0,
		Count:     int(raw[0] & 0x7F),
		Readiness: [3]byte{raw[1], raw[2], raw[3]},
	}, true
}

// Codes decodes a Mode 03/07/0A payload: the first byte is the number of DTCs,
// followed by two bytes per code. Padding (00 00 / AA AA) is ignored.
func Codes(payload []byte) []string {
	if len(payload) == 0 {
		return nil
	}
	n := int(payload[0])
	payload = payload[1:]
	if n > len(payload)/2 {
		n = len(payload) / 2
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		code, ok := Format(payload[2*i], payload[2*i+1])
		if !ok {
			continue
		}
		out = append(out, code)
	}
	return out
}

// Format converts a two-byte DTC into its five-character form (P0133).
func Format(a, b byte) (string, bool) {
	if (a == 0 && b == 0) || (a == 0xAA && b == 0xAA) {
		return "", false
	}
	var out [5]byte
	switch a >> 6 {
	case 0:
		out[0] = 'P'
	case 1:
		out[0] = 'C'
	case 2:
		out[0] = 'B'
	default:
		out[0] = 'U'
	}
	out[1] = hexDigit((a >> 4) & 0x03)
	out[2] = hexDigit(a & 0x0F)
	out[3] = hexDigit(b >> 4)
	out[4] = hexDigit(b & 0x0F)
	return string(out[:]), true
}

func hexDigit(v byte) byte {
	if v < 10 {
		return '0' + v
	}
	return 'A' + (v - 10)
}
