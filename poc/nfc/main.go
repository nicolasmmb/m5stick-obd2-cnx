// POC: M5Stack Unit NFC (U216, ST25R3916) protocol probe.
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

	pageA = iota
	pageF
	pageB
	pageV
	pageT2
	pageCL
	pageEmuF
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

	dev  *nfc.Device
	rfOn = true

	writeArmUntil time.Time
	writeMsg      string
	writeMsgUntil time.Time
	chipTyp       uint8
	chipRev       uint8
	scans         uint32
	errors        uint32
	emuEvt        uint32
)

type uiState struct {
	page     int
	online   bool
	rf       bool
	card     bool
	uid      [7]byte
	uidLen   uint8
	atqa     uint16
	sak      uint8
	typeID   uint8
	mem      uint16
	idm      [8]byte
	pmm      [8]byte
	fType    uint8
	sysCodes [16]uint16
	sysCount uint8
	bPupi    [4]byte
	bApp     [4]byte
	bProto   [3]byte
	vUID     [8]byte
	vDSFID   uint8
	t2Data   [4]byte
	t2OK     bool
	clData   [16]byte
	clOK     bool
	chipTyp  uint8
	chipRev  uint8
	detectUS int64
	scans    uint32
	errors   uint32
	emu      bool
	emuEvt   uint32
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
			scans:   atomic.LoadUint32(&scans),
			errors:  atomic.LoadUint32(&errors),
		}
		if page == pageEmuF {
			if act, err := dev.EmulationActivity(); err == nil && act {
				atomic.AddUint32(&emuEvt, 1)
			}
			st.emu = dev.Emulating()
			st.emuEvt = atomic.LoadUint32(&emuEvt)
		} else if rfOn {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
			start := time.Now()
			ok := false
			var err error
			switch page {
			case pageA:
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
			case pageF:
				var c nfc.FeliCaInfo
				c, ok, err = dev.DetectFInfo(ctx)
				if ok {
					st.idm = c.IDm
					st.pmm = c.PMm
					st.fType = c.Type
					st.sysCodes = c.SystemCodes
					st.sysCount = c.SystemCount
				}
			case pageB:
				var c nfc.NFCBCard
				c, ok, err = dev.DetectB(ctx)
				if ok {
					st.bPupi = c.PUPI
					st.bApp = c.Application
					st.bProto = c.Protocol
				}
			case pageV:
				var c nfc.ISO15693Card
				c, ok, err = dev.DetectV(ctx)
				if ok {
					st.vUID = c.UID
					st.vDSFID = c.DSFID
				}
			case pageT2:
				var c nfc.Card
				c, ok, err = dev.Detect(ctx)
				if ok && c.IsType2() {
					st.uid = c.UID
					st.uidLen = c.UIDLen
					st.typeID = c.TypeID
					st.t2Data, st.t2OK, _ = dev.Type2ReadPage(context.Background(), 4)
					_ = dev.Halt(context.Background())
				} else if ok {
					ok = false // card is NFC-A but not Type 2
				}
			case pageCL:
				var c nfc.Card
				c, ok, err = dev.Detect(ctx)
				if ok && c.IsClassic() {
					st.uid = c.UID
					st.uidLen = c.UIDLen
					st.typeID = c.TypeID
					key := [6]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
					st.clData, st.clOK, _ = dev.ClassicReadBlock(context.Background(), c.UID[:c.UIDLen], 1, key)
					_ = dev.Halt(context.Background())
				} else if ok {
					ok = false
				}
			}
			cancel()
			st.detectUS = int64(time.Since(start) / time.Microsecond)
			if err != nil {
				atomic.AddUint32(&errors, 1)
				println("nfc: detect", err.Error())
			}
			st.card = ok
			atomic.AddUint32(&scans, 1)
			st.scans = atomic.LoadUint32(&scans)
			st.errors = atomic.LoadUint32(&errors)
		}
		render(st)

		if time.Since(lastLog) > time.Second {
			switch {
			case st.card && st.page == pageA:
				println("nfc A", "uid", fmt.Sprintf("% X", st.uid[:st.uidLen]),
					"type", nfc.TypeName(st.typeID), "atqa", st.atqa,
					"sak", st.sak, "mem", st.mem, "detect_us", st.detectUS,
					"scans", st.scans, "err", st.errors)
			case st.card && st.page == pageF:
				sys := ""
				for i := 0; i < int(st.sysCount) && i < 4; i++ {
					sys += fmt.Sprintf(" %04X", st.sysCodes[i])
				}
				println("nfc F", "idm", fmt.Sprintf("% X", st.idm[:]),
					"pmm", fmt.Sprintf("% X", st.pmm[:]),
					"type", nfc.FeliCaInfo{Type: st.fType}.TypeName(),
					"sys", sys,
					"detect_us", st.detectUS, "scans", st.scans, "err", st.errors)
			case st.card && st.page == pageB:
				println("nfc B", "pupi", fmt.Sprintf("% X", st.bPupi[:]),
					"app", fmt.Sprintf("% X", st.bApp[:]), "proto", fmt.Sprintf("% X", st.bProto[:]),
					"detect_us", st.detectUS, "scans", st.scans, "err", st.errors)
			case st.card && st.page == pageV:
				println("nfc V", "uid", fmt.Sprintf("% X", st.vUID[:]),
					"dsfid", st.vDSFID, "detect_us", st.detectUS,
					"scans", st.scans, "err", st.errors)
			case st.card && st.page == pageT2:
				println("nfc T2", "uid", fmt.Sprintf("% X", st.uid[:st.uidLen]),
					"page4", fmt.Sprintf("% X", st.t2Data[:]), "ok", st.t2OK,
					"detect_us", st.detectUS, "err", st.errors)
			case st.card && st.page == pageCL:
				println("nfc CL", "uid", fmt.Sprintf("% X", st.uid[:st.uidLen]),
					"block1", fmt.Sprintf("% X", st.clData[:]), "auth_ok", st.clOK,
					"detect_us", st.detectUS, "err", st.errors)
			case st.page == pageEmuF:
				println("nfc EMU", "emulating", st.emu, "events", st.emuEvt, "err", st.errors)
			default:
				println("nfc", pageName(st.page), "no card; scans", st.scans,
					"err", st.errors, "detect_us", st.detectUS, "rf", rfOn)
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

func reinitChip() {
	drawBoot("reiniciando")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	err := dev.Begin(ctx)
	cancel()
	if err != nil {
		println("nfc: begin", err.Error())
		drawOffline(err.Error())
		return
	}
	if page <= pageB || page == pageT2 || page == pageCL {
		_ = configurePage(page)
	}
	chipTyp, chipRev, _ = dev.Identity()
	atomic.StoreUint32(&scans, 0)
	atomic.StoreUint32(&errors, 0)
	rfOn = true
	last = uiState{}
	println("nfc: reinit ok")
}

func nextPage() {
	page = (page + 1) % pageCount
	if dev.Emulating() && page != pageEmuF {
		_ = dev.StopEmulation(context.Background())
	}
	if err := configurePage(page); err != nil {
		println("nfc: configure", err.Error())
	} else {
		rfOn = true
	}
	last = uiState{}
	println("nfc: tela", pageName(page))
}

func writeTest() {
	drawWriting("GRAVANDO...")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ok := false
	var err error
	switch page {
	case pageT2:
		var c nfc.Card
		c, ok, err = dev.Detect(ctx)
		if err == nil && ok && c.IsType2() {
			ok, err = dev.Type2WritePage(ctx, 4, [4]byte{0xCA, 0xFE, 0xBA, 0xBE})
			_ = dev.Halt(ctx)
			if err == nil && ok {
				writeMsg = "PAG 4 GRAVADA OK"
			} else {
				writeMsg = "FALHA AO GRAVAR"
			}
		} else {
			writeMsg = "SEM CARTAO TYPE 2"
		}
	case pageCL:
		var c nfc.Card
		c, ok, err = dev.Detect(ctx)
		if err == nil && ok && c.IsClassic() {
			data := [16]byte{0xDE, 0xAD, 0xBE, 0xEF,
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				0x09, 0x0A, 0x0B, 0x0C}
			ok, err = dev.ClassicWriteBlock(ctx, c.UID[:c.UIDLen], 1, [6]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, data)
			_ = dev.Halt(ctx)
			if err == nil && ok {
				writeMsg = "BLOCO 1 GRAVADO OK"
			} else if err != nil {
				writeMsg = "FALHA: " + err.Error()
			} else {
				writeMsg = "FALHA AO GRAVAR"
			}
		} else {
			writeMsg = "SEM CARTAO CLASSIC"
		}
	default:
		writeMsg = "TELA SEM ESCRITA"
	}
	writeMsgUntil = time.Now().Add(3 * time.Second)
	println("nfc: write", writeMsg)
	last = uiState{}
}

func handleButtons() {
	now := time.Now()
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 400*time.Millisecond {
		lastB = now
		if page == pageT2 || page == pageCL {
			if now.Before(writeArmUntil) {
				writeArmUntil = time.Time{}
				writeTest()
			} else {
				writeArmUntil = now.Add(3 * time.Second)
				writeMsg = "B DE NOVO P/ GRAVAR"
				writeMsgUntil = writeArmUntil
				last = uiState{}
			}
		} else {
			reinitChip()
		}
	}
	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 250*time.Millisecond {
		lastA = now
		if rfOn {
			_ = dev.RFOff()
			rfOn = false
		} else {
			_ = dev.RFOn()
			rfOn = true
		}

		if atomic.SwapUint32(&btnC, 0) > 0 && now.Sub(lastC) > 250*time.Millisecond {
			lastC = now
			nextPage()
		}
		println("nfc: rf", rfOn)
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

	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "UNIT NFC "+pageName(st.page), cyan)
	state, col := "OK", green
	if !st.online {
		state, col = "OFFLINE", red
	} else if !st.rf {
		state, col = "RF OFF", yellow
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 196, 14, state, col)
	fb.fillRect(0, 18, screenW, 1, darkGray)

	switch st.page {
	case pageA:
		drawA(st)
	case pageF:
		drawF(st)
	case pageB:
		drawB(st)
	case pageV:
		drawV(st)
	case pageT2:
		drawT2(st)
	case pageCL:
		drawCL(st)
	case pageEmuF:
		drawEmuF(st)
	case pageInfo:
		drawInfo(st)
	}

	fb.fillRect(0, 120, screenW, 1, darkGray)
	hint := "A: RF  B: reiniciar  C: pagina"
	if st.page == pageT2 || st.page == pageCL {
		hint = "B 2x: GRAVAR   C: pagina   A: RF"
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 133, hint, gray)
	fb.flush()
}

func drawA(st uiState) {
	if !st.card {
		drawSearch("NFC-A / MIFARE")
		return
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36, "NFC-A / UID", gray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 52,
		fmt.Sprintf("% X", st.uid[:st.uidLen]), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 70, nfc.TypeName(st.typeID), green)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 86,
		fmt.Sprintf("ATQA %04X  SAK %02X", st.atqa, st.sak), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 100,
		fmt.Sprintf("MEM %d B   %d us", st.mem, st.detectUS), white)
}

func drawF(st uiState) {
	if !st.card {
		drawSearch("NFC-F / FeliCa")
		return
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 34, "NFC-F", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 42, 34,
		nfc.FeliCaInfo{Type: st.fType}.TypeName(), green)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 50,
		fmt.Sprintf("% X", st.idm[:]), green)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 70,
		fmt.Sprintf("PMm % X", st.pmm[:]), white)
	if st.sysCount > 0 {
		n := int(st.sysCount)
		if n > 4 {
			n = 4
		}
		line := "SYS "
		for i := 0; i < n; i++ {
			line += fmt.Sprintf(" %04X", st.sysCodes[i])
		}
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 88, line, white)
	} else {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 88, "SYS --", gray)
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 106,
		fmt.Sprintf("%d us", st.detectUS), gray)
}

func drawB(st uiState) {
	if !st.card {
		drawSearch("NFC-B / ISO14443-B")
		return
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36, "NFC-B / ATQB", gray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 52,
		fmt.Sprintf("% X", st.bPupi[:]), green)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 72,
		fmt.Sprintf("APP   % X", st.bApp[:]), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 88,
		fmt.Sprintf("PROTO % X", st.bProto[:]), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 104,
		fmt.Sprintf("%d us", st.detectUS), white)
}

func drawT2(st uiState) {
	if !st.card {
		drawSearch("NFC-A / TYPE 2")
		return
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 34, "TYPE 2 / "+nfc.TypeName(st.typeID), gray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 50,
		fmt.Sprintf("% X", st.uid[:st.uidLen]), green)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 72, "PAGE 4", gray)
	if st.t2OK {
		tinyfont.WriteLine(&fb, &freemono.Bold12pt7b, 4, 96,
			fmt.Sprintf("% X", st.t2Data[:]), white)
	} else {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 96, "READ FALHOU", red)
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 104,
		fmt.Sprintf("%d us", st.detectUS), gray)
	if time.Now().Before(writeMsgUntil) {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 116, writeMsg, green)
	}
}

func drawCL(st uiState) {
	if !st.card {
		drawSearch("NFC-A / MIFARE CLASSIC")
		return
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 34, "CLASSIC / "+nfc.TypeName(st.typeID), gray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 50,
		fmt.Sprintf("% X", st.uid[:st.uidLen]), green)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 72, "BLOCO 1 (chave A)", gray)
	if st.clOK {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 88,
			fmt.Sprintf("% X", st.clData[:8]), white)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 102,
			fmt.Sprintf("% X", st.clData[8:]), white)
	} else {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 88, "AUTH/READ FALHOU", red)
	}
	if time.Now().Before(writeMsgUntil) {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 116, writeMsg, green)
	}
}

func drawEmuF(st uiState) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 34, "EMULACAO NFC-F", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 50, "FeliCa Lite (PT memory)", green)
	idm := [8]byte{0x01, 0x2E, 0x50, 0xE5, 0x3C, 0x4B, 0x4F, 0x29}
	pmm := [8]byte{0x00, 0xF1, 0x00, 0x00, 0x00, 0x01, 0x43, 0x00}
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 66, fmt.Sprintf("% X", idm[:]), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 82, fmt.Sprintf("PMm % X", pmm[:]), white)
	if st.emu {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 100, "aproxime um leitor/telefone", gray)
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 112, fmt.Sprintf("eventos RF %d", st.emuEvt), white)
	} else {
		tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 100, "EMULACAO INATIVA", red)
	}
}

func drawV(st uiState) {
	if !st.card {
		drawSearch("NFC-V / ISO15693")
		return
	}
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36, "NFC-V / ISO15693", gray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 52,
		fmt.Sprintf("% X", st.vUID[:]), green)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 74,
		fmt.Sprintf("DSFID %02X   %d us", st.vDSFID, st.detectUS), white)
}

func drawSearch(proto string) {
	tinyfont.WriteLine(&fb, &freemono.Bold12pt7b, 4, 56, "APROXIME", yellow)
	tinyfont.WriteLine(&fb, &freemono.Bold12pt7b, 4, 80, "UM CARTAO", yellow)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 102, proto, gray)
}

func drawInfo(st uiState) {
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 36,
		fmt.Sprintf("CHIP %d  REV %d  ADDR 0x50", st.chipTyp, st.chipRev), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 52,
		fmt.Sprintf("RF %v  TELA %s  DETECT %d us", st.rf, pageName(st.page), st.detectUS), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 68,
		fmt.Sprintf("SCANS %d  ERRORS %d", st.scans, st.errors), white)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 88, "FASE 2: READ T2 + CLASSIC", gray)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 102, "WRITE/NDEF: proxima fase", gray)
}

func pageName(p int) string {
	switch p {
	case pageA:
		return "A"
	case pageF:
		return "F"
	case pageB:
		return "B"
	case pageV:
		return "V"
	case pageT2:
		return "T2"
	case pageCL:
		return "CL"
	case pageEmuF:
		return "EMU"
	}
	return "INFO"
}

func configurePage(p int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	switch p {
	case pageA:
		return dev.ConfigureA(ctx)
	case pageF:
		return dev.ConfigureF(ctx)
	case pageB:
		return dev.ConfigureB(ctx)
	case pageV:
		return dev.ConfigureV(ctx)
	case pageT2, pageCL:
		return dev.ConfigureA(ctx)
	case pageEmuF:
		return dev.StartEmulationF(ctx, nfc.EmulateFConfig{
			IDm:        [8]byte{0x01, 0x2E, 0x50, 0xE5, 0x3C, 0x4B, 0x4F, 0x29},
			PMm:        [8]byte{0x00, 0xF1, 0x00, 0x00, 0x00, 0x01, 0x43, 0x00},
			SystemCode: 0xFFFF,
		})
	}
	return nil
}

func drawWriting(msg string) {
	fb.clear()
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 14, "UNIT NFC", cyan)
	fb.fillRect(0, 18, screenW, 1, darkGray)
	tinyfont.WriteLine(&fb, &freemono.Bold9pt7b, 4, 56, msg, yellow)
	tinyfont.WriteLine(&fb, &proggy.TinySZ8pt7b, 4, 78, "nao remova o cartao", white)
	fb.flush()
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
