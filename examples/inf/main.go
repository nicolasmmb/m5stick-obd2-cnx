//go:build esp32

// Example: read the 512-byte INF page.
package main

import (
	"fmt"
	"time"

	"m5stick-obd2-cnx/examples/common"
)

func main() {
	time.Sleep(2 * time.Second)
	dev, err := common.Open(10 * time.Second)
	if err != nil {
		println("open:", err.Error())
		return
	}
	data, err := dev.ReadINF()
	if err != nil {
		println("inf:", err.Error())
		return
	}
	println("inf bytes", len(data))
	if len(data) >= 16 {
		println("head:", fmt.Sprintf("% X", data[:16]))
	}
}
