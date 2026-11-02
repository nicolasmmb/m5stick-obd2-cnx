package obdlog

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoaderReplaySample(t *testing.T) {
	src, err := os.Open("../../testdata/obd-logger-sample.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	l, err := NewLoader(Options{OutDir: t.TempDir(), SessionID: "test-session"})
	if err != nil {
		t.Fatal(err)
	}

	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		if err := l.ProcessLine(scanner.Bytes(), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	readCSV := func(name string) [][]string {
		f, err := os.Open(filepath.Join(l.Dir(), name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		rows, err := csv.NewReader(f).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	long := readCSV("samples_long.csv")
	if len(long) != 15 { // header + 14 samples
		t.Fatalf("long rows = %d, want 15", len(long))
	}
	if long[0][4] != "pid" || long[1][4] != "0C" {
		t.Fatalf("unexpected long header/row: %v / %v", long[0], long[1])
	}

	wide := readCSV("samples_wide_1hz.csv")
	if len(wide) != 4 { // header + seconds 1000, 2000, 2500
		t.Fatalf("wide rows = %d, want 4", len(wide))
	}
	if wide[0][2] != "rpm" || wide[0][3] != "rpm_age_ms" {
		t.Fatalf("unexpected wide header: %v", wide[0])
	}

	raw, err := os.ReadFile(filepath.Join(l.Dir(), "raw.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(splitLines(raw)); got != 17 {
		t.Fatalf("raw lines = %d, want 17", got)
	}

	metaBytes, err := os.ReadFile(filepath.Join(l.Dir(), "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta Metadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Samples != 14 || meta.Events != 1 || meta.TankDL != 440 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if meta.VIN != "9BGEA48A0LG185881" {
		t.Fatalf("vin = %q", meta.VIN)
	}
}

func TestLoaderReplayCompact(t *testing.T) {
	src, err := os.Open("../../testdata/obd-logger-sample-compact.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	l, err := NewLoader(Options{OutDir: t.TempDir(), SessionID: "compact"})
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		if err := l.ProcessLine(scanner.Bytes(), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	decoded, err := os.ReadFile(filepath.Join(l.Dir(), "decoded.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(splitLines(decoded)); got != 17 {
		t.Fatalf("decoded lines = %d, want 17", got)
	}
	raw, err := os.ReadFile(filepath.Join(l.Dir(), "raw.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(splitLines(raw)); got != 17 {
		t.Fatalf("raw lines = %d, want 17", got)
	}
	long, err := os.ReadFile(filepath.Join(l.Dir(), "samples_long.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(splitLines(long)); got != 15 {
		t.Fatalf("long lines = %d, want 15", got)
	}
	if !bytes.Contains(decoded, []byte(`"v":1726`)) {
		t.Fatalf("decoded sample missing decoded value: %s", decoded)
	}
}

func splitLines(b []byte) []string {
	var out []string
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				out = append(out, string(b[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, string(b[start:]))
	}
	return out
}
