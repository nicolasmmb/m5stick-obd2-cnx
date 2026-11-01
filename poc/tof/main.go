// POC: M5Stack Unit Mini ToF-90° (VL53L0CXV0DH) test screen.
//
// Grove I2C1: SDA=G32, SCL=G33, address 0x29. The Grove port is shared with
// the other units, so only one can be connected at a time.
package main

import (
	"fmt"
	"image/color"
	"machine"
	"sync/atomic"
	"time"

	tof90 "github.com/nicolasmmb/M5Unit-ToF90-Go"

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

	pageMeasure = iota
	pageSetup
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

	page      int
	useCM     bool
	modeIndex = 1

	last       uiState
	lastRender time.Time
	lastLog    time.Time
	lastRate   time.Time
	rateStart  uint32

	minMM int16
	maxMM int16

	dev *tof90.Device
)

type mode struct {
	name   string
	budget time.Duration
	long   bool
}

var modes = [...]mode{
	{"20 ms", 20 * time.Millisecond, false},
	{"33 ms", 33 * time.Millisecond, false},
	{"50 ms", 50 * time.Millisecond, false},
	{"100 ms", 100 * time.Millisecond, false},
	{"200 ms", 200 * time.Millisecond, false},
	{"LONG", 200 * time.Millisecond, true},
}

type uiState struct {
	page     int
	useCM    bool
	mode     int
	online   bool
	model    uint8
	revision uint8
	rangeMM  int16
	status   uint8
	valid    bool
	minMM    int16
	maxMM    int16
	rate     int
	samples  uint32
	errors   uint32
	local    uint32
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
		println("tof: i2c", err.Error())
	}

	dev = tof90.New(i2c)
	dev.SetTimeout(100 * time.Millisecond)

	online := false
	var initErr string
	for !online {
		if err := dev.Init(); err != nil {
			initErr = err.Error()
			println("tof: init", initErr)
			drawOffline(initErr)
			time.Sleep(2 * time.Second)
			continue
		}
		applyMode(modeIndex)
		if err := dev.StartContinuous(); err != nil {
			initErr = err.Error()
			println("tof: start", initErr)
			drawOffline(initErr)
			time.Sleep(2 * time.Second)
			continue
		}
		online = true
	}
	model, _ := dev.ModelID()
	revision, _ := dev.RevisionID()
	println("tof: online model", model, "revision", revision,
		"budget", int64(modeBudget()/time.Microsecond), "us")

	rateStart = 0
	lastRate = time.Now()

	for {
		handleButtons()
		m, err := dev.ReadRange()
		var st uiState
		if err != nil {
			atomic.AddUint32(&statErrors, 1)
			println("tof: read", err.Error())
			st = snapshot()
			st.online = false
			st.errors = loadU32(&statErrors)
		} else {
			atomic.AddUint32(&statSamples, 1)
			if m.Valid {
				updateMinMax(m.RangeMM)
			}
			st = snapshot()
			st.rangeMM = m.RangeMM
			st.status = m.Status
			st.valid = m.Valid
		}
		st.samples = loadU32(&statSamples)
		st.errors = loadU32(&statErrors)
		st.model = model
		st.revision = revision
		render(st)

		if time.Since(lastLog) > time.Second {
			println("tof", int(st.rangeMM), "mm", "valid", st.valid,
				"status", st.status, "mode", modes[modeIndex].name,
				"rate", st.rate, "min", int(st.minMM), "max", int(st.maxMM),
				"samples", st.samples, "errors", st.errors)
			lastLog = time.Now()
		}
		time.Sleep(20 * time.Millisecond)
	}
}

var (
	statSamples uint32
	statErrors  uint32
)

func loadU32(p *uint32) uint32 { return atomic.LoadUint32(p) }

func modeBudget() time.Duration { return modes[modeIndex].budget }

// applyMode configures budget and signal rate for the selected mode. The
// sensor latches its timing configuration when continuous mode starts, so the
// stream is restarted around the change.
func applyMode(i int) {
	m := modes[i]
	wasRunning := dev.IsContinuous()
	if wasRunning {
		_ = dev.StopContinuous()
	}
	_ = dev.SetTimingBudget(m.budget)
	if m.long {
		_ = dev.SetSignalRateLimit(0.1)
	} else {
		_ = dev.SetSignalRateLimit(0.25)
	}
	if wasRunning {
		_ = dev.StartContinuous()
	}
}

func updateMinMax(mm int16) {
	if minMM == 0 || mm < minMM {
		minMM = mm
	}
	if mm > maxMM {
		maxMM = mm
	}
}

func snapshot() uiState {
	return uiState{
		page:   page,
		useCM:  useCM,
		mode:   modeIndex,
		online: true,
		minMM:  minMM,
		maxMM:  maxMM,
		rate:   currentRate(),
	}
}

func currentRate() int {
	now := time.Now()
	samples := loadU32(&statSamples)
	if rateStart == 0 {
		rateStart = samples
		lastRate = now
		return 0
	}
	elapsed := now.Sub(lastRate)
	if elapsed < time.Second {
		return rateValue
	}
	rateValue = int(int64(samples-rateStart) * int64(time.Second) / int64(elapsed))
	rateStart = samples
	lastRate = now
	return rateValue
}

var rateValue int

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
		useCM = !useCM
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 250*time.Millisecond {
		lastB = now
		page = (page + 1) % pageCount
	}
	if atomic.SwapUint32(&btnC, 0) > 0 && now.Sub(lastC) > 400*time.Millisecond {
		lastC = now
		modeIndex = (modeIndex + 1) % len(modes)
		applyMode(modeIndex)
		minMM, maxMM = 0, 0
		atomic.StoreUint32(&statSamples, 0)
		atomic.StoreUint32(&statErrors, 0)
		rateStart = 0
		rateValue = 0
		println("tof: mode", modes[modeIndex].name,
			"budget", int64(modes[modeIndex].budget/time.Microsecond), "us")
	}
}

func render(st uiState) {
	if st == last {
		return
	}
	if time.Since(lastRender) < 100*time.Millisecond {
		return
	}
	last = st
	lastRender = time.Now()
	fb.clear()

	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "MINI TOF-90", cyan)
	stateText, stateColor := "SEARCH", yellow
	if !st.online {
		stateText, stateColor = "OFFLINE", red
	} else if st.valid {
		stateText, stateColor = "OK", green
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 196, 14, stateText, stateColor)
	fb.fillRect(0, 18, screenW, 1, darkGray)

	switch st.page {
	case pageMeasure:
		drawMeasure(st)
	case pageSetup:
		drawSetup(st)
	case pageInfo:
		drawInfo(st)
	}

	fb.fillRect(0, 120, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "A: unidade  B: pagina  C: modo", gray)
	fb.flush()
}

func drawMeasure(st uiState) {
	unit := "mm"
	value := fmt.Sprintf("%d", st.rangeMM)
	if st.useCM {
		unit = "cm"
		value = fmt.Sprintf("%.1f", float64(st.rangeMM)/10)
	}
	if !st.online || !st.valid {
		value = "--"
	}
	col := white
	if !st.online {
		col = red
	} else if !st.valid {
		col = yellow
	}
	tinyfont.WriteLine(&fb, &freemono.Bold24pt7b, 8, 72, value, col)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 90, unit, gray)

	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 160, 44,
		fmt.Sprintf("RATE %d Hz", st.rate), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 160, 60,
		fmt.Sprintf("MIN %s", fmtRange(st.minMM, st.useCM)), gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 160, 72,
		fmt.Sprintf("MAX %s", fmtRange(st.maxMM, st.useCM)), gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 108,
		"STATUS "+statusName(st.status), gray)
}

func drawSetup(st uiState) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 36, "MODO / BUDGET", gray)
	tinyfont.WriteLine(&fb, &freemono.Bold12pt7b, 8, 60, modes[st.mode].name, white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 80,
		fmt.Sprintf("Budget %d ms", int64(modes[st.mode].budget/time.Millisecond)), white)
	rate := "0.25"
	if modes[st.mode].long {
		rate = "0.10"
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 94,
		"Signal rate "+rate+" MCPS", white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 112,
		"200 cm: LONG + ambiente escuro", gray)
}

func drawInfo(st uiState) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 36,
		fmt.Sprintf("MODEL 0x%02X  REV 0x%02X", st.model, st.revision), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 52,
		fmt.Sprintf("ADDR 0x29  BUDGET %d ms", int64(modes[st.mode].budget/time.Millisecond)), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 68,
		fmt.Sprintf("SAMPLES %d  ERRORS %d", st.samples, st.errors), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 84,
		"Grove I2C1  SDA G32 / SCL G33", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 8, 100,
		"1.9 MHz laser class 1", gray)
}

func fmtRange(mm int16, cm bool) string {
	if mm == 0 {
		return "--"
	}
	if cm {
		return fmt.Sprintf("%.1f cm", float64(mm)/10)
	}
	return fmt.Sprintf("%d mm", mm)
}

func statusName(s uint8) string {
	switch s {
	case 0:
		return "VALIDA"
	case 1:
		return "SIGMA FAIL"
	case 2:
		return "SINAL FAIL"
	case 3:
		return "MIN RANGE OK"
	case 4:
		return "FORA DE LIMITE"
	case 5:
		return "HARDWARE"
	case 6:
		return "NO WRAP OK"
	case 7:
		return "WRAP TARGET"
	case 8:
		return "PROCESSING"
	case 9:
		return "XTALK"
	case 10:
		return "SYNC"
	case 11:
		return "MERGED OK"
	}
	return "DESCONHECIDO"
}

func drawBoot(msg string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "MINI TOF-90", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 56, "INICIANDO", yellow)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 78, msg, white)
	fb.flush()
}

func drawOffline(err string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "MINI TOF-90", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 52, "SENSOR OFFLINE", red)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 74, "verifique o Grove G32/G33", white)
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
