//go:build esp32

// Example: upload the current image (80x208, 4 bits per pixel) and print a
// coarse ASCII preview over serial.
package main

import (
	"time"

	"m5stick-obd2-cnx/examples/common"
	"m5stick-obd2-cnx/lib/finger2"
)

func main() {
	time.Sleep(2 * time.Second)
	dev, err := common.Open(10 * time.Second)
	if err != nil {
		println("open:", err.Error())
		return
	}

	buf := make([]byte, finger2.ImageWidth*finger2.ImageHeight/2)
	println("image: place a finger")
	common.WaitFinger(dev)
	time.Sleep(300 * time.Millisecond)
	common.WaitFinger(dev)

	n, err := dev.UpImageTo(buf)
	if err != nil {
		println("image:", err.Error())
		return
	}
	println("image bytes", n)
	preview(buf[:n])
}

func preview(img []byte) {
	const cols, rows = 40, 20
	const ramp = " .:-=+*#%@"
	for ry := 0; ry < rows; ry++ {
		line := make([]byte, 0, cols)
		for rx := 0; rx < cols; rx++ {
			x := rx * finger2.ImageWidth / cols
			y := ry * finger2.ImageHeight / rows
			p := y*finger2.ImageWidth + x
			b := img[p>>1]
			var v uint8
			if p&1 == 0 {
				v = b & 0x0F
			} else {
				v = b >> 4
			}
			line = append(line, ramp[int(v)*len(ramp)/16])
		}
		println(string(line))
	}
}
