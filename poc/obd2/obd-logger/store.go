//go:build tinygo

package main

import (
	"time"

	"m5stick-obd2-cnx/internal/obdline"
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
	applyReading(reading, source)
	return true
}

// decodeScratch keeps the decoder input out of the heap: passing a slice that
// points into a stack record can make the record escape on every sample.
var decodeScratch [8]byte

// ingestBytes is the replay path: the PID arrives as two ASCII bytes and is
// never converted to a string.
func ingestBytes(pid [2]byte, raw [obdline.MaxRawBytes]byte, rawLen uint8, source obdpid.Source) bool {
	n := int(rawLen)
	if n > len(decodeScratch) {
		n = len(decodeScratch)
	}
	for i := 0; i < n; i++ {
		decodeScratch[i] = raw[i]
	}
	reading, err := pidRegistry.DecodeBytes(pid, decodeScratch[:n])
	if err != nil {
		readingStore.PutQualityBytes(pid, obdpid.QualityInvalid)
		return false
	}
	applyReading(reading, source)
	return true
}

func applyReading(reading obdpid.Reading, source obdpid.Source) {
	reading.Source = source
	readingStore.Put(reading)
	markPIDDirty(reading.PID)
	samples++

	// reading.PID is the registry's canonical string, so these comparisons do
	// not allocate.
	if reading.PID == "0D" {
		now := time.Now()
		if !tripLast.IsZero() {
			tripKm += reading.Value * now.Sub(tripLast).Hours()
		}
		tripLast = now
	}
	if reading.PID == "2F" && tripFuelStart < 0 {
		tripFuelStart = reading.Value
	}
}
