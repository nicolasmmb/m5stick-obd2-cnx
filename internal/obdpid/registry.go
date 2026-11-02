package obdpid

import "time"

type Registry struct {
	decoders map[uint16]Decoder
}

func NewRegistry(table map[string]Decoder) *Registry {
	if table == nil {
		table = Table()
	}
	m := make(map[uint16]Decoder, len(table))
	for pid, d := range table {
		m[pidKey(pid)] = d
	}
	return &Registry{decoders: m}
}

func DefaultRegistry() *Registry {
	return NewRegistry(Table())
}

func (r *Registry) Get(pid string) (Decoder, bool) {
	d, ok := r.decoders[pidKey(pid)]
	return d, ok
}

func (r *Registry) Supported(pid string) bool {
	_, ok := r.decoders[pidKey(pid)]
	return ok
}

func (r *Registry) All() []Decoder {
	out := make([]Decoder, 0, len(r.decoders))
	for _, d := range r.decoders {
		out = append(out, d)
	}
	return out
}

// Decode resolves a string PID. The returned Reading uses the registry's
// canonical PID string, not the caller's buffer.
func (r *Registry) Decode(pid string, raw []byte) (Reading, error) {
	return r.decode(pidKey(pid), raw)
}

// DecodeBytes is the allocation-free entry point used by the firmware, where
// PIDs arrive as two ASCII bytes from the serial protocol.
func (r *Registry) DecodeBytes(pid [2]byte, raw []byte) (Reading, error) {
	key := uint16(pid[0])<<8 | uint16(pid[1])
	return r.decode(key, raw)
}

func (r *Registry) decode(key uint16, raw []byte) (Reading, error) {
	d, ok := r.decoders[key]
	if !ok {
		return Reading{Quality: QualityUnsupported, At: time.Now()}, ErrUnknownPID
	}
	v, err := d.Decode(raw)
	if err != nil {
		return Reading{
			PID: d.PID(), Quality: QualityInvalid, At: time.Now(),
		}, err
	}
	return Reading{
		PID: d.PID(), Value: v, Unit: d.Unit(),
		Quality: QualityOK, At: time.Now(),
	}, nil
}

func pidKey(pid string) uint16 {
	var k uint16
	for i := 0; i < len(pid) && i < 2; i++ {
		k = k<<8 | uint16(pid[i])
	}
	return k
}
