//go:build esp32

// Example: enroll a finger with the module state machine (AutoEnroll).
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

	page, err := dev.FirstFreePage()
	if err != nil {
		println("free page:", err.Error())
		return
	}
	println("enroll: page", page, "- place and lift the finger twice")
	if err := dev.StartAutoEnroll(page, 2, finger2.AutoEnrollDefault); err != nil {
		println("enroll start:", err.Error())
		return
	}
	for {
		ev, err := dev.NextEnrollEvent(5 * time.Second)
		if errors.Is(err, finger2.ErrTimeout) {
			continue
		}
		if err != nil {
			println("enroll:", err.Error())
			return
		}
		println("stage", ev.Stage, "confirm", ev.Confirm, "param", ev.Param)
		if ev.Stage == finger2.EnrollStageStore {
			println("stored page", page)
			break
		}
	}
	dev.RefreshTemplates()
	println("templates:", dev.Templates)
}
