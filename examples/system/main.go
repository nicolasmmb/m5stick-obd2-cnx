//go:build esp32

// Example: system information and settings (firmware, SN, parameters, random,
// sensor self-test, work mode, sleep time, match level and LED).
package main

import (
	"fmt"
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

	println("firmware", dev.Firmware)
	if sn, err := dev.GetSN(); err == nil {
		println("sn", len(sn), "bytes:", fmt.Sprintf("% X", sn))
	} else {
		println("sn:", err.Error())
	}
	if p, err := dev.ReadSysParams(); err == nil {
		println("params: capacity", p.Capacity, "level", p.MatchLevel,
			"packet", p.PacketSize, "baud", p.BaudRate, "status", p.Status)
	}
	if r, err := dev.GetRandom(); err == nil {
		println("random", r)
	}
	if err := dev.CheckSensor(); err != nil {
		println("sensor:", err.Error())
	} else {
		println("sensor ok")
	}
	if wm, err := dev.GetWorkMode(); err == nil {
		println("work mode", wm)
	}
	if st, err := dev.GetSleepTime(); err == nil {
		println("sleep time", st)
		if err := dev.SetSleepTime(30); err == nil {
			println("sleep time set to 30")
			_ = dev.SetSleepTime(st)
		}
	}
	if c, err := dev.SetMatchLevel(5); err == nil {
		println("match level confirm", c)
	}
	if err := dev.ControlLED(finger2.LEDBlink, finger2.LEDRed, finger2.LEDRed, 3); err != nil {
		println("led:", err.Error())
	}
	time.Sleep(2 * time.Second)
	_ = dev.ControlLED(finger2.LEDBreath, finger2.LEDBlue, finger2.LEDBlue, 0)
}
