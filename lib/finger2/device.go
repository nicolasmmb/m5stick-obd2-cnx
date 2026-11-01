//go:build esp32

package finger2

import (
	"time"

	"machine"
)

// SystemParams mirrors the 16-byte system parameter block (0x0F).
type SystemParams struct {
	Status       uint16
	TemplateSize uint16
	Capacity     uint16
	MatchLevel   uint16
	Address      uint32
	PacketSize   uint16
	BaudRate     uint16
}

// SearchResult is returned by Search and SearchNow. Matched is true when the
// module confirmed a match (Confirm == ConfirmOK); Score is informational.
type SearchResult struct {
	Matched bool
	Page    uint16
	Score   uint16
	Confirm uint8
}

// MatchResult is returned by Match. Matched is true when the two feature
// buffers belong to the same finger.
type MatchResult struct {
	Matched bool
	Score   uint16
	Confirm uint8
}

// Device is an A-K323CP fingerprint unit attached to a UART.
//
// Fields updated by Init are informational copies of the module state. They
// are safe to read from the goroutine that owns the Device.
type Device struct {
	uart    *machine.UART
	address uint32
	timeout time.Duration
	rxBuf   []byte
	wakeup  func([]byte)

	// Firmware is the module firmware version (0xD7).
	Firmware uint8
	// Params is the last system parameter block read from the module.
	Params SystemParams
	// Capacity is the number of template slots (0..Capacity-1).
	Capacity uint16
	// Templates is the number of stored templates.
	Templates uint16
	// PacketSize is the module packet size register value.
	PacketSize uint16
	// BaudRate is the module baud rate register value.
	BaudRate uint16
	// MatchLevel is the security level (1..5).
	MatchLevel uint16
	// Status is the module status word.
	Status uint16
}

// New returns a Device bound to uart. Configure the UART for 115200 8N1 before
// calling Init.
func New(uart *machine.UART) *Device {
	return &Device{
		uart:    uart,
		address: DefaultAddress,
		timeout: 1500 * time.Millisecond,
	}
}

// SetTimeout changes the default command timeout.
func (d *Device) SetTimeout(t time.Duration) { d.timeout = t }

// SetAddress changes the module address (broadcast by default).
func (d *Device) SetAddress(a uint32) { d.address = a }

// SetWakeupHandler registers a callback for the module wakeup packet
// (EF 01 FF FF FF FF 07 00 03 FF 01 09), sent when a finger touches the unit
// while it sleeps. Wakeup packets are also skipped inside command reads.
func (d *Device) SetWakeupHandler(h func([]byte)) { d.wakeup = h }

var wakeupPattern = [12]byte{0xEF, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x07, 0x00, 0x03, 0xFF, 0x01, 0x09}

func isWakeupPacket(pkt []byte) bool {
	if len(pkt) != len(wakeupPattern) {
		return false
	}
	for i := range wakeupPattern {
		if pkt[i] != wakeupPattern[i] {
			return false
		}
	}
	return true
}

// skipWakeup consumes wakeup packets and fires the handler.
func (d *Device) skipWakeup(pkt []byte) bool {
	if !isWakeupPacket(pkt) {
		return false
	}
	if d.wakeup != nil {
		d.wakeup(pkt)
	}
	return true
}

// WaitWakeup blocks until the module reports a touch or the timeout expires.
func (d *Device) WaitWakeup(timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false, ErrTimeout
		}
		pkt, err := d.readPacket(remaining)
		if err != nil {
			return false, err
		}
		if d.skipWakeup(pkt) {
			return true, nil
		}
	}
}

// Activate wakes the fingerprint module (0xD4).
func (d *Device) Activate() error {
	_, confirm, err := d.transceive(CmdActivate, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// GetFirmwareVersion reads the module firmware version (0xD7).
func (d *Device) GetFirmwareVersion() (uint8, error) {
	data, confirm, err := d.transceive(CmdFirmware, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK {
		return 0, Code(confirm)
	}
	if len(data) < 1 {
		return 0, ErrShortReply
	}
	return data[0], nil
}

// Init wakes the module, reads its parameters and applies the defaults used by
// this package: MatchLevel 5 and a breathing blue LED.
//
// Some units answer 0xFE ("module not open") for a short while after waking,
// so activation is retried before giving up.
func (d *Device) Init() error {
	d.flush()
	_, _, _ = d.transceive(CmdHandshake, nil, 500*time.Millisecond)

	if data, confirm, err := d.transceive(CmdModuleStatus, nil, 800*time.Millisecond); err == nil && confirm == ConfirmOK && len(data) > 0 {
		d.Status = uint16(data[0])
	}

	var (
		params   SystemParams
		firmware uint8
		ready    bool
		lastErr  = ErrTimeout
	)
	for attempt := 0; attempt < 15; attempt++ {
		_, confirm, err := d.transceive(CmdActivate, nil, 800*time.Millisecond)
		switch {
		case err != nil:
			lastErr = err
		case confirm != ConfirmOK:
			lastErr = Code(confirm)
		default:
			time.Sleep(120 * time.Millisecond)
			data, fwConfirm, fwErr := d.transceive(CmdFirmware, nil, time.Second)
			switch {
			case fwErr != nil:
				lastErr = fwErr
			case fwConfirm != ConfirmOK:
				lastErr = Code(fwConfirm)
			case len(data) < 1:
				lastErr = ErrShortReply
			default:
				p, err := d.ReadSysParams()
				if err == nil {
					firmware, params, ready = data[0], p, true
				} else {
					lastErr = err
				}
			}
		}
		if ready {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		return lastErr
	}

	d.Firmware = firmware
	d.Params = params
	d.Capacity = params.Capacity
	d.PacketSize = params.PacketSize
	d.BaudRate = params.BaudRate
	d.Status = params.Status
	d.MatchLevel = params.MatchLevel

	if _, err := d.SetMatchLevel(5); err == nil {
		d.MatchLevel = 5
	}
	if count, err := d.ValidTemplates(); err == nil {
		d.Templates = count
	}
	_ = d.ControlLED(LEDBreath, LEDBlue, LEDBlue, 0)
	return nil
}

// RefreshTemplates re-reads the number of stored templates.
func (d *Device) RefreshTemplates() {
	if count, err := d.ValidTemplates(); err == nil {
		d.Templates = count
	}
}
