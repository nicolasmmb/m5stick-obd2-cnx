// POC: M5Stack Unit NFC (U216, ST25R3916) NFC-A probe.
//
// Grove I2C1: SDA=G32, SCL=G33, address 0x50. The Grove port is shared with
// the other units, so only one can be connected at a time.
package main

import (
	"context"
	"fmt"
	"image/color"
	"machine"
	"sync/atomic"
	"time"

	nfc "github.com/nicolasmmb/M5Unit-NFC-Go"

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

	pageCard = iota
	pageInfo
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

	page    int
	last    uiState
	lastAt  time.Time
	lastLog time.Time

	dev     *nfc.Device
	rfOn    = true
	proto   = 1 // 1 NFC-A, 2 NFC-F
	misses  int
	chipTyp uint8
	chipRev uint8
	scans   uint32
	errors  uint32
)

type uiState struct {
	page     int
	online   bool
	rf       bool
	proto    int
	card     bool
	uid      [7]byte
	uidLen   uint8
	atqa     uint16
	sak      uint8
	typeID   uint8
	mem      uint16
	idm      [8]byte
	pmm      [8]byte
	chipTyp  uint8
	chipRev  uint8
	detectUS int64
	scans    uint32
	errors   uint32
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

	drawBoot("I2C 400 kHz")

	i2c := machine.I2C1
	if err := i2c.Configure(machine.I2CConfig{
		SDA:       machine.GPIO32,
		SCL:       machine.GPIO33,
		Frequency: 400 * machine.KHz,
	}); err != nil {
		println("nfc: i2c", err.Error())
	}

	dev = nfc.New(i2c)
	online := false
	for !online {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := dev.Begin(ctx)
		cancel()
		if err != nil {
			println("nfc: begin", err.Error())
			drawOffline(err.Error())
			time.Sleep(2 * time.Second)
			continue
		}
		online = true
	}
	chipTyp, chipRev, _ = dev.Identity()
	println("nfc: online type", chipTyp, "rev", chipRev, "addr", dev.Address())

	for {
		handleButtons()

		st := uiState{
			page:    page,
			online:  true,
			rf:      rfOn,
			chipTyp: chipTyp,
			chipRev: chipRev,
			proto:   proto,
			scans:   atomic.LoadUint32(&scans),
			errors:  atomic.LoadUint32(&errors),
		}
		if rfOn {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
			start := time.Now()
			ok := false
			var err error
			if proto == 1 {
				var c nfc.Card
				c, ok, err = dev.Detect(ctx)
				if ok {
					st.uid = c.UID
					st.uidLen = c.UIDLen
					st.atqa = c.ATQA
					st.sak = c.SAK
					st.typeID = c.TypeID
					st.mem = c.UserMemory
				}
			} else {
				var c nfc.FeliCaCard
				c, ok, err = dev.DetectF(ctx)
				if ok {
					st.idm = c.IDm
					st.pmm = c.PMm
				}
			}
			cancel()
			st.detectUS = int64(time.Since(start) / time.Microsecond)
			if err != nil {
				atomic.AddUint32(&errors, 1)
				println("nfc: detect", err.Error())
			}
			st.card = ok
			st.proto = proto
			if ok {
				misses = 0
			} else if err == nil {
				misses++
				if misses >= 3 {
					misses = 0
					if proto == 1 {
						if err := dev.ConfigureF(context.Background()); err == nil {
							proto = 2
						}
					} else {
						if err := dev.ConfigureA(context.Background()); err == nil {
							proto = 1
						}
					}
				}
			}
			atomic.AddUint32(&scans, 1)
			st.scans = atomic.LoadUint32(&scans)
			st.errors = atomic.LoadUint32(&errors)
		}
		render(st)

		if time.Since(lastLog) > time.Second {
			if st.card && st.proto == 1 {
				println("nfc A", "uid", fmt.Sprintf("% X", st.uid[:st.uidLen]),
					"type", nfc.TypeName(st.typeID), "atqa", st.atqa,
					"sak", st.sak, "mem", st.mem, "detect_us", st.detectUS,
					"scans", st.scans, "err", st.errors)
			} else if st.card {
				println("nfc F", "idm", fmt.Sprintf("% X", st.idm[:]),
					"pmm", fmt.Sprintf("% X", st.pmm[:]), "detect_us", st.detectUS,
					"scans", st.scans, "err", st.errors)
			} else {
				println("nfc: no card; scans", st.scans, "err", st.errors,
					"detect_us", st.detectUS, "rf", rfOn)
			}
			lastLog = time.Now()
		}
		time.Sleep(60 * time.Millisecond)
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

func handleButtons() {
	now := time.Now()
	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 250*time.Millisecond {
		lastA = now
		if rfOn {
			_ = dev.RFOff()
			rfOn = false
		} else {
			_ = dev.RFOn()
			rfOn = true
		}
		println("nfc: rf", rfOn)
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 500*time.Millisecond {
		lastB = now
		drawBoot("reiniciando")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := dev.Begin(ctx)
		cancel()
		if err != nil {
			println("nfc: begin", err.Error())
			drawOffline(err.Error())
		} else {
			chipTyp, chipRev, _ = dev.Identity()
			atomic.StoreUint32(&scans, 0)
			atomic.StoreUint32(&errors, 0)
			rfOn = true
			proto = 1
			misses = 0
			println("nfc: reinit ok")
		}
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
	if time.Since(lastAt) < 80*time.Millisecond {
		return
	}
	last = st
	lastAt = time.Now()
	fb.clear()

	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "UNIT NFC", cyan)
	state, col := "OK", green
	if !st.online {
		state, col = "OFFLINE", red
	} else if !st.rf {
		state, col = "RF OFF", yellow
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 196, 14, state, col)
	fb.fillRect(0, 18, screenW, 1, darkGray)

	switch st.page {
	case pageCard:
		drawCard(st)
	case pageInfo:
		drawInfo(st)
	}

	fb.fillRect(0, 120, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "A: RF  B: reiniciar  C: pagina", gray)
	fb.flush()
}

func drawCard(st uiState) {
	if !st.card {
		tinyfont.WriteLine(&fb, &freemono.Bold12pt7b, 4, 56, "APROXIME", yellow)
		tinyfont.WriteLine(&fb, &freemono.Bold12pt7b, 4, 80, "UM CARTAO", yellow)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 102,
			fmt.Sprintf("procurando NFC-%s", protoName(st.proto)), gray)
		return
	}
	if st.proto == 1 {
		uid := fmt.Sprintf("% X", st.uid[:st.uidLen])
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36, "NFC-A / UID", gray)
		tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 52, uid, white)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 70, nfc.TypeName(st.typeID), green)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 86,
			fmt.Sprintf("ATQA %04X  SAK %02X", st.atqa, st.sak), white)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 100,
			fmt.Sprintf("MEM %d B   %d us", st.mem, st.detectUS), white)
		return
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36, "NFC-F / FeliCa", gray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 52,
		fmt.Sprintf("% X", st.idm[:]), green)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 72,
		fmt.Sprintf("PMm % X", st.pmm[:]), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 92,
		fmt.Sprintf("Suica/Pasmo/FeliCa  %d us", st.detectUS), white)
}

func drawInfo(st uiState) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36,
		fmt.Sprintf("CHIP %d  REV %d  ADDR 0x50", st.chipTyp, st.chipRev), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 52,
		fmt.Sprintf("RF %v  MODO NFC-%s  DETECT %d us", st.rf, protoName(st.proto), st.detectUS), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 68,
		fmt.Sprintf("SCANS %d  ERRORS %d", st.scans, st.errors), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 88, "FASE 1: PROBE NFC-A + F", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 102, "NFC-B/V: proxima fase", gray)
}

func protoName(p int) string {
	if p == 2 {
		return "F"
	}
	return "A"
}

func drawBoot(msg string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "UNIT NFC", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 56, "INICIANDO", yellow)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 78, msg, white)
	fb.flush()
}

func drawOffline(err string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "UNIT NFC", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 52, "SENSOR OFFLINE", red)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 74, "Grove G32/G33  0x50", white)
	if len(err) > 34 {
		err = err[:34]
	}
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
