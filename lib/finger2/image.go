//go:build esp32

package finger2

import "time"

// UpImage uploads the current image (0x0A) as 80x208, 4 bits per pixel.
// Each byte holds two pixels: the low nibble is the even pixel and the high
// nibble the odd one, where 0 is black and 15 is white.
//
// The transfer ends with an end-data packet; partial data is returned when the
// module stops early.
func (d *Device) UpImage() ([]byte, error) {
	d.flush()
	if err := d.send(CmdUpImage, nil); err != nil {
		return nil, err
	}
	pkt, err := d.readAck(10 * time.Second)
	if err != nil {
		return nil, err
	}
	confirm, _ := ackData(pkt)
	if confirm != ConfirmOK {
		return nil, Code(confirm)
	}
	data, err := d.readData(imageBytes+1024, 10*time.Second)
	if len(data) > 0 {
		return data, nil
	}
	return nil, err
}

// ImageInfo reads the area and quality of the image in the buffer (0x3D).
func (d *Device) ImageInfo() (area, quality uint8, confirm uint8, err error) {
	data, confirm, err := d.transceive(CmdImageInfo, nil, time.Second)
	if err != nil {
		return 0, 0, confirm, err
	}
	if len(data) >= 2 {
		area, quality = data[0], data[1]
	}
	return area, quality, confirm, nil
}

// ReadINF reads the 512-byte INF page (0x16).
func (d *Device) ReadINF() ([]byte, error) {
	d.flush()
	if err := d.send(CmdReadINF, nil); err != nil {
		return nil, err
	}
	pkt, err := d.readAck(5 * time.Second)
	if err != nil {
		return nil, err
	}
	confirm, _ := ackData(pkt)
	if confirm != ConfirmOK {
		return nil, Code(confirm)
	}
	return d.readData(infBytes, 5*time.Second)
}
