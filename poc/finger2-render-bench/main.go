// Render benchmark: measures the cost of the old "always redraw" loop against
// the new state-diff render path.
//
// It runs without a finger and prints results over UART.
package main

import (
	"image/color"
	"machine"
	"time"

	"m5stick-obd2-cnx/lib/finger2"

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
	iter    = 20
)

var (
	white    = color.RGBA{235, 235, 235, 255}
	gray     = color.RGBA{110, 110, 110, 255}
	darkGray = color.RGBA{45, 45, 45, 255}
	cyan     = color.RGBA{0, 220, 220, 255}
	magenta  = color.RGBA{220, 90, 220, 255}

	display st7789.Device
	fb      canvas
	gray4   [16]pixel.RGB565BE
	img     []byte
)

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

// renderState mirrors the state used by the app to skip redraws.
type renderState struct {
	mode       int
	status     string
	col        color.RGBA
	id, score  uint16
	templates  uint16
	infoPage   int
	enroll     bool
	del        bool
	delPending bool
	clearPend  bool
	imgLoaded  bool
	hint       string
	imgErr     string
	imgSeq     uint64
}

func main() {
	time.Sleep(2 * time.Second)

	initDisplay()
	fb = newCanvas()
	for i := range gray4 {
		g := uint8(i * 17)
		gray4[i] = pixel.NewColor[pixel.RGB565BE](g, g, g)
	}
	img = make([]byte, finger2.ImageWidth*finger2.ImageHeight/2)
	for i := range img {
		img[i] = byte(i * 7)
	}

	benchFrames()
	benchState()
	benchDiagnostics()

	println("BENCH|done")
	for {
		time.Sleep(time.Second)
	}
}

func benchFrames() {
	// Warm up the display path.
	drawFrame(false)
	fb.flush()

	start := time.Now()
	for i := 0; i < iter; i++ {
		drawFrame(false)
		fb.flush()
	}
	noImage := time.Since(start) / iter

	start = time.Now()
	for i := 0; i < iter; i++ {
		drawFrame(true)
		fb.flush()
	}
	withImage := time.Since(start) / iter

	start = time.Now()
	for i := 0; i < iter; i++ {
		fb.flush()
	}
	flushOnly := time.Since(start) / iter

	println("BENCH|frame|no_image_us", noImage.Microseconds(),
		"with_image_us", withImage.Microseconds(),
		"flush_us", flushOnly.Microseconds())

	// Old loop: 10 full redraws per second.
	oldIdleMs := 10 * noImage.Milliseconds()
	oldImageMs := 10 * withImage.Milliseconds()
	// New loop: one image redraw per captured frame (~1/s) and no idle flush.
	newImageMs := withImage.Milliseconds()
	println("BENCH|per_second_ms|old_idle", oldIdleMs,
		"new_idle", 0,
		"old_image", oldImageMs,
		"new_image", newImageMs)
}

func benchState() {
	a := renderState{
		mode: 2, status: "ENCOSTE O DEDO", col: cyan,
		id: 0xFFFF, templates: 2, hint: "", imgSeq: 1,
	}
	b := a

	start := time.Now()
	n := 0
	for i := 0; i < 100000; i++ {
		b.imgSeq = uint64(i) + 1 // keep b changing so the compare is not hoisted
		if a == b {
			n++
		}
	}
	elapsed := time.Since(start)
	println("BENCH|state|ns_per_check", elapsed.Nanoseconds()/100000, "hits", n)
}

func benchDiagnostics() {
	// Cost of the removed min/max/avg loop over one image.
	start := time.Now()
	var sink uint8
	for i := 0; i < 50; i++ {
		minV, maxV := uint8(15), uint8(0)
		var sum uint64
		for _, b := range img {
			for _, v := range [2]uint8{b & 0x0F, b >> 4} {
				if v < minV {
					minV = v
				}
				if v > maxV {
					maxV = v
				}
				sum += uint64(v)
			}
		}
		sink = minV + maxV
	}
	elapsed := time.Since(start) / 50
	println("BENCH|diag_us", elapsed.Microseconds(), "sink", sink)
}

func drawFrame(withImage bool) {
	fb.clear()
	for i := 0; i < 6; i++ {
		x := int16((i % 3) * 80)
		y := int16((i / 3) * 16)
		col := darkGray
		if i == 0 {
			col = cyan
		}
		fb.fillRect(x, y, 80, 16, col)
	}
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 62, "ENCOSTE O DEDO", cyan)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 80, "confianca 123", white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 96, "ID --   score 0", gray)
	fb.fillRect(0, screenH-14, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "A/B: aba   C: acao", gray)
	if withImage {
		drawImage()
	}
}

func drawImage() {
	const (
		iw = finger2.ImageWidth
		ih = finger2.ImageHeight
		dx = 16
		dy = 38
	)
	for y := 0; y < ih; y++ {
		for x := 0; x < iw; x++ {
			p := y*iw + x
			b := img[p>>1]
			var v uint8
			if p&1 == 0 {
				v = b & 0x0F
			} else {
				v = b >> 4
			}
			fb.img.Set(dx+ih-1-y, dy+x, gray4[v])
		}
	}
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
