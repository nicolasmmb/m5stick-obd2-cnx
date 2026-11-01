package finger2

import (
	"context"
	"fmt"
	"time"
)

const (
	templateChunk      = 128 // upload chunk size
	templateWriteChunk = 100 // download chunk size
)

// ValidTemplates reads the number of stored templates (0x1D).
func (d *Device) ValidTemplates() (uint16, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	return d.validTemplatesLocked()
}

func (d *Device) validTemplatesLocked() (uint16, error) {
	data, confirm, err := d.transceive(CmdValidTemplates, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK {
		return 0, Code(confirm)
	}
	if len(data) < 2 {
		return 0, ErrShortReply
	}
	return uint16(data[0])<<8 | uint16(data[1]), nil
}

// ReadIndexTable reads one 32-byte page of the template index (0x1F).
// Each bit represents one template slot: set means occupied.
func (d *Device) ReadIndexTable(page uint8) ([]byte, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return nil, err
	}
	defer d.unlockOp()
	return d.readIndexTableLocked(page)
}

func (d *Device) readIndexTableLocked(page uint8) ([]byte, error) {
	data, confirm, err := d.transceive(CmdReadIndexTable, []byte{page}, time.Second)
	if err != nil {
		return nil, err
	}
	if confirm != ConfirmOK {
		return nil, Code(confirm)
	}
	if len(data) < 32 {
		return nil, ErrShortReply
	}
	return append([]byte(nil), data[:32]...), nil
}

// FirstFreePage returns the first unused template slot. It reads only the
// first index page, which covers the module capacity of 100 slots.
func (d *Device) FirstFreePage() (uint16, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	table, err := d.readIndexTableLocked(0)
	if err != nil {
		return 0, err
	}
	capacity := d.Capacity
	if capacity == 0 {
		capacity = 100
	}
	for i := 0; i < len(table); i++ {
		for bit := 0; bit < 8; bit++ {
			if table[i]&(1<<bit) != 0 {
				continue
			}
			page := uint16(i*8 + bit)
			if page < capacity {
				return page, nil
			}
		}
	}
	return 0, ErrNoSpace
}

// UploadTemplate reads size bytes of the loaded template from the module
// (0x7A). Load a template into a feature buffer with LoadChar first. It
// returns the bytes, the actual size reported by the module and the
// confirmation code.
func (d *Device) UploadTemplate(offset, size uint16) ([]byte, uint16, uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return nil, 0, 0, err
	}
	defer d.unlockOp()
	if size == 0 {
		return nil, 0, ConfirmPacketError, fmt.Errorf("%w: UploadTemplate size is zero", ErrInvalidParam)
	}
	payload := []byte{byte(offset >> 8), byte(offset), byte(size >> 8), byte(size)}
	data, confirm, err := d.transceive(CmdUploadTemplate, payload, 10*time.Second)
	if err != nil {
		return nil, 0, confirm, err
	}
	if confirm != ConfirmOK {
		return nil, 0, confirm, Code(confirm)
	}
	if len(data) < 2 {
		return nil, 0, confirm, ErrShortReply
	}
	actual := uint16(data[0])<<8 | uint16(data[1])
	if len(data) > 2 {
		return append([]byte(nil), data[2:]...), actual, confirm, nil
	}
	return nil, actual, confirm, nil
}

// DownloadTemplate writes template data at offset into the module buffer
// (0x7B). Call Store afterwards to persist the buffer.
func (d *Device) DownloadTemplate(offset uint16, data []byte) (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	if len(data) == 0 {
		return ConfirmPacketError, fmt.Errorf("%w: DownloadTemplate empty data", ErrInvalidParam)
	}
	if len(data) > maxPacketSize-12 {
		return ConfirmPacketError, fmt.Errorf("%w: DownloadTemplate chunk too large", ErrInvalidParam)
	}
	payload := make([]byte, 4+len(data))
	payload[0] = byte(offset >> 8)
	payload[1] = byte(offset)
	payload[2] = byte(len(data) >> 8)
	payload[3] = byte(len(data))
	copy(payload[4:], data)
	_, confirm, err := d.transceive(CmdDownloadTmpl, payload, 10*time.Second)
	return confirm, err
}

// UploadTemplateAuto reads a whole template (0x7A) in chunks. size is the
// template size, typically 7262 bytes. Reading stops on a short chunk.
func (d *Device) UploadTemplateAuto(size uint16) ([]byte, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return nil, err
	}
	defer d.unlockOp()
	if size == 0 {
		return nil, fmt.Errorf("%w: UploadTemplateAuto size is zero", ErrInvalidParam)
	}
	out := make([]byte, 0, size)
	offset := 0
	for offset < int(size) {
		want := uint16(templateChunk)
		if remaining := int(size) - offset; remaining < templateChunk {
			want = uint16(remaining)
		}
		var payload [4]byte
		payload[0] = byte(offset >> 8)
		payload[1] = byte(offset)
		payload[2] = byte(want >> 8)
		payload[3] = byte(want)
		data, confirm, err := d.transceive(CmdUploadTemplate, payload[:], 10*time.Second)
		if err != nil {
			return out, err
		}
		if confirm != ConfirmOK {
			return out, Code(confirm)
		}
		if len(data) < 2 {
			return out, ErrShortReply
		}
		actual := uint16(data[0])<<8 | uint16(data[1])
		out = append(out, data[2:]...)
		if actual == 0 {
			break
		}
		offset += int(actual)
		if actual < want {
			break
		}
	}
	return out, nil
}

// DownloadTemplateAuto writes a whole template (0x7B) in chunks.
func (d *Device) DownloadTemplateAuto(data []byte) error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	if len(data) == 0 {
		return fmt.Errorf("%w: DownloadTemplateAuto empty data", ErrInvalidParam)
	}
	if len(data) > 0xFFFF {
		return fmt.Errorf("%w: template larger than 65535 bytes", ErrInvalidParam)
	}
	payload := make([]byte, 4+templateWriteChunk)
	for offset := 0; offset < len(data); {
		n := templateWriteChunk
		if len(data)-offset < n {
			n = len(data) - offset
		}
		payload[0] = byte(offset >> 8)
		payload[1] = byte(offset)
		payload[2] = byte(n >> 8)
		payload[3] = byte(n)
		copy(payload[4:4+n], data[offset:offset+n])

		_, confirm, err := d.transceive(CmdDownloadTmpl, payload[:4+n], 10*time.Second)
		if err != nil {
			return err
		}
		if confirm != ConfirmOK {
			return Code(confirm)
		}
		offset += n
	}
	return nil
}
