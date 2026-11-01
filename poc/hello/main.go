package main

import (
	"machine"
	"time"
)

func main() {
	// M5StickC Plus2: HOLD is on GPIO4. Keeping it high prevents the board
	// from powering off when it is running on battery.
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	// Red LED shares GPIO19 with the IR emitter on the Plus2.
	led := machine.GPIO19
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})

	println("M5StickC Plus2 + TinyGo", "hello")

	var tick int
	for {
		tick++
		println("tick", tick)
		if tick%2 == 0 {
			led.High()
		} else {
			led.Low()
		}
		time.Sleep(time.Second)
	}
}
