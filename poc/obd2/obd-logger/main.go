package main

import (
	"machine"
	"strconv"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"

	"m5stick-obd2-cnx/internal/obdpid"
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

type pidSpec struct {
	cmd  string
	pid  string
	unit string
	kind byte
}

const (
	kRPM byte = iota
	kSpeed
	kTemp
	kPct
	kMAP
	kMAF
	kVoltage
	kCatalyst
	kLambda
	kTrim
	kBaro
)

var fastPIDs = []pidSpec{
	{"010C", "0C", "rpm", kRPM},
	{"010D", "0D", "km/h", kSpeed},
	{"0111", "11", "%", kPct},
	{"0104", "04", "%", kPct},
	{"010B", "0B", "kPa", kMAP},
	{"0110", "10", "g/s", kMAF},
	{"0144", "44", "lambda", kLambda},
	{"0106", "06", "%", kTrim},
	{"0107", "07", "%", kTrim},
}

var slowPIDs = []pidSpec{
	{"0105", "05", "C", kTemp},
	{"010F", "0F", "C", kTemp},
	{"012F", "2F", "%", kPct},
	{"013C", "3C", "C", kCatalyst},
	{"0142", "42", "V", kVoltage},
	{"0152", "52", "%", kPct},
	{"0146", "46", "C", kTemp},
	{"0145", "45", "%", kPct},
	{"0149", "49", "%", kPct},
	{"014A", "4A", "%", kPct},
	{"014C", "4C", "%", kPct},
	{"0133", "33", "kPa", kBaro},
}

var errReplay = &simpleError{"replay mode"}

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
	go uiLoop()

	time.Sleep(time.Second)
	println("obd-logger: starting")
	if err := adapter.Enable(); err != nil {
		println("enable failed:", err.Error())
		for {
			time.Sleep(time.Second)
		}
	}
	adapter.SetScanType(bluetooth.ScanTypeActive)

	for {
		if appMode == ModeReplay {
			uiPaused = false
			time.Sleep(200 * time.Millisecond)
			continue
		}
		err := runSession()
		uiPaused = false
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

func debugStep(s string) {
	ms := int64(0)
	if !started.IsZero() {
		ms = int64(time.Since(started) / time.Millisecond)
	}
	println("E," + strconv.FormatInt(ms, 10) + ",DEBUG," + s)
}

func runSession() error {
	if appMode == ModeReplay {
		return errReplay
	}
	bleState = bleScan
	elmState = elmWait
	var addr bluetooth.Address
	found := false
	done := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Second)
		adapter.StopScan()
		close(done)
	}()
	err := adapter.Scan(func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
		if appMode == ModeReplay {
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
	})
	if err != nil {
		<-done
		return err
	}
	<-done
	if appMode == ModeReplay {
		return errReplay
	}
	if !found {
		debugStep("no_device")
		return &simpleError{"no OBDII BLE device found"}
	}
	debugStep("found")

	time.Sleep(150 * time.Millisecond)
	// Pause the full-screen UI only during Connect, where it could compete
	// with the BLE controller.
	debugStep("connecting")
	uiPaused = true
	// If Connect blocks longer than expected, let the UI resume so the
	// display does not stay frozen on LINK forever.
	go func() {
		time.Sleep(3 * time.Second)
		uiPaused = false
	}()
	dev, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		debugStep("connect_error")
		return err
	}
	bleState = bleConnected
	elmState = elmInit
	uiPaused = false
	debugStep("connected")
	debugStep("services")
	services, err := dev.DiscoverServices([]bluetooth.UUID{uuidFFF0})
	if err != nil {
		debugStep("services_error")
		return err
	}
	debugStep("chars")
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{uuidFFF1, uuidFFF2})
	if err != nil {
		debugStep("chars_error")
		return err
	}
	notifyChar := chars[0]
	writeChar := chars[1]
	debugStep("notify")
	if err := notifyChar.EnableNotifications(func(buf []byte) {
		mu.Lock()
		rx = append(rx, buf...)
		mu.Unlock()
	}); err != nil {
		debugStep("notify_error")
		return err
	}
	elmState = elmInit
	e := &elm{write: writeChar}
	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		if _, err := e.command(cmd, 3*time.Second); err != nil {
			debugStep("handshake_error_" + cmd)
			return &simpleError{"handshake " + cmd + ": " + err.Error()}
		}
		time.Sleep(100 * time.Millisecond)
	}
	elmState = elmOK
	debugStep("handshake_ok")
	readDiagnostics(e)

	vin := readVIN(e)
	start := time.Now()
	cycle := 0
	emitStart(vin)

	for {
		if appMode == ModeReplay {
			return errReplay
		}
		for _, p := range fastPIDs {
			if appMode == ModeReplay {
				return errReplay
			}
			if !emitPID(e, p, start) {
				time.Sleep(200 * time.Millisecond)
			}
		}
		cycle++
		if cycle%4 == 0 {
			for _, p := range slowPIDs {
				if appMode == ModeReplay {
					return errReplay
				}
				emitPID(e, p, start)
				time.Sleep(20 * time.Millisecond)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func emitStart(vin string) {
	txLines++
	println("I,0,0.1," + vin + ",440")
}

func emitPID(e *elm, p pidSpec, start time.Time) bool {
	t0 := time.Now()
	resp, err := e.command(p.cmd, 2*time.Second)
	lat := int64(time.Since(t0) / time.Millisecond)
	ms := int64(time.Since(start) / time.Millisecond)

	if err != nil || strings.Contains(resp, "NO DATA") {
		errCount++
		txLines++
		readingStore.PutQuality(p.pid, obdpid.QualityNoData)
		println("E," + strconv.FormatInt(ms, 10) + ",NO_DATA," + p.pid)
		return false
	}

	data, ok := pidData(resp, p.pid)
	if !ok {
		errCount++
		txLines++
		readingStore.PutQuality(p.pid, obdpid.QualityInvalid)
		println("E," + strconv.FormatInt(ms, 10) + ",PARSE_ERROR," + p.pid)
		return false
	}

	ingest(p.pid, data, obdpid.SourceLive)
	txLines++
	println("P," + strconv.FormatInt(ms, 10) + "," + p.pid + "," + hexBytes(data) +
		"," + strconv.FormatInt(lat, 10) + ",OK")
	return true
}

func readDiagnostics(e *elm) {
	// MIL from PID 01, byte A bit 7.
	if resp, err := e.command("0101", 2*time.Second); err == nil {
		if d, ok := pidData(resp, "01"); ok && len(d) > 0 {
			milOn = d[0]&0x80 != 0
		}
	}

	// Stored DTCs from Mode 03. Response starts with 43, not 41.
	resp, err := e.command("03", 3*time.Second)
	if err != nil {
		return
	}
	s := strings.ToUpper(resp)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ">", "")
	i := strings.Index(s, "43")
	if i < 0 {
		return
	}
	h := s[i+2:]
	count := 0
	for len(h) >= 4 {
		b1, err1 := strconv.ParseUint(h[:2], 16, 8)
		b2, err2 := strconv.ParseUint(h[2:4], 16, 8)
		if err1 != nil || err2 != nil {
			break
		}
		if (b1 == 0 && b2 == 0) || (b1 == 0xAA && b2 == 0xAA) {
			break
		}
		count++
		h = h[4:]
	}
	dtcCount = count
}

func (e *elm) command(cmd string, timeout time.Duration) (string, error) {
	mu.Lock()
	rx = rx[:0]
	mu.Unlock()

	if _, err := e.write.WriteWithoutResponse([]byte(cmd + "\r")); err != nil {
		return "", err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		s := string(rx)
		mu.Unlock()
		if strings.Contains(s, ">") {
			return s, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", &simpleError{"timeout: " + cmd}
}

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

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

func hexBytes(d []byte) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, len(d)*2)
	for _, b := range d {
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out)
}

func readVIN(e *elm) string {
	resp, err := e.command("0902", 4*time.Second)
	if err != nil {
		return ""
	}
	s := strings.ToUpper(resp)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ">", "")
	var hex strings.Builder
	framed := strings.Contains(s, ":")
	for _, part := range strings.Split(s, "|") {
		if framed {
			i := strings.Index(part, ":")
			if i < 0 {
				continue
			}
			part = part[i+1:]
		}
		hex.WriteString(part)
	}
	raw := hex.String()
	i := strings.Index(raw, "490201")
	if i < 0 {
		return ""
	}
	h := raw[i+6:]
	var out []byte
	for len(h) >= 2 {
		v, err := strconv.ParseUint(h[:2], 16, 8)
		if err != nil {
			break
		}
		if v == 0xAA {
			break
		}
		out = append(out, byte(v))
		h = h[2:]
	}
	if len(out) != 17 {
		return ""
	}
	for _, b := range out {
		if b < 0x20 || b > 0x7E {
			return ""
		}
	}
	return string(out)
}
