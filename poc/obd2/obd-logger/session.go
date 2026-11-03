package main

import (
	"strconv"
	"strings"
	"time"

	"m5stick-obd2-cnx/internal/obddtc"
	"m5stick-obd2-cnx/internal/obdpid"
)

var (
	errReplay   = &simpleError{"replay mode"}
	errStop     = &simpleError{"host stop"}
	errLinkLost = &simpleError{"ble link lost"}
	errNoDevice = &simpleError{"no OBDII BLE device found"}
)

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

// sessionEpoch is the monotonic reference for every live/simulated session.
// It is set once at boot and never reset on reconnect.
var sessionEpoch = time.Now()

func liveMs() int64 { return time.Since(sessionEpoch).Milliseconds() }

// Clock keeps Session independent from wall time in tests.
type Clock interface {
	Now() time.Time
	Ms() int64
	Sleep(d time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Ms() int64             { return liveMs() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

// Sink receives everything a session produces: protocol lines and UI state.
type Sink interface {
	Start(ms int64, vin string)
	Event(ms int64, typ, detail string)
	PID(ms int64, pid string, raw []byte, latencyMs int64)
	End(ms int64, reason string)
	Quality(pid string, q obdpid.Quality)
	MIL(on bool)
	DTCCount(count int)
	HandshakeOK()
}

type pidResult uint8

const (
	pidOK    pidResult = iota
	pidRetry           // got a response, but no usable data: the link is alive
	pidTimeout
	pidLinkLost
)

type pidSpec struct {
	cmd  string
	pid  string
	unit string
	kind byte
}

const (
	kRPM byte = iota
	kSpeed
	kTemp
	kPct
	kMAP
	kMAF
	kVoltage
	kCatalyst
	kLambda
	kTrim
	kBaro
)

var fastPIDs = []pidSpec{
	{"010C", "0C", "rpm", kRPM},
	{"010D", "0D", "km/h", kSpeed},
	{"0111", "11", "%", kPct},
	{"0104", "04", "%", kPct},
	{"010B", "0B", "kPa", kMAP},
	{"0110", "10", "g/s", kMAF},
	{"0144", "44", "lambda", kLambda},
	{"0106", "06", "%", kTrim},
	{"0107", "07", "%", kTrim},
}

var slowPIDs = []pidSpec{
	{"0105", "05", "C", kTemp},
	{"010F", "0F", "C", kTemp},
	{"012F", "2F", "%", kPct},
	{"013C", "3C", "C", kCatalyst},
	{"0142", "42", "V", kVoltage},
	{"0152", "52", "%", kPct},
	{"0146", "46", "C", kTemp},
	{"0145", "45", "%", kPct},
	{"0149", "49", "%", kPct},
	{"014A", "4A", "%", kPct},
	{"014C", "4C", "%", kPct},
	{"0133", "33", "kPa", kBaro},
}

// Session is the transport-agnostic live loop. It runs unchanged against BLE
// on the car and against the serial simulator on the bench.
type Session struct {
	T     Transport
	Sink  Sink
	Clock Clock
	// Mode is checked between commands. It returns errReplay, errStop or nil.
	Mode func() error
}

func (s *Session) Run() error {
	if s.Clock == nil {
		s.Clock = realClock{}
	}
	if s.Mode == nil {
		s.Mode = func() error { return nil }
	}
	if err := s.Mode(); err != nil {
		return err
	}

	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		if _, r := s.T.Command(cmd, 3*time.Second); r != ReplyOK {
			if r == ReplyWrite {
				return errLinkLost
			}
			return &simpleError{"handshake " + cmd + ": " + r.Error()}
		}
		s.Clock.Sleep(100 * time.Millisecond)
	}
	s.Sink.HandshakeOK()

	// Emit I before the diagnostics so the host sees the session start first.
	vin, r := readVIN(s.T, s.Clock)
	if r == ReplyWrite {
		return errLinkLost
	}
	s.Sink.Start(s.Clock.Ms(), vin)

	endReason := "error"
	defer func() {
		s.Sink.End(s.Clock.Ms(), endReason)
	}()

	if r := readDiagnostics(s.T, s.Sink, s.Clock); r == ReplyWrite {
		endReason = "ble_lost"
		return errLinkLost
	}

	cycle := 0
	fails := 0
	for {
		if err := s.Mode(); err != nil {
			endReason = endReasonFor(err)
			return err
		}
		for _, p := range fastPIDs {
			if err := s.Mode(); err != nil {
				endReason = endReasonFor(err)
				return err
			}
			switch s.poll(p) {
			case pidLinkLost:
				endReason = "ble_lost"
				return errLinkLost
			case pidTimeout:
				fails++
				if fails >= 3 {
					endReason = "ble_lost"
					return errLinkLost
				}
			default:
				// OK or NO DATA/PARSE_ERROR: the adapter answered, so the
				// link is healthy.
				fails = 0
			}
		}
		cycle++
		if cycle%4 == 0 {
			for _, p := range slowPIDs {
				if err := s.Mode(); err != nil {
					endReason = endReasonFor(err)
					return err
				}
				switch s.poll(p) {
				case pidLinkLost:
					endReason = "ble_lost"
					return errLinkLost
				case pidTimeout:
					fails++
					if fails >= 3 {
						endReason = "ble_lost"
						return errLinkLost
					}
				default:
					fails = 0
				}
				s.Clock.Sleep(20 * time.Millisecond)
			}
		}
		s.Clock.Sleep(100 * time.Millisecond)
	}
}

func endReasonFor(err error) string {
	switch err {
	case errReplay:
		return "replay"
	case errStop:
		return "host"
	default:
		return "error"
	}
}

func (s *Session) poll(p pidSpec) pidResult {
	t0 := s.Clock.Now()
	resp, r := s.T.Command(p.cmd, 2*time.Second)
	lat := int64(s.Clock.Now().Sub(t0) / time.Millisecond)
	ms := s.Clock.Ms()

	switch r {
	case ReplyWrite:
		return pidLinkLost
	case ReplyTimeout:
		s.Sink.Quality(p.pid, obdpid.QualityNoData)
		s.Sink.Event(ms, "NO_DATA", p.pid)
		return pidTimeout
	}
	if strings.Contains(resp, "NO DATA") {
		s.Sink.Quality(p.pid, obdpid.QualityNoData)
		s.Sink.Event(ms, "NO_DATA", p.pid)
		return pidRetry
	}

	data, ok := pidData(resp, p.pid)
	if !ok {
		s.Sink.Quality(p.pid, obdpid.QualityInvalid)
		s.Sink.Event(ms, "PARSE_ERROR", p.pid)
		return pidRetry
	}
	s.Sink.PID(ms, p.pid, data, lat)
	return pidOK
}

func readDiagnostics(t Transport, sink Sink, clock Clock) Reply {
	statusOK := false
	if resp, r := t.Command("0101", 2*time.Second); r == ReplyWrite {
		return r
	} else if r == ReplyOK {
		if d := servicePayload(resp, "4101"); len(d) >= 4 {
			if st, ok := obddtc.ParseStatus(d); ok {
				statusOK = true
				ms := clock.Ms()
				sink.MIL(st.MIL)
				sink.DTCCount(st.Count)
				sink.Event(ms, "MIL", onOff(st.MIL))
				sink.Event(ms, "DTC_COUNT", strconv.Itoa(st.Count))
				sink.Event(ms, "READINESS", hexBytes(st.Readiness[:]))
			}
		}
	}

	stored := 0
	modes := [...]struct {
		cmd    string
		header string
		typ    string
	}{
		{"03", "43", "DTC"},
		{"07", "47", "DTC_PENDING"},
		{"0A", "4A", "DTC_PERMANENT"},
	}
	for _, m := range modes {
		resp, r := t.Command(m.cmd, 3*time.Second)
		if r == ReplyWrite {
			return r
		}
		if r != ReplyOK {
			continue
		}
		codes := obddtc.Codes(servicePayload(resp, m.header))
		if m.cmd == "03" {
			stored = len(codes)
		}
		for _, code := range codes {
			sink.Event(clock.Ms(), m.typ, code)
		}
	}
	if !statusOK {
		sink.DTCCount(stored)
	}
	return ReplyOK
}

func readVIN(t Transport, clock Clock) (string, Reply) {
	resp, r := t.Command("0902", 4*time.Second)
	if r != ReplyOK {
		return "", r
	}
	s := strings.ToUpper(resp)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ">", "")
	var hex strings.Builder
	framed := strings.Contains(s, ":")
	for _, part := range strings.Split(s, "|") {
		if framed {
			i := strings.Index(part, ":")
			if i < 0 {
				continue
			}
			part = part[i+1:]
		}
		hex.WriteString(part)
	}
	raw := hex.String()
	i := strings.Index(raw, "490201")
	if i < 0 {
		return "", ReplyOK
	}
	h := raw[i+6:]
	var out []byte
	for len(h) >= 2 {
		v, err := strconv.ParseUint(h[:2], 16, 8)
		if err != nil {
			break
		}
		if v == 0xAA {
			break
		}
		out = append(out, byte(v))
		h = h[2:]
	}
	if len(out) != 17 {
		return "", ReplyOK
	}
	for _, b := range out {
		if b < 0x20 || b > 0x7E {
			return "", ReplyOK
		}
	}
	return string(out), ReplyOK
}

// cleanELM removes echo/whitespace and the ISO-TP frame prefixes that the
// ELM327 inserts for multi-frame responses ("0:490201...").
func cleanELM(resp string) string {
	s := strings.ToUpper(resp)
	var b strings.Builder
	for _, part := range strings.Split(s, "|") {
		if i := strings.IndexByte(part, ':'); i >= 0 {
			part = part[i+1:]
		}
		for i := 0; i < len(part); i++ {
			switch part[i] {
			case ' ', '\r', '\n', '>', '|':
				continue
			}
			b.WriteByte(part[i])
		}
	}
	return b.String()
}

// servicePayload returns the bytes that follow the response header, e.g.
// "4302 0133 C123" with header "43" yields {02,01,33,C1,23}.
func servicePayload(resp, header string) []byte {
	s := cleanELM(resp)
	i := strings.Index(s, header)
	if i < 0 {
		return nil
	}
	h := s[i+len(header):]
	out := make([]byte, 0, len(h)/2)
	for len(h) >= 2 {
		v, err := strconv.ParseUint(h[:2], 16, 8)
		if err != nil {
			break
		}
		out = append(out, byte(v))
		h = h[2:]
	}
	return out
}

func pidData(resp, pid string) ([]byte, bool) {
	s := strings.ToUpper(resp)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ">", "")
	i := strings.Index(s, "41"+pid)
	if i < 0 {
		return nil, false
	}
	h := s[i+4:]
	var out []byte
	for len(h) >= 2 {
		v, err := strconv.ParseUint(h[:2], 16, 8)
		if err != nil {
			break
		}
		out = append(out, byte(v))
		h = h[2:]
	}
	return out, len(out) > 0
}

func hexBytes(d []byte) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, len(d)*2)
	for _, b := range d {
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out)
}

func onOff(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}
