//go:build esp32

// Example: context-aware startup and auto-enroll session, cancelled with
// context to release the device without touching the module.
package main

import (
	"context"
	"time"

	"m5stick-obd2-cnx/examples/common"
	"m5stick-obd2-cnx/lib/finger2"
)

func main() {
	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	dev, err := common.OpenContext(ctx, 5*time.Second)
	cancel()
	if err != nil {
		println("open:", err.Error())
		return
	}

	page, err := dev.FirstFreePage()
	if err != nil {
		println("free page:", err.Error())
		return
	}
	sctx, scancel := context.WithCancel(context.Background())
	if err := dev.StartAutoEnrollContext(sctx, page, 2, finger2.AutoEnrollDefault); err != nil {
		println("session:", err.Error())
		return
	}
	println("session started; cancelling in 2s (no finger needed)")
	time.Sleep(2 * time.Second)
	scancel()

	deadline := time.Now().Add(3 * time.Second)
	for {
		p, err := dev.ReadSysParams()
		if err == nil {
			println("device free again, capacity", p.Capacity)
			return
		}
		if time.Now().After(deadline) {
			println("device still busy:", err.Error())
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}
