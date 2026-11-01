package finger2

import (
	"context"
	"sync/atomic"
	"time"
)

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
		cmdMu:     make(chan struct{}, 1),
		sessionMu: make(chan struct{}, 1),
	}
}

// acquire takes sem or gives up when ctx is done.
func acquire(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// lockOp serializes a one-shot operation against sessions and other commands.
func (d *Device) lockOp(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := acquire(ctx, d.sessionMu); err != nil {
		return err
	}
	if err := acquire(context.Background(), d.cmdMu); err != nil {
		<-d.sessionMu
		return err
	}
	return nil
}

func (d *Device) unlockOp() {
	<-d.cmdMu
	<-d.sessionMu
}

// startSession claims the device for an auto-enroll/identify session.
func (d *Device) startSession(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sessionCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	if err := sessionCtx.Err(); err != nil {
		cancel()
		return err
	}
	if err := acquire(sessionCtx, d.sessionMu); err != nil {
		cancel()
		return err
	}
	d.sessionCtx = sessionCtx
	d.sessionCancel = cancel
	atomic.StoreUint32(&d.sessionOn, 1)
	go func() {
		<-sessionCtx.Done()
		d.endSession()
	}()
	return nil
}

// endSession releases the session lock exactly once.
func (d *Device) endSession() {
	if atomic.SwapUint32(&d.sessionOn, 0) == 1 {
		if d.sessionCancel != nil {
			d.sessionCancel()
		}
		<-d.sessionMu
	}
}

func (d *Device) inSession() bool {
	return atomic.LoadUint32(&d.sessionOn) == 1
}

// startCommand flushes pending input and sends a command under the command lock.
func (d *Device) startCommand(cmd byte, payload []byte) error {
	if err := acquire(context.Background(), d.cmdMu); err != nil {
		return err
	}
	defer func() { <-d.cmdMu }()
	d.flush()
	return d.send(cmd, payload)
}

// sendLocked sends a command while holding only the command lock.
func (d *Device) sendLocked(cmd byte, payload []byte) error {
	if err := acquire(context.Background(), d.cmdMu); err != nil {
		return err
	}
	defer func() { <-d.cmdMu }()
	return d.send(cmd, payload)
}
