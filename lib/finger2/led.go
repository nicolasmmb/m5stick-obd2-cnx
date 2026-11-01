package finger2

import (
	"context"
	"fmt"
	"time"
)

// ControlLED drives the RGB ring (0x3C). mode is one of LEDBreath,
// LEDBlink, LEDOn, LEDOff, LEDFadeIn or LEDFadeOut; colors are LEDBlack..
// LEDWhite; cycle is the loop count (0 for continuous where supported).
func (d *Device) ControlLED(mode, color, endColor, cycle uint8) error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	return d.controlLEDLocked(mode, color, endColor, cycle)
}

func (d *Device) controlLEDLocked(mode, color, endColor, cycle uint8) error {
	if mode < LEDBreath || mode > LEDFadeOut {
		return fmt.Errorf("%w: LED mode %d out of range (1..6)", ErrInvalidParam, mode)
	}
	if color > LEDWhite || endColor > LEDWhite {
		return fmt.Errorf("%w: LED color out of range (0..7)", ErrInvalidParam)
	}
	_, confirm, err := d.transceive(CmdControlLED, []byte{mode, color, endColor, cycle}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// SetLED is an alias for ControlLED kept for readability at call sites.
func (d *Device) SetLED(mode, color, endColor, cycle uint8) error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	return d.controlLEDLocked(mode, color, endColor, cycle)
}
