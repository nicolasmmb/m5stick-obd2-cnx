package obdpid

import "time"

type Registry struct {
	decoders map[string]Decoder
}

func NewRegistry(table map[string]Decoder) *Registry {
	if table == nil {
		table = Table()
	}
	return &Registry{decoders: table}
}

func DefaultRegistry() *Registry {
	return NewRegistry(Table())
}

func (r *Registry) Get(pid string) (Decoder, bool) {
	d, ok := r.decoders[pid]
	return d, ok
}

func (r *Registry) Supported(pid string) bool {
	_, ok := r.decoders[pid]
	return ok
}

func (r *Registry) All() []Decoder {
	out := make([]Decoder, 0, len(r.decoders))
	for _, d := range r.decoders {
		out = append(out, d)
	}
	return out
}

func (r *Registry) Decode(pid string, raw []byte) (Reading, error) {
	d, ok := r.decoders[pid]
	if !ok {
		return Reading{
			PID: pid, Raw: raw, Quality: QualityUnsupported,
			At: time.Now(),
		}, ErrUnknownPID
	}
	v, err := d.Decode(raw)
	if err != nil {
		return Reading{
			PID: pid, Raw: raw, Quality: QualityInvalid,
			At: time.Now(),
		}, err
	}
	return Reading{
		PID: pid, Raw: raw, Value: v, Unit: d.Unit(),
		Quality: QualityOK, At: time.Now(),
	}, nil
}
