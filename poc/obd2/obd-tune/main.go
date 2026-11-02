package main

import (
	"fmt"
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

type elm struct {
	write bluetooth.DeviceCharacteristic
}

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()
	time.Sleep(time.Second)
	println("obd-tune: start")

	if err := adapter.Enable(); err != nil {
		println("enable failed:", err.Error())
		return
	}
	adapter.SetScanType(bluetooth.ScanTypeActive)

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
			adapter.StopScan()
		}
	})
	if err != nil {
		println("scan failed:", err.Error())
	}
	<-done
	if !found {
		println("no adapter")
		return
	}
	dev, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		println("connect failed:", err.Error())
		return
	}
	services, err := dev.DiscoverServices([]bluetooth.UUID{uuidFFF0})
	if err != nil {
		println("service failed:", err.Error())
		return
	}
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{uuidFFF1, uuidFFF2})
	if err != nil {
		println("char failed:", err.Error())
		return
	}
	if err := chars[0].EnableNotifications(func(buf []byte) {
		mu.Lock()
		rx = append(rx, buf...)
		mu.Unlock()
	}); err != nil {
		println("notify failed:", err.Error())
		return
	}
	e := &elm{write: chars[1]}

	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		resp, d, _ := e.command(cmd, 3*time.Second)
		println(cmd, "->", strings.ReplaceAll(strings.TrimSpace(resp), "\r", "|"), "in", ms(d), "ms")
		time.Sleep(100 * time.Millisecond)
	}

	configs := []struct {
		name string
		cmds []string
	}{
		{"default", nil},
		{"ATAT0", []string{"ATAT0"}},
		{"ATAT2", []string{"ATAT2"}},
		{"ATST0A", []string{"ATST0A"}},
		{"ATST14", []string{"ATST14"}},
		{"ATST20", []string{"ATST20"}},
		{"ATST32", []string{"ATST32"}},
		{"ATAT0+ATST0A", []string{"ATAT0", "ATST0A"}},
	}

	for _, cfg := range configs {
		for _, c := range cfg.cmds {
			resp, _, err := e.command(c, 2*time.Second)
			if err != nil {
				println(c, "failed:", err.Error())
			} else {
				println(c, "->", strings.ReplaceAll(strings.TrimSpace(resp), "\r", "|"))
			}
		}
		var min, max, total time.Duration
		ok := 0
		for i := 0; i < 10; i++ {
			resp, d, err := e.command("010C", 1500*time.Millisecond)
			if err != nil || strings.Contains(resp, "NO DATA") || !strings.Contains(resp, "410C") {
				continue
			}
			if ok == 0 || d < min {
				min = d
			}
			if ok == 0 || d > max {
				max = d
			}
			total += d
			ok++
		}
		avg := time.Duration(0)
		hz := 0.0
		if ok > 0 {
			avg = total / time.Duration(ok)
			hz = float64(time.Second) / float64(avg)
		}
		println("CFG", cfg.name, "ok", ok, "/10 min", ms(min), "avg", ms(avg), "max", ms(max), "ms ->", fmt.Sprintf("%.1f Hz", hz))
	}

	println("obd-tune: done")
}

func ms(d time.Duration) int64 { return int64(d / time.Millisecond) }

func (e *elm) command(cmd string, timeout time.Duration) (string, time.Duration, error) {
	mu.Lock()
	rx = rx[:0]
	mu.Unlock()
	start := time.Now()
	if _, err := e.write.WriteWithoutResponse([]byte(cmd + "\r")); err != nil {
		return "", 0, err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		s := string(rx)
		mu.Unlock()
		if strings.Contains(s, ">") {
			return s, time.Since(start), nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	s := string(rx)
	mu.Unlock()
	return s, time.Since(start), fmt.Errorf("timeout")
}
