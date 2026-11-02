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
	ingest(rec.PID, raw, obdpid.SourceReplay)
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
	readingStore.Clear()
	dtcCount = -1
	milOn = false
	tripKm = 0
	tripFuelStart = -1
	tripLast = time.Now()
}
