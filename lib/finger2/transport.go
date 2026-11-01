package finger2

import "time"

// Transport is the byte stream the driver talks to. machine.UART satisfies it;
// tests provide an in-memory implementation.
type Transport interface {
	Write(p []byte) (int, error)
	ReadByte() (byte, error)
	Buffered() int
}

func newDevice(t Transport) *Device {
	return &Device{
		transport: t,
		address:   DefaultAddress,
		timeout:   1500 * time.Millisecond,
	}
}
