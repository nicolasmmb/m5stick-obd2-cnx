package obdwire

import (
	"fmt"
	"strconv"
	"strings"
)

type Kind uint8

const (
	KindAck Kind = iota
	KindReplay
	KindLive
	KindClear
	KindRecord
)

type RecordType uint8

const (
	RecordStart RecordType = iota
	RecordPID
	RecordEvent
	RecordEnd
)

type Record struct {
	Type    RecordType
	Ms      int64
	PID     string
	Raw     string
	Latency int
	Quality string
	Payload string
}

type Command struct {
	Kind   Kind
	Record Record
}

func ParseCommand(line string) (Command, error) {
	line = strings.TrimSpace(line)
	switch line {
	case "A":
		return Command{Kind: KindAck}, nil
	case "C,REPLAY":
		return Command{Kind: KindReplay}, nil
	case "C,LIVE":
		return Command{Kind: KindLive}, nil
	case "C,CLEAR":
		return Command{Kind: KindClear}, nil
	}
	if strings.HasPrefix(line, "R,") {
		rec, err := ParseRecord(line[2:])
		if err != nil {
			return Command{}, err
		}
		return Command{Kind: KindRecord, Record: rec}, nil
	}
	return Command{}, fmt.Errorf("unknown command %q", line)
}

func ParseRecord(line string) (Record, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Record{}, fmt.Errorf("empty record")
	}
	switch line[0] {
	case 'I':
		parts := strings.SplitN(line, ",", 5)
		if len(parts) < 5 {
			return Record{}, fmt.Errorf("invalid I record")
		}
		return Record{Type: RecordStart, Payload: line}, nil
	case 'P':
		parts := strings.SplitN(line, ",", 6)
		if len(parts) < 6 {
			return Record{}, fmt.Errorf("invalid P record")
		}
		ms, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return Record{}, err
		}
		lat, _ := strconv.Atoi(parts[4])
		return Record{
			Type: RecordPID, Ms: ms, PID: strings.ToUpper(parts[2]),
			Raw: strings.ToUpper(parts[3]), Latency: lat,
			Quality: strings.ToUpper(parts[5]), Payload: line,
		}, nil
	case 'E':
		parts := strings.SplitN(line, ",", 4)
		if len(parts) < 4 {
			return Record{}, fmt.Errorf("invalid E record")
		}
		ms, _ := strconv.ParseInt(parts[1], 10, 64)
		return Record{Type: RecordEvent, Ms: ms, Payload: line}, nil
	case 'X':
		parts := strings.SplitN(line, ",", 3)
		if len(parts) < 2 {
			return Record{}, fmt.Errorf("invalid X record")
		}
		ms, _ := strconv.ParseInt(parts[1], 10, 64)
		return Record{Type: RecordEnd, Ms: ms, Payload: line}, nil
	}
	return Record{}, fmt.Errorf("unknown record %q", line[0])
}

func FormatRecord(r Record) string {
	if r.Payload != "" {
		return r.Payload
	}
	switch r.Type {
	case RecordPID:
		return "P," + strconv.FormatInt(r.Ms, 10) + "," + r.PID + "," +
			r.Raw + "," + strconv.Itoa(r.Latency) + "," + r.Quality
	case RecordEvent:
		return "E," + strconv.FormatInt(r.Ms, 10) + ",EVENT,"
	case RecordEnd:
		return "X," + strconv.FormatInt(r.Ms, 10) + ",end"
	default:
		return "I,0,0.1,,440"
	}
}
