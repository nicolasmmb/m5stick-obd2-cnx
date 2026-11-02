package main

import (
	"machine"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	// Keep the board powered when running from battery.
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	time.Sleep(time.Second)

	println("ble-scan: enabling BLE stack...")
	if err := adapter.Enable(); err != nil {
		println("could not enable BLE stack:", err.Error())
		return
	}

	// Active scanning sends SCAN_REQ so devices return their names.
	adapter.SetScanType(bluetooth.ScanTypeActive)

	// addr -> last printed name, so we can print again when a scan response
	// reveals the name.
	printed := make(map[string]string)

	println("ble-scan: scanning (BLE only, Classic devices will not appear)...")
	err := adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
		addr := result.Address.String()
		name := result.LocalName()

		if prev, ok := printed[addr]; ok {
			// Only print an unnamed advertisement once per address, and never
			// fall back to "(waiting)" after the real name was already seen.
			if name == "" || prev == name {
				return
			}
		}

		if name == "" {
			printed[addr] = "(waiting name)"
			println("DEV:", addr, "RSSI", result.RSSI, "NAME: (waiting scan response)")
			return
		}

		printed[addr] = name
		println("DEV:", addr, "RSSI", result.RSSI, "NAME:", name)

		// Print advertised service UUIDs. ELM327 BLE adapters usually
		// advertise FFF0, FFE0 or 18F0 even when they have no name.
		var svc strings.Builder
		for _, u := range result.ServiceUUIDs() {
			svc.WriteString(" ")
			svc.WriteString(strings.ToLower(u.String()))
		}
		if svc.Len() > 0 {
			println("    SVC:", svc.String())
		}

		lower := strings.ToLower(name)
		if strings.Contains(lower, "obd") || strings.Contains(lower, "elm") ||
			strings.Contains(lower, "vlink") || strings.Contains(lower, "v-link") ||
			strings.Contains(lower, "icar") {
			println("*** POSSIVEL ADAPTADOR OBD2 BLE:", name, "***")
		}
	})
	if err != nil {
		println("scan failed:", err.Error())
	}
}
