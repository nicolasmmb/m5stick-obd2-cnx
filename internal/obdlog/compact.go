package obdlog

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseAnyLine accepts both the new compact protocol and legacy NDJSON.
func ParseAnyLine(line []byte) (Record, error) {
	s := strings.TrimSpace(string(line))
	if strings.HasPrefix(s, "{") {
		return ParseLine([]byte(s))
	}
	return ParseCompactLine(s)
}

// ParseCompactLine parses:
//
//	I,<ms>,<fw>,<vin>,<tank_dl>
//	P,<ms>,<pid>,<raw>,<lat>,<q>
//	E,<ms>,<type>,<detail>
//	X,<ms>,<reason>
func ParseCompactLine(line string) (Record, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Record{}, fmt.Errorf("empty line")
	}
	switch line[0] {
	case 'I':
		parts := strings.SplitN(line, ",", 5)
		if len(parts) < 5 {
			return Record{}, fmt.Errorf("invalid I line")
		}
		fw := parts[2]
		vin := parts[3]
		tank, _ := strconv.Atoi(parts[4])
		return Record{Type: "S", Start: &Start{T: "S", V: 1, FW: fw, VIN: vin, TankDL: tank}}, nil
	case 'P':
		parts := strings.SplitN(line, ",", 6)
		if len(parts) < 6 {
			return Record{}, fmt.Errorf("invalid P line")
		}
		ms, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return Record{}, fmt.Errorf("invalid ms: %w", err)
		}
		pid := strings.ToUpper(strings.TrimSpace(parts[2]))
		raw := strings.ToUpper(strings.TrimSpace(parts[3]))
		lat, _ := strconv.Atoi(parts[4])
		q := strings.ToUpper(strings.TrimSpace(parts[5]))
		if q == "" {
			q = "OK"
		}
		value, unit, ok := DecodePID(pid, raw)
		if !ok {
			unit = "raw"
			value = 0
			if q == "OK" {
				q = "NO_DECODER"
			}
		}
		return Record{Type: "P", Sample: &Sample{
			T: "P", Ms: ms, PID: pid, Raw: raw, Value: value,
			Unit: unit, Quality: q, Latency: lat,
		}}, nil
	case 'E':
		parts := strings.SplitN(line, ",", 4)
		if len(parts) < 4 {
			return Record{}, fmt.Errorf("invalid E line")
		}
		ms, _ := strconv.ParseInt(parts[1], 10, 64)
		return Record{Type: "E", Event: &Event{T: "E", Ms: ms, Type: parts[2], Msg: parts[3]}}, nil
	case 'X':
		parts := strings.SplitN(line, ",", 3)
		if len(parts) < 2 {
			return Record{}, fmt.Errorf("invalid X line")
		}
		ms, _ := strconv.ParseInt(parts[1], 10, 64)
		reason := ""
		if len(parts) > 2 {
			reason = parts[2]
		}
		return Record{Type: "X", End: &End{T: "X", Ms: ms, Reason: reason}}, nil
	default:
		return Record{}, fmt.Errorf("unknown compact line %q", line[0])
	}
}

// DecodePID converts the raw data bytes of a PID into a value and unit.
// It covers the PIDs emitted by the firmware logger.
func DecodePID(pid, rawHex string) (float64, string, bool) {
	d, err := hexBytes(rawHex)
	if err != nil || len(d) == 0 {
		return 0, "", false
	}
	switch pid {
	case "0C":
		if len(d) < 2 {
			return 0, "", false
		}
		return float64(int(d[0])*256+int(d[1])) / 4, "rpm", true
	case "0D":
		return float64(d[0]), "km/h", true
	case "05", "0F", "46":
		return float64(int(d[0]) - 40), "C", true
	case "11", "04", "2F", "52", "45", "49", "4A", "4C":
		return float64(d[0]) * 100 / 255, "%", true
	case "0B", "33":
		return float64(d[0]), "kPa", true
	case "10":
		if len(d) < 2 {
			return 0, "", false
		}
		return float64(int(d[0])*256+int(d[1])) / 100, "g/s", true
	case "42":
		if len(d) < 2 {
			return 0, "", false
		}
		return float64(int(d[0])*256+int(d[1])) / 1000, "V", true
	case "3C":
		if len(d) < 2 {
			return 0, "", false
		}
		return float64(int(d[0])*256+int(d[1]))/10 - 40, "C", true
	case "44":
		if len(d) < 2 {
			return 0, "", false
		}
		return 2 * float64(int(d[0])*256+int(d[1])) / 65536, "lambda", true
	case "06", "07":
		return (float64(d[0]) - 128) * 100 / 128, "%", true
	case "0E":
		return float64(d[0])/2 - 64, "deg", true
	case "1F":
		if len(d) < 2 {
			return 0, "", false
		}
		return float64(int(d[0])*256 + int(d[1])), "s", true
	case "21", "31":
		if len(d) < 2 {
			return 0, "", false
		}
		return float64(int(d[0])*256 + int(d[1])), "km", true
	case "43":
		if len(d) < 2 {
			return 0, "", false
		}
		return float64(int(d[0])*256+int(d[1])) * 100 / 255, "%", true
	default:
		return 0, "", false
	}
}

func hexBytes(s string) ([]byte, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
	if len(s) < 2 || len(s)%2 != 0 {
		return nil, fmt.Errorf("invalid hex length")
	}
	out := make([]byte, 0, len(s)/2)
	for len(s) >= 2 {
		v, err := strconv.ParseUint(s[:2], 16, 8)
		if err != nil {
			return nil, err
		}
		out = append(out, byte(v))
		s = s[2:]
	}
	return out, nil
}
