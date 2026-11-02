package main

import (
	"time"

	"m5stick-obd2-cnx/internal/obdpid"
	"m5stick-obd2-cnx/internal/obdstore"
)

var readingStore = obdstore.NewCache()

func ingest(pid string, raw []byte, source obdpid.Source) bool {
	reading, err := pidRegistry.Decode(pid, raw)
	if err != nil {
		readingStore.PutQuality(pid, obdpid.QualityInvalid)
		return false
	}
	reading.Source = source
	readingStore.Put(reading)
	markPIDDirty(pid)
	samples++

	if pid == "0D" {
		now := time.Now()
		if !tripLast.IsZero() {
			tripKm += reading.Value * now.Sub(tripLast).Hours()
		}
		tripLast = now
	}
	if pid == "2F" && tripFuelStart < 0 {
		tripFuelStart = reading.Value
	}
	return true
}
