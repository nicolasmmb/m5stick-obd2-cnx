package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.bug.st/serial"
)

const protocolVersion = "1"

var errTimeout = errors.New("timeout")

type m5 struct {
	port    serial.Port
	lines   chan string
	seq     uint32
	echo    bool
	verbose bool
	mu      sync.Mutex

	raw      *os.File
	rawBuf   *bufio.Writer
	lastSync time.Time
	bleReady bool
}

func (m *m5) nextSeq() uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	return m.seq
}

func (m *m5) send(line string) error {
	if m.verbose {
		log.Printf("> %s", line)
	}
	_, err := m.port.Write([]byte(line + "\n"))
	return err
}

func (m *m5) next(timeout time.Duration) (string, error) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case line, ok := <-m.lines:
		if !ok {
			return "", io.EOF
		}
		if m.verbose {
			log.Printf("< %s", line)
		}
		return line, nil
	case <-t.C:
		return "", errTimeout
	}
}

func (m *m5) readLoop() {
	defer close(m.lines)
	_ = m.port.SetReadTimeout(200 * time.Millisecond)
	buf := make([]byte, 4096)
	var pending []byte
	for {
		n, err := m.port.Read(buf)
		if n > 0 {
			if m.echo {
				_, _ = os.Stdout.Write(buf[:n])
			}
			pending = append(pending, buf[:n]...)
			for {
				i := bytes.IndexByte(pending, '\n')
				if i < 0 {
					break
				}
				line := strings.TrimSpace(string(pending[:i]))
				pending = pending[i+1:]
				if isKnownLine(line) {
					m.lines <- line
				}
			}
			if len(pending) > 1<<20 {
				pending = pending[:0]
			}
		}
		if err != nil {
			return
		}
	}
}

func isKnownLine(line string) bool {
	for _, prefix := range []string{
		"S,", "Q,", "R,", "E,", "X,", "M,",
		"HELLO_ACK,", "HB_ACK,", "START_ACK,", "STOP_ACK,", "BYE_ACK,", "ERR,",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func (m *m5) logRaw(line string) {
	if m.rawBuf == nil {
		return
	}
	_, _ = m.rawBuf.WriteString(time.Now().Format(time.RFC3339Nano) + "\t" + line + "\n")
	_ = m.rawBuf.Flush()
	if time.Since(m.lastSync) >= time.Second {
		_ = m.raw.Sync()
		m.lastSync = time.Now()
	}
}

func (m *m5) note(line string) {
	if isState(line, "BLE", "ON") {
		m.bleReady = true
	} else if isState(line, "BLE", "SCAN") || isState(line, "BLE", "OFF") ||
		strings.HasPrefix(line, "X,") {
		m.bleReady = false
	}
	switch {
	case strings.HasPrefix(line, "HB_ACK,"),
		strings.HasPrefix(line, "HELLO_ACK,"),
		strings.HasPrefix(line, "START_ACK,"),
		strings.HasPrefix(line, "STOP_ACK,"),
		strings.HasPrefix(line, "BYE_ACK,"):
		return
	}
	m.logRaw(line)
}

func (m *m5) waitPrefix(prefix string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		line, err := m.next(time.Until(deadline))
		if err != nil {
			return "", err
		}
		m.note(line)
		if strings.HasPrefix(line, prefix) {
			return line, nil
		}
		if strings.HasPrefix(line, "ERR,") {
			return line, fmt.Errorf("%s", line)
		}
	}
	return "", errTimeout
}

func (m *m5) hello(session string) error {
	for attempt := 0; attempt < 20; attempt++ {
		seq := m.nextSeq()
		if err := m.send(fmt.Sprintf("HELLO,%s,%s,%d", protocolVersion, session, seq)); err != nil {
			return err
		}
		line, err := m.waitPrefix("HELLO_ACK,", 500*time.Millisecond)
		if err == nil {
			log.Printf("serial connected: %s", line)
			return nil
		}
	}
	return fmt.Errorf("no HELLO_ACK")
}

func (m *m5) waitBLE(timeout time.Duration) error {
	m.bleReady = false
	lastState := ""
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		line, err := m.next(time.Until(deadline))
		if err != nil {
			return err
		}
		m.note(line)
		if isState(line, "BLE", "ON") {
			m.bleReady = true
			log.Printf("BLE: ON")
			return nil
		}
		if isState(line, "BLE", "SCAN") || isState(line, "BLE", "OFF") {
			parts := strings.Split(line, ",")
			if len(parts) >= 4 && parts[3] != lastState {
				lastState = parts[3]
				log.Printf("BLE: %s", parts[3])
			}
		}
		if strings.HasPrefix(line, "ERR,") {
			return fmt.Errorf("%s", line)
		}
	}
	return fmt.Errorf("BLE not ready")
}

func isState(line, name, value string) bool {
	parts := strings.Split(line, ",")
	return len(parts) >= 4 && parts[0] == "S" && parts[2] == name && parts[3] == value
}

func (m *m5) command(cmd string, timeout time.Duration) ([]byte, int64, error) {
	start := time.Now()
	if err := m.send("C," + cmd); err != nil {
		return nil, 0, err
	}
	var raw []byte
	for {
		line, err := m.next(timeout)
		if err != nil {
			return raw, time.Since(start).Milliseconds(), fmt.Errorf("timeout %s", cmd)
		}
		m.note(line)
		switch {
		case strings.HasPrefix(line, "R,"):
			data, derr := parseHexLine(line)
			if derr == nil {
				raw = append(raw, data...)
			}
			if bytes.Contains(raw, []byte(">")) {
				return raw, time.Since(start).Milliseconds(), nil
			}
		case strings.HasPrefix(line, "ERR,"):
			return raw, time.Since(start).Milliseconds(), fmt.Errorf("%s", line)
		case strings.HasPrefix(line, "X,"):
			return raw, time.Since(start).Milliseconds(), fmt.Errorf("%s", line)
		}
	}
}

func parseHexLine(line string) ([]byte, error) {
	parts := strings.SplitN(line, ",", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("bad R line")
	}
	return hex.DecodeString(parts[2])
}

func (m *m5) mem() {
	if err := m.send("C,MEM"); err != nil {
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		line, err := m.next(time.Until(deadline))
		if err != nil {
			return
		}
		m.note(line)
		if strings.HasPrefix(line, "M,") {
			log.Printf("MEM: %s", line)
			return
		}
	}
}

func (m *m5) recover() error {
	log.Printf("waiting BLE reconnect...")
	if err := m.waitBLE(30 * time.Second); err != nil {
		return err
	}
	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		if _, _, err := m.command(cmd, 4*time.Second); err != nil {
			return err
		}
	}
	return m.start()
}

func (m *m5) start() error {
	seq := m.nextSeq()
	if err := m.send(fmt.Sprintf("START,%d", seq)); err != nil {
		return err
	}
	_, err := m.waitPrefix(fmt.Sprintf("START_ACK,%d", seq), 3*time.Second)
	return err
}

func (m *m5) stop() error {
	seq := m.nextSeq()
	if err := m.send(fmt.Sprintf("STOP,%d", seq)); err != nil {
		return err
	}
	_, err := m.waitPrefix(fmt.Sprintf("STOP_ACK,%d", seq), 2*time.Second)
	return err
}

func (m *m5) bye() {
	seq := m.nextSeq()
	_ = m.send(fmt.Sprintf("BYE,%d", seq))
}

func (m *m5) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = m.send(fmt.Sprintf("HB,%d", m.nextSeq()))
		}
	}
}

func main() {
	var (
		portName   = flag.String("port", "", "serial port")
		out        = flag.String("out", "data", "output directory")
		session    = flag.String("session", "", "session id (default timestamp)")
		pidsFlag   = flag.String("pids", "010C,010D,0111", "PIDs or 'all'")
		multi      = flag.Int("multi", 1, "PIDs per command (1=individual)")
		interval   = flag.Duration("interval", 0, "delay between commands")
		timeout    = flag.Duration("timeout", 2*time.Second, "per-command timeout")
		echo       = flag.Bool("echo", false, "echo raw serial bytes")
		verbose    = flag.Bool("verbose", false, "print every line")
		mem        = flag.Bool("mem", false, "request C,MEM at start/end")
		memEvery   = flag.Duration("mem-every", 0, "request C,MEM periodically")
		reset      = flag.Bool("reset", true, "pulse reset before handshake")
		bleTimeout = flag.Duration("ble-timeout", 20*time.Second, "wait for BLE=ON")
	)
	flag.Parse()
	if *portName == "" {
		log.Fatal("--port is required")
	}
	if *multi < 1 {
		*multi = 1
	}

	ctx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	p, err := serial.Open(*portName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()
	if *reset {
		_ = p.SetDTR(false)
		_ = p.SetRTS(true)
		time.Sleep(150 * time.Millisecond)
		_ = p.SetRTS(false)
		time.Sleep(50 * time.Millisecond)
		_ = p.SetDTR(false)
		time.Sleep(800 * time.Millisecond)
	}

	sid := *session
	if sid == "" {
		sid = time.Now().Format("20060102-150405")
	}
	dir := filepath.Join(*out, sid)
	if *session == "" {
		dir, err = uniqueDir(dir)
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}

	rawPath := filepath.Join(dir, "raw_elm.log")
	raw, err := os.OpenFile(rawPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer raw.Close()
	rawBuf := bufio.NewWriterSize(raw, 64*1024)
	_, _ = rawBuf.WriteString("# SESSION_START " + sid + "\n")
	_ = rawBuf.Flush()

	cmdPath := filepath.Join(dir, "commands.csv")
	cmdFile, err := os.OpenFile(cmdPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer cmdFile.Close()
	cmdWriter := csv.NewWriter(cmdFile)
	if fi, _ := cmdFile.Stat(); fi != nil && fi.Size() == 0 {
		_ = cmdWriter.Write([]string{"ts_host", "cmd", "latency_ms", "bytes", "ok", "multi"})
		cmdWriter.Flush()
	}

	m := &m5{
		port:     p,
		lines:    make(chan string, 4096),
		echo:     *echo,
		verbose:  *verbose,
		raw:      raw,
		rawBuf:   rawBuf,
		lastSync: time.Now(),
	}
	go m.readLoop()
	go m.heartbeat(ctx)

	log.Printf("session: %s", dir)
	if err := m.hello(sid); err != nil {
		log.Fatalf("serial handshake: %v", err)
	}
	if *mem {
		m.mem()
	}
	log.Printf("waiting BLE...")
	if err := m.waitBLE(*bleTimeout); err != nil {
		if *mem {
			m.mem()
		}
		log.Fatalf("BLE: %v (check the OBD adapter power)", err)
	}
	log.Printf("BLE ready; ELM handshake")

	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		log.Printf("ELM: %s", cmd)
		_, _, err := m.command(cmd, 4*time.Second)
		if err != nil {
			log.Fatalf("handshake %s: %v", cmd, err)
		}
	}

	pids, err := resolvePIDs(m, *pidsFlag)
	if err != nil {
		log.Fatalf("pids: %v", err)
	}
	log.Printf("PIDs (%d): %s", len(pids), strings.Join(pids, " "))

	if err := m.start(); err != nil {
		log.Fatalf("START: %v", err)
	}
	log.Printf("recording started")
	if *mem {
		m.mem()
	}
	lastMem := time.Now()
	meta := map[string]any{
		"session_id": sid,
		"started_at": time.Now().Format(time.RFC3339Nano),
		"port":       *portName,
		"protocol":   protocolVersion,
		"pids":       pids,
		"multi":      *multi,
	}
	writeJSON(filepath.Join(dir, "metadata.json"), meta)

	groups := buildGroups(pids, *multi)
	total := 0
	fail := 0
	totalBytes := 0
	for _, group := range groups {
		select {
		case <-ctx.Done():
			goto done
		default:
		}
		if *memEvery > 0 && time.Since(lastMem) >= *memEvery {
			m.mem()
			lastMem = time.Now()
		}
		cmd := makeCommand(group)
		rawResp, latency, err := m.command(cmd, *timeout)
		if err != nil && !m.bleReady {
			if rerr := m.recover(); rerr != nil {
				log.Printf("recover failed: %v", rerr)
			}
		}
		ok := err == nil && allExpected(rawResp, group)
		if !ok && len(group) > 1 {
			log.Printf("multi failed (%s), falling back", cmd)
			for _, pid := range group {
				rawResp, latency, err = m.command("01"+pid, *timeout)
				ok = err == nil && allExpected(rawResp, []string{pid})
				writeCommand(cmdWriter, "01"+pid, latency, len(rawResp), ok, false)
				total++
				totalBytes += len(rawResp)
				if !ok {
					fail++
				}
			}
		} else {
			writeCommand(cmdWriter, cmd, latency, len(rawResp), ok, len(group) > 1)
			total++
			totalBytes += len(rawResp)
			if !ok {
				fail++
			}
		}
		if *interval > 0 {
			time.Sleep(*interval)
		}
	}

done:
	if *mem {
		m.mem()
	}
	_ = m.stop()
	m.bye()
	_, _ = rawBuf.WriteString("# SESSION_END " + sid + "\n")
	_ = rawBuf.Flush()
	_ = raw.Sync()
	cmdWriter.Flush()
	meta["ended_at"] = time.Now().Format(time.RFC3339Nano)
	meta["commands"] = total
	meta["failures"] = fail
	meta["raw_bytes"] = totalBytes
	writeJSON(filepath.Join(dir, "metadata.json"), meta)
	summary := fmt.Sprintf("session: %s\ncommands: %d\nfailures: %d\nraw_bytes: %d\npids: %s\n",
		sid, total, fail, totalBytes, strings.Join(pids, " "))
	_ = os.WriteFile(filepath.Join(dir, "summary.txt"), []byte(summary), 0o644)
	log.Printf("saved %s (%d commands, %d failures)", dir, total, fail)
}

func uniqueDir(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, nil
	}
	for i := 1; i < 1000; i++ {
		p := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p, nil
		}
	}
	return "", fmt.Errorf("could not create unique session dir")
}

func writeJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, append(b, '\n'), 0o644)
	}
}

func writeCommand(w *csv.Writer, cmd string, latency int64, bytes int, ok bool, multi bool) {
	_ = w.Write([]string{
		time.Now().Format(time.RFC3339Nano),
		cmd,
		strconv.FormatInt(latency, 10),
		strconv.Itoa(bytes),
		strconv.FormatBool(ok),
		strconv.FormatBool(multi),
	})
	w.Flush()
}

func makeCommand(group []string) string {
	parts := make([]string, 0, len(group))
	for _, pid := range group {
		parts = append(parts, "01"+pid)
	}
	return strings.Join(parts, " ")
}

func buildGroups(pids []string, size int) [][]string {
	if size < 1 {
		size = 1
	}
	var out [][]string
	for i := 0; i < len(pids); i += size {
		end := i + size
		if end > len(pids) {
			end = len(pids)
		}
		out = append(out, pids[i:end])
	}
	return out
}

func allExpected(raw []byte, pids []string) bool {
	for _, pid := range pids {
		needle := "41" + strings.ToUpper(pid)
		if !bytes.Contains(raw, []byte(needle)) {
			return false
		}
	}
	return true
}

func resolvePIDs(m *m5, spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec != "all" {
		return splitPIDs(spec), nil
	}
	var out []string
	base := 0x00
	for base <= 0xE0 {
		cmd := fmt.Sprintf("01%02X", base)
		log.Printf("discover: %s", cmd)
		raw, _, err := m.command(cmd, 3*time.Second)
		if err != nil {
			break
		}
		mask, ok := responseMask(raw, byte(base))
		if !ok {
			break
		}
		for i := 0; i < 32; i++ {
			if mask&(1<<(31-i)) == 0 {
				continue
			}
			pid := base + i + 1
			if pid == 0x20 || pid == 0x40 || pid == 0x60 ||
				pid == 0x80 || pid == 0xA0 || pid == 0xC0 || pid == 0xE0 {
				continue
			}
			out = append(out, fmt.Sprintf("%02X", pid))
		}
		if mask&1 == 0 {
			break
		}
		base += 0x20
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no supported PIDs discovered")
	}
	return out, nil
}

func responseMask(raw []byte, base byte) (uint32, bool) {
	needle := []byte(fmt.Sprintf("41%02X", base))
	i := bytes.Index(raw, needle)
	if i < 0 || i+12 > len(raw) {
		return 0, false
	}
	v, err := strconv.ParseUint(string(raw[i+4:i+12]), 16, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

func hexDigit(v byte) byte {
	if v < 10 {
		return '0' + v
	}
	return 'A' + (v - 10)
}

func splitPIDs(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	}) {
		p = strings.ToUpper(strings.TrimSpace(p))
		p = strings.TrimPrefix(p, "01")
		if len(p) == 2 {
			out = append(out, p)
		}
	}
	return out
}
