package main

import (
	"machine"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var hostBuf [512]byte
var hostN int

func serialWriter() {
	var buf [256]byte
	for {
		if !hostAlive() {
			// Discard status lines while no host is listening.
			for logRing.read(buf[:]) > 0 {
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		n := logRing.read(buf[:])
		if n == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		_, _ = machine.Serial.Write(buf[:n])
	}
}

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
		line := strings.TrimSpace(string(hostBuf[:i]))
		handleHostLine(line)
		copy(hostBuf[:], hostBuf[i+1:hostN])
		hostN -= i + 1
	}
}

func handleHostLine(line string) {
	if line == "" {
		return
	}
	switch {
	case line == "A":
		if serialReady {
			touchHost()
		}
	case strings.HasPrefix(line, "HELLO,"):
		parts := strings.SplitN(line, ",", 4)
		if len(parts) != 4 || parts[1] != "1" {
			emit("ERR," + strconv.FormatInt(nowMs(), 10) + ",PROTO," + line)
			return
		}
		sessionID = parts[2]
		serialReady = true
		touchHost()
		bleWant = true
		emit("HELLO_ACK," + protocolVersionString() + "," + parts[3])
		emitState()
	case strings.HasPrefix(line, "HB,"):
		if !serialReady {
			return
		}
		touchHost()
		seq := strings.TrimPrefix(line, "HB,")
		emit("HB_ACK," + seq + "," + strconv.FormatInt(nowMs(), 10))
	case strings.HasPrefix(line, "START,"):
		if !serialReady || bleState != bleOn {
			emit("ERR," + strconv.FormatInt(nowMs(), 10) + ",START,NO_BLE")
			return
		}
		recording = true
		touchHost()
		emit("START_ACK," + strings.TrimPrefix(line, "START,"))
	case strings.HasPrefix(line, "STOP,"):
		recording = false
		touchHost()
		emit("STOP_ACK," + strings.TrimPrefix(line, "STOP,"))
	case strings.HasPrefix(line, "BYE,"):
		recording = false
		serialReady = false
		bleWant = false
		closeBLE()
		emit("BYE_ACK," + strings.TrimPrefix(line, "BYE,"))
	case line == "C,MEM":
		printMem()
	case strings.HasPrefix(line, "C,"):
		if !serialReady || bleState != bleOn {
			emit("ERR," + strconv.FormatInt(nowMs(), 10) + ",CMD,NO_BLE")
			return
		}
		cmd := strings.TrimPrefix(line, "C,")
		if cmd == "" {
			return
		}
		if err := sendBLE(cmd); err != nil {
			emit("ERR," + strconv.FormatInt(nowMs(), 10) + ",CMD,WRITE")
			return
		}
		atomic.AddUint32(&txCount, 1)
		atomic.StoreInt64(&lastCmdNano, time.Now().UnixNano())
		emit("Q," + strconv.FormatInt(nowMs(), 10) + "," + cmd)
	}
}

func protocolVersionString() string {
	if protocolVersion == 1 {
		return "1"
	}
	return strconv.Itoa(protocolVersion)
}
