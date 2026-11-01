//go:build esp32

package finger2

import (
	"errors"
	"fmt"
	"time"
)

// Packet framing.
const (
	header0 = 0xEF
	header1 = 0x01

	pidCommand = 0x01
	pidData    = 0x02
	pidAck     = 0x07
	pidEndData = 0x08

	// DefaultAddress is the broadcast address used by the module out of reset.
	DefaultAddress = 0xFFFFFFFF
)

// Module commands.
const (
	CmdGetImage       = 0x01
	CmdGenChar        = 0x02
	CmdMatch          = 0x03
	CmdSearch         = 0x04
	CmdRegModel       = 0x05
	CmdStore          = 0x06
	CmdLoadChar       = 0x07
	CmdUpImage        = 0x0A
	CmdDelete         = 0x0C
	CmdEmpty          = 0x0D
	CmdWriteReg       = 0x0E
	CmdReadSysPara    = 0x0F
	CmdGetRandom      = 0x14
	CmdReadINF        = 0x16
	CmdWriteNotepad   = 0x18
	CmdReadNotepad    = 0x19
	CmdValidTemplates = 0x1D
	CmdReadIndexTable = 0x1F
	CmdGetEnrollImage = 0x29
	CmdCancel         = 0x30
	CmdAutoEnroll     = 0x31
	CmdAutoIdentify   = 0x32
	CmdGetSN          = 0x34
	CmdHandshake      = 0x35
	CmdCheckSensor    = 0x36
	CmdControlLED     = 0x3C
	CmdImageInfo      = 0x3D
	CmdSearchNow      = 0x3E
	CmdUploadTemplate = 0x7A
	CmdDownloadTmpl   = 0x7B

	CmdSetSleepTime = 0xD0
	CmdGetSleepTime = 0xD1
	CmdSetWorkMode  = 0xD2
	CmdGetWorkMode  = 0xD3
	CmdActivate     = 0xD4
	CmdModuleStatus = 0xD5
	CmdSaveConfig   = 0xD6
	CmdFirmware     = 0xD7
)

// Confirmation codes returned by the module.
const (
	ConfirmOK               = 0x00
	ConfirmPacketError      = 0x01
	ConfirmNoFinger         = 0x02
	ConfirmImageFail        = 0x03
	ConfirmImageMessy       = 0x06
	ConfirmTooFewFeatures   = 0x07
	ConfirmNotFound         = 0x09
	ConfirmMergeFail        = 0x0A
	ConfirmPageOutOfRange   = 0x0B
	ConfirmReadTemplateFail = 0x0C
	ConfirmUploadFail       = 0x0D
	ConfirmReceiveError     = 0x0E
	ConfirmUploadImageFail  = 0x0F
	ConfirmDeleteFail       = 0x10
	ConfirmEmptyFail        = 0x11
	ConfirmLowPowerFail     = 0x12
	ConfirmPasswordError    = 0x13
	ConfirmResetFail        = 0x14
	ConfirmNoValidImage     = 0x15
	ConfirmFlashError       = 0x18
	ConfirmInvalidRegister  = 0x1A
	ConfirmInvalidAddress   = 0x1B
	ConfirmAutoEnrollFail   = 0x1E
	ConfirmModuleNotOpen    = 0xFE
	ConfirmPassiveWakeup    = 0xFF
)

// LED animation modes accepted by ControlLED.
const (
	LEDBreath  = 1
	LEDBlink   = 2
	LEDOn      = 3
	LEDOff     = 4
	LEDFadeIn  = 5
	LEDFadeOut = 6
)

// LED colors accepted by ControlLED.
const (
	LEDBlack   = 0
	LEDBlue    = 1
	LEDGreen   = 2
	LEDCyan    = 3
	LEDRed     = 4
	LEDMagenta = 5
	LEDYellow  = 6
	LEDWhite   = 7
)

// Sensor geometry and transfer sizes.
const (
	// ImageWidth and ImageHeight describe the raw sensor image in pixels.
	ImageWidth  = 80
	ImageHeight = 208

	// imageBytes is the 4-bit-per-pixel image payload size.
	imageBytes = ImageWidth * ImageHeight / 2
	// templateBytes is the size of one stored fingerprint template.
	templateBytes = 7262
	// infBytes is the size of the INF page.
	infBytes = 512

	maxPacketSize = 8192
	maxNotepadLen = 32
)

// Errors reported by the package. Module confirmations are returned as Code
// values so callers can match both the generic error and the exact code.
var (
	ErrTimeout      = errors.New("finger2: timeout")
	ErrPacket       = errors.New("finger2: invalid packet")
	ErrNoSpace      = errors.New("finger2: no free fingerprint page")
	ErrInvalidParam = errors.New("finger2: invalid parameter")
	ErrShortReply   = errors.New("finger2: short response")
)

// Code is a confirmation code returned by the module.
type Code uint8

// Error implements error.
func (c Code) Error() string { return codeName(uint8(c)) }

// Is allows errors.Is(err, finger2.Code(...)) comparisons by value.
func (c Code) Is(target error) bool {
	other, ok := target.(Code)
	return ok && c == other
}

func codeName(c uint8) string {
	switch c {
	case ConfirmOK:
		return "ok"
	case ConfirmPacketError:
		return "packet error"
	case ConfirmNoFinger:
		return "no finger"
	case ConfirmImageFail:
		return "invalid image"
	case ConfirmImageMessy:
		return "image too messy"
	case ConfirmTooFewFeatures:
		return "too few features"
	case ConfirmNotFound:
		return "not found"
	case ConfirmMergeFail:
		return "merge failed"
	case ConfirmPageOutOfRange:
		return "page out of range"
	case ConfirmReadTemplateFail:
		return "template read failed"
	case ConfirmUploadFail:
		return "upload failed"
	case ConfirmReceiveError:
		return "module cannot receive data"
	case ConfirmUploadImageFail:
		return "image upload failed"
	case ConfirmDeleteFail:
		return "delete failed"
	case ConfirmEmptyFail:
		return "clear library failed"
	case ConfirmLowPowerFail:
		return "cannot enter low power"
	case ConfirmPasswordError:
		return "wrong password"
	case ConfirmResetFail:
		return "reset failed"
	case ConfirmNoValidImage:
		return "no valid image in buffer"
	case ConfirmFlashError:
		return "flash error"
	case ConfirmInvalidRegister:
		return "invalid register"
	case ConfirmInvalidAddress:
		return "invalid address"
	case ConfirmAutoEnrollFail:
		return "auto enroll failed"
	case ConfirmModuleNotOpen:
		return "fingerprint module not open"
	case ConfirmPassiveWakeup:
		return "passive activation"
	}
	return fmt.Sprintf("confirm 0x%02X", c)
}

// checksum returns the 16-bit sum used by the module.
func checksum(b []byte) uint16 {
	var sum uint16
	for _, v := range b {
		sum += uint16(v)
	}
	return sum
}

// flush discards any pending bytes from the UART.
func (d *Device) flush() {
	d.rxBuf = nil
	for d.uart.Buffered() > 0 {
		_, _ = d.uart.ReadByte()
	}
}

// send serializes and writes a command packet.
func (d *Device) send(cmd byte, payload []byte) error {
	pkt := make([]byte, 12+len(payload))
	pkt[0] = header0
	pkt[1] = header1
	pkt[2] = byte(d.address >> 24)
	pkt[3] = byte(d.address >> 16)
	pkt[4] = byte(d.address >> 8)
	pkt[5] = byte(d.address)
	pkt[6] = pidCommand
	length := 3 + len(payload) // command + payload + checksum
	pkt[7] = byte(length >> 8)
	pkt[8] = byte(length)
	pkt[9] = cmd
	copy(pkt[10:], payload)
	sum := checksum(pkt[6 : len(pkt)-2])
	pkt[len(pkt)-2] = byte(sum >> 8)
	pkt[len(pkt)-1] = byte(sum)
	_, err := d.uart.Write(pkt)
	return err
}

// readPacket reads one valid packet, resynchronizing on the header and
// verifying the checksum. Leftover bytes are kept for the next call.
//
// The returned packet aliases the device scratch buffer and is only valid
// until the next read. Public methods copy or consume it before returning.
func (d *Device) readPacket(timeout time.Duration) ([]byte, error) {
	if cap(d.rxScratch) < 1024 {
		d.rxScratch = make([]byte, 0, 1024)
	}
	buf := append(d.rxScratch[:0], d.rxBuf...)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		for d.uart.Buffered() > 0 {
			b, err := d.uart.ReadByte()
			if err != nil {
				break
			}
			buf = append(buf, b)

			// Discard bytes until the two-byte start code.
			for len(buf) >= 2 && !(buf[0] == header0 && buf[1] == header1) {
				buf = buf[1:]
			}
			if len(buf) < 9 {
				continue
			}

			length := int(buf[7])<<8 | int(buf[8])
			total := 9 + length
			if total > maxPacketSize {
				buf = buf[1:]
				continue
			}
			if len(buf) < total {
				continue
			}

			sum := checksum(buf[6 : total-2])
			if buf[total-2] == byte(sum>>8) && buf[total-1] == byte(sum) {
				d.rxBuf = append(d.rxBuf[:0], buf[total:]...)
				d.rxScratch = buf[:0]
				return buf[:total], nil
			}
			buf = buf[1:] // checksum mismatch: resync one byte ahead
		}
		time.Sleep(time.Millisecond)
	}
	d.rxBuf = append(d.rxBuf[:0], buf...)
	d.rxScratch = buf[:0]
	return nil, ErrTimeout
}

// readAck waits for an ACK packet, skipping command echoes.
func (d *Device) readAck(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, ErrTimeout
		}
		pkt, err := d.readPacket(remaining)
		if err != nil {
			return nil, err
		}
		if d.skipWakeup(pkt) {
			continue
		}
		if pkt[6] == pidAck {
			return pkt, nil
		}
	}
}

// ackData splits an ACK payload into its confirmation byte and data.
func ackData(pkt []byte) (confirm byte, data []byte) {
	if len(pkt) < 12 {
		return ConfirmPacketError, nil
	}
	return pkt[9], pkt[10 : len(pkt)-2]
}

// transceive sends a command and returns the ACK data and confirmation.
func (d *Device) transceive(cmd byte, payload []byte, timeout time.Duration) ([]byte, byte, error) {
	d.flush()
	if err := d.send(cmd, payload); err != nil {
		return nil, 0, err
	}
	pkt, err := d.readAck(timeout)
	if err != nil {
		return nil, 0, err
	}
	confirm, data := ackData(pkt)
	// Copy ACK data so callers never hold a scratch-buffer alias.
	if len(data) > 0 {
		data = append([]byte(nil), data...)
	}
	return data, confirm, nil
}

// readData collects data packets into a fresh buffer.
func (d *Device) readData(expected int, timeout time.Duration) ([]byte, error) {
	data := make([]byte, expected)
	n, err := d.readDataInto(data, timeout)
	return data[:n], err
}

// readDataInto copies data packet payloads into dst until an end-data packet
// arrives or dst is full. It returns the number of bytes written, which may be
// smaller than len(dst) when the module ends the transfer early.
func (d *Device) readDataInto(dst []byte, timeout time.Duration) (int, error) {
	written := 0
	deadline := time.Now().Add(timeout)
	for written < len(dst) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return written, ErrTimeout
		}
		pkt, err := d.readPacket(remaining)
		if err != nil {
			return written, err
		}
		if d.skipWakeup(pkt) {
			continue
		}
		if pkt[6] != pidData && pkt[6] != pidEndData {
			continue
		}

		// Data packets have no confirmation byte: the payload starts at 9.
		if end := len(pkt) - 2; end > 9 {
			written += copy(dst[written:], pkt[9:end])
		}
		if pkt[6] == pidEndData {
			return written, nil
		}
	}
	return written, nil
}
