package finger2

import (
	"errors"
	"sync"

	"m5stick-obd2-cnx/lib/finger2/internal/wire"
)

var errFakeEmpty = errors.New("fake transport: empty")

// fakeTransport implements Transport for host tests. The handler receives each
// command and returns the ACK confirmation and data.
type fakeTransport struct {
	mu      sync.Mutex
	rx      []byte
	handler func(cmd byte, payload []byte) (confirm byte, data []byte)
}

func (f *fakeTransport) Write(p []byte) (int, error) {
	if len(p) < 12 {
		return len(p), nil
	}
	cmd := p[9]
	payload := p[10 : len(p)-2]

	confirm, data := byte(ConfirmOK), []byte(nil)
	if f.handler != nil {
		confirm, data = f.handler(cmd, payload)
	}
	ack := buildPacket(wire.PIDAck, append([]byte{confirm}, data...))

	f.mu.Lock()
	f.rx = append(f.rx, ack...)
	f.mu.Unlock()
	return len(p), nil
}

func (f *fakeTransport) ReadByte() (byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.rx) == 0 {
		return 0, errFakeEmpty
	}
	b := f.rx[0]
	f.rx = f.rx[1:]
	return b, nil
}

func (f *fakeTransport) Buffered() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rx)
}

func buildPacket(pid byte, payload []byte) []byte {
	pkt := make([]byte, 11+len(payload))
	pkt[0] = wire.Header0
	pkt[1] = wire.Header1
	pkt[2], pkt[3], pkt[4], pkt[5] = 0xFF, 0xFF, 0xFF, 0xFF
	pkt[6] = pid
	length := len(payload) + 2
	pkt[7] = byte(length >> 8)
	pkt[8] = byte(length)
	copy(pkt[9:], payload)
	sum := wire.Checksum(pkt[6 : len(pkt)-2])
	pkt[len(pkt)-2] = byte(sum >> 8)
	pkt[len(pkt)-1] = byte(sum)
	return pkt
}

func sysParamsData() []byte {
	return []byte{
		0x00, 0x0C, // status
		0x00, 0x00, // template size
		0x00, 100, // capacity
		0x00, 5, // match level
		0xFF, 0xFF, 0xFF, 0xFF, // address
		0x00, 2, // packet size
		0x00, 6, // baud rate
	}
}

func defaultHandler(cmd byte, payload []byte) (byte, []byte) {
	switch cmd {
	case CmdFirmware:
		return ConfirmOK, []byte{1}
	case CmdModuleStatus:
		return ConfirmOK, []byte{0x0C}
	case CmdReadSysPara:
		return ConfirmOK, sysParamsData()
	case CmdValidTemplates:
		return ConfirmOK, []byte{0x00, 2}
	}
	return ConfirmOK, nil
}
