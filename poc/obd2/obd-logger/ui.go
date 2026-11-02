package main

import (
	"image/color"
	"machine"
	"strconv"
	"sync/atomic"
	"time"

	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
	"tinygo.org/x/tinyfont/proggy"

	"m5stick-obd2-cnx/internal/obdpid"
)

const (
	screenW  = 240
	screenH  = 135
	bandRows = 4
	inset    = 1
	gap      = 2
	contentH = screenH - inset*2 - 3
)

var (
	display st7789.Device
	fb      [screenW * screenH / 2]byte
	lineBuf [screenW * bandRows * 2]byte

	black     = color.RGBA{0, 0, 0, 255}
	white     = color.RGBA{235, 235, 235, 255}
	gray      = color.RGBA{80, 80, 80, 255}
	cyan      = color.RGBA{0, 220, 220, 255}
	green     = color.RGBA{0, 220, 90, 255}
	red       = color.RGBA{255, 80, 80, 255}
	amber     = color.RGBA{255, 180, 0, 255}
	blue      = color.RGBA{80, 160, 255, 255}
	panel     = color.RGBA{22, 26, 34, 255}
	darkGreen = color.RGBA{0, 55, 32, 255}
	darkRed   = color.RGBA{70, 18, 18, 255}
	darkAmber = color.RGBA{70, 48, 0, 255}
	darkCyan  = color.RGBA{0, 50, 60, 255}

	palette = [...]color.RGBA{
		black, white, gray, cyan, green, red, amber,
		blue, panel, darkGreen, darkRed, darkAmber, darkCyan,
	}

	page int

	btnA, btnB uint32
	lastA      time.Time
	lastB      time.Time
	lastAState bool
	lastBState bool

	serialAckNano int64
	bootTime      time.Time
	bleState      int
	elmState      int
	txLines       int
	errCount      int
	samples       int
	started       time.Time

	dtcCount      int
	milOn         bool
	tripKm        float64
	tripFuelStart float64
	tripLast      time.Time
	uiPaused      bool

	dirtyTiles    uint16
	dirtyAll      bool
	heartOn       bool
	lastHeartbeat time.Time
	lastStatusUI  time.Time
	lastCompose   time.Time
)

const (
	iconBLE = iota
	iconLink
	iconRate
	iconTx
	iconErr
	iconClock
	iconGauge
	iconFuel
	iconThermo
	iconAir
	iconCat
)

const (
	bleOff = iota
	bleScan
	bleConnecting
	bleConnected
	bleErr
)

const (
	elmWait = iota
	elmInit
	elmOK
	elmErr
)

type rect struct {
	x, y, w, h int16
}

func (r rect) bottom() int16 { return r.y + r.h }

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
	// The Plus2 panel used here renders RGB565 with RGB order.
	// With BGR enabled, orange (R+G) was appearing blue.
	display.IsBGR(false)
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
	// GPIO34-39 are input-only and have no internal pull-up on ESP32.
	// The M5StickC Plus2 board provides external pull-ups, so use plain input.
	for _, p := range []machine.Pin{machine.GPIO37, machine.GPIO39} {
		p.Configure(machine.PinConfig{Mode: machine.PinInput})
	}
	_ = machine.GPIO37.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnA, 1)
	})
	_ = machine.GPIO39.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnB, 1)
	})
	lastAState = machine.GPIO37.Get()
	lastBState = machine.GPIO39.Get()
}

func reading(pid string) (obdpid.Reading, bool) {
	r, ok := readingStore.Get(pid)
	if !ok {
		return obdpid.Reading{}, false
	}
	if r.Quality == obdpid.QualityOK && r.AgeMs(time.Now()) > 3000 {
		r.Quality = obdpid.QualityStale
	}
	return r, true
}

func readingNumber(pid string) (float64, bool) {
	r, ok := reading(pid)
	if !ok {
		return 0, false
	}
	if r.Quality != obdpid.QualityOK && r.Quality != obdpid.QualityStale {
		return 0, false
	}
	return r.Value, true
}

func readingStale(pid string) bool {
	r, ok := reading(pid)
	return ok && r.Quality == obdpid.QualityStale
}

func bleStatus() (string, color.RGBA, color.RGBA) {
	switch bleState {
	case bleScan:
		return "SCAN", darkAmber, amber
	case bleConnecting:
		return "LINK", darkCyan, cyan
	case bleConnected:
		return "ON", darkGreen, green
	case bleErr:
		return "ERR", darkRed, red
	default:
		return "OFF", panel, gray
	}
}

func elmStatus() (string, color.RGBA, color.RGBA) {
	switch elmState {
	case elmInit:
		return "INIT", darkAmber, amber
	case elmOK:
		return "OK", darkGreen, green
	case elmErr:
		return "ERR", darkRed, red
	default:
		return "WAIT", panel, gray
	}
}

func serialStatus() (string, color.RGBA, color.RGBA) {
	if ack := atomic.LoadInt64(&serialAckNano); ack != 0 {
		if time.Since(time.Unix(0, ack)) <= 3*time.Second {
			return "OK", darkGreen, green
		}
		return "DISC", darkRed, red
	}
	if !bootTime.IsZero() && time.Since(bootTime) < 5*time.Second {
		return "WAIT", darkAmber, amber
	}
	return "DISC", darkRed, red
}

func handleButtons() bool {
	now := time.Now()
	changed := false

	// Interrupt path (preferred).
	aPressed := atomic.SwapUint32(&btnA, 0) > 0
	bPressed := atomic.SwapUint32(&btnB, 0) > 0

	// Polling fallback for boards where the A/B interrupt does not fire.
	a := machine.GPIO37.Get()
	b := machine.GPIO39.Get()
	if !a && lastAState {
		aPressed = true
	}
	if !b && lastBState {
		bPressed = true
	}
	lastAState = a
	lastBState = b

	if aPressed && now.Sub(lastA) > 250*time.Millisecond {
		lastA = now
		page = (page + 1) % pageCount
		changed = true
	}
	if bPressed && now.Sub(lastB) > 250*time.Millisecond {
		lastB = now
		page = (page + pageCount - 1) % pageCount
		changed = true
	}
	return changed
}

func uiLoop() {
	for {
		serialLink.Poll()
		if uiPaused {
			time.Sleep(16 * time.Millisecond)
			continue
		}
		if handleButtons() {
			dirtyAll = true
		}

		now := time.Now()
		if now.Sub(lastStatusUI) >= 500*time.Millisecond {
			lastStatusUI = now
			if page == 0 {
				markAllStatus()
			}
		}
		if now.Sub(lastHeartbeat) >= 500*time.Millisecond {
			lastHeartbeat = now
			heartOn = !heartOn
			drawHeartbeat()
			hr := heartRect()
			flushRect(hr.x, hr.y, hr.w, hr.h)
		}

		if dirtyAll {
			composePage()
			drawHeartbeat()
			flushRect(0, 0, screenW, screenH)
			lastCompose = now
			dirtyAll = false
			dirtyTiles = 0
		} else if dirtyTiles != 0 && now.Sub(lastCompose) >= 100*time.Millisecond {
			composePage()
			drawHeartbeat()
			flushDirtyTiles()
			lastCompose = now
			dirtyTiles = 0
		}
		time.Sleep(16 * time.Millisecond)
	}
}

func markTile(p, index int) {
	if page == p && index >= 0 && index < 16 {
		dirtyTiles |= 1 << uint(index)
	}
}

func markAllStatus() {
	for i := 0; i < 6; i++ {
		markTile(0, i)
	}
}

func markPIDDirty(pid string) {
	switch page {
	case 1:
		switch pid {
		case "0D":
			markTile(1, 0)
		case "0C":
			markTile(1, 1)
		case "2F":
			markTile(1, 2)
		}
	case 2:
		switch pid {
		case "05":
			markTile(2, 0)
		case "0F":
			markTile(2, 1)
		case "3C":
			markTile(2, 2)
		}
	case 3:
		switch pid {
		case "04":
			markTile(3, 0)
		case "11":
			markTile(3, 1)
		case "44":
			markTile(3, 2)
		}
	case 4:
		if pid == "42" {
			markTile(4, 0)
		}
	case 5:
		if pid == "0D" || pid == "2F" {
			markTile(5, 0)
			markTile(5, 1)
			markTile(5, 2)
		}
	}
}

func heartRect() rect {
	return rect{screenW - 8, screenH - 6, 5, 4}
}

func drawHeartbeat() {
	r := heartRect()
	fillRect(r.x, r.y, r.w, r.h, black)
	c := gray
	if heartOn {
		c = green
	}
	fillRect(r.x, r.y, r.w, r.h, c)
}

func tileRectFor(p, index int) (rect, bool) {
	in := innerRect()
	if p == 0 {
		rowH := (in.h - gap) / 2
		colW := (in.w - 2*gap) / 3
		w3 := in.w - 2*colW - 2*gap
		if w3 < colW {
			colW--
			w3 = in.w - 2*colW - 2*gap
		}
		h2 := in.h - rowH - gap
		switch index {
		case 0:
			return rect{in.x, in.y, colW, rowH}, true
		case 1:
			return rect{in.x + colW + gap, in.y, colW, rowH}, true
		case 2:
			return rect{in.x + 2*(colW+gap), in.y, w3, rowH}, true
		case 3:
			return rect{in.x, in.y + rowH + gap, colW, h2}, true
		case 4:
			return rect{in.x + colW + gap, in.y + rowH + gap, colW, h2}, true
		case 5:
			return rect{in.x + 2*(colW+gap), in.y + rowH + gap, w3, h2}, true
		}
		return rect{}, false
	}

	topH := int16(65)
	topW := (in.w - gap) / 2
	bottomY := in.y + topH + gap
	bottomH := in.h - topH - gap
	switch index {
	case 0:
		return rect{in.x, in.y, topW, topH}, true
	case 1:
		return rect{in.x + topW + gap, in.y, topW, topH}, true
	case 2:
		return rect{in.x, bottomY, in.w, bottomH}, true
	}
	return rect{}, false
}

func flushDirtyTiles() {
	for i := 0; i < 16; i++ {
		if dirtyTiles&(1<<uint(i)) == 0 {
			continue
		}
		if r, ok := tileRectFor(page, i); ok {
			flushRect(r.x, r.y, r.w, r.h)
		}
	}
}

func composePage() {
	fillRect(0, 0, screenW, screenH, black)
	switch page {
	case 0:
		drawStatusPage()
	case 1:
		drawDataPage()
	case 2:
		drawTemperaturePage()
	case 3:
		drawEnginePage()
	case 4:
		drawDiagnosticsPage()
	case 5:
		drawTripPage()
	}
	drawPageDots()
}

func innerRect() rect {
	return rect{inset, inset, screenW - 2*inset, contentH}
}

func drawStatusPage() {
	in := innerRect()
	rowH := (in.h - gap) / 2
	colW := (in.w - 2*gap) / 3
	// distribute the rounding pixels to the first tiles
	w3 := in.w - 2*colW - 2*gap
	if w3 < colW {
		colW--
		w3 = in.w - 2*colW - 2*gap
	}
	h2 := in.h - rowH - gap
	rate := 0.0
	if !started.IsZero() {
		if s := time.Since(started).Seconds(); s > 0 {
			rate = float64(samples) / s
		}
	}

	serialValue, serialFill, serialAccent := serialStatus()
	bleValue, bleFill, bleAccent := bleStatus()
	elmValue, elmFill, elmAccent := elmStatus()
	errCol := green
	errFill := panel
	if errCount > 0 {
		errCol = red
		errFill = darkRed
	}

	drawStatusTile(rect{in.x, in.y, colW, rowH}, iconTx, "SERIAL", serialValue, serialFill, serialAccent)
	drawStatusTile(rect{in.x + colW + gap, in.y, colW, rowH}, iconBLE, "BLE", bleValue, bleFill, bleAccent)
	drawStatusTile(rect{in.x + 2*(colW+gap), in.y, w3, rowH}, iconLink, "ELM", elmValue, elmFill, elmAccent)
	drawStatusTile(rect{in.x, in.y + rowH + gap, colW, h2}, iconRate, "PID/s", strconv.FormatFloat(rate, 'f', 1, 64), panel, cyan)
	drawStatusTile(rect{in.x + colW + gap, in.y + rowH + gap, colW, h2}, iconTx, "TX", strconv.Itoa(txLines), darkAmber, amber)
	drawStatusTile(rect{in.x + 2*(colW+gap), in.y + rowH + gap, w3, h2}, iconErr, "ERR", strconv.Itoa(errCount), errFill, errCol)
}

func drawStatusTile(r rect, icon int, label, value string, fill, accent color.RGBA) {
	fillRect(r.x, r.y, r.w, r.h, fill)
	fillRect(r.x, r.y, r.w, 2, accent)
	drawIcon(icon, r.x+7, r.y+9, accent)
	text(&proggy.TinySZ8pt7b, r.x+22, r.y+16, label, gray)
	drawFitCentered(r, r.y+48, value, accent)
}

func drawTemperaturePage() {
	in := innerRect()
	topH := int16(65)
	topW := (in.w - gap) / 2
	bottomY := in.y + topH + gap
	bottomH := in.h - topH - gap

	t := pages[2].Tiles
	cool, coolOK := readingNumber(t[0].PID)
	intake, intakeOK := readingNumber(t[1].PID)
	cat, catOK := readingNumber(t[2].PID)
	coolV, intakeV, catV := -1, -1, -1
	if coolOK {
		coolV = int(cool)
	}
	if intakeOK {
		intakeV = int(intake)
	}
	if catOK {
		catV = int(cat)
	}
	drawTempTile(rect{in.x, in.y, topW, topH}, t[0].Icon, t[0].Label, coolV, readingStale(t[0].PID), 120, 100, 110)
	drawTempTile(rect{in.x + topW + gap, in.y, topW, topH}, t[1].Icon, t[1].Label, intakeV, readingStale(t[1].PID), 80, 50, 70)
	drawCatalystTile(rect{in.x, bottomY, in.w, bottomH}, t[2].Label, catV, readingStale(t[2].PID))
}

func drawTempTile(r rect, icon int, label string, v int, stale bool, max, warn, hot int) {
	c := tempColor(v, warn, hot)
	if stale {
		c = amber
	}
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(icon, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, label, gray)
	val := "--"
	if v >= 0 {
		val = strconv.Itoa(v)
	}
	drawFitCentered(r, r.y+43, val, c)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+57, "C", gray)
	fillRect(r.x+12, r.y+59, r.w-24, 4, gray)
	if v > 0 {
		n := int16(int(r.w-24) * v / max)
		if n > r.w-24 {
			n = r.w - 24
		}
		fillRect(r.x+12, r.y+59, n, 4, c)
	}
}

func drawCatalystTile(r rect, label string, cat int, stale bool) {
	c := tempColor(cat, 700, 900)
	if stale {
		c = amber
	}
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(iconCat, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, label, gray)
	val := "--"
	if cat >= 0 {
		val = strconv.Itoa(cat) + " C"
	}
	text(&freemono.Bold18pt7b, r.x+70, r.y+42, val, c)
	fillRect(r.x+12, r.y+50, r.w-24, 6, gray)
	if cat > 0 {
		n := int16(int(r.w-24) * cat / 900)
		if n > r.w-24 {
			n = r.w - 24
		}
		fillRect(r.x+12, r.y+50, n, 6, c)
	}
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

func drawEnginePage() {
	in := innerRect()
	topH := int16(65)
	topW := (in.w - gap) / 2
	bottomY := in.y + topH + gap
	bottomH := in.h - topH - gap

	t := pages[3].Tiles
	load, loadOK := readingNumber(t[0].PID)
	thr, thrOK := readingNumber(t[1].PID)
	lambda, lambdaOK := readingNumber(t[2].PID)
	if !loadOK {
		load = -1
	}
	if !thrOK {
		thr = -1
	}
	if !lambdaOK {
		lambda = -1
	}
	loadC := loadColor(load)
	thrC := throttleColor(thr)
	if readingStale(t[0].PID) {
		loadC = amber
	}
	if readingStale(t[1].PID) {
		thrC = amber
	}
	drawPercentTile(rect{in.x, in.y, topW, topH}, t[0].Icon, t[0].Label, load, loadC)
	drawPercentTile(rect{in.x + topW + gap, in.y, topW, topH}, t[1].Icon, t[1].Label, thr, thrC)
	drawLambdaTile(rect{in.x, bottomY, in.w, bottomH}, lambda, readingStale(t[2].PID))
}

func drawPercentTile(r rect, icon int, label string, v float64, c color.RGBA) {
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(icon, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, label, gray)
	val := "--"
	pct := 0
	if v >= 0 {
		pct = int(v)
		val = strconv.Itoa(pct)
	}
	drawFitCentered(r, r.y+43, val, c)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+57, "%", gray)
	fillRect(r.x+12, r.y+59, r.w-24, 4, gray)
	if pct > 0 {
		fillRect(r.x+12, r.y+59, int16(int(r.w-24)*pct/100), 4, c)
	}
}

func drawLambdaTile(r rect, lambda float64, stale bool) {
	c := lambdaColor(lambda)
	if stale {
		c = amber
	}
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(iconAir, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, "LAMBDA", gray)
	val := "--"
	if lambda >= 0 {
		val = strconv.FormatFloat(lambda, 'f', 2, 64)
	}
	text(&freemono.Bold18pt7b, r.x+70, r.y+42, val, c)
	text(&proggy.TinySZ8pt7b, r.x+190, r.y+42, "lambda", gray)
	fillRect(r.x+12, r.y+50, r.w-24, 6, gray)
	if lambda > 0 {
		pct := int((lambda - 0.7) * 100 / 0.6)
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		fillRect(r.x+12, r.y+50, int16(int(r.w-24)*pct/100), 6, c)
		markerX := r.x + 12 + int16(int(r.w-24)*50/100)
		fillRect(markerX, r.y+48, 1, 10, white)
	}
}

func loadColor(v float64) color.RGBA {
	if v < 0 {
		return gray
	}
	if v >= 85 {
		return red
	}
	if v >= 60 {
		return amber
	}
	return green
}

func throttleColor(v float64) color.RGBA {
	if v < 0 {
		return gray
	}
	if v >= 80 {
		return red
	}
	if v >= 50 {
		return amber
	}
	return green
}

func lambdaColor(v float64) color.RGBA {
	if v < 0 {
		return gray
	}
	if v < 0.9 || v > 1.1 {
		return red
	}
	if v < 0.95 || v > 1.05 {
		return amber
	}
	return green
}

func drawDiagnosticsPage() {
	in := innerRect()
	topH := int16(65)
	topW := (in.w - gap) / 2
	bottomY := in.y + topH + gap
	bottomH := in.h - topH - gap

	drawVoltageTile(rect{in.x, in.y, topW, topH})
	drawMILTile(rect{in.x + topW + gap, in.y, topW, topH})
	drawDTCTile(rect{in.x, bottomY, in.w, bottomH})
}

func drawVoltageTile(r rect) {
	v, ok := readingNumber("42")
	if !ok {
		v = -1
	}
	c := cyan
	if v >= 0 && v < 11.8 {
		c = red
	} else if v >= 0 && v < 12.2 {
		c = amber
	}
	if readingStale("42") {
		c = amber
	}
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(iconGauge, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, "TENSAO", gray)
	val := "--"
	if v >= 0 {
		val = strconv.FormatFloat(v, 'f', 1, 64)
	}
	drawFitCentered(r, r.y+43, val, c)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+57, "V", gray)
}

func drawMILTile(r rect) {
	c := green
	if milOn {
		c = red
	}
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(iconErr, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, "MIL", gray)
	val := "OFF"
	if milOn {
		val = "ON"
	}
	drawFitCentered(r, r.y+43, val, c)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+57, "check engine", gray)
}

func drawDTCTile(r rect) {
	c := green
	if dtcCount > 0 {
		c = red
	}
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(iconErr, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, "DTCS", gray)
	val := "--"
	if dtcCount >= 0 {
		val = strconv.Itoa(dtcCount)
	}
	text(&freemono.Bold18pt7b, r.x+70, r.y+42, val, c)
	text(&proggy.TinySZ8pt7b, r.x+190, r.y+42, "confirmados", gray)
}

func drawTripPage() {
	in := innerRect()
	topH := int16(65)
	topW := (in.w - gap) / 2
	bottomY := in.y + topH + gap
	bottomH := in.h - topH - gap

	used := 0.0
	fuel, fuelOK := readingNumber("2F")
	if tripFuelStart >= 0 && fuelOK {
		used = (tripFuelStart - fuel) * 0.44
		if used < 0 {
			used = 0
		}
	}
	avg := -1.0
	if used > 0.05 {
		avg = tripKm / used
	}

	drawTripValueTile(rect{in.x, in.y, topW, topH}, iconGauge, "DISTANCIA", strconv.FormatFloat(tripKm, 'f', 2, 64), "km", cyan)
	drawTripValueTile(rect{in.x + topW + gap, in.y, topW, topH}, iconFuel, "COMBUSTIVEL", strconv.FormatFloat(used, 'f', 2, 64), "L usados", amber)
	drawConsumptionTile(rect{in.x, bottomY, in.w, bottomH}, avg)
}

func drawTripValueTile(r rect, icon int, label, value, unit string, c color.RGBA) {
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(icon, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, label, gray)
	drawFitCentered(r, r.y+43, value, c)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+57, unit, gray)
}

func drawConsumptionTile(r rect, avg float64) {
	c := green
	if avg < 0 || avg < 5 {
		c = red
	} else if avg < 10 {
		c = amber
	}
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(iconRate, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, "CONSUMO", gray)
	val := "--"
	if avg >= 0 {
		val = strconv.FormatFloat(avg, 'f', 1, 64)
	}
	text(&freemono.Bold18pt7b, r.x+70, r.y+42, val, c)
	text(&proggy.TinySZ8pt7b, r.x+190, r.y+42, "km/L", gray)
}

func drawDataPage() {
	in := innerRect()
	topH := int16(65)
	topW := (in.w - gap) / 2
	bottomY := in.y + topH + gap
	bottomH := in.h - topH - gap

	t := pages[1].Tiles
	speed, speedOK := readingNumber(t[0].PID)
	rpm, rpmOK := readingNumber(t[1].PID)
	speedS, rpmS := "--", "--"
	if speedOK {
		speedS = strconv.Itoa(int(speed))
	}
	if rpmOK {
		rpmS = strconv.Itoa(int(rpm))
	}
	speedC, rpmC := cyan, white
	if readingStale("0D") {
		speedC = amber
	}
	if readingStale("0C") {
		rpmC = amber
	}
	drawDataTile(rect{in.x, in.y, topW, topH}, t[0].Icon, t[0].Label, speedS, t[0].Unit, speedC)
	drawDataTile(rect{in.x + topW + gap, in.y, topW, topH}, t[1].Icon, t[1].Label, rpmS, t[1].Unit, rpmC)
	fuelPct, fuelOK2 := readingNumber(t[2].PID)
	drawFuelTile(rect{in.x, bottomY, in.w, bottomH}, t[2].Label, fuelPct, fuelOK2, readingStale(t[2].PID))
}

func drawDataTile(r rect, icon int, label, value, unit string, c color.RGBA) {
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(icon, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, label, gray)
	drawFitCentered(r, r.y+45, value, c)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+59, unit, gray)
}

func drawFuelTile(r rect, label string, fuel float64, ok bool, stale bool) {
	c := green
	if stale {
		c = amber
	}
	fillRect(r.x, r.y, r.w, r.h, panel)
	fillRect(r.x, r.y, r.w, 2, c)
	drawIcon(iconFuel, r.x+8, r.y+9, c)
	text(&proggy.TinySZ8pt7b, r.x+24, r.y+16, label, gray)
	val := "--"
	pct := 0
	if ok {
		pct = int(fuel)
		val = strconv.FormatFloat(fuel*0.44, 'f', 1, 64) + " L"
	}
	text(&freemono.Bold18pt7b, r.x+70, r.y+42, val, green)
	text(&proggy.TinySZ8pt7b, r.x+190, r.y+42, strconv.Itoa(pct)+"%", gray)
	fillRect(r.x+12, r.y+50, r.w-24, 6, gray)
	if pct > 0 {
		fillRect(r.x+12, r.y+50, int16(int(r.w-24)*pct/100), 6, green)
	}
}

func drawFitCentered(r rect, baseline int16, s string, c color.RGBA) {
	f := tinyfont.Fonter(&freemono.Bold18pt7b)
	if textWidth(f, s)+8 > r.w {
		f = &freemono.Bold12pt7b
	}
	textCentered(f, r, baseline, s, c)
}

func drawPageDots() {
	x := int16(screenW/2 - pageCount*5)
	for i := 0; i < pageCount; i++ {
		c := gray
		if i == page {
			c = white
		}
		fillRect(x+int16(i*10), screenH-2, 4, 2, c)
	}
}

func drawIcon(kind int, x, y int16, c color.RGBA) {
	switch kind {
	case iconBLE:
		fillRect(x+0, y+6, 2, 4, c)
		fillRect(x+4, y+3, 2, 7, c)
		fillRect(x+8, y+0, 2, 10, c)
	case iconLink:
		drawRectOutline(x, y+1, 6, 5, c)
		drawRectOutline(x+4, y+5, 6, 5, c)
		fillRect(x+5, y+4, 2, 2, c)
	case iconRate:
		fillRect(x+0, y+7, 2, 3, c)
		fillRect(x+4, y+4, 2, 6, c)
		fillRect(x+8, y+0, 2, 10, c)
	case iconTx:
		fillRect(x+4, y+2, 2, 9, c)
		drawLine(x+4, y+2, x+1, y+5, c)
		drawLine(x+5, y+2, x+8, y+5, c)
	case iconErr:
		fillRect(x+4, y+0, 2, 7, c)
		fillRect(x+4, y+8, 2, 2, c)
	case iconClock:
		drawCircle(x+5, y+5, 4, c)
		drawLine(x+5, y+5, x+5, y+2, c)
		drawLine(x+5, y+5, x+8, y+6, c)
	case iconGauge:
		drawCircle(x+5, y+5, 4, c)
		drawLine(x+5, y+5, x+8, y+2, c)
		fillRect(x+4, y+4, 3, 2, c)
	case iconFuel:
		drawRectOutline(x+1, y+1, 6, 9, c)
		fillRect(x+7, y+2, 2, 5, c)
		fillRect(x+9, y+3, 1, 4, c)
	case iconThermo:
		drawRectOutline(x+3, y, 4, 8, c)
		drawCircle(x+5, y+8, 2, c)
	case iconAir:
		fillRect(x, y+1, 9, 1, c)
		fillRect(x+2, y+4, 8, 1, c)
		fillRect(x, y+7, 9, 1, c)
	case iconCat:
		drawRectOutline(x, y+1, 10, 8, c)
		fillRect(x+2, y+3, 6, 1, c)
		fillRect(x+2, y+6, 6, 1, c)
	}
}

func drawRectOutline(x, y, w, h int16, c color.RGBA) {
	fillRect(x, y, w, 1, c)
	fillRect(x, y+h-1, w, 1, c)
	fillRect(x, y, 1, h, c)
	fillRect(x+w-1, y, 1, h, c)
}

func drawLine(x0, y0, x1, y1 int16, c color.RGBA) {
	dx := abs16(x1 - x0)
	dy := -abs16(y1 - y0)
	sx := int16(-1)
	if x0 < x1 {
		sx = 1
	}
	sy := int16(-1)
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		fillRect(x0, y0, 1, 1, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func drawCircle(cx, cy, r int16, c color.RGBA) {
	x := r
	y := int16(0)
	err := int16(1) - r
	for x >= y {
		pixel4(cx, cy, x, y, c)
		y++
		if err < 0 {
			err += 2*y + 1
		} else {
			x--
			err += 2*(y-x) + 1
		}
	}
}

func pixel4(cx, cy, x, y int16, c color.RGBA) {
	fillRect(cx+x, cy+y, 1, 1, c)
	fillRect(cx-x, cy+y, 1, 1, c)
	fillRect(cx+x, cy-y, 1, 1, c)
	fillRect(cx-x, cy-y, 1, 1, c)
	fillRect(cx+y, cy+x, 1, 1, c)
	fillRect(cx-y, cy+x, 1, 1, c)
	fillRect(cx+y, cy-x, 1, 1, c)
	fillRect(cx-y, cy-x, 1, 1, c)
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}

func statusColor(ok bool) color.RGBA {
	if ok {
		return green
	}
	return red
}

func onOff(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}

func yesNo(v bool) string {
	if v {
		return "OK"
	}
	return "ERR"
}

func pad2(v int) string {
	if v < 10 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}

func text(f tinyfont.Fonter, x, y int16, s string, c color.RGBA) {
	tinyfont.WriteLine(cv, f, x, y, s, c)
}

func textWidth(f tinyfont.Fonter, s string) int16 {
	w, _ := tinyfont.LineWidth(f, s)
	return int16(w)
}

func textCentered(f tinyfont.Fonter, r rect, baseline int16, s string, c color.RGBA) {
	x := r.x + (r.w-textWidth(f, s))/2
	text(f, x, baseline, s, c)
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

func flushRect(x, y, w, h int16) {
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
	for by := int16(0); by < h; by += bandRows {
		bh := h - by
		if bh > bandRows {
			bh = bandRows
		}
		for r := int16(0); r < bh; r++ {
			for xx := int16(0); xx < w; xx++ {
				c := palette[getPixel(int(x+xx), int(y+by+r))]
				v := rgb565(c)
				off := (int(r)*int(w) + int(xx)) * 2
				lineBuf[off] = byte(v >> 8)
				lineBuf[off+1] = byte(v)
			}
		}
		display.DrawRGBBitmap8(x, y+by, lineBuf[:int(bh*w)*2], w, bh)
	}
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

func rgb565(c color.RGBA) uint16 {
	return uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3)
}
