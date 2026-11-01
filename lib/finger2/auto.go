package finger2

import (
	"context"
	"fmt"
	"time"

	"m5stick-obd2-cnx/lib/finger2/internal/wire"
)

// AutoEnroll stages reported in Param1.
const (
	EnrollStageLegalCheck = 0x00
	EnrollStageGetImage   = 0x01
	EnrollStageGenChar    = 0x02
	EnrollStageLiftFinger = 0x03
	EnrollStageMerge      = 0x04
	EnrollStageVerify     = 0x05
	EnrollStageStore      = 0x06
)

// AutoEnroll flags (0x31).
const (
	AutoEnrollDefault        = 0x0000
	AutoEnrollNoStatus       = 1 << 2
	AutoEnrollOverwrite      = 1 << 3
	AutoEnrollForbidDup      = 1 << 4
	AutoEnrollNoLiftRequired = 1 << 5
)

// IdentifyStage values reported by AutoIdentify in Param.
const (
	IdentifyStageLegalCheck = 0x00
	IdentifyStageGetImage   = 0x01
	IdentifyStageVerify     = 0x05
)

// AutoIdentify flags (0x32).
const (
	AutoIdentifyDefault  = 0x0000
	AutoIdentifyNoStatus = 1 << 2
)

// EnrollProgress is one status packet emitted during AutoEnroll.
type EnrollProgress struct {
	// Confirm is the module confirmation code for this step.
	Confirm uint8
	// Stage is the EnrollStage* value (Param1).
	Stage uint8
	// Param is the raw Param2 value (0x00, 0xF0, 0xF1 or 0xF2).
	Param uint8
}

// IdentifyProgress is one status packet emitted during AutoIdentify.
type IdentifyProgress struct {
	Confirm uint8
	// Stage is the IdentifyStage* value.
	Stage uint8
	// ID is the matched template page (0xFFFF while searching).
	ID uint16
	// Score is the match confidence.
	Score uint16
}

// StartAutoEnroll starts the module enrollment state machine (0x31).
// count is the number of captures (1..6); call NextEnrollEvent afterwards.
func (d *Device) StartAutoEnroll(page uint16, count uint8, flags uint16) error {
	return d.StartAutoEnrollContext(context.Background(), page, count, flags)
}

// StartAutoEnrollContext starts the enrollment state machine. The session keeps
// the device locked until a terminal stage, Cancel or ctx expiry (60s max).
func (d *Device) StartAutoEnrollContext(ctx context.Context, page uint16, count uint8, flags uint16) error {
	if count == 0 || count > 6 {
		return fmt.Errorf("%w: enroll count %d out of range (1..6)", ErrInvalidParam, count)
	}
	if d.Capacity > 0 && page >= d.Capacity {
		return fmt.Errorf("%w: page %d >= capacity %d", ErrInvalidParam, page, d.Capacity)
	}
	if err := d.startSession(ctx); err != nil {
		return err
	}
	payload := []byte{byte(page >> 8), byte(page), count, byte(flags >> 8), byte(flags)}
	if err := d.startCommand(CmdAutoEnroll, payload); err != nil {
		d.endSession()
		return err
	}
	return nil
}

// NextEnrollEvent waits for the next AutoEnroll status packet.
func (d *Device) NextEnrollEvent(timeout time.Duration) (EnrollProgress, error) {
	if !d.inSession() {
		return EnrollProgress{}, ErrNoSession
	}
	if err := acquire(context.Background(), d.cmdMu); err != nil {
		return EnrollProgress{}, err
	}
	defer func() { <-d.cmdMu }()

	deadline := time.Now().Add(timeout)
	for {
		if err := d.sessionCtx.Err(); err != nil {
			d.endSession()
			return EnrollProgress{}, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return EnrollProgress{}, ErrTimeout
		}
		pkt, err := d.readPacket(remaining)
		if err != nil {
			if ctxErr := d.sessionCtx.Err(); ctxErr != nil {
				d.endSession()
				return EnrollProgress{}, ctxErr
			}
			return EnrollProgress{}, err
		}
		if d.skipWakeup(pkt) {
			continue
		}
		if pkt[6] != wire.PIDAck {
			continue
		}
		confirm, data := ackData(pkt)
		progress := EnrollProgress{Confirm: confirm}
		if len(data) >= 2 {
			progress.Stage = data[0]
			progress.Param = data[1]
		}
		if progress.Stage == EnrollStageStore {
			d.endSession()
		}
		return progress, nil
	}
}

// AutoEnroll runs the enrollment state machine until the store stage.
// The callback receives every intermediate progress; return false to abort.
func (d *Device) AutoEnroll(page uint16, count uint8, flags uint16, cb func(p EnrollProgress) bool) (uint8, error) {
	if err := d.StartAutoEnroll(page, count, flags); err != nil {
		return 0, err
	}
	last := uint8(0xFF)
	for {
		progress, err := d.NextEnrollEvent(30 * time.Second)
		if err != nil {
			d.endSession()
			return last, err
		}
		last = progress.Confirm
		if cb != nil && !cb(progress) {
			_ = d.Cancel()
			return progress.Confirm, nil
		}
		if progress.Stage == EnrollStageStore {
			return progress.Confirm, nil
		}
	}
}

// StartAutoIdentify starts the module identification loop (0x32).
// id 0xFFFF searches the whole library. Call NextIdentifyEvent afterwards.
func (d *Device) StartAutoIdentify(security uint8, id uint16, flags uint16) error {
	return d.StartAutoIdentifyContext(context.Background(), security, id, flags)
}

// StartAutoIdentifyContext starts the identification loop. The session keeps
// the device locked until the verify stage, Cancel or ctx expiry (60s max).
func (d *Device) StartAutoIdentifyContext(ctx context.Context, security uint8, id uint16, flags uint16) error {
	if security > 1 {
		return fmt.Errorf("%w: security level %d out of range (0..1)", ErrInvalidParam, security)
	}
	if err := d.startSession(ctx); err != nil {
		return err
	}
	payload := []byte{security, byte(id >> 8), byte(id), byte(flags >> 8), byte(flags)}
	if err := d.startCommand(CmdAutoIdentify, payload); err != nil {
		d.endSession()
		return err
	}
	return nil
}

// NextIdentifyEvent waits for the next AutoIdentify status packet.
func (d *Device) NextIdentifyEvent(timeout time.Duration) (IdentifyProgress, error) {
	if !d.inSession() {
		return IdentifyProgress{}, ErrNoSession
	}
	if err := acquire(context.Background(), d.cmdMu); err != nil {
		return IdentifyProgress{}, err
	}
	defer func() { <-d.cmdMu }()

	deadline := time.Now().Add(timeout)
	for {
		if err := d.sessionCtx.Err(); err != nil {
			d.endSession()
			return IdentifyProgress{}, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return IdentifyProgress{}, ErrTimeout
		}
		pkt, err := d.readPacket(remaining)
		if err != nil {
			if ctxErr := d.sessionCtx.Err(); ctxErr != nil {
				d.endSession()
				return IdentifyProgress{}, ctxErr
			}
			return IdentifyProgress{}, err
		}
		if d.skipWakeup(pkt) {
			continue
		}
		if pkt[6] != wire.PIDAck {
			continue
		}
		confirm, data := ackData(pkt)
		progress := IdentifyProgress{Confirm: confirm}
		if len(data) >= 1 {
			progress.Stage = data[0]
		}
		if len(data) >= 3 {
			progress.ID = uint16(data[1])<<8 | uint16(data[2])
		}
		if len(data) >= 5 {
			progress.Score = uint16(data[3])<<8 | uint16(data[4])
		}
		if progress.Stage == IdentifyStageVerify {
			d.endSession()
		}
		return progress, nil
	}
}

// AutoIdentify runs identification until the module reaches the verify stage.
// The callback receives every intermediate progress; return false to abort.
func (d *Device) AutoIdentify(security uint8, id uint16, flags uint16, cb func(p IdentifyProgress) bool) (IdentifyProgress, error) {
	if err := d.StartAutoIdentify(security, id, flags); err != nil {
		return IdentifyProgress{}, err
	}
	var last IdentifyProgress
	for {
		progress, err := d.NextIdentifyEvent(30 * time.Second)
		if err != nil {
			d.endSession()
			return last, err
		}
		last = progress
		if cb != nil && !cb(progress) {
			_ = d.Cancel()
			return progress, nil
		}
		if progress.Stage == IdentifyStageVerify {
			return progress, nil
		}
	}
}
