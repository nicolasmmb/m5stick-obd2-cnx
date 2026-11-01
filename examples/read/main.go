//go:build esp32

// Example: read a finger with Capture -> GenChar -> Search.
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
	println("read: place a finger")

	for {
		ok, confirm, err := dev.Capture()
		if err != nil {
			println("capture:", err.Error())
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if !ok {
			if confirm != finger2.ConfirmNoFinger {
				println("capture:", finger2.Code(confirm).Error())
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if c, err := dev.GenChar(1); err != nil || c != finger2.ConfirmOK {
			println("genchar:", finger2.Code(c).Error())
			time.Sleep(200 * time.Millisecond)
			continue
		}
		res, err := dev.Search(1, 0, dev.Capacity)
		if err != nil {
			println("search:", err.Error())
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if res.Matched {
			println("match page", res.Page, "score", res.Score)
		} else {
			println("not found:", finger2.Code(res.Confirm).Error())
		}
		time.Sleep(500 * time.Millisecond)
	}
}
