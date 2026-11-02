package main

import (
	"machine"
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

var (
	mu sync.Mutex
	rx []byte
)

func main() {
	// Keep the board powered when running from battery.
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	time.Sleep(time.Second)
	println("obd-client: starting")
	if err := adapter.Enable(); err != nil {
		println("enable failed:", err.Error())
		return
	}
	adapter.SetScanType(bluetooth.ScanTypeActive)

	println("obd-client: scanning for OBDII...")
	var addr bluetooth.Address
	found := false
	done := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Second)
		adapter.StopScan()
		close(done)
	}()
	err := adapter.Scan(func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
		if found {
			return
		}
		if r.HasServiceUUID(uuidFFF0) || strings.Contains(strings.ToLower(r.LocalName()), "obd") {
			addr = r.Address
			found = true
			println("obd-client: found", r.Address.String(), r.LocalName())
			adapter.StopScan()
		}
	})
	if err != nil {
		println("scan failed:", err.Error())
	}
	<-done
	if !found {
		println("obd-client: no OBDII BLE device found")
		return
	}

	println("obd-client: connecting...")
	dev, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		println("connect failed:", err.Error())
		return
	}
	println("obd-client: connected")

	services, err := dev.DiscoverServices([]bluetooth.UUID{uuidFFF0})
	if err != nil {
		println("service discovery failed:", err.Error())
		return
	}
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{uuidFFF1, uuidFFF2})
	if err != nil {
		println("characteristic discovery failed:", err.Error())
		return
	}
	notifyChar := chars[0] // FFF1
	writeChar := chars[1]  // FFF2

	if err := notifyChar.EnableNotifications(func(buf []byte) {
		mu.Lock()
		rx = append(rx, buf...)
		mu.Unlock()
	}); err != nil {
		println("enable notifications failed:", err.Error())
		return
	}
	println("obd-client: notifications enabled")

	send := func(cmd string, timeout time.Duration) string {
		mu.Lock()
		rx = rx[:0]
		mu.Unlock()
		println("TX:", cmd)
		if _, err := writeChar.WriteWithoutResponse([]byte(cmd + "\r")); err != nil {
			println("write failed:", err.Error())
			return ""
		}
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			mu.Lock()
			s := string(rx)
			mu.Unlock()
			if strings.Contains(s, ">") {
				return s
			}
			time.Sleep(20 * time.Millisecond)
		}
		mu.Lock()
		s := string(rx)
		mu.Unlock()
		return s
	}

	println("--- ELM327 handshake ---")
	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		resp := send(cmd, 3*time.Second)
		println("RX:", strings.ReplaceAll(strings.TrimSpace(resp), "\r", "\\r"))
		time.Sleep(200 * time.Millisecond)
	}

	pid := send("0100", 3*time.Second)
	println("RX 0100:", strings.ReplaceAll(strings.TrimSpace(pid), "\r", "\\r"))

	println("--- live data ---")
	for {
		for _, cmd := range []string{"010C", "010D", "0105"} {
			resp := send(cmd, 2*time.Second)
			println(cmd, "=>", strings.ReplaceAll(strings.TrimSpace(resp), "\r", "\\r"))
		}
		time.Sleep(500 * time.Millisecond)
	}
}
