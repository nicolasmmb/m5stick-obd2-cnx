package main

import (
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
	dev   bluetooth.Device
	write bluetooth.DeviceCharacteristic
}

var (
	currentLink    *bleLink
	bleInitialized bool
)

func sendBLE(cmd string) error {
	if currentLink == nil {
		return errNoBLE
	}
	_, err := currentLink.write.WriteWithoutResponse([]byte(cmd + "\r"))
	return err
}

func closeBLE() {
	if currentLink != nil {
		_ = currentLink.dev.Disconnect()
		currentLink = nil
	}
}

func openBLE() (*bleLink, error) {
	if !bleInitialized {
		if err := adapter.Enable(); err != nil {
			return nil, err
		}
		adapter.SetScanType(bluetooth.ScanTypeActive)
		bleInitialized = true
	}
	adapter.StopScan()
	time.Sleep(150 * time.Millisecond)

	var addr bluetooth.Address
	var foundName string
	found := false
	scan := func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
		if found {
			return
		}
		name := r.LocalName()
		if looksLikeELM(name) || (name != "" && r.HasServiceUUID(uuidFFF0)) {
			addr = r.Address
			foundName = name
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
	println("simple: found", addr.String(), foundName)

	time.Sleep(150 * time.Millisecond)
	println("simple: connecting")
	dev, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, err
	}
	println("simple: connected")
	println("simple: discovering")
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
	println("simple: notify")
	if err := chars[0].EnableNotifications(func(buf []byte) {
		rxMu.Lock()
		rx = append(rx, buf...)
		rxMu.Unlock()
	}); err != nil {
		_ = dev.Disconnect()
		return nil, err
	}
	println("simple: ready")
	return link, nil
}

func looksLikeELM(s string) bool {
	for _, token := range []string{"obd", "elm", "vgate", "icar"} {
		if containsFold(s, token) {
			return true
		}
	}
	return false
}

func containsFold(s, token string) bool {
	if len(token) == 0 || len(s) < len(token) {
		return false
	}
	for i := 0; i+len(token) <= len(s); i++ {
		ok := true
		for j := 0; j < len(token); j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != token[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

var (
	errNoBLE    = &simpleError{"no ble link"}
	errNoDevice = &simpleError{"no obd device"}
)
