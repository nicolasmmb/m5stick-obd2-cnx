//go:build tinygo

package main

import (
	"machine"
	"time"
)

// reconnectAttempt drives the backoff. It is reset once the ELM handshake
// succeeds.
var reconnectAttempt int

// noDeviceLogged avoids filling the serial TX buffer with the same scan error
// when nobody is reading the USB port.
var noDeviceLogged bool

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
	sessionEpoch = bootTime
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
			time.Sleep(200 * time.Millisecond)
			continue
		case ModeSim:
			runSimLoop()
			continue
		}
		err := runLive()
		bleStopScan()
		switch {
		case err == errNoDevice:
			if !noDeviceLogged {
				println("obd-logger:", err.Error())
				noDeviceLogged = true
			}
		case err != nil && err != errReplay && err != errStop:
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

func runLive() error {
	switch appMode {
	case ModeReplay:
		return errReplay
	case ModePaused, ModeSim:
		return errStop
	}
	link, err := openBLE()
	if err != nil {
		return err
	}
	defer link.Close()
	println("obd-logger: session start")
	return (&Session{
		T:     link,
		Sink:  liveSink{},
		Clock: realClock{},
		Mode:  liveModeError,
	}).Run()
}

// runSimLoop reuses the exact session loop against the serial simulator.
func runSimLoop() {
	for appMode == ModeSim {
		bleState = bleOff
		elmState = elmWait
		err := (&Session{
			T:     simTransport{},
			Sink:  liveSink{},
			Clock: realClock{},
			Mode:  simModeError,
		}).Run()
		if appMode != ModeSim {
			return
		}
		if err != nil && err != errStop && err != errReplay {
			println("obd-logger:", err.Error())
		}
		reconnectAttempt++
		waitBackoff(reconnectDelay(reconnectAttempt))
	}
}

func liveModeError() error {
	switch appMode {
	case ModeReplay:
		return errReplay
	case ModePaused, ModeSim:
		return errStop
	default:
		return nil
	}
}

func simModeError() error {
	if appMode != ModeSim {
		return errStop
	}
	return nil
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
