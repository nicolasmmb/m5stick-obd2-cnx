package main

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	adapter  = bluetooth.DefaultAdapter
	uuidFFF0 = bluetooth.New16BitUUID(0xFFF0)
	uuidFFF1 = bluetooth.New16BitUUID(0xFFF1)
	uuidFFF2 = bluetooth.New16BitUUID(0xFFF2)
)

type bleLink struct {
	dev    bluetooth.Device
	write  bluetooth.DeviceCharacteristic
	failed uint32
}

var (
	linkMu      sync.Mutex
	currentLink *bleLink
)

func sendBLE(cmd string) error {
	linkMu.Lock()
	l := currentLink
	linkMu.Unlock()
	if l == nil {
		return errNoBLE
	}
	if _, err := l.write.WriteWithoutResponse([]byte(cmd + "\r")); err != nil {
		atomic.StoreUint32(&l.failed, 1)
		return err
	}
	return nil
}

func closeBLE() {
	linkMu.Lock()
	l := currentLink
	currentLink = nil
	linkMu.Unlock()
	if l != nil {
		_ = l.dev.Disconnect()
	}
}

func bleLoop() {
	for {
		if err := adapter.Enable(); err == nil {
			break
		}
		bleState = bleOff
		time.Sleep(time.Second)
	}
	adapter.SetScanType(bluetooth.ScanTypeActive)

	backoff := time.Second
	for {
		if !bleWant {
			bleState = bleOff
			time.Sleep(100 * time.Millisecond)
			continue
		}
		bleState = bleScan
		emitState()
		l, err := openBLE()
		if err != nil {
			emit("ERR," + strconv.FormatInt(nowMs(), 10) + ",BLE," + err.Error())
			time.Sleep(backoff)
			if backoff < 10*time.Second {
				backoff *= 2
			}
			continue
		}
		linkMu.Lock()
		currentLink = l
		linkMu.Unlock()
		bleState = bleOn
		backoff = time.Second
		emitState()

		for bleWant && atomic.LoadUint32(&l.failed) == 0 {
			time.Sleep(100 * time.Millisecond)
		}
		linkMu.Lock()
		if currentLink == l {
			currentLink = nil
		}
		linkMu.Unlock()
		_ = l.dev.Disconnect()
		if bleWant {
			bleState = bleScan
			emit("X," + strconv.FormatInt(nowMs(), 10) + ",ble_lost")
			emitState()
			time.Sleep(backoff)
		}
	}
}

func openBLE() (*bleLink, error) {
	adapter.StopScan()
	time.Sleep(150 * time.Millisecond)

	var addr bluetooth.Address
	found := false
	scan := func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
		if !bleWant {
			adapter.StopScan()
			return
		}
		if found {
			return
		}
		if r.HasServiceUUID(uuidFFF0) || strings.Contains(strings.ToLower(r.LocalName()), "obd") {
			addr = r.Address
			found = true
			adapter.StopScan()
		}
	}
	timer := time.AfterFunc(10*time.Second, func() { adapter.StopScan() })
	err := adapter.Scan(scan)
	timer.Stop()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errNoDevice
	}

	dev, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, err
	}
	services, err := dev.DiscoverServices([]bluetooth.UUID{uuidFFF0})
	if err != nil {
		_ = dev.Disconnect()
		return nil, err
	}
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{uuidFFF1, uuidFFF2})
	if err != nil {
		_ = dev.Disconnect()
		return nil, err
	}
	link := &bleLink{dev: dev, write: chars[1]}
	if err := chars[0].EnableNotifications(func(buf []byte) {
		rxRing.write(buf)
	}); err != nil {
		_ = dev.Disconnect()
		return nil, err
	}
	return link, nil
}

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

var (
	errNoBLE    = &simpleError{"no ble link"}
	errNoDevice = &simpleError{"no obd device"}
)
