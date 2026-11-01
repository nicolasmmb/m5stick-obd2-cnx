package main

import (
	"image/color"
	"machine"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"tinygo.org/x/bluetooth"
	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/espradio"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
	"tinygo.org/x/tinyfont/proggy"
)

const (
	screenW     = 240
	lineHeight  = 10
	visibleRows = 9

	footerY = 132

	bleScanTime = 8 * time.Second
	scanPause   = 5 * time.Second
)

var (
	black = color.RGBA{0, 0, 0, 255}
	white = color.RGBA{235, 235, 235, 255}
	gray  = color.RGBA{70, 70, 70, 255}
	cyan  = color.RGBA{0, 220, 220, 255}
	green = color.RGBA{0, 220, 90, 255}
	red   = color.RGBA{255, 80, 80, 255}
)

var (
	display st7789.Device
	adapter = bluetooth.DefaultAdapter

	wifiList []espradio.AccessPoint
	bleList  []bleInfo

	wifiSel, bleSel       int
	wifiScroll, bleScroll int
	tab                   int // 0 = WiFi, 1 = BLE

	btnA, btnB, btnC    uint32
	lastA, lastB, lastC time.Time

	btnAErr, btnBErr, btnCErr error
)

type bleInfo struct {
	name string
	addr string
	rssi int16
}

func main() {
	// M5StickC Plus2: keep power on when running from battery.
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	initButtons()
	initDisplay()
	renderAll()
	time.Sleep(500 * time.Millisecond)

	println("display-scan: initializing WiFi...")
	drawFullStatus("WIFI", "inicializando radio...", cyan)
	if err := espradio.Enable(espradio.Config{}); err != nil {
		println("wifi enable failed:", err.Error())
		drawFullStatus("WIFI", "erro: "+err.Error(), red)
		time.Sleep(5 * time.Second)
		return
	}
	if err := espradio.Start(); err != nil {
		println("wifi start failed:", err.Error())
		drawFullStatus("WIFI", "erro: "+err.Error(), red)
		time.Sleep(5 * time.Second)
		return
	}

	println("display-scan: initializing BLE...")
	drawFullStatus("BLE", "inicializando stack...", green)
	if err := adapter.Enable(); err != nil {
		println("ble enable failed:", err.Error())
		drawFullStatus("BLE", "erro: "+err.Error(), red)
		time.Sleep(5 * time.Second)
		return
	}
	adapter.SetScanType(bluetooth.ScanTypeActive)

	wifiCh := make(chan []espradio.AccessPoint, 1)
	bleCh := make(chan []bleInfo, 1)
	go scanLoop(wifiCh, bleCh)

	renderAll()
	for {
		select {
		case aps := <-wifiCh:
			wifiList = aps
			wifiSel = clamp(wifiSel, len(wifiList))
			wifiScroll = adjustScroll(wifiSel, wifiScroll, len(wifiList))
			println("wifi:", len(wifiList), "APs")
			renderHeader()
			if tab == 0 {
				renderListFull()
			}
		case devs := <-bleCh:
			bleList = devs
			bleSel = clamp(bleSel, len(bleList))
			bleScroll = adjustScroll(bleSel, bleScroll, len(bleList))
			println("ble:", len(bleList), "devices")
			renderHeader()
			if tab == 1 {
				renderListFull()
			}
		default:
		}

		handleButtons()
		time.Sleep(20 * time.Millisecond)
	}
}

func scanLoop(wifiCh chan<- []espradio.AccessPoint, bleCh chan<- []bleInfo) {
	// Printed again here because the boot output may be missed by the monitor.
	println("buttons idle A/B/C:",
		machine.GPIO37.Get(), machine.GPIO39.Get(), machine.GPIO35.Get())
	if btnAErr != nil {
		println("btnA interrupt error:", btnAErr.Error())
	}
	if btnBErr != nil {
		println("btnB interrupt error:", btnBErr.Error())
	}
	if btnCErr != nil {
		println("btnC interrupt error:", btnCErr.Error())
	}

	for {
		start := time.Now()
		aps, err := espradio.Scan()
		if err != nil {
			println("wifi scan failed:", err.Error())
		} else {
			println("scan wifi:", len(aps), "APs in", int64(time.Since(start)/time.Millisecond), "ms")
			select {
			case wifiCh <- aps:
			default:
			}
		}

		start = time.Now()
		devs := scanBLE(bleScanTime)
		println("scan ble:", len(devs), "devices in", int64(time.Since(start)/time.Millisecond), "ms")
		select {
		case bleCh <- devs:
		default:
		}

		time.Sleep(scanPause)
	}
}

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
		machine.GPIO12, // RST
		machine.GPIO14, // DC
		machine.GPIO5,  // CS
		machine.GPIO27, // BL
	)
	// The M5StickC Plus2 panel uses BGR color order.
	display.IsBGR(true)

	// M5StickC Plus2: 135x240 ST7789V2. Landscape, offsets for the
	// 240x320 controller RAM (M5Stack: colstart 40, rowstart 53).
	display.Configure(st7789.Config{
		Width:        135,
		Height:       240,
		Rotation:     drivers.Rotation90,
		RowOffset:    40,
		ColumnOffset: 52,
		FrameRate:    st7789.FRAMERATE_60,
	})
}

func initButtons() {
	// A = GPIO37 (front/center), B = GPIO39, C = GPIO35 (power side button).
	for _, p := range []machine.Pin{machine.GPIO37, machine.GPIO39, machine.GPIO35} {
		p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	set := func(p machine.Pin, counter *uint32) error {
		return p.SetInterrupt(machine.PinFalling, func(machine.Pin) {
			atomic.AddUint32(counter, 1)
		})
	}
	btnAErr = set(machine.GPIO37, &btnA)
	btnBErr = set(machine.GPIO39, &btnB)
	btnCErr = set(machine.GPIO35, &btnC)
}

func handleButtons() {
	now := time.Now()

	if atomic.SwapUint32(&btnA, 0) > 0 && now.Sub(lastA) > 200*time.Millisecond {
		lastA = now
		tab = 1 - tab
		println("button A: tab", tab)
		renderHeader()
		renderListFull()
	}
	if atomic.SwapUint32(&btnB, 0) > 0 && now.Sub(lastB) > 200*time.Millisecond {
		lastB = now
		println("button B: up")
		moveSelection(-1)
	}
	if atomic.SwapUint32(&btnC, 0) > 0 && now.Sub(lastC) > 200*time.Millisecond {
		lastC = now
		println("button C: down")
		moveSelection(1)
	}
}

func moveSelection(delta int) {
	n := listLen()
	if n == 0 {
		return
	}

	oldSel := listSel()
	oldScroll := listScroll()
	newSel := clamp(oldSel+delta, n)
	newScroll := adjustScroll(newSel, oldScroll, n)

	if newSel == oldSel && newScroll == oldScroll {
		return
	}

	if tab == 0 {
		wifiSel, wifiScroll = newSel, newScroll
	} else {
		bleSel, bleScroll = newSel, newScroll
	}

	if newScroll != oldScroll {
		// The visible window moved: redraw the list area only.
		renderListFull()
	} else {
		// Same window: only the old and new selected rows change.
		renderRow(oldSel, false)
		renderRow(newSel, true)
	}
}

func renderAll() {
	renderHeader()
	renderListFull()
	renderFooter()
}

func renderHeader() {
	display.FillRectangle(0, 0, screenW, 21, black)

	wifiColor := gray
	bleColor := gray
	if tab == 0 {
		wifiColor = cyan
	} else {
		bleColor = green
	}

	tinyfont.WriteLine(&display, &freemono.Bold9pt7b, 4, 15, "WIFI", wifiColor)
	tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 62, 14, strconv.Itoa(len(wifiList)), wifiColor)
	tinyfont.WriteLine(&display, &freemono.Bold9pt7b, 150, 15, "BLE", bleColor)
	tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 198, 14, strconv.Itoa(len(bleList)), bleColor)

	if tab == 0 {
		display.FillRectangle(4, 17, 48, 2, cyan)
	} else {
		display.FillRectangle(150, 17, 40, 2, green)
	}
	display.FillRectangle(0, 20, screenW, 1, gray)
}

func renderListFull() {
	n := listLen()
	sel := listSel()
	scroll := listScroll()

	// Redraw row by row instead of clearing the whole list area first. This
	// avoids the full-screen "blink" when switching tabs or scrolling.
	for row := 0; row < visibleRows; row++ {
		idx := scroll + row
		y := int16(32 + row*lineHeight)
		if idx >= n {
			display.FillRectangle(0, y-8, screenW, lineHeight, black)
			continue
		}
		renderRow(idx, idx == sel)
	}
}

func renderRow(idx int, selected bool) {
	row := idx - listScroll()
	if row < 0 || row >= visibleRows {
		return
	}

	y := int16(32 + row*lineHeight)
	bg, fg := black, white
	if selected {
		bg, fg = white, black
	}

	display.FillRectangle(0, y-8, screenW, lineHeight, bg)
	tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 4, y, listText(idx), fg)
}

func renderFooter() {
	tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 4, footerY,
		"A:tab  B:up  C:down", gray)
}

func drawFullStatus(title, msg string, c color.RGBA) {
	display.FillScreen(black)
	tinyfont.WriteLine(&display, &freemono.Bold9pt7b, 4, 30, title, c)
	tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 4, 50, truncate(msg, 29), white)
}

func listLen() int {
	if tab == 0 {
		return len(wifiList)
	}
	return len(bleList)
}

func listSel() int {
	if tab == 0 {
		return wifiSel
	}
	return bleSel
}

func listScroll() int {
	if tab == 0 {
		return wifiScroll
	}
	return bleScroll
}

func listText(i int) string {
	if tab == 0 {
		ap := wifiList[i]
		return strconv.Itoa(ap.RSSI) + " " + truncate(ap.SSID, 24)
	}

	dev := bleList[i]
	name := dev.name
	if name == "" {
		name = "?" + dev.addr[len(dev.addr)-5:]
	}
	return strconv.Itoa(int(dev.rssi)) + " " + truncate(name, 24)
}

func scanBLE(d time.Duration) []bleInfo {
	var items []bleInfo
	index := make(map[string]int)
	done := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		select {
		case <-time.After(d):
			adapter.StopScan()
		case <-done:
		}
		close(stopped)
	}()

	err := adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
		addr := result.Address.String()
		name := result.LocalName()

		if i, ok := index[addr]; ok {
			if name != "" {
				items[i].name = name
			}
			items[i].rssi = result.RSSI
			return
		}
		index[addr] = len(items)
		items = append(items, bleInfo{name: name, addr: addr, rssi: result.RSSI})
	})

	close(done)
	<-stopped

	if err != nil {
		println("ble scan failed:", err.Error())
	}

	sort.Slice(items, func(i, j int) bool { return items[i].rssi > items[j].rssi })
	return items
}

func clamp(v, n int) int {
	if n == 0 {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v >= n {
		return n - 1
	}
	return v
}

func adjustScroll(sel, scroll, n int) int {
	if n <= visibleRows {
		return 0
	}
	if sel < scroll {
		scroll = sel
	}
	if sel >= scroll+visibleRows {
		scroll = sel - visibleRows + 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > n-visibleRows {
		scroll = n - visibleRows
	}
	return scroll
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "~"
}
