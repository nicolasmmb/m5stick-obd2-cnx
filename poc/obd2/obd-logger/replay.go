package main

import (
	"strconv"
	"strings"
	"time"

	"m5stick-obd2-cnx/internal/obdpid"
	"m5stick-obd2-cnx/internal/obdwire"
)

var pidRegistry = obdpid.DefaultRegistry()

func applyReplayRecord(rec obdwire.Record) {
	if rec.Type != obdwire.RecordPID {
		return
	}
	raw, ok := decodeHex(rec.Raw)
	if !ok {
		return
	}
	reading, err := pidRegistry.Decode(rec.PID, raw)
	if err != nil || reading.Quality != obdpid.QualityOK {
		return
	}
	setLatest(rec.PID, reading.Value)
	samples++
}

func decodeHex(s string) ([]byte, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 2 || len(s)%2 != 0 {
		return nil, false
	}
	out := make([]byte, 0, len(s)/2)
	for len(s) >= 2 {
		v, err := strconv.ParseUint(s[:2], 16, 8)
		if err != nil {
			return nil, false
		}
		out = append(out, byte(v))
		s = s[2:]
	}
	return out, true
}

func resetReplayValues() {
	latestFuel = -1
	latestCoolant = -1
	latestIntake = -1
	latestCat = -1
	latestLoad = -1
	latestThrottle = -1
	latestLambda = -1
	latestVoltage = -1
	dtcCount = -1
	tripKm = 0
	tripFuelStart = -1
	tripLast = time.Now()
}
