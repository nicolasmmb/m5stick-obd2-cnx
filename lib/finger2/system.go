package finger2

import (
	"context"
	"fmt"
	"time"
)

// ReadSysParams reads the 16-byte system parameter block (0x0F).
func (d *Device) ReadSysParams() (SystemParams, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return SystemParams{}, err
	}
	defer d.unlockOp()
	return d.readSysParamsLocked()
}

func (d *Device) readSysParamsLocked() (SystemParams, error) {
	data, confirm, err := d.transceive(CmdReadSysPara, nil, d.timeout)
	if err != nil {
		return SystemParams{}, err
	}
	if confirm != ConfirmOK {
		return SystemParams{}, Code(confirm)
	}
	if len(data) < 16 {
		return SystemParams{}, ErrShortReply
	}
	return SystemParams{
		Status:       uint16(data[0])<<8 | uint16(data[1]),
		TemplateSize: uint16(data[2])<<8 | uint16(data[3]),
		Capacity:     uint16(data[4])<<8 | uint16(data[5]),
		MatchLevel:   uint16(data[6])<<8 | uint16(data[7]),
		Address:      uint32(data[8])<<24 | uint32(data[9])<<16 | uint32(data[10])<<8 | uint32(data[11]),
		PacketSize:   uint16(data[12])<<8 | uint16(data[13]),
		BaudRate:     uint16(data[14])<<8 | uint16(data[15]),
	}, nil
}

// ModuleStatus reads the fingerprint module status byte (0xD5).
func (d *Device) ModuleStatus() (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	data, confirm, err := d.transceive(CmdModuleStatus, nil, time.Second)
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

// SetSleepTime sets the inactivity timeout before sleep (0xD0), in seconds.
func (d *Device) SetSleepTime(sec uint8) error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	if sec < 10 {
		return fmt.Errorf("%w: sleep time %d out of range (10..254)", ErrInvalidParam, sec)
	}
	_, confirm, err := d.transceive(CmdSetSleepTime, []byte{sec}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// GetSleepTime reads the inactivity timeout before sleep (0xD1), in seconds.
func (d *Device) GetSleepTime() (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	data, confirm, err := d.transceive(CmdGetSleepTime, nil, time.Second)
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

// SetWorkMode selects 0 = timed sleep and 1 = always on (0xD2).
func (d *Device) SetWorkMode(mode uint8) error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	if mode > 1 {
		return fmt.Errorf("%w: work mode %d out of range (0..1)", ErrInvalidParam, mode)
	}
	_, confirm, err := d.transceive(CmdSetWorkMode, []byte{mode}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// GetWorkMode reads the current work mode (0xD3).
func (d *Device) GetWorkMode() (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	data, confirm, err := d.transceive(CmdGetWorkMode, nil, time.Second)
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

// SaveConfig persists configuration to flash (0xD6): 0 = sleep time,
// 1 = work mode.
func (d *Device) SaveConfig(option uint8) error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	if option > 1 {
		return fmt.Errorf("%w: save option %d out of range (0..1)", ErrInvalidParam, option)
	}
	_, confirm, err := d.transceive(CmdSaveConfig, []byte{option}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// Handshake checks that the module is reachable (0x35).
func (d *Device) Handshake() error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	_, confirm, err := d.transceive(CmdHandshake, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// CheckSensor runs the sensor self-test (0x36).
func (d *Device) CheckSensor() error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	_, confirm, err := d.transceive(CmdCheckSensor, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// GetSN reads the 32-byte chip serial number (0x34). This unit returns the
// serial right-aligned with leading zero bytes.
func (d *Device) GetSN() ([]byte, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return nil, err
	}
	defer d.unlockOp()
	data, confirm, err := d.transceive(CmdGetSN, []byte{0}, time.Second)
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

// GetRandom reads a 32-bit random value from the module (0x14).
func (d *Device) GetRandom() (uint32, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	data, confirm, err := d.transceive(CmdGetRandom, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK {
		return 0, Code(confirm)
	}
	if len(data) < 4 {
		return 0, ErrShortReply
	}
	return uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]), nil
}

// WriteReg writes a module register (0x0E). Register 4 is reserved; IDs above
// 9 are sent as 0x10..0x13, matching the vendor library.
func (d *Device) WriteReg(reg, value uint8) error {
	if err := d.lockOp(context.Background()); err != nil {
		return err
	}
	defer d.unlockOp()
	if reg == 4 || reg > 13 {
		return fmt.Errorf("%w: register %d out of range", ErrInvalidParam, reg)
	}
	id := reg
	if reg > 9 {
		id = 0x10 + (reg - 10)
	}
	_, confirm, err := d.transceive(CmdWriteReg, []byte{id, value}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// SetMatchLevel writes the security level (register 5) and returns the
// confirmation code. Values are clamped to 1..5.
func (d *Device) SetMatchLevel(level uint8) (uint8, error) {
	if err := d.lockOp(context.Background()); err != nil {
		return 0, err
	}
	defer d.unlockOp()
	if level < 1 {
		level = 1
	}
	if level > 5 {
		level = 5
	}
	_, confirm, err := d.transceive(CmdWriteReg, []byte{5, level}, time.Second)
	if err != nil {
		return confirm, err
	}
	if confirm != ConfirmOK {
		return confirm, Code(confirm)
	}
	return confirm, nil
}
