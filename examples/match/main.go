//go:build esp32

// Example: manual match (Capture -> GenChar 1 -> lift -> Capture ->
// GenChar 2 -> Match) and optional RegModel/Store.
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

	println("match: place the finger")
	common.WaitFinger(dev)
	if c, err := dev.GenChar(1); err != nil || c != finger2.ConfirmOK {
		println("genchar 1:", finger2.Code(c).Error())
		return
	}

	println("lift the finger")
	common.WaitLift(dev)
	println("place the same finger again")
	common.WaitFinger(dev)
	if c, err := dev.GenChar(2); err != nil || c != finger2.ConfirmOK {
		println("genchar 2:", finger2.Code(c).Error())
		return
	}

	m, err := dev.Match()
	if err != nil {
		println("match:", err.Error())
		return
	}
	println("matched", m.Matched, "score", m.Score, "confirm", finger2.Code(m.Confirm).Error())
	if !m.Matched {
		return
	}

	// RegModel merges buffers 1 and 2 into buffer 1; Store persists it.
	if c, err := dev.RegModel(); err != nil || c != finger2.ConfirmOK {
		println("regmodel:", finger2.Code(c).Error())
		return
	}
	page, err := dev.FirstFreePage()
	if err != nil {
		println("free page:", err.Error())
		return
	}
	if c, err := dev.Store(1, page); err != nil || c != finger2.ConfirmOK {
		println("store:", finger2.Code(c).Error())
		return
	}
	println("stored merged template at page", page)
	dev.RefreshTemplates()
	println("templates:", dev.Templates)

	// Remove the demo template so repeated runs do not fill the library.
	_, _ = dev.Delete(page, 1)
	dev.RefreshTemplates()
	println("demo template removed; templates:", dev.Templates)
}
