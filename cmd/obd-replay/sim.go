package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"go.bug.st/serial"

	"m5stick-obd2-cnx/internal/obdwire"
)

type simOptions struct {
	portName  string
	file      string
	echo      bool
	reset     bool
	verbose   bool
	dropAfter int
	dropFor   time.Duration
	mil       bool
	readiness string
	dtcs      []string
}

// simSnapshot is the last value seen for each PID in the replay file. The
// session loop is what gets validated; the simulated car can stay static.
type simSnapshot struct {
	vin string
	raw map[string]string
}

func loadSnapshot(path string) (*simSnapshot, error) {
	src, err := openSource(path)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	snap := &simSnapshot{raw: make(map[string]string)}
	for {
		rec, _, err := src.Next()
		if err == io.EOF {
			return snap, nil
		}
		if err != nil {
			return nil, err
		}
		switch rec.Type {
		case obdwire.RecordStart:
			parts := strings.SplitN(rec.Payload, ",", 5)
			if len(parts) >= 5 {
				snap.vin = strings.TrimSpace(parts[3])
			}
		case obdwire.RecordPID:
			snap.raw[rec.PID] = rec.Raw
		}
	}
}

func (s *simSnapshot) answer(cmd string, opts simOptions) string {
	switch cmd {
	case "ATZ":
		return "ELM327 v2.1"
	case "ATE0", "ATL0", "ATS0", "ATSP0":
		return "OK"
	case "0902":
		return "490201" + strings.ToUpper(hex.EncodeToString([]byte(s.vin))) + "||>"
	case "0101":
		a := byte(len(opts.dtcs))
		if opts.mil {
			a |= 0x80
		}
		readiness := strings.ToUpper(strings.TrimSpace(opts.readiness))
		if len(readiness) != 6 {
			readiness = "07E100"
		}
		return fmt.Sprintf("4101%02X%sAA||>", a, readiness)
	case "03":
		return dtcResponse("43", opts.dtcs)
	case "07":
		return dtcResponse("47", nil)
	case "0A":
		return dtcResponse("4A", nil)
	}
	if strings.HasPrefix(cmd, "01") && len(cmd) >= 4 {
		pid := cmd[2:4]
		if raw, ok := s.raw[pid]; ok {
			return "41" + pid + raw + "AA||>"
		}
	}
	return "NO DATA||>"
}

func dtcResponse(header string, codes []string) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString(fmt.Sprintf("%02X", len(codes)))
	for _, code := range codes {
		a, c, ok := dtcBytes(code)
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("%02X%02X", a, c))
	}
	b.WriteString("AA||>")
	return b.String()
}

func dtcBytes(code string) (byte, byte, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 5 {
		return 0, 0, false
	}
	var typ byte
	switch code[0] {
	case 'P':
		typ = 0
	case 'C':
		typ = 1
	case 'B':
		typ = 2
	case 'U':
		typ = 3
	default:
		return 0, 0, false
	}
	digit := func(i int) (byte, bool) {
		c := code[i]
		switch {
		case c >= '0' && c <= '9':
			return c - '0', true
		case c >= 'A' && c <= 'F':
			return c - 'A' + 10, true
		}
		return 0, false
	}
	d1, ok1 := digit(1)
	d2, ok2 := digit(2)
	d3, ok3 := digit(3)
	d4, ok4 := digit(4)
	if !ok1 || !ok2 || !ok3 || !ok4 || d1 > 3 {
		return 0, 0, false
	}
	return typ<<6 | d1<<4 | d2, d3<<4 | d4, true
}

type simSink struct {
	port    serial.Port
	queries chan string
	ack     chan struct{}
	echo    bool
}

func openSimSink(portName string, echo, reset bool) (*simSink, error) {
	p, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return nil, err
	}
	if reset {
		_ = p.SetDTR(false)
		_ = p.SetRTS(true)
		time.Sleep(150 * time.Millisecond)
		_ = p.SetRTS(false)
		time.Sleep(50 * time.Millisecond)
		_ = p.SetDTR(false)
	}
	s := &simSink{port: p, queries: make(chan string, 32), echo: echo, ack: make(chan struct{}, 1)}
	go s.readLoop()
	return s, nil
}

func (s *simSink) write(line string) error {
	_, err := s.port.Write([]byte(line + "\n"))
	return err
}

func (s *simSink) EnterSim() error {
	for attempt := 0; attempt < 20; attempt++ {
		log.Println("C,SIM")
		if err := s.write("C,SIM"); err != nil {
			return err
		}
		select {
		case <-s.ack:
			return nil
		case <-time.After(300 * time.Millisecond):
		}
	}
	return fmt.Errorf("firmware did not acknowledge C,SIM")
}

func (s *simSink) Ack() { _ = s.write("A") }

func (s *simSink) Stop() {
	_ = s.write("C,STOP")
	time.Sleep(200 * time.Millisecond)
}

func (s *simSink) Close() error { return s.port.Close() }

func (s *simSink) readLoop() {
	_ = s.port.SetReadTimeout(200 * time.Millisecond)
	buf := make([]byte, 512)
	var line []byte
	for {
		n, err := s.port.Read(buf)
		if n > 0 {
			if s.echo {
				_, _ = os.Stdout.Write(buf[:n])
			}
			for _, b := range buf[:n] {
				if b == '\n' || b == '\r' {
					s.handleLine(string(line))
					line = line[:0]
					continue
				}
				if len(line) < 1024 {
					line = append(line, b)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *simSink) handleLine(line string) {
	switch {
	case line == "K,SIM":
		select {
		case s.ack <- struct{}{}:
		default:
		}
	case line == "K,STOP", strings.HasPrefix(line, "X,"):
		log.Println(line)
	case strings.HasPrefix(line, "Q,"):
		select {
		case s.queries <- line[2:]:
		default:
		}
	}
}

func runSim(ctx context.Context, opts simOptions) error {
	snap, err := loadSnapshot(opts.file)
	if err != nil {
		return err
	}
	sink, err := openSimSink(opts.portName, opts.echo, opts.reset)
	if err != nil {
		return err
	}
	defer sink.Close()
	if err := sink.EnterSim(); err != nil {
		return err
	}
	defer sink.Stop()

	log.Printf("sim: %d PIDs, VIN=%q, drop-after=%d", len(snap.raw), snap.vin, opts.dropAfter)

	answered := 0
	var silentUntil time.Time
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			sink.Ack()
		case cmd := <-sink.queries:
			now := time.Now()
			if !silentUntil.IsZero() {
				if now.Before(silentUntil) {
					continue
				}
				silentUntil = time.Time{}
				answered = 0
			}
			if opts.dropAfter > 0 && answered >= opts.dropAfter {
				silentUntil = now.Add(opts.dropFor)
				log.Printf("sim: dropping responses for %s", opts.dropFor)
				continue
			}
			resp := snap.answer(cmd, opts)
			if opts.verbose {
				log.Printf("  %s -> %s", cmd, resp)
			}
			if err := sink.write("S," + resp); err != nil {
				return err
			}
			answered++
		}
	}
}

func parseDTCList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.ToUpper(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
