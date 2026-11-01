// Benchmark for the finger2 image path.
//
// It prints, over UART:
//   - upload time and allocations for UpImageTo (reused buffer) vs UpImage;
//   - render time for the 16-entry palette vs per-pixel color conversion.
//
// The UART baud (115200) dominates upload time, so the expected difference is
// in allocation count and render time, not in upload throughput.
package main

import (
	"runtime"
	"time"

	"machine"

	"github.com/nicolasmmb/M5Unit-Fingerprint2-Go"

	"tinygo.org/x/drivers/pixel"
)

const (
	imageBytes = finger2.ImageWidth * finger2.ImageHeight / 2
	iterations = 4
)

var gray4 [16]pixel.RGB565BE

func main() {
	time.Sleep(2 * time.Second)

	uart := machine.UART1
	if err := uart.Configure(machine.UARTConfig{
		BaudRate: 115200,
		TX:       machine.GPIO32,
		RX:       machine.GPIO33,
	}); err != nil {
		fail("uart: " + err.Error())
	}
	dev := finger2.New(uart)
	dev.SetTimeout(900 * time.Millisecond)
	for {
		if err := dev.Init(); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	println("BENCH|init|fw", dev.Firmware, "capacity", dev.Capacity, "packet", dev.PacketSize, "baud", dev.BaudRate)

	for i := range gray4 {
		g := uint8(i * 17)
		gray4[i] = pixel.NewColor[pixel.RGB565BE](g, g, g)
	}
	fb := pixel.NewImage[pixel.RGB565BE](240, 135)

	benchCommands(dev)
	benchTemplates(dev)

	println("BENCH|wait|place and hold a finger")
	for {
		if ok, _, err := dev.Capture(); err == nil && ok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	println("BENCH|finger|ok")

	benchRead(dev)

	reused := make([]byte, imageBytes)

	// Warm up: one upload of each kind, not measured.
	if _, err := dev.UpImageTo(reused); err != nil {
		fail("warmup UpImageTo: " + err.Error())
	}
	if _, err := dev.UpImage(); err != nil {
		fail("warmup UpImage: " + err.Error())
	}

	for i := 0; i < iterations; i++ {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)

		start := time.Now()
		n, err := dev.UpImageTo(reused)
		elapsed := time.Since(start)
		runtime.ReadMemStats(&after)
		if err != nil {
			println("BENCH|UpImageTo|error", err.Error())
			continue
		}
		println("BENCH|UpImageTo|bytes", n,
			"us", elapsed.Microseconds(),
			"mallocs", after.Mallocs-before.Mallocs,
			"heap", after.HeapAlloc)
	}

	// Try 256-byte packets (register 6) and restore afterwards.
	if err := dev.WriteReg(6, 3); err == nil {
		time.Sleep(500 * time.Millisecond)
		for i := 0; i < 2; i++ {
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			n, err := dev.UpImageTo(reused)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			if err == nil {
				println("BENCH|UpImageTo256|bytes", n, "us", elapsed.Microseconds(),
					"mallocs", after.Mallocs-before.Mallocs)
			}
		}
		_ = dev.WriteReg(6, 2)
		time.Sleep(500 * time.Millisecond)
	}

	for i := 0; i < iterations; i++ {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)

		start := time.Now()
		img, err := dev.UpImage()
		elapsed := time.Since(start)
		runtime.ReadMemStats(&after)
		if err != nil || len(img) == 0 {
			println("BENCH|UpImage|error")
			continue
		}
		println("BENCH|UpImage|bytes", len(img),
			"us", elapsed.Microseconds(),
			"mallocs", after.Mallocs-before.Mallocs,
			"heap", after.HeapAlloc)
		benchRender(fb, reused)
		benchRender(fb, img)
	}
}

func benchCommands(dev *finger2.Device) {
	const n = 200
	_, _ = dev.ReadSysParams() // warm up
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	for i := 0; i < n; i++ {
		_, _ = dev.ReadSysParams()
	}
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	println("BENCH|sysparams|count", n, "us_each", elapsed.Microseconds()/n,
		"mallocs", after.Mallocs-before.Mallocs)
}

func benchRead(dev *finger2.Device) {
	const n = 20
	// Warm up the command path.
	dev.Capture()
	dev.GenChar(1)
	dev.Search(1, 0, dev.Capacity)

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	matched := 0
	for i := 0; i < n; i++ {
		if det, _, err := dev.Capture(); err == nil && det {
		}
		if c, err := dev.GenChar(1); err == nil && c == finger2.ConfirmOK {
		}
		if res, err := dev.Search(1, 0, dev.Capacity); err == nil && res.Matched {
			matched++
		}
	}
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	println("BENCH|read|count", n, "us_each", elapsed.Microseconds()/n,
		"mallocs", after.Mallocs-before.Mallocs, "matched", matched)
}

func benchTemplates(dev *finger2.Device) {
	dev.RefreshTemplates()
	if dev.Templates == 0 {
		println("BENCH|template|none")
		return
	}
	if c, err := dev.LoadChar(1, 0); err != nil || c != finger2.ConfirmOK {
		println("BENCH|template|loadchar failed", c)
		return
	}

	var tpl []byte
	for i := 0; i < 2; i++ {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		data, err := dev.UploadTemplateAuto(finger2.TemplateBytes)
		elapsed := time.Since(start)
		runtime.ReadMemStats(&after)
		if err != nil {
			println("BENCH|tplupload|error", err.Error())
			return
		}
		tpl = data
		println("BENCH|tplupload|bytes", len(data), "us", elapsed.Microseconds(),
			"mallocs", after.Mallocs-before.Mallocs)
	}
	if len(tpl) == 0 {
		return
	}

	for i := 0; i < 2; i++ {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		err := dev.DownloadTemplateAuto(tpl)
		elapsed := time.Since(start)
		runtime.ReadMemStats(&after)
		if err != nil {
			println("BENCH|tpldownload|error", err.Error())
			return
		}
		println("BENCH|tpldownload|bytes", len(tpl), "us", elapsed.Microseconds(),
			"mallocs", after.Mallocs-before.Mallocs)
	}
}

func benchRender(fb pixel.Image[pixel.RGB565BE], img []byte) {
	if len(img) < imageBytes {
		return
	}
	const (
		iw = finger2.ImageWidth
		ih = finger2.ImageHeight
		dx = 16
		dy = 38
	)

	// Palette lookup.
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	for y := 0; y < ih; y++ {
		for x := 0; x < iw; x++ {
			v := nibbleAt(img, y*iw+x)
			fb.Set(dx+ih-1-y, dy+x, gray4[v])
		}
	}
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	println("BENCH|render|palette|us", elapsed.Microseconds(),
		"mallocs", after.Mallocs-before.Mallocs)

	// Per-pixel conversion.
	runtime.GC()
	runtime.ReadMemStats(&before)
	start = time.Now()
	for y := 0; y < ih; y++ {
		for x := 0; x < iw; x++ {
			v := nibbleAt(img, y*iw+x)
			g := v * 17
			fb.Set(dx+ih-1-y, dy+x, pixel.NewColor[pixel.RGB565BE](g, g, g))
		}
	}
	elapsed = time.Since(start)
	runtime.ReadMemStats(&after)
	println("BENCH|render|convert|us", elapsed.Microseconds(),
		"mallocs", after.Mallocs-before.Mallocs)
}

func nibbleAt(img []byte, p int) uint8 {
	b := img[p>>1]
	if p&1 == 0 {
		return b & 0x0F
	}
	return b >> 4
}

func fail(msg string) {
	for {
		println("BENCH|fatal|" + msg)
		time.Sleep(time.Second)
	}
}
