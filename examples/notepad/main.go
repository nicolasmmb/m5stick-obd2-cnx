//go:build esp32

// Example: write and read one 32-byte notepad entry.
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

	var want [32]byte
	for i := range want {
		want[i] = byte(i*7 + 3)
	}
	if c, err := dev.WriteNotepad(1, want[:]); err != nil || c != finger2.ConfirmOK {
		println("write:", finger2.Code(c).Error())
		return
	}
	got, err := dev.ReadNotepad(1)
	if err != nil {
		println("read:", err.Error())
		return
	}
	ok := len(got) == len(want)
	for i := 0; ok && i < len(want); i++ {
		ok = got[i] == want[i]
	}
	println("notepad round trip ok:", ok)
}
