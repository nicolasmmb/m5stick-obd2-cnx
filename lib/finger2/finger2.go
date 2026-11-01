//go:build esp32

// Package finger2 implements the M5Stack Unit Fingerprint2 (A-K323CP)
// UART protocol entirely in Go.
//
// No C, C++, CGo, Arduino or ESP-IDF is used. Only machine.UART handles the
// Grove serial link (115200 8N1).
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
	pidData    = 2
	pidAck     = 7
	pidEndData = 8

	// Commands.
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
	CmdIAPReset     = 0xFD

	// Confirmation codes.
	ConfirmOK       = 0x00
	ConfirmNoFinger = 0x02
	ConfirmNotFound = 0x09

	// LED modes.
	LEDBreath  = 1
	LEDBlink   = 2
	LEDOn      = 3
	LEDOff     = 4
	LEDFadeIn  = 5
	LEDFadeOut = 6
	LEDRainbow = 7

	// LED colors.
	LEDBlack   = 0
	LEDBlue    = 1
	LEDGreen   = 2
	LEDCyan    = 3
	LEDRed     = 4
	LEDMagenta = 5
	LEDYellow  = 6
	LEDWhite   = 7

	DefaultAddress = 0xFFFFFFFF

	imageBytes    = 8230
	templateBytes = 7262
	infBytes      = 512
)

var (
	ErrTimeout = errors.New("finger2: timeout")
	ErrPacket  = errors.New("finger2: invalid packet")
	ErrNoSpace = errors.New("finger2: no free fingerprint page")
)

// Code is a confirmation code returned by the module.
type Code uint8

func (c Code) Error() string { return codeName(uint8(c)) }

func codeName(c uint8) string {
	switch c {
	case 0x00:
		return "OK"
	case 0x01:
		return "erro de pacote"
	case 0x02:
		return "sem dedo"
	case 0x03:
		return "imagem suja"
	case 0x06:
		return "imagem com poucas caracteristicas"
	case 0x07:
		return "caracteristicas nao batem"
	case 0x08:
		return "nao encontrado"
	case 0x09:
		return "nao encontrado"
	case 0x0A:
		return "merge falhou"
	case 0x0B:
		return "pagina fora da faixa"
	case 0x0C:
		return "leitura de template falhou"
	case 0x0D:
		return "upload falhou"
	case 0x0E:
		return "modulo nao aceita dados"
	case 0x0F:
		return "upload de imagem falhou"
	case 0x10:
		return "delete falhou"
	case 0x11:
		return "limpar biblioteca falhou"
	case 0x12:
		return "senha incorreta"
	case 0x13:
		return "imagem invalida"
	case 0x15:
		return "gerar imagem falhou"
	case 0x18:
		return "erro de flash"
	case 0x1A:
		return "registrador invalido"
	case 0x1B:
		return "endereco incorreto"
	case 0x1D:
		return "digital incorreta"
	case 0x1E:
		return "falha ao gravar template"
	case 0x1F:
		return "pagina invalida"
	case 0x20:
		return "endereco do dispositivo incorreto"
	}
	return fmt.Sprintf("confirm 0x%02X", c)
}

// SystemParams are the module parameters read at startup.
type SystemParams struct {
	Status       uint16
	TemplateSize uint16
	Capacity     uint16
	MatchLevel   uint16
	Address      uint32
	PacketSize   uint16
	BaudRate     uint16
}

// SearchResult is returned by Search/SearchNow.
type SearchResult struct {
	Matched bool
	Page    uint16
	Score   uint16
	Confirm uint8
}

// MatchResult is returned by Match.
type MatchResult struct {
	Matched bool
	Score   uint16
	Confirm uint8
}

// Device is an A-K323CP fingerprint unit.
type Device struct {
	uart    *machine.UART
	address uint32
	timeout time.Duration
	rxBuf   []byte

	Firmware   uint8
	Params     SystemParams
	Capacity   uint16
	Templates  uint16
	PacketSize uint16
	BaudRate   uint16
	MatchLevel uint16
	Status     uint16
}

// New creates a device on the given UART. Configure it for 115200 8N1 first.
func New(uart *machine.UART) *Device {
	return &Device{
		uart:    uart,
		address: DefaultAddress,
		timeout: 1500 * time.Millisecond,
	}
}

func (d *Device) SetTimeout(t time.Duration) { d.timeout = t }
func (d *Device) SetAddress(a uint32)        { d.address = a }

// Init wakes the module and reads its parameters.
func (d *Device) Init() error {
	println("finger2 lib: init start")
	d.flush()
	_, _, _ = d.transceive(CmdHandshake, nil, 500*time.Millisecond)
	println("finger2 lib: handshake done")

	// Some units answer the status query before activate.
	if data, confirm, err := d.transceive(CmdModuleStatus, nil, 800*time.Millisecond); err == nil && confirm == ConfirmOK && len(data) > 0 {
		d.Status = uint16(data[0])
	}
	for i := 0; i < 3; i++ {
		_, confirm, err := d.transceive(CmdActivate, nil, 800*time.Millisecond)
		if err == nil && confirm == ConfirmOK {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	println("finger2 lib: activate done")

	data, confirm, err := d.transceive(CmdFirmware, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK || len(data) < 1 {
		return Code(confirm)
	}
	d.Firmware = data[0]

	println("finger2 lib: firmware done")
	params, err := d.ReadSysParams()
	if err != nil {
		return err
	}
	println("finger2 lib: params done")
	d.Params = params
	d.Capacity = params.Capacity
	d.PacketSize = params.PacketSize
	d.BaudRate = params.BaudRate
	d.MatchLevel = params.MatchLevel
	d.Status = params.Status

	if c, err := d.SetMatchLevel(5); err != nil {
		println("finger2 lib: matchlevel failed", c, err.Error())
	} else {
		d.MatchLevel = 5
		println("finger2 lib: matchlevel 5 ok")
	}
	d.Templates, _ = d.ValidTemplates()
	_ = d.ControlLED(LEDBreath, LEDBlue, LEDBlue, 0)
	println("finger2 lib: ready")
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
	length := 3 + len(payload)
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
				if total > 8192 {
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
						d.rxBuf = append([]byte(nil), buf[total:]...)
						return pkt, nil
					}
					buf = buf[1:]
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	d.rxBuf = buf
	return nil, ErrTimeout
}

// readAck waits for an ACK, ignoring command echoes.
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
		if pkt[6] == pidAck {
			return pkt, nil
		}
	}
}

func ackData(pkt []byte) (confirm byte, data []byte) {
	if len(pkt) < 12 {
		return 0xFF, nil
	}
	return pkt[9], pkt[10 : len(pkt)-2]
}

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
	return data, confirm, nil
}

func (d *Device) readData(expected int, timeout time.Duration) ([]byte, error) {
	data := make([]byte, 0, expected)
	deadline := time.Now().Add(timeout)
	for len(data) < expected {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		pkt, err := d.readPacket(remaining)
		if err != nil {
			return data, err
		}
		if pkt[6] == pidData || pkt[6] == pidEndData {
			confirm, chunk := ackData(pkt)
			_ = confirm
			data = append(data, chunk...)
			if pkt[6] == pidEndData {
				break
			}
		}
	}
	return data, nil
}

// --- System commands ---

func (d *Device) ReadSysParams() (SystemParams, error) {
	var p SystemParams
	data, confirm, err := d.transceive(CmdReadSysPara, nil, d.timeout)
	if err != nil {
		return p, err
	}
	if confirm != ConfirmOK || len(data) < 16 {
		return p, Code(confirm)
	}
	p.Status = uint16(data[0])<<8 | uint16(data[1])
	p.TemplateSize = uint16(data[2])<<8 | uint16(data[3])
	p.Capacity = uint16(data[4])<<8 | uint16(data[5])
	p.MatchLevel = uint16(data[6])<<8 | uint16(data[7])
	p.Address = uint32(data[8])<<24 | uint32(data[9])<<16 | uint32(data[10])<<8 | uint32(data[11])
	p.PacketSize = uint16(data[12])<<8 | uint16(data[13])
	p.BaudRate = uint16(data[14])<<8 | uint16(data[15])
	return p, nil
}

func (d *Device) ModuleStatus() (uint8, error) {
	data, confirm, err := d.transceive(CmdModuleStatus, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK || len(data) < 1 {
		return 0, Code(confirm)
	}
	return data[0], nil
}

func (d *Device) SetSleepTime(sec uint8) error {
	_, confirm, err := d.transceive(CmdSetSleepTime, []byte{sec}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

func (d *Device) GetSleepTime() (uint8, error) {
	data, confirm, err := d.transceive(CmdGetSleepTime, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK || len(data) < 1 {
		return 0, Code(confirm)
	}
	return data[0], nil
}

func (d *Device) SetWorkMode(mode uint8) error {
	_, confirm, err := d.transceive(CmdSetWorkMode, []byte{mode}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

func (d *Device) GetWorkMode() (uint8, error) {
	data, confirm, err := d.transceive(CmdGetWorkMode, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK || len(data) < 1 {
		return 0, Code(confirm)
	}
	return data[0], nil
}

func (d *Device) SaveConfig(option uint8) error {
	_, confirm, err := d.transceive(CmdSaveConfig, []byte{option}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

func (d *Device) IAPReset() error {
	_, confirm, err := d.transceive(CmdIAPReset, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

func (d *Device) Handshake() error {
	_, confirm, err := d.transceive(CmdHandshake, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

func (d *Device) CheckSensor() error {
	_, confirm, err := d.transceive(CmdCheckSensor, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

func (d *Device) GetSN() ([]byte, error) {
	data, confirm, err := d.transceive(CmdGetSN, []byte{0}, time.Second)
	if err != nil {
		return nil, err
	}
	if confirm != ConfirmOK || len(data) < 32 {
		return nil, Code(confirm)
	}
	return append([]byte(nil), data[:32]...), nil
}

func (d *Device) GetRandom() (uint32, error) {
	data, confirm, err := d.transceive(CmdGetRandom, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK || len(data) < 4 {
		return 0, Code(confirm)
	}
	return uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]), nil
}

func (d *Device) WriteReg(reg, value uint8) error {
	_, confirm, err := d.transceive(CmdWriteReg, []byte{reg, value}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// SetMatchLevel sets register 5 (MatchLevel, 1-5).
func (d *Device) SetMatchLevel(level uint8) (uint8, error) {
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

// --- Biometric commands ---

func (d *Device) Capture() (bool, uint8, error) {
	_, confirm, err := d.transceive(CmdGetImage, nil, d.timeout)
	if err != nil {
		return false, 0, err
	}
	return confirm == ConfirmOK, confirm, nil
}

func (d *Device) GetEnrollImage() (bool, uint8, error) {
	_, confirm, err := d.transceive(CmdGetEnrollImage, nil, d.timeout)
	if err != nil {
		return false, 0, err
	}
	return confirm == ConfirmOK, confirm, nil
}

func (d *Device) GenChar(bufferID uint8) (uint8, error) {
	_, confirm, err := d.transceive(CmdGenChar, []byte{bufferID}, d.timeout)
	return confirm, err
}

func (d *Device) RegModel() (uint8, error) {
	_, confirm, err := d.transceive(CmdRegModel, nil, d.timeout)
	return confirm, err
}

func (d *Device) Store(bufferID uint8, page uint16) (uint8, error) {
	_, confirm, err := d.transceive(CmdStore, []byte{bufferID, byte(page >> 8), byte(page)}, d.timeout)
	return confirm, err
}

func (d *Device) LoadChar(bufferID uint8, page uint16) (uint8, error) {
	_, confirm, err := d.transceive(CmdLoadChar, []byte{bufferID, byte(page >> 8), byte(page)}, d.timeout)
	return confirm, err
}

func (d *Device) Match(buffers ...uint8) (MatchResult, error) {
	_ = buffers
	data, confirm, err := d.transceive(CmdMatch, nil, d.timeout)
	if err != nil {
		return MatchResult{}, err
	}
	r := MatchResult{Matched: confirm == ConfirmOK, Confirm: confirm}
	if len(data) >= 2 {
		r.Score = uint16(data[0])<<8 | uint16(data[1])
	}
	return r, nil
}

// Search compares the features in bufferID (1 or 2) against the library.
func (d *Device) Search(bufferID uint8, startPage, pageCount uint16) (SearchResult, error) {
	if bufferID == 0 {
		bufferID = 1
	}
	payload := []byte{bufferID, byte(startPage >> 8), byte(startPage), byte(pageCount >> 8), byte(pageCount)}
	data, confirm, err := d.transceive(CmdSearch, payload, d.timeout)
	if err != nil {
		return SearchResult{}, err
	}
	r := SearchResult{Confirm: confirm}
	if confirm == ConfirmOK && len(data) >= 4 {
		r.Matched = true
		r.Page = uint16(data[0])<<8 | uint16(data[1])
		r.Score = uint16(data[2])<<8 | uint16(data[3])
	}
	return r, nil
}

func (d *Device) SearchNow(startPage, pageCount uint16) (SearchResult, error) {
	payload := []byte{byte(startPage >> 8), byte(startPage), byte(pageCount >> 8), byte(pageCount)}
	data, confirm, err := d.transceive(CmdSearchNow, payload, d.timeout)
	if err != nil {
		return SearchResult{}, err
	}
	r := SearchResult{Confirm: confirm}
	if confirm == ConfirmOK && len(data) >= 4 {
		r.Matched = true
		r.Page = uint16(data[0])<<8 | uint16(data[1])
		r.Score = uint16(data[2])<<8 | uint16(data[3])
	}
	return r, nil
}

func (d *Device) Delete(page, count uint16) (uint8, error) {
	_, confirm, err := d.transceive(CmdDelete, []byte{byte(page >> 8), byte(page), byte(count >> 8), byte(count)}, d.timeout)
	return confirm, err
}

func (d *Device) Empty() (uint8, error) {
	_, confirm, err := d.transceive(CmdEmpty, nil, d.timeout)
	return confirm, err
}

func (d *Device) ValidTemplates() (uint16, error) {
	data, confirm, err := d.transceive(CmdValidTemplates, nil, time.Second)
	if err != nil {
		return 0, err
	}
	if confirm != ConfirmOK || len(data) < 2 {
		return 0, Code(confirm)
	}
	return uint16(data[0])<<8 | uint16(data[1]), nil
}

func (d *Device) ReadIndexTable(page uint8) ([]byte, error) {
	data, confirm, err := d.transceive(CmdReadIndexTable, []byte{page}, time.Second)
	if err != nil {
		return nil, err
	}
	if confirm != ConfirmOK || len(data) < 32 {
		return nil, Code(confirm)
	}
	return append([]byte(nil), data[:32]...), nil
}

func (d *Device) FirstFreePage() (uint16, error) {
	data, err := d.ReadIndexTable(0)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 32; i++ {
		for bit := 0; bit < 8; bit++ {
			if data[i]&(1<<bit) == 0 {
				page := uint16(i*8 + bit)
				if page < d.Capacity {
					return page, nil
				}
			}
		}
	}
	return 0, ErrNoSpace
}

func (d *Device) ImageInfo() (area, quality uint8, confirm uint8, err error) {
	data, confirm, err := d.transceive(CmdImageInfo, nil, time.Second)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(data) >= 2 {
		area, quality = data[0], data[1]
	}
	return area, quality, confirm, nil
}

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
	return d.readData(imageBytes, 10*time.Second)
}

func (d *Device) UploadTemplate(offset, size uint16) ([]byte, uint16, uint8, error) {
	data, confirm, err := d.transceive(CmdUploadTemplate, []byte{byte(offset >> 8), byte(offset), byte(size >> 8), byte(size)}, 10*time.Second)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(data) < 2 {
		return nil, 0, confirm, Code(confirm)
	}
	actual := uint16(data[0])<<8 | uint16(data[1])
	if len(data) > 2 {
		return append([]byte(nil), data[2:]...), actual, confirm, nil
	}
	return nil, actual, confirm, nil
}

func (d *Device) DownloadTemplate(offset uint16, data []byte) (uint8, error) {
	payload := make([]byte, 4+len(data))
	payload[0] = byte(offset >> 8)
	payload[1] = byte(offset)
	payload[2] = byte(len(data) >> 8)
	payload[3] = byte(len(data))
	copy(payload[4:], data)
	_, confirm, err := d.transceive(CmdDownloadTmpl, payload, 10*time.Second)
	return confirm, err
}

func (d *Device) UploadTemplateAuto(size uint16) ([]byte, error) {
	out := make([]byte, 0, size)
	offset := 0
	for offset < int(size) {
		want := uint16(128)
		if remaining := int(size) - offset; remaining < 128 {
			want = uint16(remaining)
		}
		data, actual, confirm, err := d.UploadTemplate(uint16(offset), want)
		if err != nil {
			return out, err
		}
		if confirm != ConfirmOK {
			return out, Code(confirm)
		}
		out = append(out, data...)
		if actual == 0 {
			break
		}
		offset += int(actual)
	}
	return out, nil
}

func (d *Device) DownloadTemplateAuto(data []byte) error {
	for offset := 0; offset < len(data); {
		n := 100
		if len(data)-offset < n {
			n = len(data) - offset
		}
		confirm, err := d.DownloadTemplate(uint16(offset), data[offset:offset+n])
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

func (d *Device) ReadNotepad(id uint8) ([]byte, error) {
	data, confirm, err := d.transceive(CmdReadNotepad, []byte{id}, time.Second)
	if err != nil {
		return nil, err
	}
	if confirm != ConfirmOK || len(data) < 32 {
		return nil, Code(confirm)
	}
	return append([]byte(nil), data[:32]...), nil
}

func (d *Device) WriteNotepad(id uint8, data []byte) (uint8, error) {
	payload := make([]byte, 1+32)
	payload[0] = id
	copy(payload[1:], data)
	_, confirm, err := d.transceive(CmdWriteNotepad, payload, time.Second)
	return confirm, err
}

func (d *Device) SetLED(mode, color, endColor, cycle uint8) error {
	return d.ControlLED(mode, color, endColor, cycle)
}

func (d *Device) RefreshTemplates() {
	if n, err := d.ValidTemplates(); err == nil {
		d.Templates = n
	}
}

func (d *Device) ControlLED(mode, color, endColor, cycle uint8) error {
	_, confirm, err := d.transceive(CmdControlLED, []byte{mode, color, endColor, cycle}, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

func (d *Device) ControlLEDRainbow(colors []uint8, cycle uint8) error {
	params := make([]byte, 8)
	params[0] = LEDRainbow
	params[1] = 0
	for i := 0; i < 10; i++ {
		var c uint8
		if i&1 == 0 {
			if i < len(colors) {
				c = colors[i]<<4 | 0x80
			}
		} else {
			if i < len(colors) {
				c |= colors[i] | 0x08
			}
			params[2+i/2] = c
		}
	}
	params[7] = cycle
	_, confirm, err := d.transceive(CmdControlLED, params, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

func (d *Device) Cancel() error {
	_, confirm, err := d.transceive(CmdCancel, nil, time.Second)
	if err != nil {
		return err
	}
	if confirm != ConfirmOK {
		return Code(confirm)
	}
	return nil
}

// AutoEnroll stages reported by the module in the Param1 field.
const (
	EnrollStageLegalCheck = 0x00
	EnrollStageGetImage   = 0x01
	EnrollStageGenChar    = 0x02
	EnrollStageLiftFinger = 0x03
	EnrollStageMerge      = 0x04
	EnrollStageVerify     = 0x05
	EnrollStageStore      = 0x06
)

// AutoEnroll flags.
const (
	AutoEnrollDefault        = 0x0000
	AutoEnrollNoStatus       = 1 << 2
	AutoEnrollOverwrite      = 1 << 3
	AutoEnrollForbidDup      = 1 << 4
	AutoEnrollNoLiftRequired = 1 << 5
)

// EnrollProgress is one status packet emitted during AutoEnroll.
// Stage is the Param1 value (EnrollStage*) and Param the Param2 value.
type EnrollProgress struct {
	Confirm uint8
	Stage   uint8
	Param   uint8
}

// StartAutoEnroll asks the module to run its own enrollment state machine.
// Afterwards call NextEnrollEvent until it reports EnrollStageStore.
func (d *Device) StartAutoEnroll(page uint16, count uint8, flags uint16) error {
	payload := []byte{byte(page >> 8), byte(page), count, byte(flags >> 8), byte(flags)}
	d.flush()
	return d.send(CmdAutoEnroll, payload)
}

// NextEnrollEvent waits for the next AutoEnroll status packet.
func (d *Device) NextEnrollEvent(timeout time.Duration) (EnrollProgress, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return EnrollProgress{}, ErrTimeout
		}
		pkt, err := d.readPacket(remaining)
		if err != nil {
			return EnrollProgress{}, err
		}
		if pkt[6] != pidAck {
			continue
		}
		confirm, data := ackData(pkt)
		p := EnrollProgress{Confirm: confirm}
		if len(data) >= 2 {
			p.Stage = data[0]
			p.Param = data[1]
		}
		return p, nil
	}
}

// AutoEnroll performs enrollment with up to count captures.
// The callback receives every intermediate progress; return false to stop.
func (d *Device) AutoEnroll(page uint16, count uint8, flags uint16, cb func(p EnrollProgress) bool) (uint8, error) {
	if err := d.StartAutoEnroll(page, count, flags); err != nil {
		return 0, err
	}
	last := uint8(0xFF)
	for {
		p, err := d.NextEnrollEvent(30 * time.Second)
		if err != nil {
			return last, err
		}
		last = p.Confirm
		if cb != nil && !cb(p) {
			_ = d.Cancel()
			return p.Confirm, nil
		}
		if p.Stage == EnrollStageStore {
			return p.Confirm, nil
		}
	}
}

// AutoIdentify runs continuous identification.
func (d *Device) AutoIdentify(security uint8, id uint16, flags uint16, cb func(confirm uint8, data []byte) bool) (uint8, error) {
	payload := []byte{security, byte(id >> 8), byte(id), byte(flags >> 8), byte(flags)}
	d.flush()
	if err := d.send(CmdAutoIdentify, payload); err != nil {
		return 0, err
	}
	deadline := time.Now().Add(60 * time.Second)
	last := uint8(0xFF)
	for time.Now().Before(deadline) {
		pkt, err := d.readAck(2 * time.Second)
		if err != nil {
			return last, err
		}
		confirm, data := ackData(pkt)
		last = confirm
		if cb != nil && !cb(confirm, data) {
			_ = d.Cancel()
			return confirm, nil
		}
		if confirm == ConfirmOK {
			return confirm, nil
		}
	}
	return last, ErrTimeout
}
