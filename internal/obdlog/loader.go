package obdlog

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	OutDir    string
	SessionID string
	Port      string
	Baud      int
	Replay    bool
}

type Metadata struct {
	SessionID       string `json:"session_id"`
	SourceSessionID string `json:"source_session_id,omitempty"`
	StartedAt       string `json:"started_at"`
	EndedAt         string `json:"ended_at,omitempty"`
	Port            string `json:"port,omitempty"`
	Baud            int    `json:"baud,omitempty"`
	Replay          bool   `json:"replay"`
	Firmware        string `json:"firmware,omitempty"`
	VIN             string `json:"vin,omitempty"`
	TankDL          int    `json:"tank_dl,omitempty"`
	Samples         int    `json:"samples"`
	Events          int    `json:"events"`
	InvalidLines    int    `json:"invalid_lines"`
	EndReason       string `json:"end_reason,omitempty"`
	DurationMs      int64  `json:"duration_ms"`
}

type field struct {
	pid  string
	name string
}

// PIDs included in the 1 Hz wide CSV. The long CSV contains every PID.
var wideFields = []field{
	{"0C", "rpm"},
	{"0D", "speed"},
	{"04", "load"},
	{"05", "coolant"},
	{"0F", "intake"},
	{"11", "throttle"},
	{"0B", "map"},
	{"10", "maf"},
	{"06", "stft"},
	{"07", "ltft"},
	{"44", "lambda"},
	{"2F", "fuel_pct"},
	{"3C", "catalyst"},
	{"42", "voltage"},
	{"52", "ethanol"},
	{"46", "ambient"},
	{"45", "relative_throttle"},
	{"49", "pedal_d"},
	{"4A", "pedal_e"},
	{"4C", "commanded_throttle"},
	{"33", "baro"},
}

type Loader struct {
	dir string

	raw      *os.File
	decoded  *os.File
	longFile *os.File
	long     *csv.Writer
	wideFile *os.File
	wide     *csv.Writer
	events   *os.File
	invalid  *os.File

	meta Metadata
	opts Options

	start      time.Time
	last       map[string]Sample
	lastMs     int64
	nextWide   int64
	lastWideMs int64
	flushCount int
	closed     bool
}

func NewLoader(opts Options) (*Loader, error) {
	out := opts.OutDir
	if out == "" {
		out = "data"
	}
	sid := opts.SessionID
	if sid == "" {
		sid = time.Now().Format("20060102-150405")
	}
	sid = safeName(sid)

	dir, err := uniqueDir(filepath.Join(out, sid))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	l := &Loader{
		dir:  dir,
		opts: opts,
		last: make(map[string]Sample),
		meta: Metadata{
			SessionID: sid,
			StartedAt: time.Now().Format(time.RFC3339Nano),
			Port:      opts.Port,
			Baud:      opts.Baud,
			Replay:    opts.Replay,
		},
	}

	if l.raw, err = os.Create(filepath.Join(dir, "raw.log")); err != nil {
		return nil, err
	}
	if l.decoded, err = os.Create(filepath.Join(dir, "decoded.ndjson")); err != nil {
		l.Close()
		return nil, err
	}
	if l.events, err = os.Create(filepath.Join(dir, "events.jsonl")); err != nil {
		l.Close()
		return nil, err
	}
	if l.invalid, err = os.Create(filepath.Join(dir, "invalid.log")); err != nil {
		l.Close()
		return nil, err
	}

	l.longFile, err = os.Create(filepath.Join(dir, "samples_long.csv"))
	if err != nil {
		l.Close()
		return nil, err
	}
	l.long = csv.NewWriter(l.longFile)
	if err := l.long.Write([]string{"ts_host", "session_id", "seq", "ms", "pid", "raw", "value", "unit", "quality", "latency_ms"}); err != nil {
		l.Close()
		return nil, err
	}
	l.long.Flush()

	l.wideFile, err = os.Create(filepath.Join(dir, "samples_wide_1hz.csv"))
	if err != nil {
		l.Close()
		return nil, err
	}
	l.wide = csv.NewWriter(l.wideFile)
	header := []string{"ms", "ts_host"}
	for _, f := range wideFields {
		header = append(header, f.name, f.name+"_age_ms")
	}
	if err := l.wide.Write(header); err != nil {
		l.Close()
		return nil, err
	}
	l.wide.Flush()

	return l, nil
}

func (l *Loader) Dir() string { return l.dir }

// ProcessLine records one raw NDJSON line and, if valid, updates the CSV state.
func (l *Loader) ProcessLine(line []byte, received time.Time) error {
	if l.closed {
		return fmt.Errorf("loader closed")
	}
	line = bytes.TrimRight(line, "\r\n")
	if _, err := l.raw.Write(append(line, '\n')); err != nil {
		return err
	}

	rec, err := ParseAnyLine(line)
	if err != nil {
		l.meta.InvalidLines++
		if _, werr := l.invalid.Write(append(line, '\n')); werr != nil {
			return werr
		}
		return nil
	}
	if err := l.writeDecoded(rec); err != nil {
		return err
	}

	switch rec.Type {
	case "S":
		if l.start.IsZero() {
			l.start = received
			l.meta.StartedAt = received.Format(time.RFC3339Nano)
		}
		if rec.Start.SID != "" {
			l.meta.SourceSessionID = rec.Start.SID
		}
		if rec.Start.FW != "" {
			l.meta.Firmware = rec.Start.FW
		}
		if rec.Start.VIN != "" {
			l.meta.VIN = rec.Start.VIN
		}
		if rec.Start.TankDL > 0 {
			l.meta.TankDL = rec.Start.TankDL
		}
	case "P":
		if l.start.IsZero() {
			l.start = received
			l.meta.StartedAt = received.Format(time.RFC3339Nano)
		}
		s := *rec.Sample
		if l.nextWide == 0 {
			l.nextWide = (s.Ms/1000 + 1) * 1000
		}
		for l.nextWide < s.Ms {
			if err := l.writeWideRow(l.nextWide); err != nil {
				return err
			}
			l.nextWide += 1000
		}

		if err := l.writeLong(received, s); err != nil {
			return err
		}
		l.last[s.PID] = s
		l.lastMs = s.Ms
		l.meta.Samples++

		if l.nextWide == s.Ms {
			if err := l.writeWideRow(l.nextWide); err != nil {
				return err
			}
			l.nextWide += 1000
		}
	case "E":
		if _, err := l.events.Write(append(line, '\n')); err != nil {
			return err
		}
		l.meta.Events++
	case "X":
		l.meta.EndReason = rec.End.Reason
		l.lastMs = rec.End.Ms
	}

	l.flushCount++
	if l.flushCount >= 100 {
		l.flushCount = 0
		l.long.Flush()
		l.wide.Flush()
	}
	return nil
}

func (l *Loader) writeDecoded(rec Record) error {
	var v any
	switch rec.Type {
	case "S":
		v = rec.Start
	case "P":
		v = rec.Sample
	case "E":
		v = rec.Event
	case "X":
		v = rec.End
	default:
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = l.decoded.Write(append(b, '\n'))
	return err
}

func (l *Loader) writeLong(received time.Time, s Sample) error {
	l.long.Write([]string{
		received.Format(time.RFC3339Nano),
		l.meta.SessionID,
		strconv.FormatUint(s.Seq, 10),
		strconv.FormatInt(s.Ms, 10),
		s.PID,
		s.Raw,
		formatFloat(s.Value),
		s.Unit,
		s.Quality,
		strconv.Itoa(s.Latency),
	})
	return l.long.Error()
}

func (l *Loader) writeWideRow(ms int64) error {
	ts := l.start.Add(time.Duration(ms) * time.Millisecond).Format(time.RFC3339Nano)
	row := []string{strconv.FormatInt(ms, 10), ts}
	for _, f := range wideFields {
		s, ok := l.last[f.pid]
		if !ok {
			row = append(row, "", "")
			continue
		}
		age := ms - s.Ms
		if age < 0 {
			age = 0
		}
		row = append(row, formatFloat(s.Value), strconv.FormatInt(age, 10))
	}
	l.wide.Write(row)
	l.lastWideMs = ms
	return l.wide.Error()
}

func (l *Loader) Close() error {
	if l.closed {
		return nil
	}
	l.closed = true

	if !l.start.IsZero() && l.lastMs > l.lastWideMs {
		_ = l.writeWideRow(l.lastMs)
	}
	if l.long != nil {
		l.long.Flush()
	}
	if l.longFile != nil {
		_ = l.longFile.Close()
	}
	if l.wide != nil {
		l.wide.Flush()
	}
	if l.wideFile != nil {
		_ = l.wideFile.Close()
	}
	if l.raw != nil {
		_ = l.raw.Close()
	}
	if l.decoded != nil {
		_ = l.decoded.Close()
	}
	if l.events != nil {
		_ = l.events.Close()
	}
	if l.invalid != nil {
		_ = l.invalid.Close()
	}

	l.meta.EndedAt = time.Now().Format(time.RFC3339Nano)
	if l.lastMs > 0 {
		l.meta.DurationMs = l.lastMs
	} else if !l.start.IsZero() {
		l.meta.DurationMs = time.Since(l.start).Milliseconds()
	}
	b, err := json.MarshalIndent(l.meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.dir, "metadata.json"), append(b, '\n'), 0o644)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "session"
	}
	return out
}

func uniqueDir(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, nil
	}
	for i := 1; i < 1000; i++ {
		p := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p, nil
		}
	}
	return "", fmt.Errorf("could not create unique session directory")
}
