//go:build tinygo

package main

import (
	"time"

	"m5stick-obd2-cnx/internal/obdline"
	"m5stick-obd2-cnx/internal/obdpid"
)

var pidRegistry = obdpid.DefaultRegistry()

func applyReplayRecord(rec obdline.Record) {
	ingestBytes(rec.PID, rec.Raw, rec.RawLen, obdpid.SourceReplay)
}

func resetReplayValues() {
	readingStore.Clear()
	snapshots.Clear()
	dtcCount = -1
	milOn = false
	tripKm = 0
	tripFuelStart = -1
	tripLast = time.Now()
}
