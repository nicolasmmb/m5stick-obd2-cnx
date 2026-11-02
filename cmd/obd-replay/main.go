package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.bug.st/serial"

	"m5stick-obd2-cnx/internal/obdlog"
	"m5stick-obd2-cnx/internal/obdwire"
)

type source struct {
	path   string
	file   *os.File
	scan   *bufio.Scanner
	lastMs int64
	first  bool
}

func openSource(path string) (*source, error) {
	s := &source{path: path}
	if err := s.Reset(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *source) Reset() error {
	if s.file != nil {
		_ = s.file.Close()
	}
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	s.file = f
	s.scan = bufio.NewScanner(f)
	s.scan.Buffer(make([]byte, 64*1024), 1024*1024)
	s.lastMs = 0
	s.first = true
	return nil
}

func (s *source) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

func (s *source) Next() (obdwire.Record, time.Duration, error) {
	for s.scan.Scan() {
		line := strings.TrimSpace(s.scan.Text())
		if line == "" {
			continue
		}
		rec, err := parseLine(line)
		if err != nil {
			continue
		}
		// Events and X are not rendered by the Stick and their timestamps in
		// old logs can be unreliable. Replay only I/P records.
		if rec.Type == obdwire.RecordEvent || rec.Type == obdwire.RecordEnd {
			continue
		}
		var wait time.Duration
		if !s.first && rec.Ms > s.lastMs {
			wait = time.Duration(rec.Ms-s.lastMs) * time.Millisecond
		}
		s.first = false
		if rec.Ms > s.lastMs {
			s.lastMs = rec.Ms
		}
		return rec, wait, nil
	}
	if err := s.scan.Err(); err != nil {
		return obdwire.Record{}, 0, err
	}
	return obdwire.Record{}, 0, io.EOF
}

func parseLine(line string) (obdwire.Record, error) {
	if strings.HasPrefix(line, "{") {
		rec, err := obdlog.ParseAnyLine([]byte(line))
		if err != nil {
			return obdwire.Record{}, err
		}
		return fromLogRecord(rec)
	}
	return obdwire.ParseRecord(line)
}

func fromLogRecord(rec obdlog.Record) (obdwire.Record, error) {
	switch rec.Type {
	case "S":
		payload := "I,0," + rec.Start.FW + "," + rec.Start.VIN + "," + fmt.Sprint(rec.Start.TankDL)
		return obdwire.Record{Type: obdwire.RecordStart, Payload: payload}, nil
	case "P":
		s := rec.Sample
		payload := fmt.Sprintf("P,%d,%s,%s,%d,%s", s.Ms, s.PID, s.Raw, s.Latency, s.Quality)
		return obdwire.Record{Type: obdwire.RecordPID, Ms: s.Ms, PID: s.PID, Raw: s.Raw, Latency: s.Latency, Quality: s.Quality, Payload: payload}, nil
	case "E":
		e := rec.Event
		payload := fmt.Sprintf("E,%d,%s,%s", e.Ms, e.Type, e.Msg)
		return obdwire.Record{Type: obdwire.RecordEvent, Ms: e.Ms, Payload: payload}, nil
	case "X":
		x := rec.End
		payload := fmt.Sprintf("X,%d,%s", x.Ms, x.Reason)
		return obdwire.Record{Type: obdwire.RecordEnd, Ms: x.Ms, Payload: payload}, nil
	}
	return obdwire.Record{}, fmt.Errorf("unsupported record %s", rec.Type)
}

type sink interface {
	EnterReplay() error
	Ack() error
	Send(obdwire.Record) error
	ExitReplay() error
	Close() error
}

type drySink struct{ verbose bool }

func (d drySink) EnterReplay() error { log.Println("C,REPLAY"); return nil }
func (d drySink) Ack() error         { return nil }
func (d drySink) Send(r obdwire.Record) error {
	log.Println("R," + obdwire.FormatRecord(r))
	return nil
}
func (d drySink) ExitReplay() error { log.Println("C,LIVE"); return nil }
func (d drySink) Close() error      { return nil }

type serialSink struct {
	port serial.Port
}

func openSerialSink(portName string) (*serialSink, error) {
	p, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return nil, err
	}
	return &serialSink{port: p}, nil
}

func (s *serialSink) write(line string) error {
	_, err := s.port.Write([]byte(line + "\n"))
	return err
}

func (s *serialSink) EnterReplay() error {
	log.Println("C,REPLAY")
	return s.write("C,REPLAY")
}
func (s *serialSink) Ack() error { return s.write("A") }
func (s *serialSink) Send(r obdwire.Record) error {
	line := "R," + obdwire.FormatRecord(r)
	log.Println(line)
	return s.write(line)
}
func (s *serialSink) ExitReplay() error {
	log.Println("C,LIVE")
	if err := s.write("C,LIVE"); err != nil {
		return err
	}
	return s.port.Drain()
}
func (s *serialSink) Close() error { return s.port.Close() }

func (s *serialSink) echo() {
	_ = s.port.SetReadTimeout(200 * time.Millisecond)
	buf := make([]byte, 512)
	for {
		n, err := s.port.Read(buf)
		if n > 0 {
			_, _ = os.Stdout.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func main() {
	var (
		portName = flag.String("port", "", "serial port")
		file     = flag.String("file", "", "raw.log or decoded.ndjson")
		speed    = flag.Float64("speed", 1, "replay speed; 0 = no wait")
		loop     = flag.Bool("loop", false, "repeat the file")
		startMs  = flag.Int64("start", -1, "start at device ms")
		endMs    = flag.Int64("end", -1, "stop at device ms")
		dryRun   = flag.Bool("dry-run", false, "do not open serial")
		verbose  = flag.Bool("verbose", false, "print every record")
		echo     = flag.Bool("echo", false, "echo firmware serial output")
	)
	flag.Parse()
	if *file == "" {
		log.Fatal("--file is required")
	}
	if !*dryRun && *portName == "" {
		log.Fatal("--port is required unless --dry-run")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	src, err := openSource(*file)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()

	var out sink
	if *dryRun {
		out = drySink{verbose: *verbose}
	} else {
		s, err := openSerialSink(*portName)
		if err != nil {
			log.Fatal(err)
		}
		out = s
		if *echo {
			go s.echo()
		}
	}
	defer out.Close()
	defer out.ExitReplay()

	if err := out.EnterReplay(); err != nil {
		log.Fatal(err)
	}

	lastAck := time.Time{}
	for {
		rec, wait, err := src.Next()
		if err == io.EOF {
			if *loop {
				_ = src.Reset()
				continue
			}
			return
		}
		if err != nil {
			log.Fatal(err)
		}
		if *startMs >= 0 && rec.Ms < *startMs {
			continue
		}
		if *endMs >= 0 && rec.Ms > *endMs {
			return
		}

		if *speed > 0 {
			w := time.Duration(float64(wait) / *speed)
			if w > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(w):
				}
			}
		}
		if time.Since(lastAck) >= time.Second {
			if err := out.Ack(); err != nil {
				log.Fatal(err)
			}
			lastAck = time.Now()
		}
		if err := out.Send(rec); err != nil {
			log.Fatal(err)
		}
		if *verbose {
			log.Printf("  ms=%d wait=%s", rec.Ms, wait)
		}
	}
}
