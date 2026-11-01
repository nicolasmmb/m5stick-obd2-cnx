// Package wire implements the A-K323CP packet framing.
//
// It has no hardware dependencies, so it can be unit tested on any host with
// "go test ./lib/finger2/internal/wire".
package wire

const (
	Header0 = 0xEF
	Header1 = 0x01

	PIDCommand = 0x01
	PIDData    = 0x02
	PIDAck     = 0x07
	PIDEndData = 0x08
)

// WakeupPattern is the fixed ACK sent when a finger wakes the sleeping module.
var WakeupPattern = [12]byte{0xEF, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x07, 0x00, 0x03, 0xFF, 0x01, 0x09}

// Checksum returns the 16-bit sum the module expects over the PID, length and
// payload bytes.
func Checksum(b []byte) uint16 {
	var sum uint16
	for _, v := range b {
		sum += uint16(v)
	}
	return sum
}

// BuildCommand frames a command packet:
//
//	EF 01 | address(4) | 01 | length(2) | cmd | payload | checksum(2)
func BuildCommand(address uint32, cmd byte, payload []byte) []byte {
	pkt := make([]byte, 12+len(payload))
	return BuildCommandInto(pkt, address, cmd, payload)
}

// BuildCommandInto frames a command packet into pkt, which must have at least
// 12+len(payload) bytes. It returns the used slice, or nil if pkt is too small.
func BuildCommandInto(pkt []byte, address uint32, cmd byte, payload []byte) []byte {
	n := 12 + len(payload)
	if len(pkt) < n {
		return nil
	}
	pkt = pkt[:n]
	pkt[0] = Header0
	pkt[1] = Header1
	pkt[2] = byte(address >> 24)
	pkt[3] = byte(address >> 16)
	pkt[4] = byte(address >> 8)
	pkt[5] = byte(address)
	pkt[6] = PIDCommand
	length := 3 + len(payload) // cmd + payload + checksum
	pkt[7] = byte(length >> 8)
	pkt[8] = byte(length)
	pkt[9] = cmd
	copy(pkt[10:], payload)
	sum := Checksum(pkt[6 : len(pkt)-2])
	pkt[len(pkt)-2] = byte(sum >> 8)
	pkt[len(pkt)-1] = byte(sum)
	return pkt
}

// AckData splits an ACK packet into its confirmation byte and data.
// It returns (0xFF, nil) for packets shorter than the minimum ACK size.
func AckData(pkt []byte) (confirm byte, data []byte) {
	if len(pkt) < 12 {
		return 0xFF, nil
	}
	return pkt[9], pkt[10 : len(pkt)-2]
}

// IsWakeup reports whether pkt is the fixed wakeup packet.
func IsWakeup(pkt []byte) bool {
	if len(pkt) != len(WakeupPattern) {
		return false
	}
	for i := range WakeupPattern {
		if pkt[i] != WakeupPattern[i] {
			return false
		}
	}
	return true
}
