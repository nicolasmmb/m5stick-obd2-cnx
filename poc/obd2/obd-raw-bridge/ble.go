package main

import (
	"runtime"
	"strconv"
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
			runtime.GC()
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
			if lowerASCII(s[i+j]) != token[j] {
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

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

func openBLE() (*bleLink, error) {
	adapter.StopScan()
	time.Sleep(150 * time.Millisecond)

	var addr bluetooth.Address
	var foundName string
	found := false
	scan := func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
		if !bleWant {
			adapter.StopScan()
			return
		}
		if found {
			return
		}
		name := r.LocalName()
		// Empty-name devices advertising FFF0 are often beacons, not ELM327.
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
	emit("E," + strconv.FormatInt(nowMs(), 10) + ",FOUND," + addr.String() + "," + foundName)

	time.Sleep(200 * time.Millisecond)
	emit("E," + strconv.FormatInt(nowMs(), 10) + ",CONNECTING," + addr.String())
	dev, err := adapter.Connect(addr, bluetooth.ConnectionParams{
		ConnectionTimeout: bluetooth.NewDuration(5 * time.Second),
		Timeout:           bluetooth.NewDuration(5 * time.Second),
	})
	if err != nil {
		emit("E," + strconv.FormatInt(nowMs(), 10) + ",CONNECT_FAIL")
		return nil, err
	}
	emit("E," + strconv.FormatInt(nowMs(), 10) + ",CONNECTED," + addr.String())
	time.Sleep(1 * time.Second)
	emit("E," + strconv.FormatInt(nowMs(), 10) + ",DISCOVER")
	services, err := dev.DiscoverServices([]bluetooth.UUID{uuidFFF0})
	if err != nil {
		emit("E," + strconv.FormatInt(nowMs(), 10) + ",SERVICE_FAIL")
		_ = dev.Disconnect()
		return nil, err
	}
	if len(services) == 0 {
		emit("E," + strconv.FormatInt(nowMs(), 10) + ",SERVICE_EMPTY")
		_ = dev.Disconnect()
		return nil, errNoDevice
	}
	emit("E," + strconv.FormatInt(nowMs(), 10) + ",SERVICES")
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{uuidFFF1, uuidFFF2})
	if err != nil {
		emit("E," + strconv.FormatInt(nowMs(), 10) + ",CHAR_FAIL")
		_ = dev.Disconnect()
		return nil, err
	}
	if len(chars) < 2 {
		emit("E," + strconv.FormatInt(nowMs(), 10) + ",CHAR_EMPTY")
		_ = dev.Disconnect()
		return nil, errNoDevice
	}
	emit("E," + strconv.FormatInt(nowMs(), 10) + ",CHARS")
	link := &bleLink{dev: dev, write: chars[1]}
	adapter.StopScan()
	time.Sleep(500 * time.Millisecond)
	emit("E," + strconv.FormatInt(nowMs(), 10) + ",NOTIFY")
	if err := chars[0].EnableNotifications(func(buf []byte) {
		rxRing.write(buf)
	}); err != nil {
		emit("E," + strconv.FormatInt(nowMs(), 10) + ",NOTIFY_FAIL")
		_ = dev.Disconnect()
		return nil, err
	}
	emit("E," + strconv.FormatInt(nowMs(), 10) + ",READY")
	return link, nil
}

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

var (
	errNoBLE    = &simpleError{"no ble link"}
	errNoDevice = &simpleError{"no obd device"}
)
