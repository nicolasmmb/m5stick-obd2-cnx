// Package finger2 implements the UART protocol for the M5Stack Unit
// Fingerprint2 (A-K323CP).
//
// Wiring on the M5StickC Plus2 Grove port:
//
//	host RX = G32, host TX = G33, 115200 8N1
package finger2

import (
	"errors"
	"fmt"
	"time"

	"machine"
)

const (
	header0 = 0xEF
	header1 = 0x01

	pidCommand = 1
	pidAck     = 7

	// Commands.
	CmdGetImage      = 0x01
	CmdSearch        = 0x04
	CmdReadSysParam  = 0x0F
	CmdControlLED    = 0x3C
	CmdActivate      = 0xD4
	CmdModuleStatus  = 0xD5
	CmdFirmware      = 0xD7
	CmdTemplateCount = 0x1D
	CmdHandshake     = 0x35

	// Confirmation codes.
	ConfirmOK       = 0x00
	ConfirmNoFinger = 0x02
	ConfirmNotFound = 0x09

	// LED modes.
	LEDBreath = 1
	LEDOn     = 3
	LEDOff    = 4

	// LED colors.
	LEDBlue   = 1
	LEDGreen  = 2
	LEDCyan   = 3
	LEDRed    = 4
	LEDYellow = 6
	LEDWhite  = 7

	DefaultAddress = 0xFFFFFFFF
)

var (
	ErrTimeout = errors.New("finger2: timeout")
	ErrPacket  = errors.New("finger2: invalid packet")
)

// Device is an A-K323CP fingerprint unit.
type Device struct {
	uart    *machine.UART
	address uint32
	timeout time.Duration

	Firmware   uint8
	Capacity   uint16
	Templates  uint16
	PacketSize uint16
	BaudRate   uint16

	rxBuf []byte
}

// New creates a device on the given UART. Configure the UART for 115200 8N1
// before calling Init.
func New(uart *machine.UART) *Device {
	return &Device{
		uart:    uart,
		address: DefaultAddress,
		timeout: 1500 * time.Millisecond,
	}
}

// SetTimeout changes the response timeout.
func (d *Device) SetTimeout(t time.Duration) {
	d.timeout = t
}

// Init wakes the module, reads firmware/parameters and sets the idle LED.
func (d *Device) Init() error {
	d.flush()

	// Some units answer the status query before the activate command.
	if data, confirm, err := d.transceive(CmdModuleStatus, nil, time.Second); err == nil {
		println("finger2: module status confirm", confirm, "data", len(data))
	} else {
		println("finger2: module status:", err.Error())
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		_, confirm, err := d.transceive(CmdActivate, nil, time.Second)
		if err == nil {
			lastErr = nil
			if confirm == ConfirmOK {
				break
			}
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Some modules do not answer D4 until after a handshake.
	if lastErr != nil {
		_, _, _ = d.transceive(CmdHandshake, nil, time.Second)
	}

	data, confirm, err := d.transceive(CmdFirmware, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK || len(data) < 1 {
		return fmt.Errorf("finger2: firmware confirm 0x%02X", confirm)
	}
	d.Firmware = data[0]

	data, confirm, err = d.transceive(CmdReadSysParam, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK || len(data) < 16 {
		return fmt.Errorf("finger2: params confirm 0x%02X", confirm)
	}
	d.Capacity = uint16(data[4])<<8 | uint16(data[5])
	d.PacketSize = uint16(data[12])<<8 | uint16(data[13])
	d.BaudRate = uint16(data[14])<<8 | uint16(data[15])

	d.Templates, _ = d.templateCount()

	_ = d.SetLED(LEDBreath, LEDBlue, LEDBlue, 0)
	return nil
}

func (d *Device) flush() {
	d.rxBuf = nil
	for d.uart.Buffered() > 0 {
		_, _ = d.uart.ReadByte()
	}
}

func (d *Device) send(cmd byte, payload []byte) error {
	n := 12 + len(payload)
	pkt := make([]byte, n)
	pkt[0] = header0
	pkt[1] = header1
	pkt[2] = byte(d.address >> 24)
	pkt[3] = byte(d.address >> 16)
	pkt[4] = byte(d.address >> 8)
	pkt[5] = byte(d.address)
	pkt[6] = pidCommand
	length := 3 + len(payload) // command + checksum + payload
	pkt[7] = byte(length >> 8)
	pkt[8] = byte(length)
	pkt[9] = cmd
	copy(pkt[10:], payload)
	var sum uint16
	for i := 6; i < n-2; i++ {
		sum += uint16(pkt[i])
	}
	pkt[n-2] = byte(sum >> 8)
	pkt[n-1] = byte(sum)
	_, err := d.uart.Write(pkt)
	return err
}

func (d *Device) readPacket(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	// Keep bytes that arrived after a previous packet (the unit can send
	// the command echo and the ACK back-to-back).
	buf := append([]byte(nil), d.rxBuf...)
	d.rxBuf = nil

	for time.Now().Before(deadline) {
		for d.uart.Buffered() > 0 {
			b, err := d.uart.ReadByte()
			if err != nil {
				break
			}
			buf = append(buf, b)

			for len(buf) >= 2 && !(buf[0] == header0 && buf[1] == header1) {
				buf = buf[1:]
			}
			if len(buf) >= 9 {
				length := int(buf[7])<<8 | int(buf[8])
				total := 9 + length
				if total > 1024 {
					buf = buf[1:]
					continue
				}
				if len(buf) >= total {
					pkt := make([]byte, total)
					copy(pkt, buf[:total])

					var sum uint16
					for i := 6; i < total-2; i++ {
						sum += uint16(pkt[i])
					}
					if pkt[total-2] == byte(sum>>8) && pkt[total-1] == byte(sum) {
						println("finger2: rx pid", pkt[6], "len", len(pkt))
						for _, b := range pkt {
							print(b, " ")
						}
						println()
						d.rxBuf = append([]byte(nil), buf[total:]...)
						return pkt, nil
					}
					println("finger2: bad checksum")
					for _, b := range pkt {
						print(b, " ")
					}
					println()
					buf = buf[1:]
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	d.rxBuf = buf
	return nil, ErrTimeout
}

// transceive sends a command and waits for an ACK. It returns the data after
// the confirmation code and the confirmation code itself.
func (d *Device) transceive(cmd byte, payload []byte, timeout time.Duration) ([]byte, byte, error) {
	d.flush()
	if err := d.send(cmd, payload); err != nil {
		return nil, 0, err
	}

	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, 0, ErrTimeout
		}
		pkt, err := d.readPacket(remaining)
		if err != nil {
			return nil, 0, err
		}
		if pkt[6] != pidAck {
			// The unit can echo the command before the ACK. Ignore it.
			continue
		}
		confirm := pkt[9]
		data := pkt[10 : len(pkt)-2]
		println("finger2: ack cmd", cmd, "confirm", confirm, "data", len(data))
		return data, confirm, nil
	}
}

// Capture asks the module to capture an image.
// detected is true when a finger is present.
func (d *Device) Capture() (detected bool, confirm byte, err error) {
	_, confirm, err = d.transceive(CmdGetImage, nil, d.timeout)
	if err != nil {
		return false, 0, err
	}
	return confirm == ConfirmOK, confirm, nil
}

// Search searches the stored templates for the captured image.
func (d *Device) Search(startPage, pageCount uint16) (matched bool, page uint16, score uint16, err error) {
	payload := []byte{
		1, // buffer ID
		byte(startPage >> 8), byte(startPage),
		byte(pageCount >> 8), byte(pageCount),
	}
	data, confirm, err := d.transceive(CmdSearch, payload, d.timeout)
	if err != nil {
		return false, 0, 0, err
	}
	if confirm == ConfirmOK && len(data) >= 4 {
		page = uint16(data[0])<<8 | uint16(data[1])
		score = uint16(data[2])<<8 | uint16(data[3])
		return true, page, score, nil
	}
	if confirm == ConfirmNotFound {
		return false, 0, 0, nil
	}
	return false, 0, 0, fmt.Errorf("finger2: search confirm 0x%02X", confirm)
}

// SetLED controls the RGB ring.
func (d *Device) SetLED(mode, color, endColor, cycle uint8) error {
	data, confirm, err := d.transceive(CmdControlLED, []byte{mode, color, endColor, cycle}, time.Second)
	_ = data
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return fmt.Errorf("finger2: led confirm 0x%02X", confirm)
	}
	return nil
}

func (d *Device) templateCount() (uint16, error) {
	data, confirm, err := d.transceive(CmdTemplateCount, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK || len(data) < 2 {
		return 0, fmt.Errorf("finger2: count confirm 0x%02X", confirm)
	}
	return uint16(data[0])<<8 | uint16(data[1]), nil
}

// RefreshTemplates updates the cached number of stored templates.
func (d *Device) RefreshTemplates() {
	if n, err := d.templateCount(); err == nil {
		d.Templates = n
	}
}
