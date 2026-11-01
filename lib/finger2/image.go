package finger2

import (
	"context"
	"fmt"
	"time"
)

// UpImage uploads the current image (0x0A) into a new buffer.
// It is a convenience wrapper around UpImageTo.
func (d *Device) UpImage() ([]byte, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return nil, err
	}
	defer d.unlockOp()
	return d.upImageLocked()
}

func (d *Device) upImageLocked() ([]byte, error) {
	buf := make([]byte, imageBytes)
	n, err := d.upImageToLocked(buf)
	if n > 0 {
		return buf[:n], nil
	}
	return nil, err
}

// UpImageTo uploads the current image (0x0A) into dst, which must hold at
// least ImageWidth*ImageHeight/2 bytes. Reusing dst across frames avoids an
// allocation per image.
//
// The image is 80x208 at 4 bits per pixel: the low nibble is the even pixel
// and the high nibble the odd one, where 0 is black and 15 is white. The
// transfer ends with an end-data packet; a short image is returned as-is.
func (d *Device) UpImageTo(dst []byte) (int, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	return d.upImageToLocked(dst)
}

func (d *Device) upImageToLocked(dst []byte) (int, error) {
	if len(dst) < imageBytes {
		return 0, fmt.Errorf("%w: UpImageTo buffer %d < %d", ErrInvalidParam, len(dst), imageBytes)
	}
	d.flush()
	if err := d.send(CmdUpImage, nil); err != nil {
		return 0, err
	}
	pkt, err := d.readAck(10 * time.Second)
	if err != nil {
		return 0, err
	}
	confirm, _ := ackData(pkt)
	if confirm != ConfirmOK {
		return 0, Code(confirm)
	}
	return d.readDataInto(dst[:imageBytes], 10*time.Second)
}

// ImageInfo reads the area and quality of the image in the buffer (0x3D).
func (d *Device) ImageInfo() (area, quality uint8, confirm uint8, err error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, 0, 0, err
	}
	defer d.unlockOp()
	data, confirm, err := d.transceive(CmdImageInfo, nil, time.Second)
	if err != nil {
		return 0, 0, confirm, err
	}
	if len(data) >= 2 {
		area, quality = data[0], data[1]
	}
	return area, quality, confirm, nil
}

// ReadINF reads the 512-byte INF page (0x16) into a new buffer.
func (d *Device) ReadINF() ([]byte, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return nil, err
	}
	defer d.unlockOp()
	return d.readINFLocked()
}

func (d *Device) readINFLocked() ([]byte, error) {
	buf := make([]byte, infBytes)
	n, err := d.readINFToLocked(buf)
	if n > 0 {
		return buf[:n], nil
	}
	return nil, err
}

// ReadINFTo reads the INF page (0x16) into dst, which must hold at least 512
// bytes.
func (d *Device) ReadINFTo(dst []byte) (int, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	return d.readINFToLocked(dst)
}

func (d *Device) readINFToLocked(dst []byte) (int, error) {
	if len(dst) < infBytes {
		return 0, fmt.Errorf("%w: ReadINFTo buffer %d < %d", ErrInvalidParam, len(dst), infBytes)
	}
	d.flush()
	if err := d.send(CmdReadINF, nil); err != nil {
		return 0, err
	}
	pkt, err := d.readAck(5 * time.Second)
	if err != nil {
		return 0, err
	}
	confirm, _ := ackData(pkt)
	if confirm != ConfirmOK {
		return 0, Code(confirm)
	}
	return d.readDataInto(dst[:infBytes], 5*time.Second)
}
