//go:build tinygo

package main

import (
	"machine"
	"sync/atomic"
	"time"

	"m5stick-obd2-cnx/internal/obdline"
)

type Mode uint8

const (
	ModeLive Mode = iota
	ModeReplay
	ModePaused
	ModeSim
)

var appMode = ModeLive

type SerialLink struct {
	buf     [2048]byte
	n       int
	invalid int
}

var serialLink = &SerialLink{}

// Poll drains the serial port into a fixed buffer and applies every complete
// line. The buffer is reused, so replay does not allocate per record.
func (s *SerialLink) Poll() {
	for {
		space := len(s.buf) - s.n
		if space == 0 {
			s.n = 0
			s.invalid++
			break
		}
		n := machine.Serial.Buffered()
		if n <= 0 {
			break
		}
		if n > space {
			n = space
		}
		m, _ := machine.Serial.Read(s.buf[s.n : s.n+n])
		if m <= 0 {
			break
		}
		s.n += m
	}

	for {
		i := -1
		for j := 0; j < s.n; j++ {
			if s.buf[j] == '\n' {
				i = j
				break
			}
		}
		if i < 0 {
			break
		}
		line := s.buf[:i]
		if len(line) >= 2 && line[0] == 'S' && line[1] == ',' {
			// Simulated ELM response. The slice points into s.buf, which is
			// still valid here; simReply copies it.
			simReply(string(line[2:]))
		} else if l, ok := obdline.Parse(line); ok {
			s.handle(l)
		} else {
			s.invalid++
		}
		copy(s.buf[:], s.buf[i+1:s.n])
		s.n -= i + 1
	}
}

func (s *SerialLink) handle(line obdline.Line) {
	switch line.Kind {
	case obdline.KindAck:
		atomic.StoreInt64(&serialAckNano, time.Now().UnixNano())
	case obdline.KindReplay:
		bleStopScan()
		emitEnd("replay")
		appMode = ModeReplay
		resetReplayValues()
		page = 1
		dirtyAll = true
		bleState = bleOff
		elmState = elmWait
		println("K,REPLAY")
	case obdline.KindLive:
		bleStopScan()
		appMode = ModeLive
		dirtyAll = true
		println("K,LIVE")
	case obdline.KindStop:
		bleStopScan()
		emitEnd("host")
		appMode = ModePaused
		dirtyAll = true
		bleState = bleOff
		elmState = elmWait
		println("K,STOP")
	case obdline.KindSim:
		bleStopScan()
		emitEnd("sim")
		appMode = ModeSim
		resetReplayValues()
		page = 1
		dirtyAll = true
		bleState = bleOff
		elmState = elmWait
		println("K,SIM")
	case obdline.KindClear:
		resetReplayValues()
	case obdline.KindRecord:
		applyReplayRecord(line.Rec)
	}
}
