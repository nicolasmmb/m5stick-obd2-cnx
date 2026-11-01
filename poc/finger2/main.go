package main

import (
	"errors"
	"fmt"
	"image/color"
	"machine"
	"sync/atomic"
	"time"

	"github.com/nicolasmmb/M5Unit-Fingerprint2-Go"

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

const (
	modeRead = iota
	modeImage
	modeEnroll
	modeDelete
	modeClear
	modeInfo
	modeCount
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
	orange   = color.RGBA{255, 140, 0, 255}
	magenta  = color.RGBA{220, 90, 220, 255}

	display st7789.Device
	fb      canvas

	// gray4 maps the sensor 4-bit levels to RGB565 once.
	gray4 [16]pixel.RGB565BE

	mode int

	enrollMode bool
	enrollStep int
	enrollPage uint16

	deleteMode    bool
	deletePending bool
	deletePage    uint16
	deleteScore   uint16
	clearPending  bool

	imgLoaded bool
	imgErr    string
	imgBuff   []byte

	cooldown  time.Time
	lastID    uint16 = 0xFFFF
	lastScore uint16
	hint      string

	infoPage     int
	sn           []byte
	snOK         bool
	workMode     uint8
	wmOK         bool
	sleepTime    uint8
	stOK         bool
	moduleStatus uint8
	msOK         bool

	btnA, btnB, btnC    uint32
	lastA, lastB, lastC time.Time
)

type canvas struct {
	img pixel.Image[pixel.RGB565BE]
}

func newCanvas() canvas {
	return canvas{img: pixel.NewImage[pixel.RGB565BE](screenW, screenH)}
}

func (c *canvas) Size() (int16, int16) { return screenW, screenH }

func (c *canvas) SetPixel(x, y int16, col color.RGBA) {
	if x < 0 || y < 0 || x >= screenW || y >= screenH {
		return
	}
	c.img.Set(int(x), int(y), pixel.NewColor[pixel.RGB565BE](col.R, col.G, col.B))
}

func (c *canvas) Display() error { return nil }

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

func (c *canvas) drawRing(cx, cy, r int16, col color.RGBA) {
	inner := r - 2
	if inner < 1 {
		inner = 1
	}
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			d := dx*dx + dy*dy
			if d <= r*r && d >= inner*inner {
				c.SetPixel(cx+dx, cy+dy, col)
			}
		}
	}
}

func main() {
	println("finger2: boot")
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	initDisplay()
	println("finger2: display ok")
	fb = newCanvas()
	for i := range gray4 {
		g := uint8(i * 17)
		gray4[i] = pixel.NewColor[pixel.RGB565BE](g, g, g)
	}

	uart := machine.UART1
	if err := uart.Configure(machine.UARTConfig{
		BaudRate: 115200,
		TX:       machine.GPIO32,
		RX:       machine.GPIO33,
	}); err != nil {
		println("uart:", err.Error())
		drawError("UART FALHOU", err.Error())
		return
	}

	dev := finger2.New(uart)
	dev.SetTimeout(900 * time.Millisecond)
	println("finger2: uart ok")

	// Bring the UI up before touching the module so a sleeping or missing
	// sensor never blocks the boot.
	setupButtons()
	for {
		err := dev.InitWithTimeout(1500 * time.Millisecond)
		if err == nil {
			println("finger2: init ok")
			break
		}
		println("finger2 init:", err.Error())
		drawWakePrompt(err.Error())
		time.Sleep(500 * time.Millisecond)
	}
	// Drop any button presses queued while the module was waking up.
	atomic.StoreUint32(&btnA, 0)
	atomic.StoreUint32(&btnB, 0)
	atomic.StoreUint32(&btnC, 0)
	println("finger2: FW", dev.Firmware, "capacity", dev.Capacity, "templates", dev.Templates, "matchlevel", dev.MatchLevel, "status", dev.Status)

	_ = dev.SetWorkMode(1) // keep the module active while the screen is on
	if _, err := dev.SetMatchLevel(5); err == nil {
		println("finger2: match level set to 5")
	}
	if v, err := dev.GetSN(); err == nil {
		sn, snOK = v, true
	}
	if v, err := dev.GetWorkMode(); err == nil {
		workMode, wmOK = v, true
	}
	if v, err := dev.GetSleepTime(); err == nil {
		sleepTime, stOK = v, true
	}
	if v, err := dev.ModuleStatus(); err == nil {
		moduleStatus, msOK = v, true
	}

	pwm := machine.PWM0
	_ = pwm.Configure(machine.PWMConfig{Period: 1_000_000_000 / 2700})

	mode = modeRead

	status := modePrompt()
	statusColor := modeColor(mode)
	lastLED := uint8(0xFF)

	for {
		handleButtons(dev, pwm, &status, &statusColor)

		if deletePending || clearPending {
			renderStatus(dev, status, statusColor, lastID, lastScore, true)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if enrollMode {
			status, statusColor = handleEnroll(dev, pwm)
			renderStatus(dev, status, statusColor, lastID, lastScore, true)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if deleteMode {
			status, statusColor = handleDelete(dev, pwm)
			renderStatus(dev, status, statusColor, lastID, lastScore, true)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if mode == modeImage && !imgLoaded && time.Now().After(cooldown) {
			status, statusColor = "CAPTURANDO...", yellow
			hint = ""
			renderStatus(dev, status, statusColor, lastID, lastScore, true)
			ok, msg := takeImage(dev)
			if ok {
				status, statusColor = "IMAGEM CAPTURADA", green
			} else {
				status, statusColor = msg, cyan
			}
			cooldown = time.Now().Add(time.Second)
		}

		if mode == modeRead && time.Now().After(cooldown) {
			detected, capConfirm, err := dev.Capture()
			if err != nil {
				status = "TOQUE NO SENSOR"
				statusColor = yellow
				hint = "toque para acordar o modulo"
				cooldown = time.Now().Add(time.Second)
			} else if detected {
				_ = capConfirm
				status = "LENDO..."
				statusColor = yellow
				hint = ""
				renderStatus(dev, status, statusColor, lastID, lastScore, false)
				_ = dev.SetLED(finger2.LEDOn, finger2.LEDYellow, finger2.LEDYellow, 0)

				// Search compares the char buffer, so the image must be
				// converted into features (buffer 1) before searching.
				gc, err := dev.GenChar(1)
				if err != nil || gc != finger2.ConfirmOK {
					status = "FALHA NA LEITURA"
					statusColor = red
					hint = finger2.Code(gc).Error()
					println("finger2: genchar read failed", gc)
					cooldown = time.Now().Add(2 * time.Second)
					renderStatus(dev, status, statusColor, lastID, lastScore, true)
					time.Sleep(100 * time.Millisecond)
					continue
				}

				res, err := dev.Search(1, 0, dev.Capacity)
				if err != nil {
					status = "ERRO NA BUSCA"
					statusColor = red
					hint = err.Error()
				} else if res.Matched && res.Page < dev.Capacity {
					lastID, lastScore = res.Page, res.Score
					status = fmt.Sprintf("OK ID %d", res.Page)
					statusColor = green
					hint = fmt.Sprintf("confianca %d", res.Score)
					_ = dev.SetLED(finger2.LEDOn, finger2.LEDGreen, finger2.LEDGreen, 0)
					go playTone(pwm, 2700, 120)
					println("finger2: match id", res.Page, "score", res.Score, "confirm", res.Confirm)
				} else {
					lastID, lastScore = 0xFFFF, 0
					status = "NAO RECONHECIDO"
					statusColor = red
					hint = finger2.Code(res.Confirm).Error()
					_ = dev.SetLED(finger2.LEDOn, finger2.LEDRed, finger2.LEDRed, 0)
					go playTone(pwm, 900, 220)
					println("finger2: no match confirm", res.Confirm)
				}
				cooldown = time.Now().Add(2 * time.Second)
			} else {
				status = "ENCOSTE O DEDO"
				statusColor = cyan
				hint = ""
			}
		} else if mode != modeRead && mode != modeInfo && time.Now().After(cooldown) {
			status = modePrompt()
			statusColor = modeColor(mode)
			hint = ""
		}

		if time.Now().After(cooldown) && !enrollMode && !deleteMode && !deletePending && !clearPending && lastLED != finger2.LEDBlue {
			_ = dev.SetLED(finger2.LEDBreath, finger2.LEDBlue, finger2.LEDBlue, 0)
			lastLED = finger2.LEDBlue
		}
		if statusColor == green {
			lastLED = finger2.LEDGreen
		} else if statusColor == red {
			lastLED = finger2.LEDRed
		}

		renderStatus(dev, status, statusColor, lastID, lastScore, true)
		time.Sleep(100 * time.Millisecond)
	}
}

func setupButtons() {
	for _, p := range []machine.Pin{machine.GPIO37, machine.GPIO39, machine.GPIO35} {
		p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	_ = machine.GPIO37.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnA, 1)
	})
	_ = machine.GPIO39.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnB, 1)
	})
	_ = machine.GPIO35.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		atomic.AddUint32(&btnC, 1)
	})
}

func modePrompt() string {
	switch mode {
	case modeImage:
		return "C: CAPTURAR"
	case modeEnroll:
		return "C: CADASTRAR"
	case modeDelete:
		return "C: EXCLUIR"
	case modeClear:
		return "C: APAGAR TUDO"
	case modeInfo:
		return "C: PROX PAGINA"
	default:
		return "ENCOSTE O DEDO"
	}
}

func modeColor(m int) color.RGBA {
	switch m {
	case modeImage:
		return magenta
	case modeEnroll:
		return blue
	case modeDelete:
		return orange
	case modeClear:
		return red
	case modeInfo:
		return white
	default:
		return cyan
	}
}

func tabLabel(m int) string {
	return []string{"LEITURA", "IMAGEM", "CADASTRO", "EXCLUIR", "LIMPAR", "INFO"}[m]
}

func handleButtons(dev *finger2.Device, pwm *machine.LEDCPWM, status *string, col *color.RGBA) {
	now := time.Now()

	resetMode := func() {
		lastID, lastScore = 0xFFFF, 0
		hint = ""
		imgLoaded = false
		imgErr = ""
		*status = modePrompt()
		*col = modeColor(mode)
		cooldown = time.Now()
		_ = dev.SetLED(finger2.LEDBreath, finger2.LEDBlue, finger2.LEDBlue, 0)
	}

	if deletePending || clearPending {
		if atomic.SwapUint32(&btnA, 0) > 0 || atomic.SwapUint32(&btnB, 0) > 0 {
			deletePending = false
			clearPending = false
			*status = "CANCELADO"
			*col = red
			hint = ""
			cooldown = now.Add(500 * time.Millisecond)
			_ = dev.SetLED(finger2.LEDBreath, finger2.LEDBlue, finger2.LEDBlue, 0)
		}
		if atomic.SwapUint32(&btnC, 0) > 0 {
			if deletePending {
				*status, *col = confirmDelete(dev, pwm)
			} else {
				*status, *col = doClear(dev, pwm)
			}
		}
		return
	}

	if enrollMode || deleteMode {
		if atomic.SwapUint32(&btnA, 0) > 0 || atomic.SwapUint32(&btnB, 0) > 0 || atomic.SwapUint32(&btnC, 0) > 0 {
			if enrollMode {
				_ = dev.Cancel()
			}
			enrollMode = false
			deleteMode = false
			*status = "CANCELADO"
			*col = red
			hint = ""
			cooldown = now.Add(500 * time.Millisecond)
			_ = dev.SetLED(finger2.LEDBreath, finger2.LEDBlue, finger2.LEDBlue, 0)
		}
		return
	}

	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 250*time.Millisecond {
		lastA = now
		mode = (mode + 1) % modeCount
		resetMode()
		go playTone(pwm, 1000, 35)
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 250*time.Millisecond {
		lastB = now
		mode = (mode + modeCount - 1) % modeCount
		resetMode()
		go playTone(pwm, 1000, 35)
	}
	if atomic.SwapUint32(&btnC, 0) > 0 && now.Sub(lastC) > 250*time.Millisecond {
		lastC = now
		switch mode {
		case modeImage:
			imgLoaded = false
			imgErr = ""
			*status = "CAPTURANDO..."
			*col = yellow
			hint = ""
			cooldown = time.Now()
			_ = dev.SetLED(finger2.LEDOn, finger2.LEDMagenta, finger2.LEDMagenta, 0)
		case modeEnroll:
			*status, *col = startEnroll(dev)
		case modeDelete:
			*status, *col = startDelete(dev)
		case modeClear:
			clearPending = true
			*status = "APAGAR TUDO?"
			*col = red
			hint = "C:sim  A:nao"
			_ = dev.SetLED(finger2.LEDOn, finger2.LEDRed, finger2.LEDRed, 0)
		case modeInfo:
			infoPage = (infoPage + 1) % 4
			hint = ""
		default:
			*status = "ENCOSTE O DEDO"
			*col = cyan
			hint = ""
		}
	}
}

func startEnroll(dev *finger2.Device) (string, color.RGBA) {
	page, err := dev.FirstFreePage()
	if err != nil {
		println("finger2: first free page:", err.Error())
		page = dev.Templates
	}
	if page >= dev.Capacity {
		return "BASE CHEIA", red
	}
	println("finger2: enroll start page", page)
	if err := dev.StartAutoEnroll(page, 2, finger2.AutoEnrollDefault); err != nil {
		println("finger2: autoenroll start:", err.Error())
		return "FALHA AO INICIAR", red
	}
	enrollMode = true
	enrollStep = 0
	enrollPage = page
	lastID, lastScore = 0xFFFF, 0
	hint = fmt.Sprintf("pagina %d", page)
	_ = dev.SetLED(finger2.LEDOn, finger2.LEDBlue, finger2.LEDBlue, 0)
	return "ENCOSTE O DEDO", blue
}

func startDelete(dev *finger2.Device) (string, color.RGBA) {
	if dev.Templates == 0 {
		return "BASE VAZIA", red
	}
	deleteMode = true
	lastID, lastScore = 0xFFFF, 0
	hint = "encoste o dedo a excluir"
	_ = dev.SetLED(finger2.LEDOn, finger2.LEDYellow, finger2.LEDYellow, 0)
	return "ENC0STE O DEDO", orange
}

func handleDelete(dev *finger2.Device, pwm *machine.LEDCPWM) (string, color.RGBA) {
	detected, confirm, err := dev.Capture()
	if err != nil {
		return failDelete("CAPTURA")
	}
	if confirm == finger2.ConfirmNoFinger {
		return "ENCoste o dedo a excluir", orange
	}
	if confirm != finger2.ConfirmOK || !detected {
		return failDelete(finger2.Code(confirm).Error())
	}

	gc, err := dev.GenChar(1)
	if err != nil || gc != finger2.ConfirmOK {
		return failDelete(finger2.Code(gc).Error())
	}

	res, err := dev.Search(1, 0, dev.Capacity)
	if err != nil {
		return failDelete("BUSCA")
	}
	if !res.Matched || res.Page >= dev.Capacity {
		return failDelete("NAO ENCONTRADO")
	}

	deletePending = true
	deletePage = res.Page
	deleteScore = res.Score
	deleteMode = false
	cooldown = time.Now()
	_ = dev.SetLED(finger2.LEDOn, finger2.LEDYellow, finger2.LEDYellow, 0)
	return fmt.Sprintf("EXCLUIR ID %d?", res.Page), orange
}

func confirmDelete(dev *finger2.Device, pwm *machine.LEDCPWM) (string, color.RGBA) {
	c, err := dev.Delete(deletePage, 1)
	if err != nil || c != finger2.ConfirmOK {
		deletePending = false
		cooldown = time.Now().Add(2 * time.Second)
		return "FALHA AO EXCLUIR", red
	}
	dev.RefreshTemplates()
	lastID, lastScore = deletePage, deleteScore
	deletePending = false
	cooldown = time.Now().Add(3 * time.Second)
	hint = fmt.Sprintf("confianca %d", deleteScore)
	_ = dev.SetLED(finger2.LEDOn, finger2.LEDGreen, finger2.LEDGreen, 0)
	go playTone(pwm, 2700, 150)
	println("finger2: deleted page", deletePage)
	return fmt.Sprintf("EXCLUIDO ID %d", deletePage), green
}

func doClear(dev *finger2.Device, pwm *machine.LEDCPWM) (string, color.RGBA) {
	c, err := dev.Empty()
	if err != nil || c != finger2.ConfirmOK {
		clearPending = false
		cooldown = time.Now().Add(2 * time.Second)
		return "FALHA AO LIMPAR", red
	}
	dev.RefreshTemplates()
	clearPending = false
	lastID, lastScore = 0xFFFF, 0
	hint = ""
	cooldown = time.Now().Add(3 * time.Second)
	_ = dev.SetLED(finger2.LEDOn, finger2.LEDGreen, finger2.LEDGreen, 0)
	go playTone(pwm, 2700, 150)
	println("finger2: all templates deleted")
	return "BASE LIMPA", green
}

func handleEnroll(dev *finger2.Device, pwm *machine.LEDCPWM) (string, color.RGBA) {
	ev, err := dev.NextEnrollEvent(200 * time.Millisecond)
	if errors.Is(err, finger2.ErrTimeout) {
		// No event yet: the module is waiting for a finger.
		if enrollStep == 1 {
			return "LEVANTE O DEDO", yellow
		}
		return "ENCOSTE O DEDO", blue
	}
	if err != nil {
		return failEnroll("COMUNICACAO")
	}

	println("finger2: enroll evt confirm", ev.Confirm, "stage", ev.Stage, "param", ev.Param)

	// Errors that abort enrollment. A missing finger is normal and keeps waiting.
	if ev.Confirm != finger2.ConfirmOK && ev.Confirm != finger2.ConfirmNoFinger {
		return failEnroll(finger2.Code(ev.Confirm).Error())
	}

	switch ev.Stage {
	case finger2.EnrollStageLegalCheck:
		return "VALIDANDO...", yellow
	case finger2.EnrollStageGetImage:
		if ev.Confirm == finger2.ConfirmNoFinger {
			if enrollStep == 1 {
				return "LEVANTE O DEDO", yellow
			}
			return "ENCOSTE O DEDO", blue
		}
		return "CAPTURANDO...", yellow
	case finger2.EnrollStageGenChar:
		return "GERANDO...", yellow
	case finger2.EnrollStageLiftFinger:
		enrollStep = 1
		_ = dev.SetLED(finger2.LEDOn, finger2.LEDYellow, finger2.LEDYellow, 0)
		return "LEVANTE O DEDO", yellow
	case finger2.EnrollStageMerge:
		return "MESCLANDO...", yellow
	case finger2.EnrollStageVerify:
		return "VERIFICANDO...", yellow
	case finger2.EnrollStageStore:
		if ev.Confirm != finger2.ConfirmOK {
			return failEnroll(fmt.Sprintf("STORE 0x%02X", ev.Confirm))
		}
		dev.RefreshTemplates()
		_ = dev.SetWorkMode(1) // keep the unit awake for the screens
		lastID, lastScore = enrollPage, 0
		enrollMode = false
		cooldown = time.Now().Add(3 * time.Second)
		hint = ""
		_ = dev.SetLED(finger2.LEDOn, finger2.LEDGreen, finger2.LEDGreen, 0)
		go playTone(pwm, 2700, 150)
		println("finger2: enrolled page", enrollPage, "templates", dev.Templates)
		return fmt.Sprintf("CADASTRADO ID %d", enrollPage), green
	}
	return "AGUARDE...", yellow
}

func failEnroll(msg string) (string, color.RGBA) {
	println("finger2: enroll fail:", msg)
	enrollMode = false
	cooldown = time.Now().Add(2 * time.Second)
	hint = ""
	return "FALHA " + msg, red
}

func failDelete(msg string) (string, color.RGBA) {
	deleteMode = false
	deletePending = false
	cooldown = time.Now().Add(2 * time.Second)
	hint = ""
	return "FALHA " + msg, red
}

func renderStatus(dev *finger2.Device, status string, c color.RGBA, id, score uint16, flush bool) {
	fb.clear()
	drawTabs()

	if mode == modeInfo {
		renderInfo(dev)
		drawFooter()
		if flush {
			fb.flush()
		}
		return
	}

	if mode == modeImage {
		renderImage(status, c)
		drawFooter()
		if flush {
			fb.flush()
		}
		return
	}

	iconColor := modeColor(mode)
	fb.drawRing(208, 60, 14, iconColor)
	fb.drawRing(208, 60, 10, iconColor)
	fb.drawRing(208, 60, 6, iconColor)

	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 62, status, c)
	if hint != "" {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 80, hint, white)
	} else if id != 0xFFFF && !enrollMode && !deleteMode && !deletePending && !clearPending {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 80,
			fmt.Sprintf("confianca %d", score), white)
	}

	if id != 0xFFFF {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 96,
			fmt.Sprintf("ID %d   score %d", id, score), gray)
	} else {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 96, "ID --", gray)
	}

	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 110,
		fmt.Sprintf("Cap %d   Cadastradas %d", dev.Capacity, dev.Templates), gray)

	drawFooter()
	if flush {
		fb.flush()
	}
}

// drawTabs draws two rows of three tabs with the full feature names.
func drawTabs() {
	const tw, th = 80, 16
	for i := 0; i < modeCount; i++ {
		x := int16((i % 3) * tw)
		y := int16((i / 3) * th)
		label := tabLabel(i)
		tc := gray
		if i == mode {
			fb.fillRect(x, y, tw, th, modeColor(i))
			tc = black
		}
		tx := x + int16((tw-len(label)*6)/2)
		tinyfont.WriteLine(&fb, &tinyfont.Org01, tx, y+12, label, tc)
	}
	fb.fillRect(0, 2*th, screenW, 1, darkGray)
}

// renderImage shows the 80x208 4bpp image rotated 90 degrees clockwise.
func renderImage(status string, c color.RGBA) {
	if !imgLoaded {
		if imgErr != "" {
			tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 62, "FALHA "+imgErr, red)
		} else {
			tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 62, status, c)
		}
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 82,
			"encoste o dedo e aguarde", white)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 100,
			"C: nova captura", gray)
		return
	}

	const (
		dy = 38
		iw = finger2.ImageWidth
	)

	// Use the rows actually present in the upload (usually 206).
	ih := len(imgBuff) * 2 / iw
	if ih > finger2.ImageHeight {
		ih = finger2.ImageHeight
	}
	if ih < 1 {
		return
	}
	dx := int16((screenW - ih) / 2)

	// Frame so the image area is visible even when the capture is black.
	for x := int16(0); x < int16(ih)+2; x++ {
		fb.SetPixel(dx-1+x, dy-1, darkGray)
		fb.SetPixel(dx-1+x, dy+iw, darkGray)
	}
	for y := int16(0); y < iw+2; y++ {
		fb.SetPixel(dx-1, dy-1+y, darkGray)
		fb.SetPixel(dx+int16(ih), dy-1+y, darkGray)
	}

	for y := 0; y < ih; y++ {
		for x := 0; x < iw; x++ {
			idx := (y*iw + x) / 2
			if idx >= len(imgBuff) {
				continue
			}
			var v uint8
			if (y*iw+x)%2 == 0 {
				v = imgBuff[idx] & 0x0F
			} else {
				v = imgBuff[idx] >> 4
			}
			sx := int(dx) + ih - 1 - y
			sy := dy + x
			fb.img.Set(sx, sy, gray4[v])
		}
	}

}

func drawFooter() {
	text := "A/B: aba   C: acao"
	tc := gray
	if deletePending || clearPending {
		text = "C: confirmar   A: cancelar"
		tc = orange
		if clearPending {
			tc = red
		}
	} else if enrollMode || deleteMode {
		text = "C: cancelar"
		tc = yellow
	} else if mode == modeImage {
		text = "C: nova captura   A/B: aba"
	} else if mode == modeInfo {
		text = "C: prox pagina   A/B: aba"
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 132, text, tc)
}

// takeImage grabs one frame and uploads it from the sensor.
func takeImage(dev *finger2.Device) (bool, string) {
	detected, confirm, err := dev.Capture()
	if err != nil {
		imgErr = "CAPTURA"
		println("finger2: image capture err", err.Error())
		return false, "AGUARDE 1s"
	}
	if !detected {
		imgErr = ""
		println("finger2: image no finger", confirm)
		return false, "ENCOSTE O DEDO"
	}

	// Let the finger settle so the first-contact frame is not blurry.
	time.Sleep(350 * time.Millisecond)
	again, confirm2, err2 := dev.Capture()
	if err2 != nil || !again {
		println("finger2: image settle lost", confirm2)
		return false, "ENCOSTE O DEDO"
	}

	if cap(imgBuff) < finger2.ImageWidth*finger2.ImageHeight/2 {
		imgBuff = make([]byte, finger2.ImageWidth*finger2.ImageHeight/2)
	}
	img := imgBuff[:cap(imgBuff)]
	n, err := dev.UpImageTo(img)
	if n == 0 {
		imgErr = "UPLOAD"
		if err != nil {
			println("finger2: upimage err", err.Error())
		}
		return false, "FALHA NO UPLOAD"
	}
	imgBuff = img[:n]
	imgLoaded = true
	imgErr = ""

	// Diagnostics: value range and first bytes.
	minV, maxV := uint8(15), uint8(0)
	var sum uint64
	for i, b := range img {
		for _, v := range [2]uint8{b & 0x0F, b >> 4} {
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
			sum += uint64(v)
		}
		_ = i
	}
	avg := uint64(0)
	if n := uint64(len(img) * 2); n > 0 {
		avg = sum / n
	}
	println("finger2: image bytes", len(img), "min", minV, "max", maxV, "avg", avg)
	if len(img) >= 16 {
		println("finger2: image head", img[0], img[1], img[2], img[3], img[4], img[5], img[6], img[7],
			img[8], img[9], img[10], img[11], img[12], img[13], img[14], img[15])
	}
	return true, ""
}

func renderInfo(dev *finger2.Device) {
	snText := "--"
	if snOK {
		// This unit returns the SN right-aligned with leading zero bytes.
		i := 0
		for i < len(sn) && sn[i] == 0 {
			i++
		}
		tail := sn[i:]
		if len(tail) == 0 {
			snText = "00000000"
		} else if len(tail) > 8 {
			snText = fmt.Sprintf("%X", tail[len(tail)-8:])
		} else {
			snText = fmt.Sprintf("%X", tail)
		}
	}
	switch infoPage {
	case 0:
		line(38, "SISTEMA", modeColor(modeInfo))
		line(54, fmt.Sprintf("FW 0x%02X", dev.Firmware), white)
		line(70, "SN "+snText, white)
		line(86, fmt.Sprintf("Status 0x%04X", dev.Status), gray)
	case 1:
		line(38, "MEMORIA", modeColor(modeInfo))
		line(54, fmt.Sprintf("Capacidade %d", dev.Capacity), white)
		line(70, fmt.Sprintf("Cadastradas %d", dev.Templates), white)
		line(86, fmt.Sprintf("MatchLevel %d", dev.MatchLevel), gray)
	case 2:
		line(38, "ENERGIA", modeColor(modeInfo))
		if wmOK {
			line(54, fmt.Sprintf("WorkMode %d", workMode), white)
		} else {
			line(54, "WorkMode --", gray)
		}
		if stOK {
			line(70, fmt.Sprintf("SleepTime %d s", sleepTime), white)
		} else {
			line(70, "SleepTime --", gray)
		}
		if msOK {
			line(86, fmt.Sprintf("Modulo 0x%02X", moduleStatus), gray)
		}
	case 3:
		line(38, "ULTIMO RESULTADO", modeColor(modeInfo))
		if lastID != 0xFFFF {
			line(54, fmt.Sprintf("ID %d", lastID), white)
			line(70, fmt.Sprintf("Score %d", lastScore), white)
		} else {
			line(54, "ID --", gray)
		}
	}
	line(112, fmt.Sprintf("Pagina %d/4", infoPage+1), gray)
}

func line(y int16, s string, c color.RGBA) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, y, s, c)
}

func initDisplay() {
	machine.SPI2.Configure(machine.SPIConfig{
		Frequency: 40 * machine.MHz,
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
}

func drawWakePrompt(reason string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 22, "FINGER2", cyan)
	fb.fillRect(0, 26, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 56, "SENSOR OFFLINE", red)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 76, "encoste o dedo para acordar", white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 92, "o LED ring deve acender", gray)
	if reason != "" {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 110, reason, gray)
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 126, "Grove G32/G33  115200 8N1", gray)
	fb.flush()
}

func drawError(title, msg string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 30, title, red)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 54, msg, white)
	fb.flush()
}

func playTone(pwm *machine.LEDCPWM, freq uint32, ms int) {
	if err := pwm.Configure(machine.PWMConfig{Period: uint64(1_000_000_000 / freq)}); err != nil {
		return
	}
	ch, err := pwm.Channel(machine.GPIO2)
	if err != nil {
		return
	}
	pwm.Set(ch, pwm.Top()/2)
	time.Sleep(time.Duration(ms) * time.Millisecond)
	pwm.Set(ch, 0)
}
