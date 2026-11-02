//go:build !tinygo

package main

import (
	"strings"
	"testing"
	"time"

	"m5stick-obd2-cnx/internal/obdpid"
)

type fakeReply struct {
	text string
	kind Reply
}

type fakeTransport struct {
	replies      map[string]fakeReply
	defaultReply Reply
	commands     []string
}

func (f *fakeTransport) Command(cmd string, _ time.Duration) (string, Reply) {
	f.commands = append(f.commands, cmd)
	if r, ok := f.replies[cmd]; ok {
		return r.text, r.kind
	}
	return "", f.defaultReply
}

type fakeSink struct {
	start     string
	events    []string
	pids      []string
	end       string
	mil       bool
	dtcCount  int
	handshake bool
}

func (f *fakeSink) Start(_ int64, vin string) { f.start = vin }
func (f *fakeSink) Event(_ int64, typ, detail string) {
	f.events = append(f.events, typ+","+detail)
}
func (f *fakeSink) PID(_ int64, pid string, _ []byte, _ int64) {
	f.pids = append(f.pids, pid)
}
func (f *fakeSink) End(_ int64, reason string)     { f.end = reason }
func (f *fakeSink) Quality(string, obdpid.Quality) {}
func (f *fakeSink) MIL(on bool)                    { f.mil = on }
func (f *fakeSink) DTCCount(n int)                 { f.dtcCount = n }
func (f *fakeSink) HandshakeOK()                   { f.handshake = true }

func (f *fakeSink) has(event string) bool {
	for _, e := range f.events {
		if e == event {
			return true
		}
	}
	return false
}

type fakeClock struct{ ms int64 }

func (c *fakeClock) Now() time.Time {
	return time.Unix(0, 0).Add(time.Duration(c.ms) * time.Millisecond)
}
func (c *fakeClock) Ms() int64             { return c.ms }
func (c *fakeClock) Sleep(d time.Duration) { c.ms += d.Milliseconds() }

func diagnosticsReplies() map[string]fakeReply {
	return map[string]fakeReply{
		"ATZ":   {text: "ELM327 v2.1", kind: ReplyOK},
		"ATE0":  {text: "OK", kind: ReplyOK},
		"ATL0":  {text: "OK", kind: ReplyOK},
		"ATS0":  {text: "OK", kind: ReplyOK},
		"ATSP0": {text: "OK", kind: ReplyOK},
		// Multi-frame VIN from logs/obd-bench-2026-11-01.log.
		"0902": {text: "014|0:490201394247|1:4541343841304C|2:47313835383831||>", kind: ReplyOK},
		"0101": {text: "41010007E100AA||>", kind: ReplyOK},
		"03":   {text: "4300AAAAAAAAAA||>", kind: ReplyOK},
		"07":   {text: "4700AAAAAAAAAA||>", kind: ReplyOK},
		"0A":   {text: "4A00AAAAAAAAAA||>", kind: ReplyOK},
	}
}

func TestSessionDiagnosticsAndHostStop(t *testing.T) {
	transport := &fakeTransport{replies: diagnosticsReplies()}
	sink := &fakeSink{}
	session := &Session{
		T:     transport,
		Sink:  sink,
		Clock: &fakeClock{},
		Mode: func() error {
			if len(transport.commands) > 8 {
				return errStop
			}
			return nil
		},
	}

	err := session.Run()
	if err != errStop {
		t.Fatalf("err = %v, want errStop", err)
	}
	if !sink.handshake {
		t.Fatal("handshake not reported")
	}
	if sink.start != "9BGEA48A0LG185881" {
		t.Fatalf("vin = %q", sink.start)
	}
	for _, want := range []string{"MIL,OFF", "DTC_COUNT,0", "READINESS,07E100"} {
		if !sink.has(want) {
			t.Fatalf("missing event %q in %v", want, sink.events)
		}
	}
	if sink.dtcCount != 0 || sink.mil {
		t.Fatalf("mil=%v count=%d", sink.mil, sink.dtcCount)
	}
	if sink.end != "host" {
		t.Fatalf("end = %q, want host", sink.end)
	}
}

func TestSessionLinkLostAfterThreeTimeouts(t *testing.T) {
	transport := &fakeTransport{
		replies:      diagnosticsReplies(),
		defaultReply: ReplyTimeout,
	}
	sink := &fakeSink{}
	session := &Session{T: transport, Sink: sink, Clock: &fakeClock{}}

	err := session.Run()
	if err != errLinkLost {
		t.Fatalf("err = %v, want errLinkLost", err)
	}
	if sink.end != "ble_lost" {
		t.Fatalf("end = %q, want ble_lost", sink.end)
	}
	noData := 0
	for _, e := range sink.events {
		if strings.HasPrefix(e, "NO_DATA,") {
			noData++
		}
	}
	if noData != 3 {
		t.Fatalf("NO_DATA count = %d, want 3", noData)
	}
}

func TestSessionWriteErrorIsImmediateLinkLoss(t *testing.T) {
	transport := &fakeTransport{
		replies:      diagnosticsReplies(),
		defaultReply: ReplyWrite,
	}
	sink := &fakeSink{}
	session := &Session{T: transport, Sink: sink, Clock: &fakeClock{}}

	if err := session.Run(); err != errLinkLost {
		t.Fatalf("err = %v, want errLinkLost", err)
	}
	if sink.end != "ble_lost" {
		t.Fatalf("end = %q", sink.end)
	}
}

func TestSessionStoredDTCs(t *testing.T) {
	replies := diagnosticsReplies()
	replies["03"] = fakeReply{text: "4302 0133 C123 AA||>", kind: ReplyOK}
	transport := &fakeTransport{replies: replies}
	sink := &fakeSink{}
	session := &Session{
		T:     transport,
		Sink:  sink,
		Clock: &fakeClock{},
		Mode: func() error {
			if len(transport.commands) > 8 {
				return errStop
			}
			return nil
		},
	}

	if err := session.Run(); err != errStop {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"DTC,P0133", "DTC,U0123"} {
		if !sink.has(want) {
			t.Fatalf("missing %q in %v", want, sink.events)
		}
	}
	if sink.dtcCount != 0 {
		t.Fatalf("dtc_count = %d, want 0 (from 0101)", sink.dtcCount)
	}
}

func TestSessionReplayModeDoesNotStart(t *testing.T) {
	sink := &fakeSink{}
	session := &Session{
		T:     &fakeTransport{replies: diagnosticsReplies()},
		Sink:  sink,
		Clock: &fakeClock{},
		Mode:  func() error { return errReplay },
	}
	if err := session.Run(); err != errReplay {
		t.Fatalf("err = %v, want errReplay", err)
	}
	if sink.start != "" || sink.end != "" {
		t.Fatalf("unexpected output: start=%q end=%q", sink.start, sink.end)
	}
}
