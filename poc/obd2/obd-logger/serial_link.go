package main

import (
	"machine"
	"strings"
	"sync/atomic"
	"time"

	"m5stick-obd2-cnx/internal/obdwire"
)

type Mode uint8

const (
	ModeLive Mode = iota
	ModeReplay
)

var appMode = ModeLive

type SerialLink struct {
	buf     []byte
	invalid int
}

var serialLink = &SerialLink{}

func (s *SerialLink) Poll() {
	n := machine.Serial.Buffered()
	if n > 0 {
		var tmp [64]byte
		if n > len(tmp) {
			n = len(tmp)
		}
		m, _ := machine.Serial.Read(tmp[:n])
		s.buf = append(s.buf, tmp[:m]...)
		if len(s.buf) > 2048 {
			s.buf = s.buf[:0]
			s.invalid++
		}
	}

	for {
		i := -1
		for j, b := range s.buf {
			if b == '\n' {
				i = j
				break
			}
		}
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(s.buf[:i]))
		s.buf = s.buf[i+1:]
		if line == "" {
			continue
		}
		cmd, err := obdwire.ParseCommand(line)
		if err != nil {
			s.invalid++
			continue
		}
		switch cmd.Kind {
		case obdwire.KindAck:
			atomic.StoreInt64(&serialAckNano, time.Now().UnixNano())
		case obdwire.KindReplay:
			appMode = ModeReplay
			uiPaused = false
			bleState = bleOff
			elmState = elmWait
		case obdwire.KindLive:
			appMode = ModeLive
		case obdwire.KindClear:
			resetReplayValues()
		case obdwire.KindRecord:
			applyReplayRecord(cmd.Record)
		}
	}
}
