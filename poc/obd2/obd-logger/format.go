package main

// numFmt formats small numbers on the stack. The UI redraws several tiles per
// second, so it must not call strconv/String concatenation in that path.
type numFmt struct {
	b [24]byte
	n int
}

func (w *numFmt) add(s string) {
	for i := 0; i < len(s) && w.n < len(w.b); i++ {
		w.b[w.n] = s[i]
		w.n++
	}
}

func (w *numFmt) addInt(v int) {
	if v < 0 {
		w.b[w.n] = '-'
		w.n++
		v = -v
	}
	w.addUint(uint64(v))
}

// addFixed formats v with dec decimal digits, rounding half up.
func (w *numFmt) addFixed(v float64, dec int) {
	scale := 1
	for i := 0; i < dec; i++ {
		scale *= 10
	}
	if v < 0 {
		w.b[w.n] = '-'
		w.n++
		v = -v
	}
	scaled := uint64(v*float64(scale) + 0.5)
	whole := scaled / uint64(scale)
	frac := scaled % uint64(scale)
	w.addUint(whole)
	if dec > 0 {
		w.b[w.n] = '.'
		w.n++
		div := uint64(scale) / 10
		for i := 0; i < dec; i++ {
			w.b[w.n] = byte('0' + (frac/div)%10)
			w.n++
			div /= 10
		}
	}
}

func (w *numFmt) addUint(v uint64) {
	var tmp [20]byte
	i := len(tmp)
	if v == 0 {
		i--
		tmp[i] = '0'
	}
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	for ; i < len(tmp); i++ {
		w.b[w.n] = tmp[i]
		w.n++
	}
}

func (w *numFmt) str() string {
	return string(w.b[:w.n])
}

// num renders an integer.
func num(v int) string {
	var w numFmt
	w.addInt(v)
	return w.str()
}

// num1 renders a float with one decimal place.
func num1(v float64) string {
	var w numFmt
	w.addFixed(v, 1)
	return w.str()
}

// num2 renders a float with two decimal places.
func num2(v float64) string {
	var w numFmt
	w.addFixed(v, 2)
	return w.str()
}

// numS renders an integer followed by a short suffix, e.g. "45%".
func numS(v int, suffix string) string {
	var w numFmt
	w.addInt(v)
	w.add(suffix)
	return w.str()
}

// num1S renders one decimal followed by a short suffix, e.g. "3.2 L".
func num1S(v float64, suffix string) string {
	var w numFmt
	w.addFixed(v, 1)
	w.add(suffix)
	return w.str()
}
