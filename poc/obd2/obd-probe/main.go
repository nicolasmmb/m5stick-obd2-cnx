package main

import (
	"machine"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

type candidate struct {
	addr    bluetooth.Address
	name    string
	rssi    int16
	obdLike bool
}

var (
	uuidFFF0 = bluetooth.New16BitUUID(0xFFF0)
	uuidFFE0 = bluetooth.New16BitUUID(0xFFE0)
	uuid18F0 = bluetooth.New16BitUUID(0x18F0)
)

func main() {
	// Keep the board powered when running from battery.
	hold := machine.GPIO4
	hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
	hold.High()

	time.Sleep(time.Second)
	println("obd-probe: starting")
	if err := adapter.Enable(); err != nil {
		println("enable failed:", err.Error())
		return
	}
	adapter.SetScanType(bluetooth.ScanTypeActive)

	println("obd-probe: scanning 10s...")
	var found []candidate
	seen := make(map[string]bool)

	done := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Second)
		adapter.StopScan()
		close(done)
	}()

	err := adapter.Scan(func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
		addr := r.Address.String()
		name := r.LocalName()
		if seen[addr] && name == "" {
			return
		}
		seen[addr] = true

		isOBD := r.HasServiceUUID(uuidFFF0) || r.HasServiceUUID(uuidFFE0) || r.HasServiceUUID(uuid18F0)
		c := candidate{addr: r.Address, name: name, rssi: r.RSSI, obdLike: isOBD}
		updated := false
		for i := range found {
			if found[i].addr.String() == addr {
				if name != "" {
					found[i].name = name
				}
				found[i].obdLike = found[i].obdLike || isOBD
				updated = true
				found[i].rssi = r.RSSI
				break
			}
		}
		if !updated {
			found = append(found, c)
		}

		var svc strings.Builder
		for _, u := range r.ServiceUUIDs() {
			svc.WriteString(" ")
			svc.WriteString(strings.ToLower(u.String()))
		}
		println("DEV", addr, r.RSSI, name, svc.String())
	})
	if err != nil {
		println("scan failed:", err.Error())
	}
	<-done
	println("obd-probe: scan done,", len(found), "devices")

	var pick *candidate
	for i := range found {
		if found[i].obdLike {
			pick = &found[i]
			break
		}
	}
	if pick == nil {
		for i := range found {
			n := strings.ToLower(found[i].name)
			if strings.Contains(n, "obd") || strings.Contains(n, "elm") ||
				strings.Contains(n, "vlink") || strings.Contains(n, "icar") {
				pick = &found[i]
				break
			}
		}
	}
	if pick == nil {
		// Fallback: strongest device, useful to prove GATT discovery works.
		for i := range found {
			if pick == nil || found[i].rssi > pick.rssi {
				pick = &found[i]
			}
		}
	}
	if pick == nil {
		println("obd-probe: no BLE devices found (adapter may be Classic-only)")
		return
	}

	println("obd-probe: connecting to", pick.addr.String(), pick.name)
	dev, err := adapter.Connect(pick.addr, bluetooth.ConnectionParams{})
	if err != nil {
		println("connect failed:", err.Error())
		return
	}
	println("obd-probe: connected")

	services, err := dev.DiscoverServices(nil)
	if err != nil {
		println("discover services failed:", err.Error())
		return
	}
	println("obd-probe: services:", len(services))
	for _, s := range services {
		println("SERVICE", strings.ToLower(s.UUID().String()))
		chars, err := s.DiscoverCharacteristics(nil)
		if err != nil {
			println("  characteristics failed:", err.Error())
			continue
		}
		for _, c := range chars {
			mtu, _ := c.GetMTU()
			println("  CHAR", strings.ToLower(c.UUID().String()), "mtu", mtu)
		}
	}
	println("obd-probe: done")
}
