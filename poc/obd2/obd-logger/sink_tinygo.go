//go:build tinygo

package main

import (
	"strconv"
	"sync/atomic"

	"m5stick-obd2-cnx/internal/obdpid"
)

// sessionOpen guarantees that only one X is emitted per I, even when the
// serial handler closes the session while the loop is still in a command.
var sessionOpen int32

type liveSink struct{}

func (liveSink) Start(ms int64, vin string) {
	txLines++
	println("I," + strconv.FormatInt(ms, 10) + ",0.1," + vin + ",440")
	atomic.StoreInt32(&sessionOpen, 1)
}

func (liveSink) Event(ms int64, typ, detail string) {
	txLines++
	println("E," + strconv.FormatInt(ms, 10) + "," + typ + "," + detail)
}

func (liveSink) PID(ms int64, pid string, raw []byte, latencyMs int64) {
	ingest(pid, raw, obdpid.SourceLive)
	txLines++
	println("P," + strconv.FormatInt(ms, 10) + "," + pid + "," + hexBytes(raw) +
		"," + strconv.FormatInt(latencyMs, 10) + ",OK")
}

func (liveSink) End(ms int64, reason string) { emitEndAt(ms, reason) }

func (liveSink) Quality(pid string, q obdpid.Quality) {
	readingStore.PutQuality(pid, q)
	publishQuality(pid, q)
}

func (liveSink) MIL(on bool) { milOn = on }

func (liveSink) DTCCount(count int) {
	dtcCount = count
	markTile(4, 1)
	markTile(4, 2)
}

func (liveSink) HandshakeOK() {
	elmState = elmOK
	reconnectAttempt = 0
}

func emitEnd(reason string) { emitEndAt(liveMs(), reason) }

func emitEndAt(ms int64, reason string) {
	if atomic.SwapInt32(&sessionOpen, 0) == 0 {
		return
	}
	txLines++
	println("X," + strconv.FormatInt(ms, 10) + "," + reason)
}
