//go:build tinygo

package main

import (
	"time"

	"m5stick-obd2-cnx/internal/obdline"
	"m5stick-obd2-cnx/internal/obdpid"
	"m5stick-obd2-cnx/internal/obdsnapshot"
	"m5stick-obd2-cnx/internal/obdstore"
)

var (
	readingStore = obdstore.NewCache()
	snapshots    = obdsnapshot.New()
)

// publishReading makes one decoded reading visible to the UI as part of a
// consistent snapshot. The 1 KB copy per sample is negligible (~10-20/s).
func publishReading(r obdpid.Reading) {
	if len(r.PID) < 2 {
		return
	}
	at := uint32(0)
	if !r.At.IsZero() {
		at = uint32(r.At.Sub(sessionEpoch) / time.Millisecond)
	}
	key := uint16(r.PID[0])<<8 | uint16(r.PID[1])
	snap := snapshots.Begin()
	snap.Put(key, r.Value, r.Quality, at)
	snapshots.Publish(at)
}

func publishQuality(pid string, q obdpid.Quality) {
	r, ok := readingStore.Get(pid)
	if !ok {
		return
	}
	r.PID = pid
	r.Quality = q
	publishReading(r)
}

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
	publishReading(reading)
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
