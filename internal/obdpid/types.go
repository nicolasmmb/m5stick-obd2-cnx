package obdpid

import (
	"errors"
	"time"
)

var (
	ErrUnknownPID = errors.New("obdpid: unknown pid")
	ErrInvalid    = errors.New("obdpid: invalid raw data")
)

type Quality uint8

const (
	QualityOK Quality = iota
	QualityNoData
	QualityTimeout
	QualityStale
	QualityInvalid
	QualityUnsupported
)

type Source uint8

const (
	SourceLive Source = iota
	SourceReplay
)

// Reading is the canonical value of one PID, independent of transport.
type Reading struct {
	PID     string
	Raw     []byte
	Value   float64
	Unit    string
	Quality Quality
	Source  Source
	At      time.Time
}

func (r Reading) AgeMs(now time.Time) int64 {
	if r.At.IsZero() {
		return 0
	}
	return now.Sub(r.At).Milliseconds()
}

// Decoder converts raw PID bytes into a physical value.
type Decoder interface {
	PID() string
	Name() string
	Unit() string
	Decode(raw []byte) (float64, error)
}
