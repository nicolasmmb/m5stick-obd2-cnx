package main

import "time"

// Reply classifies the result of one ELM command without allocating.
type Reply uint8

const (
	ReplyOK Reply = iota
	ReplyTimeout
	ReplyWrite
)

func (r Reply) Error() string {
	switch r {
	case ReplyTimeout:
		return "timeout"
	case ReplyWrite:
		return "write failed"
	default:
		return "ok"
	}
}

// Transport is the only thing the session knows about the outside world.
// The BLE link and the serial simulator implement it.
type Transport interface {
	Command(cmd string, timeout time.Duration) (string, Reply)
}
