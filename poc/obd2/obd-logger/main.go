package main

import (
	"machine"
	"time"
)

var (
	errReplay   = &simpleError{"replay mode"}
	errStop     = &simpleError{"host stop"}
	errLinkLost = &simpleError{"ble link lost"}
)

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

// liveMs is the monotonic clock of a live session. It never resets on
// reconnects, so the host can keep resampling the wide CSV.
func liveMs() int64 { return time.Since(bootTime).Milliseconds() }

// reconnectAttempt drives the backoff. It is reset once the ELM handshake
// succeeds.
var reconnectAttempt int

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	initDisplay()
	initBuzzer()
	initButtons()
	dtcCount = -1
	tripFuelStart = -1
	tripLast = time.Now()
	bootTime = time.Now()
	started = bootTime
	dirtyAll = true
	go uiLoop()

	println("obd-logger: starting")
	// Give the host a moment to request replay before the BLE controller is
	// powered up. Enabling BLE reserves a fixed arena that cannot be freed.
	graceEnd := time.Now().Add(3 * time.Second)
	for appMode == ModeLive && time.Now().Before(graceEnd) {
		time.Sleep(50 * time.Millisecond)
	}

	for {
		switch appMode {
		case ModeReplay, ModePaused:
			uiPaused = false
			time.Sleep(200 * time.Millisecond)
			continue
		}
		err := runLive()
		uiPaused = false
		bleStopScan()
		if err != nil && err != errReplay && err != errStop {
			println("obd-logger:", err.Error())
		}
		if appMode != ModeLive {
			continue
		}
		if bleState == bleConnected {
			elmState = elmErr
		} else {
			bleState = bleScan
			elmState = elmWait
		}
		reconnectAttempt++
		waitBackoff(reconnectDelay(reconnectAttempt))
	}
}

func reconnectDelay(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return 1 * time.Second
	case attempt == 2:
		return 2 * time.Second
	case attempt == 3:
		return 5 * time.Second
	default:
		return 10 * time.Second
	}
}
