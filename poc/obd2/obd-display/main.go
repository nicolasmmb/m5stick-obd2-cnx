package main

import (
	"image/color"
	"machine"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tinygo.org/x/bluetooth"
	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
	"tinygo.org/x/tinyfont/proggy"
)

var adapter = bluetooth.DefaultAdapter

var (
	uuidFFF0 = bluetooth.New16BitUUID(0xFFF0)
	uuidFFF1 = bluetooth.New16BitUUID(0xFFF1)
	uuidFFF2 = bluetooth.New16BitUUID(0xFFF2)
)

const (
	screenW = 240
	screenH = 135

	// Tank capacity for converting PID 2F (%) to litres.
	// 440 = 44.0 L. Change this to match your car.
	tankCapacityDeciL = 440

	pageCount = 3

	bandRows = 24

	// Layout parameters. Everything below is derived from these values and
	// from the font metrics, instead of using magic screen coordinates.
	headerH = 20
	marginX = 8
	colGap  = 12
	rowGap  = 6
	topH    = 66
)

var (
	display st7789.Device

	// 4 bits per pixel: 240x135 / 2 = 16.2 KB. Updates are composed here and
	// only the changed rows are sent to the panel, so the screen never shows
	// the intermediate "clear then draw" state.
	fb [screenW * screenH / 2]byte

	// Scratch used to expand 4bpp rows to RGB565 before sending to the LCD.
	// Flushing in bands keeps this small.
	lineBuf [screenW * bandRows * 2]byte

	dirtyMin = -1
	dirtyMax = -1

	page int

	black = color.RGBA{0, 0, 0, 255}
	white = color.RGBA{235, 235, 235, 255}
	gray  = color.RGBA{80, 80, 80, 255}
	cyan  = color.RGBA{0, 220, 220, 255}
	green = color.RGBA{0, 220, 90, 255}
	red   = color.RGBA{255, 80, 80, 255}
	amber = color.RGBA{255, 180, 0, 255}

	// 16-color palette for the 4bpp framebuffer.
	palette = [...]color.RGBA{black, white, gray, cyan, green, red, amber}

	btnA, btnB uint32
	lastA      time.Time
	lastB      time.Time

	mu sync.Mutex
	rx []byte
)

type values struct {
	rpm      int
	speed    int
	fuel     int // raw PID 2F, 0..255
	coolant  int // °C
	intake   int // °C
	cat      int // °C
	ethanol  int // %
	fuelType int
}

type elm struct {
	write bluetooth.DeviceCharacteristic
}

// canvas implements tinyfont's Displayer on top of the RGB565 framebuffer.
type canvas struct{}

func (canvas) Size() (int16, int16) { return screenW, screenH }
func (canvas) Display() error       { return nil }

func (canvas) SetPixel(x, y int16, c color.RGBA) {
	if x < 0 || y < 0 || x >= screenW || y >= screenH {
		return
	}
	i := int(y)*screenW + int(x)
	b := i >> 1
	idx := paletteIndex(c)
	if i&1 == 0 {
		fb[b] = fb[b]&0x0F | idx<<4
	} else {
		fb[b] = fb[b]&0xF0 | idx
	}
}

var cv canvas

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	initDisplay()
	initButtons()
	showStatus("OBD2 BLE", "iniciando...", cyan)

	if err := adapter.Enable(); err != nil {
		showStatus("ERRO BLE", err.Error(), red)
		return
	}
	adapter.SetScanType(bluetooth.ScanTypeActive)

	for {
		if err := run(); err != nil {
			println("obd-display:", err.Error())
			showStatus("ERRO", err.Error(), red)
			time.Sleep(3 * time.Second)
			showStatus("OBD2 BLE", "reconectando...", amber)
		}
	}
}

func run() error {
	page = 0
	showStatus("OBD2 BLE", "procurando adaptador...", cyan)
	println("obd-display: scanning...")

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
			println("obd-display: found", r.Address.String(), r.LocalName())
			adapter.StopScan()
		}
	})
	if err != nil {
		<-done
		return err
	}
	<-done
	if !found {
		return errNoAdapter
	}

	showStatus("OBD2 BLE", "conectando...", amber)
	dev, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return err
	}
	println("obd-display: connected")

	services, err := dev.DiscoverServices([]bluetooth.UUID{uuidFFF0})
	if err != nil {
		return err
	}
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{uuidFFF1, uuidFFF2})
	if err != nil {
		return err
	}
	notifyChar := chars[0] // FFF1
	writeChar := chars[1]  // FFF2

	if err := notifyChar.EnableNotifications(func(buf []byte) {
		mu.Lock()
		rx = append(rx, buf...)
		mu.Unlock()
	}); err != nil {
		return err
	}

	e := &elm{write: writeChar}
	showStatus("OBD2 BLE", "iniciando ELM327...", amber)
	for _, cmd := range []string{"ATZ", "ATE0", "ATL0", "ATS0", "ATSP0"} {
		resp, err := e.command(cmd, 3*time.Second)
		if err != nil {
			return err
		}
		println("RX:", strings.ReplaceAll(strings.TrimSpace(resp), "\r", "\\r"))
		time.Sleep(150 * time.Millisecond)
	}

	showStatus("OBD2 BLE", "lendo dados iniciais...", amber)
	v := values{
		rpm: -1, speed: -1, fuel: -1,
		coolant: -1, intake: -1, cat: -1,
		ethanol: -1, fuelType: -1,
	}
	v.speed = readSpeed(e)
	v.rpm = readRPM(e)
	v.fuel = readFuel(e)
	v.coolant = readCoolant(e)
	v.intake = readIntake(e)
	v.cat = readCat(e)
	v.ethanol = readEthanol(e)
	v.fuelType = readFuelType(e)

	// Drop any button events captured during boot/connection, so the app
	// always starts on page 1.
	atomic.StoreUint32(&btnA, 0)
	atomic.StoreUint32(&btnB, 0)
	lastA = time.Now()
	lastB = time.Now()

	drawPage(v)
	last := v
	fuelTick := 0
	ethTick := 0
	misses := 0

	for {
		if handleButtons() {
			drawPage(v)
			last = v
		}

		switch page {
		case 0:
			v.speed = readSpeed(e)
			v.rpm = readRPM(e)
			fuelTick++
			// Fuel level changes slowly; no need to spend a request every pass.
			if fuelTick%8 == 0 || v.fuel < 0 {
				v.fuel = readFuel(e)
			}
			if v.speed < 0 && v.rpm < 0 && v.fuel < 0 {
				misses++
			} else {
				misses = 0
			}
		case 1:
			v.coolant = readCoolant(e)
			v.intake = readIntake(e)
			v.cat = readCat(e)
			if v.coolant < 0 && v.intake < 0 && v.cat < 0 {
				misses++
			} else {
				misses = 0
			}
		case 2:
			ethTick++
			if ethTick%10 == 0 || v.ethanol < 0 {
				v.ethanol = readEthanol(e)
			}
			if v.fuelType < 0 {
				v.fuelType = readFuelType(e)
			}
			if v.ethanol < 0 && v.fuelType < 0 {
				misses++
			} else {
				misses = 0
			}
		}
		if misses >= 3 {
			return &simpleError{"conexao OBD2 perdida"}
		}

		if handleButtons() {
			drawPage(v)
			last = v
			continue
		}

		println("page", page+1, "rpm", v.rpm, "speed", v.speed, "fuel", v.fuel,
			"cool", v.coolant, "intake", v.intake, "cat", v.cat,
			"etoh", v.ethanol, "type", v.fuelType)
		drawValues(v, &last)
		last = v
		time.Sleep(120 * time.Millisecond)
	}
}

var errNoAdapter = &simpleError{"adaptador OBD2 BLE nao encontrado"}

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

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

func (e *elm) pidBytes(cmd string, n int) ([]byte, bool) {
	resp, err := e.command(cmd, 2*time.Second)
	if err != nil {
		return nil, false
	}
	b, ok := pidData(resp, cmd[2:])
	if !ok || len(b) < n {
		return nil, false
	}
	return b[:n], true
}

func (e *elm) pidNumber(cmd string) int {
	b, ok := e.pidBytes(cmd, 1)
	if !ok {
		return -1
	}
	return int(b[0])
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

func readSpeed(e *elm) int    { return e.pidNumber("010D") }
func readFuel(e *elm) int     { return e.pidNumber("012F") }
func readEthanol(e *elm) int  { return scalePct(e.pidNumber("0152")) }
func readFuelType(e *elm) int { return e.pidNumber("0151") }

func readRPM(e *elm) int {
	b, ok := e.pidBytes("010C", 2)
	if !ok {
		return -1
	}
	return (int(b[0])*256 + int(b[1])) / 4
}

func readCoolant(e *elm) int {
	n := e.pidNumber("0105")
	if n < 0 {
		return -1
	}
	return n - 40
}

func readIntake(e *elm) int {
	n := e.pidNumber("010F")
	if n < 0 {
		return -1
	}
	return n - 40
}

func readCat(e *elm) int {
	b, ok := e.pidBytes("013C", 2)
	if !ok {
		return -1
	}
	return (int(b[0])*256+int(b[1]))/10 - 40
}

func scalePct(raw int) int {
	if raw < 0 {
		return -1
	}
	p := raw * 100 / 255
	if p > 100 {
		p = 100
	}
	return p
}

func initDisplay() {
	machine.SPI2.Configure(machine.SPIConfig{
		Frequency: 24 * machine.MHz,
		SCK:       machine.GPIO13,
		SDO:       machine.GPIO15,
		SDI:       machine.NoPin,
		Mode:      0,
	})
	display = st7789.New(
		machine.SPI2,
		machine.GPIO12,
		machine.GPIO14,
		machine.GPIO5,
		machine.GPIO27,
	)
	display.IsBGR(true)
	display.Configure(st7789.Config{
		Width:        135,
		Height:       240,
		Rotation:     drivers.Rotation90,
		RowOffset:    40,
		ColumnOffset: 52,
		FrameRate:    st7789.FRAMERATE_60,
	})
	display.FillScreen(black)
}

func initButtons() {
	// A = GPIO37 (next page), B = GPIO39 (previous page).
	// C (GPIO35) is not used: a long press can power the Stick off.
	for _, p := range []machine.Pin{machine.GPIO37, machine.GPIO39} {
		p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	if err := machine.GPIO37.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnA, 1)
	}); err != nil {
		println("btnA interrupt error:", err.Error())
	}
	if err := machine.GPIO39.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnB, 1)
	}); err != nil {
		println("btnB interrupt error:", err.Error())
	}
}

func handleButtons() bool {
	now := time.Now()
	changed := false

	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 250*time.Millisecond {
		lastA = now
		page = (page + 1) % pageCount
		changed = true
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 250*time.Millisecond {
		lastB = now
		page = (page + pageCount - 1) % pageCount
		changed = true
	}
	return changed
}

func showStatus(title, msg string, c color.RGBA) {
	fillRect(0, 0, screenW, screenH, black)
	text(&freemono.Bold9pt7b, 4, 30, title, c)
	text(&proggy.TinySZ8pt7b, 4, 50, truncate(msg, 29), white)
	flushRows(0, screenH-1)
	dirtyMin, dirtyMax = -1, -1
}

func drawPage(v values) {
	fillRect(0, 0, screenW, screenH, black)
	drawHeader()

	switch page {
	case 0:
		drawPage1(v)
	case 1:
		drawPage2(v)
	case 2:
		drawPage3(v)
	}

	flushRows(0, screenH-1)
	dirtyMin, dirtyMax = -1, -1
}

func drawHeader() {
	text(&freemono.Bold9pt7b, 5, 14, "OBD2", cyan)
	text(&proggy.TinySZ8pt7b, 62, 13, strconv.Itoa(page+1)+"/3", white)
	text(&proggy.TinySZ8pt7b, 178, 13, "A/B", gray)
	text(&proggy.TinySZ8pt7b, 212, 13, "BLE", green)
	fillRect(0, headerH-2, screenW, 1, gray)
}

type rect struct {
	x, y, w, h int16
}

func (r rect) bottom() int16 { return r.y + r.h }

func page1Rects() (speed, rpm, fuel rect) {
	innerW := int16(screenW - 2*marginX)

	// Reserve space for the widest possible value in each cell, then share
	// the leftover space equally. This keeps both numbers centered.
	speedW := textWidth(&freemono.Bold24pt7b, "000") + 8
	rpmW := textWidth(&freemono.Bold24pt7b, "0000") + 8
	if extra := innerW - speedW - rpmW - colGap; extra > 0 {
		speedW += extra / 2
		rpmW += extra - extra/2
	}

	speed = rect{marginX, headerH, speedW, topH}
	rpm = rect{speed.x + speed.w + colGap, headerH, rpmW, topH}
	fuel = rect{marginX, headerH + topH + rowGap, innerW, screenH - (headerH + topH + rowGap)}
	return
}

func page2Rects() (a, b, c rect) {
	innerW := int16(screenW - 2*marginX)
	gap := int16(6)
	usable := innerW - 2*gap
	colW := usable / 3
	rem := usable % 3

	a = rect{marginX, headerH, colW, screenH - headerH}
	b = rect{a.x + a.w + gap, headerH, colW, screenH - headerH}
	c = rect{b.x + b.w + gap, headerH, colW + rem, screenH - headerH}
	return
}

func page3Rects() (left, right, typ rect) {
	innerW := int16(screenW - 2*marginX)
	gap := int16(colGap)
	typeH := int16(23)
	pctH := int16(screenH - headerH - int(typeH) - 4)

	left = rect{marginX, headerH, (innerW - gap) / 2, pctH}
	right = rect{left.x + left.w + gap, headerH, innerW - gap - left.w, pctH}
	typ = rect{marginX, headerH + pctH + 4, innerW, typeH}
	return
}

func textWidth(f tinyfont.Fonter, s string) int16 {
	w, _ := tinyfont.LineWidth(f, s)
	return int16(w)
}

func textCentered(f tinyfont.Fonter, r rect, baseline int16, s string, c color.RGBA) {
	x := r.x + (r.w-textWidth(f, s))/2
	text(f, x, baseline, s, c)
}

func drawPage1(v values) {
	speed, rpm, fuel := page1Rects()
	drawSpeedWidget(speed, v.speed)
	drawRPMWidget(rpm, v.rpm)
	drawFuelWidget(fuel, v.fuel)
}

func drawPage2(v values) {
	a, b, c := page2Rects()
	drawTempWidget(a, "AGUA", v.coolant, 120, 100, 110)
	drawTempWidget(b, "AR ADM", v.intake, 80, 50, 70)
	drawTempWidget(c, "CATALIS", v.cat, 900, 700, 900)
}

func drawPage3(v values) {
	left, right, typ := page3Rects()
	gas := -1
	if v.ethanol >= 0 {
		gas = 100 - v.ethanol
	}
	drawPctWidget(left, "ALCOOL %", v.ethanol, amber)
	drawPctWidget(right, "GASOLINA %", gas, cyan)
	drawFuelTypeWidget(typ, v.fuelType, v.ethanol)
}

func drawValues(v values, last *values) {
	switch page {
	case 0:
		speed, rpm, fuel := page1Rects()
		if v.speed != last.speed {
			drawSpeedWidget(speed, v.speed)
		}
		if v.rpm != last.rpm {
			drawRPMWidget(rpm, v.rpm)
		}
		if v.fuel != last.fuel {
			drawFuelWidget(fuel, v.fuel)
		}
	case 1:
		a, b, c := page2Rects()
		if v.coolant != last.coolant {
			drawTempWidget(a, "AGUA", v.coolant, 120, 100, 110)
		}
		if v.intake != last.intake {
			drawTempWidget(b, "AR ADM", v.intake, 80, 50, 70)
		}
		if v.cat != last.cat {
			drawTempWidget(c, "CATALIS", v.cat, 900, 700, 900)
		}
	case 2:
		left, right, typ := page3Rects()
		if v.ethanol != last.ethanol {
			gas := -1
			if v.ethanol >= 0 {
				gas = 100 - v.ethanol
			}
			drawPctWidget(left, "ALCOOL %", v.ethanol, amber)
			drawPctWidget(right, "GASOLINA %", gas, cyan)
		}
		if v.fuelType != last.fuelType || v.ethanol != last.ethanol {
			drawFuelTypeWidget(typ, v.fuelType, v.ethanol)
		}
	}
	flushDirty()
}

func drawSpeedWidget(r rect, v int) {
	fillRect(r.x, r.y, r.w, r.h, black)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+14, "VELOCIDADE", gray)
	if v >= 0 {
		textCentered(&freemono.Bold24pt7b, r, r.y+50, strconv.Itoa(v), cyan)
	} else {
		textCentered(&freemono.Bold24pt7b, r, r.y+50, "--", gray)
	}
	textCentered(&proggy.TinySZ8pt7b, r, r.y+62, "km/h", gray)
	markDirty(int(r.y), int(r.bottom()-1))
}

func drawRPMWidget(r rect, v int) {
	fillRect(r.x, r.y, r.w, r.h, black)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+14, "RPM", gray)
	if v >= 0 {
		textCentered(&freemono.Bold24pt7b, r, r.y+50, strconv.Itoa(v), white)
	} else {
		textCentered(&freemono.Bold24pt7b, r, r.y+50, "--", gray)
	}
	textCentered(&proggy.TinySZ8pt7b, r, r.y+62, "motor", gray)
	markDirty(int(r.y), int(r.bottom()-1))
}

func drawFuelWidget(r rect, raw int) {
	fillRect(r.x, r.y, r.w, r.h, black)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+12, "COMBUSTIVEL", gray)
	if raw < 0 {
		textCentered(&freemono.Bold18pt7b, r, r.y+30, "--", gray)
		markDirty(int(r.y), int(r.bottom()-1))
		return
	}

	litersX10 := raw * tankCapacityDeciL / 255
	liters := strconv.Itoa(litersX10/10) + "." + strconv.Itoa(litersX10%10) + " L"
	pct := raw * 100 / 255
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}

	textCentered(&freemono.Bold18pt7b, r, r.y+30, liters, green)

	barX := r.x + 8
	barW := r.w - 16
	barY := r.bottom() - 7
	fillRect(barX, barY, barW, 4, gray)
	if pct > 0 {
		fillRect(barX, barY, int16(int(barW)*pct/100), 4, green)
	}
	markDirty(int(r.y), int(r.bottom()-1))
}

func drawTempWidget(r rect, label string, v int, max, warn, hot int) {
	fillRect(r.x, r.y, r.w, r.h, black)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+16, label, gray)
	c := tempColor(v, warn, hot)
	s := "--"
	if v >= 0 {
		s = strconv.Itoa(v)
	}
	// Use the largest font that still fits the cell (4-digit catalyst
	// temperatures fall back to Bold12).
	f := tinyfont.Fonter(&freemono.Bold18pt7b)
	if textWidth(f, s)+4 > r.w {
		f = &freemono.Bold12pt7b
	}
	textCentered(f, r, r.y+54, s, c)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+68, "C", gray)

	barX := r.x + 4
	barW := r.w - 8
	barY := r.y + 80
	fillRect(barX, barY, barW, 6, gray)
	if v > 0 {
		n := int16(int(barW) * v / max)
		if n > barW {
			n = barW
		}
		fillRect(barX, barY, n, 6, c)
	}
	markDirty(int(r.y), int(r.y+88))
}

func drawPctWidget(r rect, label string, v int, c color.RGBA) {
	fillRect(r.x, r.y, r.w, r.h, black)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+16, label, gray)
	if v >= 0 {
		textCentered(&freemono.Bold24pt7b, r, r.y+58, strconv.Itoa(v), c)
	} else {
		textCentered(&freemono.Bold24pt7b, r, r.y+58, "--", gray)
	}

	barX := r.x + 4
	barW := r.w - 8
	barY := r.y + 70
	fillRect(barX, barY, barW, 8, gray)
	if v > 0 {
		n := int16(int(barW) * v / 100)
		if n > barW {
			n = barW
		}
		fillRect(barX, barY, n, 8, c)
	}
	markDirty(int(r.y), int(r.y+80))
}

func drawFuelTypeWidget(r rect, t, ethanol int) {
	fillRect(r.x, r.y, r.w, r.h, black)
	name := fuelTypeName(t)
	if ethanol > 0 && ethanol < 100 {
		name = "FLEX"
	}
	textCentered(&proggy.TinySZ8pt7b, r, r.y+14, "TIPO: "+name, white)
	markDirty(int(r.y), int(r.bottom()-1))
}

func tempColor(v, warn, hot int) color.RGBA {
	if v < 0 {
		return gray
	}
	if v >= hot {
		return red
	}
	if v >= warn {
		return amber
	}
	return green
}

func fuelTypeName(t int) string {
	switch t {
	case 0:
		return "N/D"
	case 1:
		return "GASOLINA"
	case 2:
		return "METANOL"
	case 3:
		return "ETANOL"
	case 4:
		return "DIESEL"
	case 5:
		return "LPG"
	case 6:
		return "CNG"
	case 7:
		return "PROPANO"
	case 8:
		return "ELETRICO"
	case 9, 10, 11, 12, 13, 14, 15, 16:
		return "BIFUEL"
	case 17, 18, 19, 20, 21, 22:
		return "HIBRIDO"
	default:
		if t < 0 {
			return "N/D"
		}
		return "TIPO " + strconv.Itoa(t)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "~"
}

func text(f tinyfont.Fonter, x, y int16, s string, c color.RGBA) {
	tinyfont.WriteLine(cv, f, x, y, s, c)
}

func fillRect(x, y, w, h int16, c color.RGBA) {
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > screenW {
		w = screenW - x
	}
	if y+h > screenH {
		h = screenH - y
	}
	if w <= 0 || h <= 0 {
		return
	}

	idx := paletteIndex(c)
	for yy := int(y); yy < int(y+h); yy++ {
		base := yy * screenW
		for xx := int(x); xx < int(x+w); xx++ {
			i := base + xx
			b := i >> 1
			if i&1 == 0 {
				fb[b] = fb[b]&0x0F | idx<<4
			} else {
				fb[b] = fb[b]&0xF0 | idx
			}
		}
	}
}

func rgb565(c color.RGBA) uint16 {
	return uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3)
}

func markDirty(y0, y1 int) {
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= screenH {
		y1 = screenH - 1
	}
	if y1 < y0 {
		return
	}
	if dirtyMin < 0 || y0 < dirtyMin {
		dirtyMin = y0
	}
	if y1 > dirtyMax {
		dirtyMax = y1
	}
}

func flushDirty() {
	if dirtyMin < 0 {
		return
	}
	y0, y1 := dirtyMin, dirtyMax
	dirtyMin, dirtyMax = -1, -1
	flushRows(int16(y0), int16(y1))
}

func flushRows(y0, y1 int16) {
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= screenH {
		y1 = screenH - 1
	}
	if y1 < y0 {
		return
	}

	for y := int(y0); y <= int(y1); y += bandRows {
		h := int(y1) - y + 1
		if h > bandRows {
			h = bandRows
		}
		for r := 0; r < h; r++ {
			for x := 0; x < screenW; x++ {
				c := palette[getPixel(x, y+r)]
				v := rgb565(c)
				off := (r*screenW + x) * 2
				lineBuf[off] = byte(v >> 8)
				lineBuf[off+1] = byte(v)
			}
		}
		display.DrawRGBBitmap8(0, int16(y), lineBuf[:h*screenW*2], screenW, int16(h))
	}
}

func getPixel(x, y int) byte {
	i := y*screenW + x
	b := i >> 1
	if i&1 == 0 {
		return fb[b] >> 4
	}
	return fb[b] & 0x0F
}

func paletteIndex(c color.RGBA) byte {
	for i, p := range palette {
		if p == c {
			return byte(i)
		}
	}
	// Nearest color fallback for any unlisted color.
	best := 0
	bestDist := int32(1 << 30)
	for i, p := range palette {
		dr := int32(c.R) - int32(p.R)
		dg := int32(c.G) - int32(p.G)
		db := int32(c.B) - int32(p.B)
		d := dr*dr + dg*dg + db*db
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return byte(best)
}
