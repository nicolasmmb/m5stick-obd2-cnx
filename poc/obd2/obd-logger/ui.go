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
)

const (
	screenW  = 240
	screenH  = 135
	bandRows = 24
)

var (
	display st7789.Device
	fb      [screenW * screenH / 2]byte
	lineBuf [screenW * bandRows * 2]byte

	black = color.RGBA{0, 0, 0, 255}
	white = color.RGBA{235, 235, 235, 255}
	gray  = color.RGBA{80, 80, 80, 255}
	cyan  = color.RGBA{0, 220, 220, 255}
	green = color.RGBA{0, 220, 90, 255}
	red   = color.RGBA{255, 80, 80, 255}
	amber = color.RGBA{255, 180, 0, 255}

	palette = [...]color.RGBA{black, white, gray, cyan, green, red, amber}

	page int

	btnA, btnB uint32
	lastA      time.Time
	lastB      time.Time

	bleOK    bool
	elmOK    bool
	txLines  int
	errCount int
	samples  int
	started  time.Time

	latestSpeed int
	latestRPM   int
	latestFuel  float64
	lastDraw    time.Time
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
	for _, p := range []machine.Pin{machine.GPIO37, machine.GPIO39} {
		p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	_ = machine.GPIO37.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnA, 1)
	})
	_ = machine.GPIO39.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnB, 1)
	})
}

func handleButtons() bool {
	now := time.Now()
	changed := false
	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 250*time.Millisecond {
		lastA = now
		page = (page + 1) % 2
		changed = true
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 250*time.Millisecond {
		lastB = now
		page = (page + 1) % 2
		changed = true
	}
	return changed
}

func tickUI() {
	if handleButtons() {
		drawPage()
		return
	}
	if time.Since(lastDraw) > 500*time.Millisecond {
		drawPage()
	}
}

func drawPage() {
	fillRect(0, 0, screenW, screenH, black)
	if page == 0 {
		drawStatusPage()
	} else {
		drawDataPage()
	}
	drawPageDots()
	flushRows(0, screenH-1)
	lastDraw = time.Now()
}

func drawStatusPage() {
	g := grid(3, 2)
	rate := 0.0
	if !started.IsZero() {
		secs := time.Since(started).Seconds()
		if secs > 0 {
			rate = float64(samples) / secs
		}
	}
	up := "00:00"
	if !started.IsZero() {
		d := time.Since(started)
		up = pad2(int(d/time.Minute)) + ":" + pad2(int((d/time.Second)%60))
	}
	drawTile(tileRect(g, 0, 0), "BLE", onOff(bleOK), statusColor(bleOK))
	drawTile(tileRect(g, 1, 0), "ELM", yesNo(elmOK), statusColor(elmOK))
	drawTile(tileRect(g, 2, 0), "PID/s", strconv.FormatFloat(rate, 'f', 1, 64), cyan)
	drawTile(tileRect(g, 0, 1), "TX", strconv.Itoa(txLines), white)
	drawTile(tileRect(g, 1, 1), "ERR", strconv.Itoa(errCount), errColor())
	drawTile(tileRect(g, 2, 1), "TIME", up, white)
}

func drawDataPage() {
	g := grid(3, 1)
	drawDataTile(tileRect(g, 0, 0), "VEL", strconv.Itoa(latestSpeed), "km/h", cyan)
	drawDataTile(tileRect(g, 1, 0), "RPM", strconv.Itoa(latestRPM), "rpm", white)
	fuel := "0.0"
	if latestFuel >= 0 {
		fuel = strconv.FormatFloat(latestFuel*0.44, 'f', 1, 64)
	}
	drawDataTile(tileRect(g, 2, 0), "COMB", fuel, "L", green)
}

func drawTile(r rect, label, value string, c color.RGBA) {
	fillRect(r.x, r.y, r.w, r.h, black)
	drawBorder(r, gray)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+14, label, gray)
	f := tinyfont.Fonter(&freemono.Bold18pt7b)
	if textWidth(f, value)+4 > r.w {
		f = &freemono.Bold12pt7b
	}
	textCentered(f, r, r.y+40, value, c)
}

func drawDataTile(r rect, label, value, unit string, c color.RGBA) {
	fillRect(r.x, r.y, r.w, r.h, black)
	drawBorder(r, gray)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+14, label, gray)
	f := tinyfont.Fonter(&freemono.Bold18pt7b)
	if textWidth(f, value)+4 > r.w {
		f = &freemono.Bold12pt7b
	}
	textCentered(f, r, r.y+42, value, c)
	textCentered(&proggy.TinySZ8pt7b, r, r.y+58, unit, gray)
}

func drawPageDots() {
	x := int16(screenW/2 - 8)
	for i := 0; i < 2; i++ {
		c := gray
		if i == page {
			c = white
		}
		fillRect(x+int16(i*10), screenH-3, 4, 2, c)
	}
}

func drawBorder(r rect, c color.RGBA) {
	fillRect(r.x, r.y, r.w, 1, c)
	fillRect(r.x, r.bottom()-1, r.w, 1, c)
	fillRect(r.x, r.y, 1, r.h, c)
	fillRect(r.x+r.w-1, r.y, 1, r.h, c)
}

type gridSpec struct {
	cols, rows int16
	side       int16
	x0, y0     int16
	gap        int16
}

func grid(cols, rows int16) gridSpec {
	const margin int16 = 2
	const gap int16 = 2
	aw := int16(screenW) - 2*margin - (cols-1)*gap
	ah := int16(screenH) - 2*margin - (rows-1)*gap
	side := aw / cols
	if v := ah / rows; v < side {
		side = v
	}
	gw := cols*side + (cols-1)*gap
	gh := rows*side + (rows-1)*gap
	return gridSpec{cols, rows, side, (int16(screenW) - gw) / 2, (int16(screenH) - gh) / 2, gap}
}

func tileRect(g gridSpec, col, row int16) rect {
	return rect{g.x0 + col*(g.side+g.gap), g.y0 + row*(g.side+g.gap), g.side, g.side}
}

func statusColor(ok bool) color.RGBA {
	if ok {
		return green
	}
	return red
}

func errColor() color.RGBA {
	if errCount == 0 {
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
