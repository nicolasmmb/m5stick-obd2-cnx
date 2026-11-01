//go:build esp32

package finger2

import "machine"

// New returns a Device bound to uart. Configure the UART for 115200 8N1 before
// calling Init.
func New(uart *machine.UART) *Device {
	return newDevice(uart)
}
