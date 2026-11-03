package main

import (
	"machine"
	"strconv"
	"strings"
	"sync"
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
	bootTime     time.Time
	lastHostNano int64
	serialReady  bool
	recording    bool
	bleState     int
	elmState     int
	txCount      uint32
	lastRxNano   int64
	lastCmdNano  int64

	rxMu sync.Mutex
	rx   []byte
)

func nowMs() int64 { return time.Since(bootTime).Milliseconds() }

func hostAlive() bool {
	ns := atomic.LoadInt64(&lastHostNano)
	return ns != 0 && time.Since(time.Unix(0, ns)) < 3*time.Second
}

func touchHost() { atomic.StoreInt64(&lastHostNano, time.Now().UnixNano()) }

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	// Keep the panel dark until BLE is fully configured.
	bl := machine.GPIO27
	bl.Configure(machine.PinConfig{Mode: machine.PinOutput})
	bl.Low()

	bootTime = time.Now()

	// 1. Serial handshake first. No UI and no BLE before this.
	for !serialReady {
		serialPoll()
		time.Sleep(10 * time.Millisecond)
	}
	println("simple: serial ready")

	// 2. BLE setup in the main goroutine, exactly like the proven logger.
	backoff := time.Second
	for {
		bleState = bleScan
		link, err := openBLE()
		if err == nil {
			currentLink = link
			bleState = bleOn
			println("simple: ble ready")
			break
		}
		println("simple: ble error:", err.Error())
		time.Sleep(backoff)
		if backoff < 10*time.Second {
			backoff *= 2
		}
	}

	// 3. Only now the UI is initialized.
	initDisplay()
	go uiLoop()
	emitState()

	// 4. Command/serial loop.
	lastState := time.Time{}
	for {
		serialPoll()
		drainRX()
		updateState()
		if time.Since(lastState) >= time.Second {
			lastState = time.Now()
			emitState()
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func updateState() {
	if serialReady && !hostAlive() {
		serialReady = false
		recording = false
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

func drainRX() {
	rxMu.Lock()
	if len(rx) == 0 {
		rxMu.Unlock()
		return
	}
	data := append([]byte(nil), rx...)
	rx = rx[:0]
	rxMu.Unlock()
	atomic.StoreInt64(&lastRxNano, time.Now().UnixNano())
	elmState = elmRX
	if hostAlive() {
		println("R," + strconv.FormatInt(nowMs(), 10) + "," + hex(data))
	}
}

func emitState() {
	ser := 0
	if hostAlive() {
		ser = 1
	}
	println("S," + strconv.FormatInt(nowMs(), 10) + ",SER," + strconv.Itoa(ser))
	ble := "OFF"
	switch bleState {
	case bleScan:
		ble = "SCAN"
	case bleOn:
		ble = "ON"
	}
	println("S," + strconv.FormatInt(nowMs(), 10) + ",BLE," + ble)
	elm := "WAIT"
	switch elmState {
	case elmRX:
		elm = "RX"
	case elmTimeout:
		elm = "TIMEOUT"
	}
	println("S," + strconv.FormatInt(nowMs(), 10) + ",ELM," + elm)
}

func hex(p []byte) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, len(p)*2)
	for _, b := range p {
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out)
}

var hostBuf [512]byte
var hostN int

func serialPoll() {
	for {
		space := len(hostBuf) - hostN
		if space == 0 {
			hostN = 0
			break
		}
		n := machine.Serial.Buffered()
		if n <= 0 {
			break
		}
		if n > space {
			n = space
		}
		m, _ := machine.Serial.Read(hostBuf[hostN : hostN+n])
		if m <= 0 {
			break
		}
		hostN += m
	}
	for {
		i := -1
		for j := 0; j < hostN; j++ {
			if hostBuf[j] == '\n' {
				i = j
				break
			}
		}
		if i < 0 {
			break
		}
		handleHostLine(strings.TrimSpace(string(hostBuf[:i])))
		copy(hostBuf[:], hostBuf[i+1:hostN])
		hostN -= i + 1
	}
}

func handleHostLine(line string) {
	switch {
	case line == "A":
		if serialReady {
			touchHost()
		}
	case strings.HasPrefix(line, "HELLO,"):
		parts := strings.SplitN(line, ",", 4)
		if len(parts) != 4 || parts[1] != "1" {
			println("ERR," + strconv.FormatInt(nowMs(), 10) + ",PROTO")
			return
		}
		serialReady = true
		touchHost()
		println("HELLO_ACK,1," + parts[3])
	case strings.HasPrefix(line, "HB,"):
		if serialReady {
			touchHost()
			println("HB_ACK," + strings.TrimPrefix(line, "HB,") + "," + strconv.FormatInt(nowMs(), 10))
		}
	case strings.HasPrefix(line, "START,"):
		if !serialReady || bleState != bleOn {
			println("ERR," + strconv.FormatInt(nowMs(), 10) + ",START,NO_BLE")
			return
		}
		recording = true
		touchHost()
		println("START_ACK," + strings.TrimPrefix(line, "START,"))
	case strings.HasPrefix(line, "STOP,"):
		recording = false
		touchHost()
		println("STOP_ACK," + strings.TrimPrefix(line, "STOP,"))
	case strings.HasPrefix(line, "BYE,"):
		recording = false
		serialReady = false
		closeBLE()
		println("BYE_ACK," + strings.TrimPrefix(line, "BYE,"))
	case line == "C,MEM":
		printMem()
	case strings.HasPrefix(line, "C,"):
		if !serialReady || bleState != bleOn {
			println("ERR," + strconv.FormatInt(nowMs(), 10) + ",CMD,NO_BLE")
			return
		}
		cmd := strings.TrimPrefix(line, "C,")
		if cmd == "" {
			return
		}
		if err := sendBLE(cmd); err != nil {
			println("ERR," + strconv.FormatInt(nowMs(), 10) + ",CMD,WRITE")
			return
		}
		atomic.AddUint32(&txCount, 1)
		atomic.StoreInt64(&lastCmdNano, time.Now().UnixNano())
		println("Q," + strconv.FormatInt(nowMs(), 10) + "," + cmd)
	}
}
