//go:build tinygo

package main

import "time"

// simReplies receives "S,<response>" lines from the host. The simulator is
// not a hot path, so a channel is fine here.
var simReplies = make(chan string, 1)

func simReply(text string) {
	select {
	case simReplies <- text:
	default:
		<-simReplies
		simReplies <- text
	}
}

func drainSimReplies() {
	for {
		select {
		case <-simReplies:
		default:
			return
		}
	}
}

type simTransport struct{}

func (simTransport) Command(cmd string, timeout time.Duration) (string, Reply) {
	drainSimReplies()
	println("Q," + cmd)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-simReplies:
		return r, ReplyOK
	case <-timer.C:
		return "", ReplyTimeout
	}
}
