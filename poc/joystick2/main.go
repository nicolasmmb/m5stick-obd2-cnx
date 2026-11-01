// POC: M5Stack Unit Joystick2 (U024-V2) test screen.
//
// Grove I2C1: SDA=G32, SCL=G33, address 0x63. The Grove port is shared with
// the other units, so only one can be connected at a time.
package main

import (
	"fmt"
	"image/color"
	"machine"
	"sync/atomic"
	"time"

	joystick2 "github.com/nicolasmmb/M5Unit-Joystick2-Go"

	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
	"tinygo.org/x/tinyfont/proggy"
)

const (
	screenW = 240
	screenH = 135

	pageVector = iota
	pageRaw
	pageInfo
	pageCount
)

type preset struct {
	name string
	c    joystick2.Color
}

var presets = [...]preset{
	{"OFF", joystick2.Color{}},
	{"VERMELHO", joystick2.Color{R: 255}},
	{"VERDE", joystick2.Color{G: 255}},
	{"AZUL", joystick2.Color{B: 255}},
	{"CIANO", joystick2.Color{G: 255, B: 255}},
	{"MAGENTA", joystick2.Color{R: 255, B: 255}},
	{"AMARELO", joystick2.Color{R: 255, G: 255}},
	{"BRANCO", joystick2.Color{R: 255, G: 255, B: 255}},
}

var brightnessLevels = [...]uint8{25, 50, 100}

var (
	black    = color.RGBA{0, 0, 0, 255}
	white    = color.RGBA{235, 235, 235, 255}
	gray     = color.RGBA{110, 110, 110, 255}
	darkGray = color.RGBA{45, 45, 45, 255}
	cyan     = color.RGBA{0, 220, 220, 255}
	green    = color.RGBA{0, 220, 90, 255}
	yellow   = color.RGBA{255, 210, 0, 255}
	red      = color.RGBA{255, 70, 70, 255}

	display st7789.Device
	fb      canvas

	btnA, btnB, btnC    uint32
	lastA, lastB, lastC time.Time

	page       int
	colorIndex int
	brightIdx  = 2

	last       uiState
	lastRender time.Time
	lastLog    time.Time

	rateSamples uint32
	rateValue   int
	lastRate    time.Time

	dev *joystick2.Device
)

type uiState struct {
	page    int
	online  bool
	x, y    int16
	pressed bool
	rawX    uint16
	rawY    uint16
	raw8X   uint8
	raw8Y   uint8
	pos8X   int8
	pos8Y   int8
	rate    int
	fw      uint8
	boot    uint8
	addr    uint8
	color   int
	bright  int
	cal     joystick2.Calibration
}

type canvas struct {
	img pixel.Image[pixel.RGB565BE]
}

func newCanvas() canvas {
	return canvas{img: pixel.NewImage[pixel.RGB565BE](screenW, screenH)}
}

func (c *canvas) clear() {
	c.img.FillSolidColor(pixel.NewColor[pixel.RGB565BE](0, 0, 0))
}

func (c *canvas) flush() { display.DrawBitmap(0, 0, c.img) }

func (c *canvas) fillRect(x, y, w, h int16, col color.RGBA) {
	pc := pixel.NewColor[pixel.RGB565BE](col.R, col.G, col.B)
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if xx >= 0 && yy >= 0 && xx < screenW && yy < screenH {
				c.img.Set(int(xx), int(yy), pc)
			}
		}
	}
}

func (c *canvas) SetPixel(x, y int16, col color.RGBA) {
	if x < 0 || y < 0 || x >= screenW || y >= screenH {
		return
	}
	c.img.Set(int(x), int(y), pixel.NewColor[pixel.RGB565BE](col.R, col.G, col.B))
}

func (c *canvas) Size() (int16, int16) { return screenW, screenH }
func (c *canvas) Display() error       { return nil }

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	initDisplay()
	fb = newCanvas()
	setupButtons()

	drawBoot("I2C 100 kHz")

	i2c := machine.I2C1
	if err := i2c.Configure(machine.I2CConfig{
		SDA:       machine.GPIO32,
		SCL:       machine.GPIO33,
		Frequency: 100 * machine.KHz,
	}); err != nil {
		println("joystick2: i2c", err.Error())
	}

	dev = joystick2.New(i2c)
	for !dev.Present() {
		drawOffline("sem resposta em 0x63")
		println("joystick2: offline")
		time.Sleep(2 * time.Second)
	}
	// This unit's Y axis is positive downwards (stick up reads -4095), which is
	// also what the official driver reports. Invert Y so the crosshair and the
	// numeric value follow the physical stick direction.
	dev.SetOrientation(joystick2.Orientation{InvertY: true})
	fw, _ := dev.FirmwareVersion()
	boot, _ := dev.BootloaderVersion()
	_ = dev.SetLED(presets[colorIndex].c)
	_ = dev.SetBrightness(brightnessLevels[brightIdx])
	println("joystick2: online fw", fw, "boot", boot, "addr", dev.Address())

	lastRate = time.Now()
	for {
		handleButtons()

		s, err := dev.Sample()
		var st uiState
		if err != nil {
			println("joystick2: sample", err.Error())
			st = uiState{page: page, online: false, fw: fw, boot: boot, addr: dev.Address()}
		} else {
			atomic.AddUint32(&rateSamples, 1)
			st = uiState{
				page:    page,
				online:  true,
				x:       s.X,
				y:       s.Y,
				pressed: s.Pressed,
				fw:      fw,
				boot:    boot,
				addr:    dev.Address(),
				color:   colorIndex,
				bright:  brightIdx,
				rate:    currentRate(),
			}
			if time.Since(lastRaw) > 200*time.Millisecond {
				rawX, rawY, _ = dev.Raw()
				raw8X, raw8Y, _ = dev.Raw8()
				pos8X, pos8Y, _ = dev.Position8()
				lastRaw = time.Now()
			}
			st.rawX, st.rawY = rawX, rawY
			st.raw8X, st.raw8Y = raw8X, raw8Y
			st.pos8X, st.pos8Y = pos8X, pos8Y
			if page == pageInfo && time.Since(lastCal) > time.Second {
				st.cal, _ = dev.Calibration()
				lastCal = time.Now()
			}
		}
		render(st)

		if time.Since(lastLog) > time.Second {
			println("joystick2", "x", int(st.x), "y", int(st.y),
				"pressed", st.pressed, "raw", int(st.rawX), int(st.rawY),
				"rate", st.rate, "mode", presets[colorIndex].name,
				"bright", brightnessLevels[brightIdx], "addr", fmt.Sprintf("0x%02X", st.addr))
			lastLog = time.Now()
		}
		time.Sleep(20 * time.Millisecond)
	}
}

var (
	lastRaw time.Time
	lastCal time.Time

	rawX, rawY   uint16
	raw8X, raw8Y uint8
	pos8X, pos8Y int8
)

func currentRate() int {
	now := time.Now()
	samples := atomic.LoadUint32(&rateSamples)
	elapsed := now.Sub(lastRate)
	if elapsed < time.Second {
		return rateValue
	}
	rateValue = int(int64(samples) * int64(time.Second) / int64(elapsed))
	atomic.StoreUint32(&rateSamples, 0)
	lastRate = now
	return rateValue
}

func setupButtons() {
	for _, p := range []machine.Pin{machine.GPIO37, machine.GPIO39, machine.GPIO35} {
		p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	_ = machine.GPIO37.SetInterrupt(machine.PinFalling, func(machine.Pin) { atomic.AddUint32(&btnA, 1) })
	_ = machine.GPIO39.SetInterrupt(machine.PinFalling, func(machine.Pin) { atomic.AddUint32(&btnB, 1) })
	_ = machine.GPIO35.SetInterrupt(machine.PinFalling, func(machine.Pin) { atomic.AddUint32(&btnC, 1) })
}

func handleButtons() {
	now := time.Now()
	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 250*time.Millisecond {
		lastA = now
		colorIndex = (colorIndex + 1) % len(presets)
		_ = dev.SetLED(presets[colorIndex].c)
		println("joystick2: led", presets[colorIndex].name)
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 250*time.Millisecond {
		lastB = now
		brightIdx = (brightIdx + 1) % len(brightnessLevels)
		_ = dev.SetBrightness(brightnessLevels[brightIdx])
		println("joystick2: brightness", brightnessLevels[brightIdx])
	}
	if atomic.SwapUint32(&btnC, 0) > 0 && now.Sub(lastC) > 250*time.Millisecond {
		lastC = now
		page = (page + 1) % pageCount
	}
}

func render(st uiState) {
	if st == last {
		return
	}
	if time.Since(lastRender) < 50*time.Millisecond {
		return
	}
	last = st
	lastRender = time.Now()
	fb.clear()

	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "JOYSTICK2", cyan)
	state, col := "OK", green
	if !st.online {
		state, col = "OFFLINE", red
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 208, 14, state, col)
	fb.fillRect(0, 18, screenW, 1, darkGray)

	switch st.page {
	case pageVector:
		drawVector(st)
	case pageRaw:
		drawRaw(st)
	case pageInfo:
		drawInfo(st)
	}

	fb.fillRect(0, 120, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "A: cor  B: brilho  C: pagina", gray)
	fb.flush()
}

func drawVector(st uiState) {
	const cx, cy, radius = 62, 72, 40
	fb.fillRect(cx-radius, cy, radius*2+1, 1, darkGray)
	fb.fillRect(cx, cy-radius, 1, radius*2+1, darkGray)
	fb.fillRect(cx-2, cy-2, 4, 4, gray)

	px := cx + int16(int32(st.x)*radius/4095)
	py := cy - int16(int32(st.y)*radius/4095)
	col := green
	if st.pressed {
		col = yellow
	}
	fb.fillRect(px-3, py-3, 6, 6, col)

	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 126, 40, fmt.Sprintf("X %+5d", st.x), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 126, 54, fmt.Sprintf("Y %+5d", st.y), white)
	btn := "SOLTO"
	btnCol := gray
	if st.pressed {
		btn, btnCol = "PRESSIONADO", yellow
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 126, 72, "BOTAO", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 126, 84, btn, btnCol)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 126, 104,
		fmt.Sprintf("%d Hz", st.rate), white)
}

func drawRaw(st uiState) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36, "ADC 16 bits", gray)
	tinyfont.WriteLine(&fb, &freemono.Bold12pt7b, 4, 54, fmt.Sprintf("%d", st.rawX), white)
	tinyfont.WriteLine(&fb, &freemono.Bold12pt7b, 124, 54, fmt.Sprintf("%d", st.rawY), white)

	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 72, "ADC 8 bits", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 84,
		fmt.Sprintf("X %3d   Y %3d", st.raw8X, st.raw8Y), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 98,
		fmt.Sprintf("POS8 X %+4d  Y %+4d", st.pos8X, st.pos8Y), white)
	btn := "solto"
	if st.pressed {
		btn = "pressionado"
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 112, "BOTAO "+btn, gray)
}

func drawInfo(st uiState) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36,
		fmt.Sprintf("FW %d  BOOT %d  ADDR 0x%02X", st.fw, st.boot, st.addr), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 50,
		fmt.Sprintf("LED %s  BRILHO %d%%", presets[st.color].name, brightnessLevels[st.bright]), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 66,
		fmt.Sprintf("CAL X  %d..%d  %d..%d", st.cal.XNegMin, st.cal.XNegMax, st.cal.XPosMin, st.cal.XPosMax), gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 78,
		fmt.Sprintf("CAL Y  %d..%d  %d..%d", st.cal.YNegMin, st.cal.YNegMax, st.cal.YPosMin, st.cal.YPosMax), gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 94, "Grove I2C1  SDA G32 / SCL G33", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 108, "Calibracao nao e alterada pelo POC", gray)
}

func drawBoot(msg string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "JOYSTICK2", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 56, "INICIANDO", yellow)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 78, msg, white)
	fb.flush()
}

func drawOffline(err string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "JOYSTICK2", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 52, "SENSOR OFFLINE", red)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 74, "verifique o Grove G32/G33", white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 90, err, gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 106, "tentando de novo...", gray)
	fb.flush()
}

func initDisplay() {
	machine.SPI2.Configure(machine.SPIConfig{
		Frequency: 60 * machine.MHz,
		SCK:       machine.GPIO13,
		SDO:       machine.GPIO15,
		SDI:       machine.NoPin,
		Mode:      0,
	})
	display = st7789.New(machine.SPI2, machine.GPIO12, machine.GPIO14, machine.GPIO5, machine.GPIO27)
	display.IsBGR(true)
	display.Configure(st7789.Config{
		Width:        135,
		Height:       240,
		Rotation:     drivers.Rotation90,
		RowOffset:    40,
		ColumnOffset: 52,
		FrameRate:    st7789.FRAMERATE_60,
	})
}
