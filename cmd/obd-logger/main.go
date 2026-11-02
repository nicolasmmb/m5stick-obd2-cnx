package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.bug.st/serial"

	"m5stick-obd2-cnx/internal/obdlog"
)

func main() {
	var (
		portName = flag.String("port", "", "serial port (ex.: /dev/cu.usbserial-XXXX)")
		baud     = flag.Int("baud", 115200, "serial baud rate")
		outDir   = flag.String("out", "data", "output directory")
		session  = flag.String("session", "", "session id (default: timestamp)")
		replay   = flag.String("replay", "", "replay a NDJSON file instead of serial")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loader, err := obdlog.NewLoader(obdlog.Options{
		OutDir:    *outDir,
		SessionID: *session,
		Port:      *portName,
		Baud:      *baud,
		Replay:    *replay != "",
	})
	if err != nil {
		log.Fatalf("create loader: %v", err)
	}

	if *replay != "" {
		err = runReplay(ctx, loader, *replay)
	} else {
		if *portName == "" {
			_ = loader.Close()
			log.Fatal("--port is required unless --replay is used")
		}
		err = runSerial(ctx, loader, *portName, *baud)
	}

	if closeErr := loader.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	log.Printf("session saved to %s", loader.Dir())
}

func runReplay(ctx context.Context, loader *obdlog.Loader, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		if err := loader.ProcessLine(scanner.Bytes(), time.Now()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func runSerial(ctx context.Context, loader *obdlog.Loader, portName string, baud int) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		port, err := serial.Open(portName, &serial.Mode{BaudRate: baud})
		if err != nil {
			log.Printf("open %s: %v; retrying in 1s", portName, err)
			if !sleepContext(ctx, time.Second) {
				return nil
			}
			continue
		}
		log.Printf("serial open: %s @ %d", portName, baud)

		err = readPort(ctx, port, loader)
		_ = port.Close()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			log.Printf("serial read: %v; reconnecting", err)
		}
		if !sleepContext(ctx, time.Second) {
			return nil
		}
	}
}

func readPort(ctx context.Context, port serial.Port, loader *obdlog.Loader) error {
	if err := port.SetReadTimeout(200 * time.Millisecond); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	var pending []byte
	var lastAck time.Time

	for {
		if ctx.Err() != nil {
			return nil
		}
		// Tell the firmware that a host is listening. The firmware shows
		// SERIAL=OK while these ACKs keep arriving.
		if time.Since(lastAck) >= time.Second {
			_, _ = port.Write([]byte("A\n"))
			lastAck = time.Now()
		}
		n, err := port.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			for {
				i := bytes.IndexByte(pending, '\n')
				if i < 0 {
					break
				}
				line := pending[:i]
				pending = pending[i+1:]
				if err := loader.ProcessLine(line, time.Now()); err != nil {
					return err
				}
			}
			if len(pending) > 1024*1024 {
				pending = pending[:0]
			}
		}
		if err != nil {
			return err
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
