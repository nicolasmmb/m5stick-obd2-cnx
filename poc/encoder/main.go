// POC: M5Stack Unit Encoder (U135) test screen.
//
// Shows the signed encoder count, direction and button, and controls the
// two SK6812 LEDs. Grove I2C1: SDA=G32, SCL=G33, address 0x40.
package main

import (
	"fmt"
	"image/color"
	"machine"
	"sync/atomic"
	"time"

	encoder "github.com/nicolasmmb/M5Unit-Encoder-Go"

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
)

var (
	black    = color.RGBA{0, 0, 0, 255}
	white    = color.RGBA{235, 235, 235, 255}
	gray     = color.RGBA{110, 110, 110, 255}
	darkGray = color.RGBA{45, 45, 45, 255}
	cyan     = color.RGBA{0, 220, 220, 255}
	green    = color.RGBA{0, 220, 90, 255}
	yellow   = color.RGBA{255, 210, 0, 255}
	red      = color.RGBA{255, 70, 70, 255}
	blue     = color.RGBA{70, 140, 255, 255}
	magenta  = color.RGBA{220, 90, 220, 255}

	display st7789.Device
	fb      canvas

	btnA, btnB, btnC    uint32
	lastA, lastB, lastC time.Time

	lastCount  int16
	lastRaw    int16
	zeroOffset int16
	haveCount  bool
	last       uiState
	lastLog    time.Time
)

type ledColor struct {
	name string
	rgb  uint32
	col  color.RGBA
}

var colors = []ledColor{
	{"off", 0x000000, gray},
	{"vermelho", 0xFF0000, red},
	{"verde", 0x00FF00, green},
	{"azul", 0x0000FF, blue},
	{"branco", 0xFFFFFF, white},
	{"amarelo", 0xFFFF00, yellow},
	{"ciano", 0x00FFFF, cyan},
	{"magenta", 0xFF00FF, magenta},
}

type uiState struct {
	offline   bool
	count     int16
	delta     int16
	pressed   bool
	led       uint8
	colorName string
	color     color.RGBA
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

	i2c := machine.I2C1
	if err := i2c.Configure(machine.I2CConfig{
		SDA:       machine.GPIO32,
		SCL:       machine.GPIO33,
		Frequency: 100 * machine.KHz,
	}); err != nil {
		println("i2c:", err.Error())
		drawOffline("I2C FALHOU")
		return
	}
	dev := encoder.NewI2C(i2c, encoder.DefaultAddress)

	_ = dev.SetLEDColor(0, 0x000000) // 0 = both LEDs

	ledIndex := uint8(1)
	colorIndex := 1 // vermelho

	for {
		handleButtons(dev, &ledIndex, &colorIndex)

		count, err := dev.Encoder()
		if err != nil {
			last = uiState{}
			drawOffline("0x40 nao responde")
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if !haveCount {
			lastCount = count
			haveCount = true
		}
		pressed, _ := dev.Button()

		lastRaw = count
		count -= zeroOffset
		delta := count - lastCount
		lastCount = count
		if time.Since(lastLog) > time.Second {
			println("encoder", count, "raw", lastRaw, "btn", pressed, "delta", delta)
			lastLog = time.Now()
		}
		render(uiState{
			count:     count,
			delta:     delta,
			pressed:   pressed,
			led:       ledIndex,
			colorName: colors[colorIndex].name,
			color:     colors[colorIndex].col,
		})
		time.Sleep(50 * time.Millisecond)
	}
}

func setupButtons() {
	for _, p := range []machine.Pin{machine.GPIO37, machine.GPIO39, machine.GPIO35} {
		p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	_ = machine.GPIO37.SetInterrupt(machine.PinFalling, func(machine.Pin) { atomic.AddUint32(&btnA, 1) })
	_ = machine.GPIO39.SetInterrupt(machine.PinFalling, func(machine.Pin) { atomic.AddUint32(&btnB, 1) })
	_ = machine.GPIO35.SetInterrupt(machine.PinFalling, func(machine.Pin) { atomic.AddUint32(&btnC, 1) })
}

func handleButtons(dev *encoder.Device, ledIndex *uint8, colorIndex *int) {
	now := time.Now()
	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 250*time.Millisecond {
		lastA = now
		if *ledIndex == 1 {
			*ledIndex = 2
		} else {
			*ledIndex = 1
		}
		applyLED(dev, *ledIndex, *colorIndex)
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 250*time.Millisecond {
		lastB = now
		*colorIndex = (*colorIndex + 1) % len(colors)
		applyLED(dev, *ledIndex, *colorIndex)
	}
	if atomic.SwapUint32(&btnC, 0) > 0 && now.Sub(lastC) > 250*time.Millisecond {
		lastC = now
		_ = dev.Reset() // no-op on some U135 firmware revisions
		zeroOffset = lastRaw
		lastCount = 0
		haveCount = true
		println("encoder: zeroed at", lastRaw)
	}
}

func applyLED(dev *encoder.Device, index uint8, colorIndex int) {
	_ = dev.SetLEDColor(index, colors[colorIndex].rgb)
	println("encoder: led", index, colors[colorIndex].name)
}

func render(st uiState) {
	if st == last {
		return
	}
	last = st

	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "ENCODER U135", cyan)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 192, 14, "0x40", gray)
	fb.fillRect(0, 18, screenW, 1, darkGray)

	countText := fmt.Sprintf("%d", st.count)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 58, countText, white)

	dir := ""
	if st.delta > 0 {
		dir = ">>"
	} else if st.delta < 0 {
		dir = "<<"
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 184, 46, dir, green)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 168, 60, fmt.Sprintf("d %d", st.delta), gray)

	// Position bar: one marker per count modulo the width.
	fb.fillRect(4, 74, 232, 2, darkGray)
	pos := int(st.count) % 232
	if pos < 0 {
		pos += 232
	}
	fb.fillRect(4+int16(pos), 71, 3, 8, cyan)

	btn := "solto"
	if st.pressed {
		btn = "PRESSIONADO"
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 94, fmt.Sprintf("botao %s", btn), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 108, fmt.Sprintf("LED %d  %s", st.led, st.colorName), st.color)

	fb.fillRect(0, 120, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "A: LED  B: cor  C: zerar", gray)
	fb.flush()
}

func drawOffline(reason string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "ENCODER U135", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 56, "ENCODER OFFLINE", red)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 76, reason, white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 94, "verifique o Grove G32/G33", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "tentando...", gray)
	fb.flush()
}

func initDisplay() {
	machine.SPI2.Configure(machine.SPIConfig{
		Frequency: 40 * machine.MHz,
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
