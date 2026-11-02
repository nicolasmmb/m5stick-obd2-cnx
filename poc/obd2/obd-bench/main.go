package main

import (
	"fmt"
	"machine"
	"sort"
	"strconv"
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

type timing struct {
	pid  string
	min  time.Duration
	avg  time.Duration
	max  time.Duration
	data string
	ok   bool
}

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()
	time.Sleep(time.Second)

	println("obd-bench: start")
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
	notifyChar := chars[0]
	writeChar := chars[1]
	if err := notifyChar.EnableNotifications(func(buf []byte) {
		mu.Lock()
		rx = append(rx, buf...)
		mu.Unlock()
	}); err != nil {
		println("notify failed:", err.Error())
		return
	}
	e := &elm{write: writeChar}

	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		resp, d, err := e.command(cmd, 3*time.Second)
		println(cmd, "->", strings.ReplaceAll(strings.TrimSpace(resp), "\r", "|"), "in", ms(d), "ms")
		if err != nil {
			println("  err:", err.Error())
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Enumerate all Mode 01 supported-PID pages.
	var supported []int
	base := 0
	for base <= 0xE0 {
		cmd := fmt.Sprintf("01%02X", base)
		resp, d, err := e.command(cmd, 2*time.Second)
		println(cmd, "->", strings.ReplaceAll(strings.TrimSpace(resp), "\r", "|"), "in", ms(d), "ms")
		if err != nil {
			break
		}
		b, ok := pidData(resp, cmd[2:])
		if !ok || len(b) < 4 {
			break
		}
		for i := 0; i < 32; i++ {
			if b[i/8]&(1<<(7-uint(i%8))) != 0 {
				supported = append(supported, base+1+i)
			}
		}
		// The last supported PID indicates whether another page exists.
		if b[3]&0x01 == 0 {
			break
		}
		base += 0x20
	}

	sort.Ints(supported)
	println("=== MODE 01 SUPPORTED PIDS ===")
	var names []string
	for _, pid := range supported {
		names = append(names, fmt.Sprintf("%02X", pid))
	}
	println("count", len(supported), ":", strings.Join(names, " "))

	// Benchmark every supported PID three times.
	println("=== MODE 01 BENCH (3 samples each) ===")
	var total time.Duration
	var count int
	var results []timing
	for _, pid := range supported {
		cmd := fmt.Sprintf("01%02X", pid)
		t := timing{pid: cmd}
		for i := 0; i < 3; i++ {
			resp, d, err := e.command(cmd, 1500*time.Millisecond)
			if err != nil || strings.Contains(resp, "NO DATA") {
				t.ok = false
				break
			}
			if i == 0 || d < t.min {
				t.min = d
			}
			if i == 0 || d > t.max {
				t.max = d
			}
			t.avg += d
			t.data = strings.ReplaceAll(strings.TrimSpace(resp), "\r", "|")
			t.ok = true
		}
		if t.ok {
			t.avg /= 3
			total += t.avg
			count++
			println(cmd, "min", ms(t.min), "avg", ms(t.avg), "max", ms(t.max), "ms", t.data)
		} else {
			println(cmd, "NO DATA / ERROR")
		}
		results = append(results, t)
	}
	if count > 0 {
		avg := total / time.Duration(count)
		println("=== FULL MODE 01 SCAN ===")
		println("PIDs:", count, "avg per PID:", ms(avg), "ms")
		println("full scan:", ms(total), "ms ->", fmt.Sprintf("%.1f Hz", float64(time.Second)/float64(total)))
		println("single PID max rate:", fmt.Sprintf("%.1f Hz", float64(time.Second)/float64(avg)))
	}

	// Just RPM repeated, which is the realistic fast gauge path.
	println("=== RPM POLL (20x 010C) ===")
	start := time.Now()
	okCount := 0
	for i := 0; i < 20; i++ {
		resp, _, err := e.command("010C", 1500*time.Millisecond)
		if err == nil && !strings.Contains(resp, "NO DATA") {
			okCount++
		}
	}
	d := time.Since(start)
	println("20x 010C in", ms(d), "ms ->", fmt.Sprintf("%.1f Hz", 20/float64(d.Seconds())), "ok", okCount)

	// Multi-PID requests are not standard, but some clones accept them.
	println("=== MULTI-PID PROBE ===")
	for _, cmd := range []string{"010C0D", "010C 010D", "010C,010D"} {
		resp, d, err := e.command(cmd, 2*time.Second)
		println(cmd, "->", strings.ReplaceAll(strings.TrimSpace(resp), "\r", "|"), "in", ms(d), "ms err", err)
	}

	// Other modes carry the slow/static data: DTCs, VIN, ECU name, tests.
	println("=== OTHER MODES ===")
	extra := []string{
		"0200", "0202", "020C",
		"03", "07", "0A",
		"0600", "0601",
		"0900", "0902", "0904", "0906", "090A",
	}
	for _, cmd := range extra {
		resp, d, err := e.command(cmd, 6*time.Second)
		s := strings.ReplaceAll(strings.TrimSpace(resp), "\r", "|")
		if len(s) > 180 {
			s = s[:180] + "..."
		}
		println(cmd, "->", s, "in", ms(d), "ms err", err)
	}

	println("obd-bench: done")
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

func pidData(resp, pid string) ([]byte, bool) {
	s := strings.ToUpper(resp)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ">", "")
	i := strings.Index(s, "41"+pid)
	if i < 0 {
		return nil, false
	}
	h := s[i+4:]
	var out []byte
	for len(h) >= 2 {
		v, err := strconv.ParseUint(h[:2], 16, 8)
		if err != nil {
			break
		}
		out = append(out, byte(v))
		h = h[2:]
	}
	return out, len(out) > 0
}
