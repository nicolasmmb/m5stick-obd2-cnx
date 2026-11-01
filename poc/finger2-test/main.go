package main

import (
	"fmt"
	"image/color"
	"machine"
	"sync/atomic"
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
	magenta  = color.RGBA{220, 90, 220, 255}

	display st7789.Device
	fb      canvas

	sel       int
	top       int
	results   [32]string
	runStatus string

	btnA, btnB, btnC    uint32
	lastA, lastB, lastC time.Time
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

type testCase struct {
	name   string
	finger bool
	run    func(*finger2.Device) (bool, string)
}

var tests = []testCase{
	{"GetSN", false, tGetSN},
	{"GetRandom", false, tRandom},
	{"CheckSensor", false, tCheckSensor},
	{"ReadINF", false, tReadINF},
	{"Notepad", false, tNotepad},
	{"LED modes", false, tLEDModes},
	{"WorkMode", false, tWorkMode},
	{"SleepTime", false, tSleepTime},
	{"SaveConfig", false, tSaveConfig},
	{"LoadChar", false, tLoadChar},
	{"Template R/W", false, tTemplate},
	{"SearchNow", true, tSearchNow},
	{"AutoIdentify", true, tAutoIdentify},
}

func main() {
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	initDisplay()
	fb = newCanvas()

	uart := machine.UART1
	if err := uart.Configure(machine.UARTConfig{BaudRate: 115200, TX: machine.GPIO32, RX: machine.GPIO33}); err != nil {
		println("uart:", err.Error())
		return
	}
	dev := finger2.New(uart)
	dev.SetTimeout(900 * time.Millisecond)

	for {
		if err := dev.Init(); err == nil {
			break
		} else {
			println("init:", err.Error())
		}
		drawRun("INIT", "toque no sensor...", yellow)
		time.Sleep(500 * time.Millisecond)
	}
	_ = dev.SetWorkMode(1)
	println("TEST|00|Init|PASS|FW", dev.Firmware, "cap", dev.Capacity, "tpl", dev.Templates, "level", dev.MatchLevel)

	setupButtons()
	drawMenu()

	for {
		handleButtons(dev)
		if runStatus != "" {
			drawRun(tests[sel].name, runStatus, yellow)
		}
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

func handleButtons(dev *finger2.Device) {
	now := time.Now()

	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 200*time.Millisecond {
		lastA = now
		sel = (sel + 1) % len(tests)
		runStatus = ""
		drawMenu()
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 200*time.Millisecond {
		lastB = now
		sel = (sel + len(tests) - 1) % len(tests)
		runStatus = ""
		drawMenu()
	}
	if atomic.SwapUint32(&btnC, 0) > 0 && now.Sub(lastC) > 300*time.Millisecond {
		lastC = now
		runTest(dev, sel)
		drawMenu()
	}
}

func runTest(dev *finger2.Device, i int) {
	tc := tests[i]
	println("TEST|START|" + tc.name)
	drawRun(tc.name, "EXECUTANDO...", cyan)
	ok, detail := tc.run(dev)
	res := "PASS"
	if !ok {
		res = "FAIL"
	}
	results[i] = res
	if detail == "" {
		detail = "-"
	}
	println("TEST|" + fmt.Sprintf("%02d", i+1) + "|" + tc.name + "|" + res + "|" + detail)
	runStatus = res + " " + detail
	drawRun(tc.name, runStatus, statusColor(ok))
	time.Sleep(1200 * time.Millisecond)
	runStatus = ""
}

func statusColor(ok bool) color.RGBA {
	if ok {
		return green
	}
	return red
}

// --- tests ---

func tGetSN(d *finger2.Device) (bool, string) {
	sn, err := d.GetSN()
	if err != nil {
		return false, err.Error()
	}
	if len(sn) < 32 {
		return false, fmt.Sprintf("len=%d", len(sn))
	}
	return true, fmt.Sprintf("sn=%X", sn)
}

func tRandom(d *finger2.Device) (bool, string) {
	a, err := d.GetRandom()
	if err != nil {
		return false, err.Error()
	}
	b, err := d.GetRandom()
	if err != nil {
		return false, err.Error()
	}
	return a != b, fmt.Sprintf("%08X %08X", a, b)
}

func tCheckSensor(d *finger2.Device) (bool, string) {
	if err := d.CheckSensor(); err != nil {
		return false, err.Error()
	}
	return true, "sensor ok"
}

func tReadINF(d *finger2.Device) (bool, string) {
	data, err := d.ReadINF()
	if err != nil {
		return false, err.Error()
	}
	if len(data) == 0 {
		return false, "vazio"
	}
	n := len(data)
	if n > 8 {
		n = 8
	}
	return len(data) == 512, fmt.Sprintf("len=%d head=%X", len(data), data[:n])
}

func tNotepad(d *finger2.Device) (bool, string) {
	var want [32]byte
	for i := range want {
		want[i] = byte(i*7 + 3)
	}
	c, err := d.WriteNotepad(2, want[:])
	if err != nil || c != finger2.ConfirmOK {
		return false, fmt.Sprintf("write 0x%02X %v", c, err)
	}
	got, err := d.ReadNotepad(2)
	if err != nil {
		return false, "read: " + err.Error()
	}
	if len(got) != len(want) {
		return false, fmt.Sprintf("len=%d", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			return false, fmt.Sprintf("byte %d: %02X != %02X", i, got[i], want[i])
		}
	}
	return true, "32 bytes conferem"
}

func tLEDModes(d *finger2.Device) (bool, string) {
	if err := d.ControlLED(finger2.LEDBlink, finger2.LEDRed, finger2.LEDRed, 3); err != nil {
		return false, "blink: " + err.Error()
	}
	time.Sleep(1500 * time.Millisecond)
	if err := d.ControlLED(finger2.LEDFadeIn, finger2.LEDGreen, finger2.LEDCyan, 0); err != nil {
		return false, "fade: " + err.Error()
	}
	time.Sleep(1500 * time.Millisecond)
	_ = d.ControlLED(finger2.LEDBreath, finger2.LEDBlue, finger2.LEDBlue, 0)
	return true, "blink vermelho + fade (visual)"
}

func tWorkMode(d *finger2.Device) (bool, string) {
	orig, err := d.GetWorkMode()
	if err != nil {
		return false, err.Error()
	}
	if err := d.SetWorkMode(1); err != nil {
		return false, "set: " + err.Error()
	}
	got, err := d.GetWorkMode()
	if err != nil {
		return false, "get: " + err.Error()
	}
	return got == 1, fmt.Sprintf("orig=%d lido=%d", orig, got)
}

func tSleepTime(d *finger2.Device) (bool, string) {
	orig, err := d.GetSleepTime()
	if err != nil {
		return false, err.Error()
	}
	if err := d.SetSleepTime(30); err != nil {
		return false, "set: " + err.Error()
	}
	got, err := d.GetSleepTime()
	if err != nil {
		return false, "get: " + err.Error()
	}
	_ = d.SetSleepTime(orig)
	return got == 30, fmt.Sprintf("orig=%d lido=%d restaurado=%d", orig, got, orig)
}

func tSaveConfig(d *finger2.Device) (bool, string) {
	if err := d.SaveConfig(0); err != nil {
		return false, err.Error()
	}
	return true, "config salva no flash"
}

func tLoadChar(d *finger2.Device) (bool, string) {
	if d.Templates == 0 {
		return false, "sem template: cadastre antes"
	}
	c, err := d.LoadChar(1, 0)
	if err != nil {
		return false, err.Error()
	}
	return c == finger2.ConfirmOK, fmt.Sprintf("page=0 confirm=0x%02X", c)
}

func tTemplate(d *finger2.Device) (bool, string) {
	if d.Templates == 0 {
		return false, "sem template: cadastre antes"
	}
	c, err := d.LoadChar(1, 0)
	if err != nil || c != finger2.ConfirmOK {
		return false, fmt.Sprintf("loadchar 0x%02X %v", c, err)
	}
	tpl, err := d.UploadTemplateAuto(7262)
	if err != nil {
		return false, "upload: " + err.Error()
	}
	if len(tpl) == 0 {
		return false, "upload vazio"
	}
	if err := d.DownloadTemplateAuto(tpl); err != nil {
		return false, "download: " + err.Error()
	}
	page, err := d.FirstFreePage()
	if err != nil {
		return false, "free page: " + err.Error()
	}
	c, err = d.Store(1, page)
	if err != nil || c != finger2.ConfirmOK {
		return false, fmt.Sprintf("store 0x%02X %v", c, err)
	}
	before := d.Templates
	d.RefreshTemplates()
	after := d.Templates
	c, _ = d.Delete(page, 1)
	d.RefreshTemplates()
	return after == before+1, fmt.Sprintf("len=%d page=%d tpl %d->%d del=0x%02X", len(tpl), page, before, after, c)
}

func waitFinger(d *finger2.Device, name, msg string) bool {
	for i := 0; i < 120; i++ {
		det, _, err := d.Capture()
		if err == nil && det {
			return true
		}
		drawRun(name, msg, yellow)
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func tSearchNow(d *finger2.Device) (bool, string) {
	if !waitFinger(d, "SearchNow", "ENCOSTE O DEDO") {
		return false, "sem dedo"
	}
	res, err := d.SearchNow(0, d.Capacity)
	if err != nil {
		return false, err.Error()
	}
	return res.Matched, fmt.Sprintf("confirm=0x%02X page=%d score=%d", res.Confirm, res.Page, res.Score)
}

func tAutoIdentify(d *finger2.Device) (bool, string) {
	if !waitFinger(d, "AutoIdentify", "ENCOSTE O DEDO") {
		return false, "sem dedo"
	}
	if err := d.StartAutoIdentify(0, 0xFFFF, 0); err != nil {
		return false, err.Error()
	}
	var last finger2.IdentifyProgress
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		p, err := d.NextIdentifyEvent(2 * time.Second)
		if err == finger2.ErrTimeout {
			continue
		}
		if err != nil {
			return false, err.Error()
		}
		println("TEST|AUTOID|stage", p.Stage, "confirm", p.Confirm, "id", p.ID, "score", p.Score)
		drawRun("AutoIdentify", fmt.Sprintf("etapa %d confirm %d", p.Stage, p.Confirm), yellow)
		last = p
		if p.Stage == finger2.IdentifyStageVerify {
			break
		}
	}
	_ = d.Cancel()
	ok := last.Stage == finger2.IdentifyStageVerify && last.Confirm == finger2.ConfirmOK
	return ok, fmt.Sprintf("id=%d score=%d confirm=0x%02X", last.ID, last.Score, last.Confirm)
}

// --- UI ---

func drawMenu() {
	top = sel - 2
	if top < 0 {
		top = 0
	}
	if top > len(tests)-5 {
		top = len(tests) - 5
	}
	if top < 0 {
		top = 0
	}

	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 15, "FINGER2 TESTES", cyan)
	fb.fillRect(0, 19, screenW, 1, darkGray)

	for row := 0; row < 5; row++ {
		idx := top + row
		if idx >= len(tests) {
			break
		}
		y := int16(38 + row*18)
		tc := tests[idx]
		col := white
		if idx == sel {
			fb.fillRect(0, y-12, screenW, 16, magenta)
			col = black
		}
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, y, fmt.Sprintf("%02d %s", idx+1, tc.name), col)
		if results[idx] != "" {
			rc := green
			if results[idx] != "PASS" {
				rc = red
			}
			tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 210, y, results[idx], rc)
		}
	}

	fb.fillRect(0, screenH-14, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "A/B: item   C: rodar", gray)
	fb.flush()
}

func drawRun(name, msg string, c color.RGBA) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 15, "TESTE", gray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 40, name, cyan)
	fb.fillRect(0, 46, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 68, msg, c)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, "aguarde / veja a serial", gray)
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
