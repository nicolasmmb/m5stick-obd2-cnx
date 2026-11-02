package main

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tinygo.org/x/bluetooth"

	"m5stick-obd2-cnx/internal/obddtc"
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

// cmdErr distinguishes an ELM timeout (the link can still be alive) from a
// failed BLE write (the link is gone). It avoids allocating per poll.
type cmdErr uint8

const (
	cmdOK cmdErr = iota
	cmdTimeout
	cmdWrite
)

func (e cmdErr) Error() string {
	switch e {
	case cmdTimeout:
		return "timeout"
	case cmdWrite:
		return "write failed"
	default:
		return "ok"
	}
}

// pidResult tells runLive whether a poll succeeded, should be retried, or
// proved that the BLE link is gone.
type pidResult uint8

const (
	pidOK pidResult = iota
	pidRetry
	pidLinkLost
)

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

func runLive() (err error) {
	if appMode == ModeReplay {
		return errReplay
	}
	if appMode == ModePaused {
		return errStop
	}
	if !bleInitialized {
		if err := adapter.Enable(); err != nil {
			return err
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
		return scanErr
	}
	if !found {
		return &simpleError{"no OBDII BLE device found"}
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
		return err
	}
	defer dev.Disconnect()
	bleState = bleConnected
	elmState = elmInit
	uiPaused = false
	dirtyAll = true
	services, err := dev.DiscoverServices([]bluetooth.UUID{uuidFFF0})
	if err != nil {
		return err
	}
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{uuidFFF1, uuidFFF2})
	if err != nil {
		return err
	}
	notifyChar := chars[0]
	writeChar := chars[1]
	if err := notifyChar.EnableNotifications(func(buf []byte) {
		mu.Lock()
		rx = append(rx, buf...)
		mu.Unlock()
	}); err != nil {
		return err
	}
	elmState = elmInit
	e := &elm{write: writeChar}
	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		if _, cerr := e.command(cmd, 3*time.Second); cerr != cmdOK {
			if cerr == cmdWrite {
				return errLinkLost
			}
			return &simpleError{"handshake " + cmd + ": " + cerr.Error()}
		}
		time.Sleep(100 * time.Millisecond)
	}
	elmState = elmOK
	reconnectAttempt = 0

	// Emit I before the diagnostics so the host sees the session start first.
	vin, cerr := readVIN(e)
	if cerr == cmdWrite {
		return errLinkLost
	}
	emitStart(vin)
	endReason := "error"
	defer func() {
		emitEnd(endReason)
	}()

	if cerr := readDiagnostics(e); cerr == cmdWrite {
		endReason = "ble_lost"
		return errLinkLost
	}

	cycle := 0
	fails := 0
	for {
		if merr := liveModeError(); merr != nil {
			endReason = endReasonFor(merr)
			return merr
		}
		for _, p := range fastPIDs {
			if merr := liveModeError(); merr != nil {
				endReason = endReasonFor(merr)
				return merr
			}
			switch emitPID(e, p) {
			case pidLinkLost:
				endReason = "ble_lost"
				return errLinkLost
			case pidOK:
				fails = 0
			default:
				fails++
				if fails >= 3 {
					endReason = "ble_lost"
					return errLinkLost
				}
			}
		}
		cycle++
		if cycle%4 == 0 {
			for _, p := range slowPIDs {
				if merr := liveModeError(); merr != nil {
					endReason = endReasonFor(merr)
					return merr
				}
				switch emitPID(e, p) {
				case pidLinkLost:
					endReason = "ble_lost"
					return errLinkLost
				case pidOK:
					fails = 0
				default:
					fails++
					if fails >= 3 {
						endReason = "ble_lost"
						return errLinkLost
					}
				}
				time.Sleep(20 * time.Millisecond)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func liveModeError() error {
	switch appMode {
	case ModeReplay:
		return errReplay
	case ModePaused:
		return errStop
	default:
		return nil
	}
}

func endReasonFor(err error) string {
	switch err {
	case errReplay:
		return "replay"
	case errStop:
		return "host"
	default:
		return "error"
	}
}

// sessionOpen guarantees that only one X is emitted per I, even when the
// serial handler closes the session while runLive is still in a command.
var sessionOpen int32

func emitStart(vin string) {
	txLines++
	println("I," + strconv.FormatInt(liveMs(), 10) + ",0.1," + vin + ",440")
	atomic.StoreInt32(&sessionOpen, 1)
}

func emitEvent(typ, detail string) {
	txLines++
	println("E," + strconv.FormatInt(liveMs(), 10) + "," + typ + "," + detail)
}

func emitEnd(reason string) {
	if atomic.SwapInt32(&sessionOpen, 0) == 0 {
		return
	}
	txLines++
	println("X," + strconv.FormatInt(liveMs(), 10) + "," + reason)
}

func emitPID(e *elm, p pidSpec) pidResult {
	t0 := time.Now()
	resp, cerr := e.command(p.cmd, 2*time.Second)
	lat := int64(time.Since(t0) / time.Millisecond)
	ms := liveMs()

	if cerr == cmdWrite {
		errCount++
		return pidLinkLost
	}
	if cerr != cmdOK || strings.Contains(resp, "NO DATA") {
		errCount++
		txLines++
		readingStore.PutQuality(p.pid, obdpid.QualityNoData)
		println("E," + strconv.FormatInt(ms, 10) + ",NO_DATA," + p.pid)
		return pidRetry
	}

	data, ok := pidData(resp, p.pid)
	if !ok {
		errCount++
		txLines++
		readingStore.PutQuality(p.pid, obdpid.QualityInvalid)
		println("E," + strconv.FormatInt(ms, 10) + ",PARSE_ERROR," + p.pid)
		return pidRetry
	}

	ingest(p.pid, data, obdpid.SourceLive)
	txLines++
	println("P," + strconv.FormatInt(ms, 10) + "," + p.pid + "," + hexBytes(data) +
		"," + strconv.FormatInt(lat, 10) + ",OK")
	return pidOK
}

func readDiagnostics(e *elm) cmdErr {
	statusOK := false
	if resp, cerr := e.command("0101", 2*time.Second); cerr == cmdWrite {
		return cmdWrite
	} else if cerr == cmdOK {
		if d := servicePayload(resp, "4101"); len(d) >= 4 {
			if st, ok := obddtc.ParseStatus(d); ok {
				statusOK = true
				milOn = st.MIL
				dtcCount = st.Count
				emitEvent("MIL", onOff(st.MIL))
				emitEvent("DTC_COUNT", strconv.Itoa(st.Count))
				emitEvent("READINESS", hexBytes(st.Readiness[:]))
			}
		}
	}

	stored := 0
	modes := [...]struct {
		cmd    string
		header string
		typ    string
	}{
		{"03", "43", "DTC"},
		{"07", "47", "DTC_PENDING"},
		{"0A", "4A", "DTC_PERMANENT"},
	}
	for _, m := range modes {
		resp, cerr := e.command(m.cmd, 3*time.Second)
		if cerr == cmdWrite {
			return cmdWrite
		}
		if cerr != cmdOK {
			continue
		}
		codes := obddtc.Codes(servicePayload(resp, m.header))
		if m.cmd == "03" {
			stored = len(codes)
		}
		for _, code := range codes {
			emitEvent(m.typ, code)
		}
	}
	if !statusOK {
		dtcCount = stored
	}
	markTile(4, 1)
	markTile(4, 2)
	return cmdOK
}

// cleanELM removes echo/whitespace and the ISO-TP frame prefixes that the
// ELM327 inserts for multi-frame responses ("0:490201...").
func cleanELM(resp string) string {
	s := strings.ToUpper(resp)
	var b strings.Builder
	for _, part := range strings.Split(s, "|") {
		if i := strings.IndexByte(part, ':'); i >= 0 {
			part = part[i+1:]
		}
		for i := 0; i < len(part); i++ {
			switch part[i] {
			case ' ', '\r', '\n', '>', '|':
				continue
			}
			b.WriteByte(part[i])
		}
	}
	return b.String()
}

// servicePayload returns the bytes that follow the response header, e.g.
// "4302 0133 C123" with header "43" yields {02,01,33,C1,23}.
func servicePayload(resp, header string) []byte {
	s := cleanELM(resp)
	i := strings.Index(s, header)
	if i < 0 {
		return nil
	}
	h := s[i+len(header):]
	out := make([]byte, 0, len(h)/2)
	for len(h) >= 2 {
		v, err := strconv.ParseUint(h[:2], 16, 8)
		if err != nil {
			break
		}
		out = append(out, byte(v))
		h = h[2:]
	}
	return out
}

func (e *elm) command(cmd string, timeout time.Duration) (string, cmdErr) {
	mu.Lock()
	rx = rx[:0]
	mu.Unlock()

	if _, err := e.write.WriteWithoutResponse([]byte(cmd + "\r")); err != nil {
		return "", cmdWrite
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		s := string(rx)
		mu.Unlock()
		if strings.Contains(s, ">") {
			return s, cmdOK
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", cmdTimeout
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

func hexBytes(d []byte) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, len(d)*2)
	for _, b := range d {
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out)
}

func readVIN(e *elm) (string, cmdErr) {
	resp, cerr := e.command("0902", 4*time.Second)
	if cerr != cmdOK {
		return "", cerr
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
		return "", cmdOK
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
		return "", cmdOK
	}
	for _, b := range out {
		if b < 0x20 || b > 0x7E {
			return "", cmdOK
		}
	}
	return string(out), cmdOK
}

var bleInitialized bool

func bleStopScan() {
	// Calling StopScan before Enable can confuse the HCI layer.
	if bleInitialized {
		_ = adapter.StopScan()
	}
}
