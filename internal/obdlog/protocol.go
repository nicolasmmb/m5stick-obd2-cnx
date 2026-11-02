package obdlog

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Record is one NDJSON line emitted by the M5Stick logger.
type Record struct {
	Type   string
	Start  *Start
	Sample *Sample
	Event  *Event
	End    *End
}

type Start struct {
	T      string `json:"t"`
	V      int    `json:"v"`
	SID    string `json:"sid,omitempty"`
	FW     string `json:"fw,omitempty"`
	VIN    string `json:"vin,omitempty"`
	TankDL int    `json:"tank_dl,omitempty"`
}

type Sample struct {
	T       string  `json:"t"`
	Seq     uint64  `json:"seq,omitempty"`
	Ms      int64   `json:"ms"`
	PID     string  `json:"pid"`
	Raw     string  `json:"raw,omitempty"`
	Value   float64 `json:"v"`
	Unit    string  `json:"u"`
	Quality string  `json:"q"`
	Latency int     `json:"lat,omitempty"`
}

type Event struct {
	T    string `json:"t"`
	Ms   int64  `json:"ms"`
	Type string `json:"type"`
	Code string `json:"code,omitempty"`
	Msg  string `json:"msg,omitempty"`
}

type End struct {
	T      string `json:"t"`
	Ms     int64  `json:"ms"`
	Reason string `json:"reason,omitempty"`
}

// ParseLine parses one NDJSON record and validates the required fields.
func ParseLine(line []byte) (Record, error) {
	line = []byte(strings.TrimSpace(string(line)))
	if len(line) == 0 {
		return Record{}, fmt.Errorf("empty line")
	}

	var typ struct {
		T string `json:"t"`
	}
	if err := json.Unmarshal(line, &typ); err != nil {
		return Record{}, fmt.Errorf("invalid json: %w", err)
	}

	switch typ.T {
	case "S":
		var r Start
		if err := json.Unmarshal(line, &r); err != nil {
			return Record{}, err
		}
		if r.V <= 0 {
			r.V = 1
		}
		return Record{Type: "S", Start: &r}, nil
	case "P":
		var r Sample
		if err := json.Unmarshal(line, &r); err != nil {
			return Record{}, err
		}
		r.PID = strings.ToUpper(strings.TrimSpace(r.PID))
		r.Quality = strings.ToUpper(strings.TrimSpace(r.Quality))
		if r.PID == "" {
			return Record{}, fmt.Errorf("sample without pid")
		}
		if r.Unit == "" {
			return Record{}, fmt.Errorf("sample without unit")
		}
		if r.Quality == "" {
			r.Quality = "OK"
		}
		if r.Ms < 0 {
			return Record{}, fmt.Errorf("negative ms")
		}
		return Record{Type: "P", Sample: &r}, nil
	case "E":
		var r Event
		if err := json.Unmarshal(line, &r); err != nil {
			return Record{}, err
		}
		if r.Type == "" {
			return Record{}, fmt.Errorf("event without type")
		}
		return Record{Type: "E", Event: &r}, nil
	case "X":
		var r End
		if err := json.Unmarshal(line, &r); err != nil {
			return Record{}, err
		}
		return Record{Type: "X", End: &r}, nil
	default:
		return Record{}, fmt.Errorf("unknown record type %q", typ.T)
	}
}
