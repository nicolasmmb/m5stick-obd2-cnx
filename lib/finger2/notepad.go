package finger2

import (
	"context"
	"fmt"
	"time"
)

// ReadNotepad reads one 32-byte notepad entry (0x19). id must be 0..3.
func (d *Device) ReadNotepad(id uint8) ([]byte, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return nil, err
	}
	defer d.unlockOp()
	if id > 3 {
		return nil, fmt.Errorf("%w: notepad id %d out of range (0..3)", ErrInvalidParam, id)
	}
	data, confirm, err := d.transceive(CmdReadNotepad, []byte{id}, time.Second)
	if err != nil {
		return nil, err
	}
	if confirm != ConfirmOK {
		return nil, Code(confirm)
	}
	if len(data) < maxNotepadLen {
		return nil, ErrShortReply
	}
	return append([]byte(nil), data[:maxNotepadLen]...), nil
}

// WriteNotepad writes one notepad entry (0x18). Data is truncated or padded
// with zeros to 32 bytes.
func (d *Device) WriteNotepad(id uint8, data []byte) (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	if id > 3 {
		return ConfirmPacketError, fmt.Errorf("%w: notepad id %d out of range (0..3)", ErrInvalidParam, id)
	}
	payload := make([]byte, 1+maxNotepadLen)
	payload[0] = id
	copy(payload[1:], data)
	_, confirm, err := d.transceive(CmdWriteNotepad, payload, time.Second)
	return confirm, err
}
