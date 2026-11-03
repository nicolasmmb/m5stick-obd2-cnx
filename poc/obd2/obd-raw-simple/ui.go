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
	bandRows = 4
)

var (
	display st7789.Device
	fb      [screenW * screenH / 2]byte
	lineBuf [screenW * bandRows * 2]byte

	bg        = color.RGBA{30, 35, 42, 255}
	card      = color.RGBA{42, 48, 56, 255}
	highlight = color.RGBA{74, 83, 97, 255}
	shadow    = color.RGBA{21, 24, 29, 255}
	textMain  = color.RGBA{230, 233, 239, 255}
	textMuted = color.RGBA{154, 164, 178, 255}
	green     = color.RGBA{53, 196, 106, 255}
	amber     = color.RGBA{245, 165, 36, 255}
	red       = color.RGBA{229, 72, 77, 255}
	info      = color.RGBA{76, 201, 240, 255}
	off       = color.RGBA{107, 114, 128, 255}

	darkGreen = color.RGBA{20, 42, 28, 255}
	darkAmber = color.RGBA{48, 36, 12, 255}
	darkRed   = color.RGBA{48, 20, 22, 255}

	palette = [...]color.RGBA{
		bg, card, highlight, shadow, textMain, textMuted,
		green, amber, red, info, off, darkGreen, darkAmber, darkRed,
	}
)

type rect struct{ x, y, w, h int16 }

type canvas struct{}

func (*canvas) Size() (int16, int16) { return screenW, screenH }
func (*canvas) Display() error       { return nil }

func (*canvas) SetPixel(x, y int16, c color.RGBA) {
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

var cv = &canvas{}

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
	display.IsBGR(false)
	display.Configure(st7789.Config{
		Width:        135,
		Height:       240,
		Rotation:     drivers.Rotation90,
		RowOffset:    40,
		ColumnOffset: 52,
		FrameRate:    st7789.FRAMERATE_60,
	})
	display.FillScreen(bg)
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
				o := (int(r)*int(w) + int(xx)) * 2
				lineBuf[o] = byte(v >> 8)
				lineBuf[o+1] = byte(v)
			}
		}
		display.DrawRGBBitmap8(x, y+by, lineBuf[:int(bh*w)*2], w, bh)
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
	return 0
}

func rgb565(c color.RGBA) uint16 {
	return uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3)
}

func text(f *tinyfont.Font, x, y int16, s string, c color.RGBA) {
	nx := x
	for _, r := range s {
		g := glyphFor(f, r)
		g.Draw(cv, nx, y, c)
		nx += int16(g.XAdvance)
	}
}

func glyphFor(f *tinyfont.Font, r rune) tinyfont.Glyph {
	lo, hi := 0, len(f.Glyphs)-1
	for lo <= hi {
		m := (lo + hi) / 2
		if f.Glyphs[m].Rune < r {
			lo = m + 1
		} else {
			hi = m - 1
		}
	}
	if lo < len(f.Glyphs) && f.Glyphs[lo].Rune == r {
		return f.Glyphs[lo]
	}
	return tinyfont.Glyph{}
}

func textWidth(f *tinyfont.Font, s string) int16 {
	w := int16(0)
	for _, r := range s {
		w += int16(glyphFor(f, r).XAdvance)
	}
	return w
}

func drawRow(y int16, label, value string, level int) {
	r := rect{6, y, screenW - 12, 22}
	fillRect(r.x, r.y, r.w, r.h, card)
	accent := textMuted
	switch level {
	case 1:
		accent = green
		fillRect(r.x, r.y, r.w, r.h, darkGreen)
	case 2:
		accent = amber
		fillRect(r.x, r.y, r.w, r.h, darkAmber)
	case 3:
		accent = red
		fillRect(r.x, r.y, r.w, r.h, darkRed)
	case 4:
		accent = info
	}
	fillRect(r.x, r.y, r.w, 1, highlight)
	fillRect(r.x, r.y, 1, r.h, highlight)
	fillRect(r.x, r.y+r.h-1, r.w, 1, shadow)
	fillRect(r.x+r.w-1, r.y, 1, r.h, shadow)
	fillRect(r.x, r.y+1, 3, r.h-2, accent)
	text(&proggy.TinySZ8pt7b, r.x+10, r.y+15, label, textMuted)
	font := &freemono.Bold18pt7b
	if textWidth(font, value) > 90 {
		font = &freemono.Bold12pt7b
	}
	text(font, r.x+120, r.y+18, value, textMain)
}

func serialStatus() (string, int) {
	if hostAlive() {
		return "OK", 1
	}
	if atomic.LoadInt64(&lastHostNano) == 0 {
		return "WAIT", 2
	}
	return "DISC", 3
}

func bleStatus() (string, int) {
	switch bleState {
	case bleScan:
		return "SCAN", 2
	case bleOn:
		return "ON", 1
	default:
		return "OFF", 0
	}
}

func elmStatus() (string, int) {
	switch elmState {
	case elmRX:
		return "RX", 1
	case elmTimeout:
		return "TIMEOUT", 3
	default:
		return "WAIT", 2
	}
}

var (
	heartOn  bool
	lastDraw time.Time
	lastBeat time.Time
)

func drawAll() {
	fillRect(0, 0, screenW, screenH, bg)
	text(&proggy.TinySZ8pt7b, 8, 14, "RAW BRIDGE", textMuted)
	c := info
	if recording {
		c = green
	}
	if heartOn {
		fillRect(224, 6, 6, 6, c)
	} else {
		fillRect(224, 6, 6, 6, textMuted)
	}

	s, sl := serialStatus()
	b, bl := bleStatus()
	e, el := elmStatus()
	drawRow(22, "SERIAL", s, sl)
	drawRow(48, "BLE", b, bl)
	drawRow(74, "ELM", e, el)
	drawRow(100, "TX", strconv.FormatUint(uint64(atomic.LoadUint32(&txCount)), 10), 0)
}

func statusKey() uint32 {
	_, sl := serialStatus()
	_, bl := bleStatus()
	_, el := elmStatus()
	return uint32(sl)<<16 | uint32(bl)<<8 | uint32(el)
}

func uiLoop() {
	var lastKey uint32
	first := true
	for {
		now := time.Now()
		key := statusKey()
		if first || key != lastKey || now.Sub(lastDraw) >= 500*time.Millisecond {
			first = false
			lastKey = key
			lastDraw = now
			drawAll()
			flushRect(0, 0, screenW, screenH)
		}
		if now.Sub(lastBeat) >= 500*time.Millisecond {
			lastBeat = now
			heartOn = !heartOn
			drawAll()
			flushRect(0, 0, screenW, screenH)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
