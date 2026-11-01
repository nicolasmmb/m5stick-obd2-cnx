// POC: M5Stack Unit GPS v1.1 (AT6668 + MAX2659) test screen.
//
// Grove UART1: TX=G32, RX=G33. The v1.1 runs at 115200; if no NMEA is seen the
// POC falls back to 9600 for the older U032 (AT6558).
package main

import (
	"fmt"
	"image/color"
	"machine"
	"sync/atomic"
	"time"

	gps "github.com/nicolasmmb/M5Unit-GPS-Go"

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

	pagePosition = iota
	pageTime
	pageStatus
	pageCount
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

	display st7789.Device
	fb      canvas

	btnA, btnB, btnC    uint32
	lastA, lastB, lastC time.Time

	page       int
	baud       uint32 = 115200
	last       uiState
	lastLog    time.Time
	lastRender time.Time
	statusAt   time.Time
	statusSent uint64
	statusErr  uint64
)

type uiState struct {
	page      int
	baud      uint32
	valid     bool
	sats      int
	quality   int
	lat       float64
	lon       float64
	alt       float64
	speed     float64
	course    float64
	utc       int64
	antenna   string
	sentences uint64
	errors    uint64
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

	uart := machine.UART1
	configureUART(uart, baud)

	// Auto-baud: the v1.1 (AT6668) uses 115200; the older U032 uses 9600.
	drawProbe("detectando 115200")
	if !probe(uart, 2*time.Second) {
		baud = 9600
		configureUART(uart, baud)
		drawProbe("detectando 9600")
		if !probe(uart, 2*time.Second) {
			baud = 115200
			configureUART(uart, baud)
		}
	}
	println("gps: baud", baud)

	dev := gps.New(uart)
	dev.Start()
	_ = dev.SetWorkMode(7)
	println("gps: waiting for NMEA")

	for {
		handleButtons()

		f := dev.Fix()
		utc := int64(0)
		if !f.UTC.IsZero() {
			utc = f.UTC.Unix()
		}
		// The status counters change on every sentence. Only refresh them
		// once per second on the STATUS page so they do not force a display
		// flush (and UART pressure) at the NMEA rate.
		if page == pageStatus && time.Since(statusAt) >= time.Second {
			statusSent = f.Sentences
			statusErr = f.ChecksumErrors
			statusAt = time.Now()
		}
		st := uiState{
			page:      page,
			baud:      baud,
			valid:     f.Valid,
			sats:      f.Satellites,
			quality:   f.Quality,
			lat:       f.Latitude,
			lon:       f.Longitude,
			alt:       f.Altitude,
			speed:     f.SpeedKnots,
			course:    f.Course,
			utc:       utc,
			antenna:   f.Antenna,
			sentences: statusSent,
			errors:    statusErr,
		}
		if time.Since(lastLog) > time.Second {
			println("gps", f.Sentences, "fix", f.Valid, "sats", f.Satellites,
				"lat", f.Latitude, "lon", f.Longitude, "alt", f.Altitude,
				"ant", f.Antenna, "baud", baud, "err", f.ChecksumErrors)
			lastLog = time.Now()
		}
		render(st)
		time.Sleep(100 * time.Millisecond)
	}
}

func configureUART(uart *machine.UART, baud uint32) {
	_ = uart.Configure(machine.UARTConfig{
		BaudRate: baud,
		TX:       machine.GPIO32,
		RX:       machine.GPIO33,
	})
}

// probe looks for a complete NMEA sentence within d.
func probe(uart *machine.UART, d time.Duration) bool {
	deadline := time.Now().Add(d)
	seen := false
	for time.Now().Before(deadline) {
		for uart.Buffered() > 0 {
			b, err := uart.ReadByte()
			if err != nil {
				break
			}
			if b == '$' {
				seen = true
			}
			if b == '\n' && seen {
				return true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
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
		page = (page + pageCount - 1) % pageCount
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 250*time.Millisecond {
		lastB = now
		page = (page + 1) % pageCount
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
	// Cap redraws at 5 Hz: a 240x135 SPI flush takes ~13 ms, longer than the
	// 128-byte UART ring buffer can hold at 115200.
	if time.Since(lastRender) < 200*time.Millisecond {
		return
	}
	last = st
	lastRender = time.Now()
	fb.clear()

	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "GPS v1.1", cyan)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 190, 14, fmt.Sprintf("%d", st.baud), gray)
	fb.fillRect(0, 18, screenW, 1, darkGray)

	switch st.page {
	case pagePosition:
		drawPosition(st)
	case pageTime:
		drawTime(st)
	case pageStatus:
		drawStatus(st)
	}

	fb.fillRect(0, 120, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "A/B: pagina  C: proxima", gray)
	fb.flush()
}

func drawPosition(st uiState) {
	status, col := "SEARCH", yellow
	if st.valid {
		status, col = "FIX", green
	}
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 150, 14, status, col)

	if st.sentences == 0 {
		tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 56, "SEM NMEA", red)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 76, "verifique o Grove G32/G33", white)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 92, "e a antena (ceu aberto)", gray)
		return
	}

	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 44, fmt.Sprintf("LAT %.5f", st.lat), white)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 62, fmt.Sprintf("LON %.5f", st.lon), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 82, fmt.Sprintf("ALT %.1f m", st.alt), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 98, fmt.Sprintf("SATS %d", st.sats), white)

	bars := 0
	switch {
	case st.sats >= 22:
		bars = 5
	case st.sats >= 18:
		bars = 4
	case st.sats >= 12:
		bars = 3
	case st.sats >= 8:
		bars = 2
	case st.sats >= 4:
		bars = 1
	}
	heights := [5]int16{6, 10, 14, 18, 22}
	for i := 0; i < 5; i++ {
		col := darkGray
		if i < bars {
			col = green
		}
		fb.fillRect(130+int16(i*12), 106-heights[i], 8, heights[i], col)
	}
}

func drawTime(st uiState) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36, "UTC", gray)
	if st.utc == 0 {
		tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 58, "--", white)
	} else {
		t := time.Unix(st.utc, 0).UTC()
		tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 54, t.Format("02/01/06"), white)
		tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 74, t.Format("15:04:05"), white)
	}
	kmh := st.speed * 1.852
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 96,
		fmt.Sprintf("SPEED %.1f kn  %.1f km/h", st.speed, kmh), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 110,
		fmt.Sprintf("COURSE %.1f deg", st.course), white)
}

func drawStatus(st uiState) {
	ant := st.antenna
	if ant == "" {
		ant = "--"
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36, "ANTENNA "+ant, white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 52, fmt.Sprintf("QUALITY %d", st.quality), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 68, fmt.Sprintf("BAUD %d", st.baud), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 84, "MODE 7 (PCAS04)", white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 100,
		fmt.Sprintf("SENT %d   ERR %d", st.sentences, st.errors), gray)
	fix := "SEARCH"
	if st.valid {
		fix = "FIX"
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 112, "STATE "+fix, gray)
}

func drawProbe(msg string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "GPS v1.1", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 56, "PROCURANDO GPS", yellow)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 78, msg, white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 94, "Grove G32/G33", gray)
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
