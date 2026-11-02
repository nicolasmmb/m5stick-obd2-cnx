package main

import (
	"machine"
	"time"
)

var errReplay = &simpleError{"replay mode"}

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	initDisplay()
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
		if appMode == ModeReplay {
			uiPaused = false
			time.Sleep(200 * time.Millisecond)
			continue
		}
		err := runLive()
		uiPaused = false
		bleStopScan()
		if err != nil && err != errReplay {
			println("obd-logger:", err.Error())
		}
		if appMode == ModeReplay {
			continue
		}
		if bleState == bleConnected {
			elmState = elmErr
		} else {
			bleState = bleScan
			elmState = elmWait
		}
		time.Sleep(5 * time.Second)
	}
}
