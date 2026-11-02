package obdpid

// U8 decodes one unsigned byte: value = A*Scale + Offset.
type U8 struct {
	ID        string
	Label     string
	UnitLabel string
	Scale     float64
	Offset    float64
}

func (d U8) PID() string  { return d.ID }
func (d U8) Name() string { return d.Label }
func (d U8) Unit() string { return d.UnitLabel }

func (d U8) Decode(raw []byte) (float64, error) {
	if len(raw) < 1 {
		return 0, ErrInvalid
	}
	return float64(raw[0])*d.Scale + d.Offset, nil
}

// U16 decodes two big-endian bytes: value = (256*A+B)*Scale + Offset.
type U16 struct {
	ID        string
	Label     string
	UnitLabel string
	Scale     float64
	Offset    float64
}

func (d U16) PID() string  { return d.ID }
func (d U16) Name() string { return d.Label }
func (d U16) Unit() string { return d.UnitLabel }

func (d U16) Decode(raw []byte) (float64, error) {
	if len(raw) < 2 {
		return 0, ErrInvalid
	}
	return float64(int(raw[0])*256+int(raw[1]))*d.Scale + d.Offset, nil
}

// Signed8 decodes a signed byte centered at 128:
// value = (A-128)*Scale + Offset.
type Signed8 struct {
	ID        string
	Label     string
	UnitLabel string
	Scale     float64
	Offset    float64
}

func (d Signed8) PID() string  { return d.ID }
func (d Signed8) Name() string { return d.Label }
func (d Signed8) Unit() string { return d.UnitLabel }

func (d Signed8) Decode(raw []byte) (float64, error) {
	if len(raw) < 1 {
		return 0, ErrInvalid
	}
	return (float64(raw[0])-128)*d.Scale + d.Offset, nil
}

// Enum8 decodes an enumeration byte.
type Enum8 struct {
	ID        string
	Label     string
	Values    map[byte]string
	UnitLabel string
}

func (d Enum8) PID() string  { return d.ID }
func (d Enum8) Name() string { return d.Label }
func (d Enum8) Unit() string { return d.UnitLabel }

func (d Enum8) Decode(raw []byte) (float64, error) {
	if len(raw) < 1 {
		return 0, ErrInvalid
	}
	return float64(raw[0]), nil
}

// Bitmask8 decodes a bit field as a number.
type Bitmask8 struct {
	ID        string
	Label     string
	UnitLabel string
}

func (d Bitmask8) PID() string  { return d.ID }
func (d Bitmask8) Name() string { return d.Label }
func (d Bitmask8) Unit() string { return d.UnitLabel }

func (d Bitmask8) Decode(raw []byte) (float64, error) {
	if len(raw) < 1 {
		return 0, ErrInvalid
	}
	return float64(raw[0]), nil
}
