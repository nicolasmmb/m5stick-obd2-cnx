//go:build esp32

// Example: continuous identification (AutoIdentify).
package main

import (
	"errors"
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

	println("identify: place a finger")
	if err := dev.StartAutoIdentify(0, 0xFFFF, 0); err != nil {
		println("identify start:", err.Error())
		return
	}
	for {
		p, err := dev.NextIdentifyEvent(5 * time.Second)
		if errors.Is(err, finger2.ErrTimeout) {
			continue
		}
		if err != nil {
			println("identify:", err.Error())
			return
		}
		println("stage", p.Stage, "confirm", p.Confirm, "id", p.ID, "score", p.Score)
		if p.Stage == finger2.IdentifyStageVerify {
			break
		}
	}
	println("done")
}
