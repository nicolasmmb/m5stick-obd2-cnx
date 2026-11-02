package main

import (
	"machine"
	"time"

	"tinygo.org/x/espradio"
)

func main() {
	// Keep the board powered when running from battery.
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	time.Sleep(time.Second)

	println("wifi-scan: initializing radio...")
	if err := espradio.Enable(espradio.Config{}); err != nil {
		println("could not enable radio:", err.Error())
		return
	}

	println("wifi-scan: starting radio...")
	if err := espradio.Start(); err != nil {
		println("could not start radio:", err.Error())
		return
	}

	for {
		println("wifi-scan: scanning...")
		aps, err := espradio.Scan()
		if err != nil {
			println("could not scan wifi:", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		println("wifi-scan: found", len(aps), "access points")
		for _, ap := range aps {
			println("AP:", ap.SSID, "RSSI", ap.RSSI)
		}

		var stats espradio.Stats
		espradio.ReadStats(&stats)
		stats.Print()
		println()

		time.Sleep(10 * time.Second)
	}
}
