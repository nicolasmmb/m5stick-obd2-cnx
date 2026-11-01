package finger2

import (
	"context"
	"fmt"
	"time"
)

// Capture acquires an image into the image buffer (0x01).
//
// A missing finger is not an error: it returns detected=false with
// confirm=ConfirmNoFinger. Transport failures return a non-nil error.
func (d *Device) Capture() (detected bool, confirm uint8, err error) {
	if err := d.lockOp(context.Background()); err != nil {
		return false, 0, err
	}
	defer d.unlockOp()
	_, confirm, err = d.transceive(CmdGetImage, nil, d.timeout)
	if err != nil {
		return false, confirm, err
	}
	return confirm == ConfirmOK, confirm, nil
}

// GetEnrollImage acquires an image using the enrollment command (0x29).
func (d *Device) GetEnrollImage() (detected bool, confirm uint8, err error) {
	if err := d.lockOp(context.Background()); err != nil {
		return false, 0, err
	}
	defer d.unlockOp()
	_, confirm, err = d.transceive(CmdGetEnrollImage, nil, d.timeout)
	if err != nil {
		return false, confirm, err
	}
	return confirm == ConfirmOK, confirm, nil
}

func validBuffer(bufferID uint8) bool { return bufferID == 1 || bufferID == 2 }

// GenChar converts the current image into a feature file (0x02).
// bufferID must be 1 or 2.
func (d *Device) GenChar(bufferID uint8) (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	if !validBuffer(bufferID) {
		return ConfirmPacketError, fmt.Errorf("%w: GenChar buffer %d", ErrInvalidParam, bufferID)
	}
	_, confirm, err := d.transceive(CmdGenChar, []byte{bufferID}, d.timeout)
	return confirm, err
}

// RegModel merges the two feature buffers into one template (0x05).
func (d *Device) RegModel() (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	_, confirm, err := d.transceive(CmdRegModel, nil, d.timeout)
	return confirm, err
}

// Store writes a feature buffer to a template slot (0x06).
func (d *Device) Store(bufferID uint8, page uint16) (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	if !validBuffer(bufferID) {
		return ConfirmPacketError, fmt.Errorf("%w: Store buffer %d", ErrInvalidParam, bufferID)
	}
	if d.Capacity > 0 && page >= d.Capacity {
		return ConfirmPacketError, fmt.Errorf("%w: page %d >= capacity %d", ErrInvalidParam, page, d.Capacity)
	}
	_, confirm, err := d.transceive(CmdStore, []byte{bufferID, byte(page >> 8), byte(page)}, d.timeout)
	return confirm, err
}

// LoadChar loads a template from flash into a feature buffer (0x07).
func (d *Device) LoadChar(bufferID uint8, page uint16) (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	if !validBuffer(bufferID) {
		return ConfirmPacketError, fmt.Errorf("%w: LoadChar buffer %d", ErrInvalidParam, bufferID)
	}
	if d.Capacity > 0 && page >= d.Capacity {
		return ConfirmPacketError, fmt.Errorf("%w: page %d >= capacity %d", ErrInvalidParam, page, d.Capacity)
	}
	_, confirm, err := d.transceive(CmdLoadChar, []byte{bufferID, byte(page >> 8), byte(page)}, d.timeout)
	return confirm, err
}

// Match compares feature buffer 1 with buffer 2 (0x03).
func (d *Device) Match() (MatchResult, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return MatchResult{}, err
	}
	defer d.unlockOp()
	data, confirm, err := d.transceive(CmdMatch, nil, d.timeout)
	if err != nil {
		return MatchResult{}, err
	}
	res := MatchResult{Matched: confirm == ConfirmOK, Confirm: confirm}
	if len(data) >= 2 {
		res.Score = uint16(data[0])<<8 | uint16(data[1])
	}
	return res, nil
}

// Search compares the features in bufferID against the library (0x04).
// The search starts at startPage and scans pageCount slots.
func (d *Device) Search(bufferID uint8, startPage, pageCount uint16) (SearchResult, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return SearchResult{}, err
	}
	defer d.unlockOp()
	if !validBuffer(bufferID) {
		return SearchResult{}, fmt.Errorf("%w: Search buffer %d", ErrInvalidParam, bufferID)
	}
	if pageCount == 0 {
		return SearchResult{}, fmt.Errorf("%w: Search pageCount is zero", ErrInvalidParam)
	}
	payload := []byte{bufferID, byte(startPage >> 8), byte(startPage), byte(pageCount >> 8), byte(pageCount)}
	data, confirm, err := d.transceive(CmdSearch, payload, d.timeout)
	if err != nil {
		return SearchResult{}, err
	}
	res := SearchResult{Confirm: confirm}
	if confirm == ConfirmOK {
		if len(data) < 4 {
			return res, ErrShortReply
		}
		res.Matched = true
		res.Page = uint16(data[0])<<8 | uint16(data[1])
		res.Score = uint16(data[2])<<8 | uint16(data[3])
	}
	return res, nil
}

// SearchNow captures a finger and searches it in one command (0x3E).
func (d *Device) SearchNow(startPage, pageCount uint16) (SearchResult, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return SearchResult{}, err
	}
	defer d.unlockOp()
	if pageCount == 0 {
		return SearchResult{}, fmt.Errorf("%w: SearchNow pageCount is zero", ErrInvalidParam)
	}
	payload := []byte{byte(startPage >> 8), byte(startPage), byte(pageCount >> 8), byte(pageCount)}
	data, confirm, err := d.transceive(CmdSearchNow, payload, d.timeout)
	if err != nil {
		return SearchResult{}, err
	}
	res := SearchResult{Confirm: confirm}
	if confirm == ConfirmOK {
		if len(data) < 4 {
			return res, ErrShortReply
		}
		res.Matched = true
		res.Page = uint16(data[0])<<8 | uint16(data[1])
		res.Score = uint16(data[2])<<8 | uint16(data[3])
	}
	return res, nil
}

// Delete removes count templates starting at page (0x0C).
func (d *Device) Delete(page, count uint16) (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	if count == 0 {
		return ConfirmPacketError, fmt.Errorf("%w: Delete count is zero", ErrInvalidParam)
	}
	payload := []byte{byte(page >> 8), byte(page), byte(count >> 8), byte(count)}
	_, confirm, err := d.transceive(CmdDelete, payload, d.timeout)
	return confirm, err
}

// Empty erases the whole template library (0x0D).
func (d *Device) Empty() (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	_, confirm, err := d.transceive(CmdEmpty, nil, d.timeout)
	return confirm, err
}

// Cancel aborts the current auto-enroll/auto-identify operation (0x30).
func (d *Device) Cancel() error {
	if d.inSession() {
		if err := acquire(context.Background(), d.cmdMu); err != nil {
			return err
		}
		err := d.cancelLocked()
		<-d.cmdMu
		d.endSession()
		return err
	}
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	return d.cancelLocked()
}

func (d *Device) cancelLocked() error {
	_, confirm, err := d.transceive(CmdCancel, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}
