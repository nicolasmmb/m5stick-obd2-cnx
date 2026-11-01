//go:build esp32

// Example: concurrent callers sharing one Device. The library serializes the
// commands, so all workers complete without corrupting the protocol.
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

	const workers, iterations = 4, 10
	done := make(chan error, workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			for i := 0; i < iterations; i++ {
				p, err := dev.ReadSysParams()
				if err != nil {
					done <- err
					return
				}
				if p.Capacity == 0 {
					done <- finger2.ErrShortReply
					return
				}
				if _, err := dev.GetSN(); err != nil {
					done <- err
					return
				}
			}
			println("worker", w, "done")
			done <- nil
		}(w)
	}
	for w := 0; w < workers; w++ {
		if err := <-done; err != nil {
			println("worker error:", err.Error())
		}
	}
	println("all workers done")
}
