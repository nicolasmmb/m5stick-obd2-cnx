package main

import (
	"machine"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

const protocolVersion = 1

const (
	bleOff = iota
	bleScan
	bleOn
)

const (
	elmWait = iota
	elmRX
	elmTimeout
)

var (
	bootTime = time.Now()

	lastHostNano int64
	serialReady  bool
	sessionID    string
	recording    bool
	bleWant      bool
	bleState     int
	elmState     int
	txCount      uint32
	lastRxNano   int64
	lastCmdNano  int64
)

var (
	rxRing  byteRing
	logRing byteRing
	lastGC  time.Time
)

func nowMs() int64 {
	return time.Since(bootTime).Milliseconds()
}

func hostAlive() bool {
	ns := atomic.LoadInt64(&lastHostNano)
	return ns != 0 && time.Since(time.Unix(0, ns)) < 3*time.Second
}

func touchHost() {
	atomic.StoreInt64(&lastHostNano, time.Now().UnixNano())
}

func emit(line string) {
	logRing.write([]byte(line))
	logRing.write([]byte{'\n'})
}

func emitState() {
	ser := 0
	if hostAlive() {
		ser = 1
	}
	emit("S," + strconv.FormatInt(nowMs(), 10) + ",SER," + strconv.Itoa(ser))
	ble := "OFF"
	switch bleState {
	case bleScan:
		ble = "SCAN"
	case bleOn:
		ble = "ON"
	}
	emit("S," + strconv.FormatInt(nowMs(), 10) + ",BLE," + ble)
	elm := "WAIT"
	switch elmState {
	case elmRX:
		elm = "RX"
	case elmTimeout:
		elm = "TIMEOUT"
	}
	emit("S," + strconv.FormatInt(nowMs(), 10) + ",ELM," + elm)
}

func drainRX() {
	var buf [128]byte
	for {
		n := rxRing.read(buf[:])
		if n == 0 {
			return
		}
		atomic.StoreInt64(&lastRxNano, time.Now().UnixNano())
		elmState = elmRX
		if hostAlive() {
			emitR(buf[:n])
		}
	}
}

func updateState() {
	if serialReady && !hostAlive() {
		serialReady = false
		recording = false
		bleWant = false
		closeBLE()
	}
	rx := atomic.LoadInt64(&lastRxNano)
	cmd := atomic.LoadInt64(&lastCmdNano)
	if rx != 0 && time.Since(time.Unix(0, rx)) < 2*time.Second {
		elmState = elmRX
	} else if cmd != 0 && time.Since(time.Unix(0, cmd)) < 2*time.Second {
		elmState = elmTimeout
	} else {
		elmState = elmWait
	}
}

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	initDisplay()
	go serialWriter()
	go bleLoop()
	go uiLoop()

	lastState := time.Time{}
	for {
		serialPoll()
		drainRX()
		updateState()
		if time.Since(lastState) >= time.Second {
			lastState = time.Now()
			emitState()
		}
		if time.Since(lastGC) >= 5*time.Second {
			lastGC = time.Now()
			runtime.GC()
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func appendInt(dst []byte, v int64) []byte {
	if v == 0 {
		return append(dst, '0')
	}
	var tmp [20]byte
	n := len(tmp)
	for v > 0 {
		n--
		tmp[n] = byte('0' + v%10)
		v /= 10
	}
	return append(dst, tmp[n:]...)
}

func emitR(p []byte) {
	const digits = "0123456789ABCDEF"
	var buf [1024]byte
	n := 0
	n += copy(buf[n:], "R,")
	n = len(appendInt(buf[:n], nowMs()))
	buf[n] = ','
	n++
	for _, b := range p {
		if n+2 > len(buf)-1 {
			break
		}
		buf[n] = digits[b>>4]
		buf[n+1] = digits[b&0x0F]
		n += 2
	}
	buf[n] = '\n'
	n++
	logRing.write(buf[:n])
}
