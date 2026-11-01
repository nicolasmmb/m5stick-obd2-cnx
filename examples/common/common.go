//go:build esp32

// Package common contains setup helpers shared by the finger2 examples.
package common

import (
	"context"
	"time"

	"machine"

	"m5stick-obd2-cnx/lib/finger2"
)

// Open configures UART1 on the Grove port (G32/G33) and waits for the module.
func Open(timeout time.Duration) (*finger2.Device, error) {
	return OpenContext(context.Background(), timeout)
}

// OpenContext is Open with a cancellable context for the initial handshake.
func OpenContext(ctx context.Context, timeout time.Duration) (*finger2.Device, error) {
	uart := machine.UART1
	if err := uart.Configure(machine.UARTConfig{
		BaudRate: 115200,
		TX:       machine.GPIO32,
		RX:       machine.GPIO33,
	}); err != nil {
		return nil, err
	}
	dev := finger2.New(uart)
	dev.SetTimeout(900 * time.Millisecond)
	if err := dev.InitContext(ctx, timeout); err != nil {
		return dev, err
	}
	_ = dev.SetWorkMode(1) // keep the module awake while the example runs
	return dev, nil
}

// WaitFinger blocks until an image is captured.
func WaitFinger(dev *finger2.Device) {
	for {
		ok, _, err := dev.Capture()
		if err == nil && ok {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// WaitLift blocks until the finger is removed.
func WaitLift(dev *finger2.Device) {
	for {
		ok, _, err := dev.Capture()
		if err == nil && !ok {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
