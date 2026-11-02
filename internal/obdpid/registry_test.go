package obdpid

import "testing"

func TestRegistryDecode(t *testing.T) {
	reg := DefaultRegistry()
	r, err := reg.Decode("0C", []byte{0x1A, 0xF8})
	if err != nil {
		t.Fatal(err)
	}
	if r.PID != "0C" || r.Value != 1726 || r.Unit != "rpm" || r.Quality != QualityOK {
		t.Fatalf("unexpected reading: %+v", r)
	}
}

func TestRegistryUnsupported(t *testing.T) {
	reg := DefaultRegistry()
	r, err := reg.Decode("FF", []byte{0x00})
	if err != ErrUnknownPID {
		t.Fatalf("err = %v, want ErrUnknownPID", err)
	}
	if r.Quality != QualityUnsupported {
		t.Fatalf("quality = %v, want unsupported", r.Quality)
	}
}
