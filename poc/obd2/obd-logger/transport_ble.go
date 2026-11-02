//go:build tinygo

package main

import (
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

var (
	uuidFFF0 = bluetooth.New16BitUUID(0xFFF0)
	uuidFFF1 = bluetooth.New16BitUUID(0xFFF1)
	uuidFFF2 = bluetooth.New16BitUUID(0xFFF2)
)

var bleInitialized bool

// bleLink is the only file that knows about the BLE adapter.
type bleLink struct {
	dev   bluetooth.Device
	write bluetooth.DeviceCharacteristic

	mu sync.Mutex
	rx []byte
}

func openBLE() (*bleLink, error) {
	if !bleInitialized {
		if err := adapter.Enable(); err != nil {
			return nil, err
		}
		adapter.SetScanType(bluetooth.ScanTypeActive)
		bleInitialized = true
	}
	// A previous scan can still be active after replay or an error.
	adapter.StopScan()
	time.Sleep(150 * time.Millisecond)
	bleState = bleScan
	elmState = elmWait
	var addr bluetooth.Address
	found := false
	scan := func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
		if appMode != ModeLive {
			adapter.StopScan()
			return
		}
		if found {
			return
		}
		if r.HasServiceUUID(uuidFFF0) || strings.Contains(strings.ToLower(r.LocalName()), "obd") {
			addr = r.Address
			found = true
			bleState = bleConnecting
			println("obd-logger: found", r.Address.String(), r.LocalName())
			adapter.StopScan()
		}
	}
	// Keep the scan bounded even when no advertisement arrives, and retry when
	// a previous scan is still winding down.
	var scanErr error
	for attempt := 0; attempt < 10; attempt++ {
		timer := time.AfterFunc(10*time.Second, func() { adapter.StopScan() })
		scanErr = adapter.Scan(scan)
		timer.Stop()
		if scanErr == nil {
			break
		}
		_ = adapter.StopScan()
		time.Sleep(300 * time.Millisecond)
	}
	if scanErr != nil {
		return nil, scanErr
	}
	if !found {
		return nil, &simpleError{"no OBDII BLE device found"}
	}

	time.Sleep(150 * time.Millisecond)
	// Pause the full-screen UI only during Connect, where it could compete
	// with the BLE controller.
	uiPaused = true
	// If Connect blocks longer than expected, let the UI resume so the
	// display does not stay frozen on LINK forever.
	go func() {
		time.Sleep(3 * time.Second)
		uiPaused = false
	}()
	dev, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, err
	}
	bleState = bleConnected
	elmState = elmInit
	uiPaused = false
	dirtyAll = true
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
		link.mu.Lock()
		link.rx = append(link.rx, buf...)
		link.mu.Unlock()
	}); err != nil {
		_ = dev.Disconnect()
		return nil, err
	}
	return link, nil
}

func (b *bleLink) Command(cmd string, timeout time.Duration) (string, Reply) {
	b.mu.Lock()
	b.rx = b.rx[:0]
	b.mu.Unlock()

	if _, err := b.write.WriteWithoutResponse([]byte(cmd + "\r")); err != nil {
		return "", ReplyWrite
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		s := string(b.rx)
		b.mu.Unlock()
		if strings.Contains(s, ">") {
			return s, ReplyOK
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", ReplyTimeout
}

func (b *bleLink) Close() error { return b.dev.Disconnect() }

func bleStopScan() {
	// Calling StopScan before Enable can confuse the HCI layer.
	if bleInitialized {
		_ = adapter.StopScan()
	}
}
