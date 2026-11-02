package obdlog

import "testing"

func TestParseLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		typ  string
	}{
		{"start", `{"t":"S","v":1,"sid":"s1","vin":"ABC","tank_dl":440}`, "S"},
		{"sample", `{"t":"P","seq":1,"ms":120,"pid":"0c","raw":"1AF8","v":1726,"u":"rpm","q":"OK","lat":138}`, "P"},
		{"event", `{"t":"E","ms":1000,"type":"dtc","code":"P0133"}`, "E"},
		{"end", `{"t":"X","ms":2000,"reason":"user"}`, "X"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := ParseLine([]byte(tt.in))
			if err != nil {
				t.Fatalf("ParseLine: %v", err)
			}
			if rec.Type != tt.typ {
				t.Fatalf("type = %q, want %q", rec.Type, tt.typ)
			}
			if rec.Type == "P" && rec.Sample.PID != "0C" {
				t.Fatalf("pid = %q, want 0C", rec.Sample.PID)
			}
		})
	}
}

func TestParseCompactLine(t *testing.T) {
	rec, err := ParseCompactLine("P,1234,0C,1AF8,138,OK")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Type != "P" || rec.Sample.PID != "0C" || rec.Sample.Value != 1726 || rec.Sample.Unit != "rpm" {
		t.Fatalf("unexpected record: %+v", rec.Sample)
	}

	rec, err = ParseCompactLine("I,0,0.1,VIN123,440")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Type != "S" || rec.Start.VIN != "VIN123" || rec.Start.TankDL != 440 {
		t.Fatalf("unexpected start: %+v", rec.Start)
	}

	rec, err = ParseCompactLine("E,5000,TIMEOUT,0C")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Type != "E" || rec.Event.Type != "TIMEOUT" {
		t.Fatalf("unexpected event: %+v", rec.Event)
	}
}

func TestDecodePID(t *testing.T) {
	tests := []struct {
		pid  string
		raw  string
		want float64
		unit string
	}{
		{"0C", "1AF8", 1726, "rpm"},
		{"0D", "0A", 10, "km/h"},
		{"05", "73", 75, "C"},
		{"11", "50", 31.372549019607842, "%"},
		{"42", "30FB", 12.539, "V"},
		{"3C", "1554", 506, "C"},
		{"44", "6D00", 0.8515625, "lambda"},
	}
	for _, tt := range tests {
		got, unit, ok := DecodePID(tt.pid, tt.raw)
		if !ok || unit != tt.unit || got != tt.want {
			t.Fatalf("DecodePID(%s,%s) = %v %q %v, want %v %q", tt.pid, tt.raw, got, unit, ok, tt.want, tt.unit)
		}
	}
}

func TestParseLineInvalid(t *testing.T) {
	for _, in := range []string{
		``,
		`not json`,
		`{"t":"P","ms":1,"v":1,"u":"rpm","q":"OK"}`,
		`{"t":"P","pid":"0C","ms":1,"v":1,"q":"OK"}`,
		`{"t":"Z"}`,
	} {
		if _, err := ParseLine([]byte(in)); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}
