package finger2

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func newFake(h func(cmd byte, payload []byte) (byte, []byte)) *Device {
	d, _ := newFakeT(h)
	return d
}

func newFakeT(h func(cmd byte, payload []byte) (byte, []byte)) (*Device, *fakeTransport) {
	f := &fakeTransport{handler: h}
	return newDevice(f), f
}

func TestInitReadsFirmwareAndParams(t *testing.T) {
	d := newFake(defaultHandler)
	if err := d.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if d.Firmware != 1 {
		t.Fatalf("firmware = %d, want 1", d.Firmware)
	}
	if d.Capacity != 100 {
		t.Fatalf("capacity = %d, want 100", d.Capacity)
	}
	if d.Templates != 2 {
		t.Fatalf("templates = %d, want 2", d.Templates)
	}
	if d.MatchLevel != 5 {
		t.Fatalf("match level = %d, want 5", d.MatchLevel)
	}
}

func TestInitDoesNotForceMatchLevel(t *testing.T) {
	d, f := newFakeT(defaultHandler)
	if err := d.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for _, cmd := range f.sent() {
		if cmd == CmdWriteReg {
			t.Fatal("Init sent CmdWriteReg; it must not change the match level")
		}
	}
	if d.MatchLevel != 5 {
		t.Fatalf("match level = %d, want the value from ReadSysParams (5)", d.MatchLevel)
	}
}

func TestInitWithTimeout(t *testing.T) {
	d := newDevice(&fakeTransport{silent: true})
	start := time.Now()
	err := d.InitWithTimeout(100 * time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("InitWithTimeout returned nil for a silent module")
	}
	if elapsed > time.Second {
		t.Fatalf("InitWithTimeout took %v, want about 100ms", elapsed)
	}
}

func TestReadSysParamsFields(t *testing.T) {
	d := newFake(defaultHandler)
	p, err := d.ReadSysParams()
	if err != nil {
		t.Fatalf("ReadSysParams: %v", err)
	}
	if p.Status != 0x000C || p.Capacity != 100 || p.MatchLevel != 5 {
		t.Fatalf("unexpected params: %+v", p)
	}
	if p.PacketSize != 2 || p.BaudRate != 6 || p.Address != 0xFFFFFFFF {
		t.Fatalf("unexpected params: %+v", p)
	}
}

func TestGetSNReturnsCopy(t *testing.T) {
	sn := bytes.Repeat([]byte{0xAA}, 32)
	d := newFake(func(cmd byte, payload []byte) (byte, []byte) {
		if cmd == CmdGetSN {
			return ConfirmOK, sn
		}
		return defaultHandler(cmd, payload)
	})

	got, err := d.GetSN()
	if err != nil {
		t.Fatalf("GetSN: %v", err)
	}
	// Any command reuses the read scratch. A returned copy must survive it.
	if _, err := d.ReadSysParams(); err != nil {
		t.Fatalf("ReadSysParams: %v", err)
	}
	if !bytes.Equal(got, sn) {
		t.Fatalf("GetSN aliased the scratch: % X", got)
	}
}

func TestReadNotepadReturnsCopy(t *testing.T) {
	note := make([]byte, 32)
	for i := range note {
		note[i] = byte(i + 1)
	}
	d := newFake(func(cmd byte, payload []byte) (byte, []byte) {
		if cmd == CmdReadNotepad {
			return ConfirmOK, note
		}
		return defaultHandler(cmd, payload)
	})

	got, err := d.ReadNotepad(1)
	if err != nil {
		t.Fatalf("ReadNotepad: %v", err)
	}
	if _, err := d.ReadSysParams(); err != nil {
		t.Fatalf("ReadSysParams: %v", err)
	}
	if !bytes.Equal(got, note) {
		t.Fatalf("ReadNotepad aliased the scratch: % X", got)
	}
}

func TestReadIndexTableReturnsCopy(t *testing.T) {
	table := bytes.Repeat([]byte{0x55}, 32)
	d := newFake(func(cmd byte, payload []byte) (byte, []byte) {
		if cmd == CmdReadIndexTable {
			return ConfirmOK, table
		}
		return defaultHandler(cmd, payload)
	})

	got, err := d.ReadIndexTable(0)
	if err != nil {
		t.Fatalf("ReadIndexTable: %v", err)
	}
	if _, err := d.ReadSysParams(); err != nil {
		t.Fatalf("ReadSysParams: %v", err)
	}
	if !bytes.Equal(got, table) {
		t.Fatalf("ReadIndexTable aliased the scratch: % X", got)
	}
}

func TestUploadTemplateReturnsCopy(t *testing.T) {
	tpl := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	d := newFake(func(cmd byte, payload []byte) (byte, []byte) {
		if cmd == CmdUploadTemplate {
			// ACK data is actualSize(2) + template bytes.
			return ConfirmOK, append([]byte{0x00, byte(len(tpl))}, tpl...)
		}
		return defaultHandler(cmd, payload)
	})

	got, actual, _, err := d.UploadTemplate(0, uint16(len(tpl)))
	if err != nil {
		t.Fatalf("UploadTemplate: %v", err)
	}
	if actual != uint16(len(tpl)) {
		t.Fatalf("actual = %d, want %d", actual, len(tpl))
	}
	if _, err := d.ReadSysParams(); err != nil {
		t.Fatalf("ReadSysParams: %v", err)
	}
	if !bytes.Equal(got, tpl) {
		t.Fatalf("UploadTemplate aliased the scratch: % X", got)
	}
}

func TestSearchParsesResult(t *testing.T) {
	d := newFake(func(cmd byte, payload []byte) (byte, []byte) {
		if cmd == CmdSearch {
			return ConfirmOK, []byte{0x00, 0x03, 0x01, 0x2C}
		}
		return defaultHandler(cmd, payload)
	})

	res, err := d.Search(1, 0, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !res.Matched || res.Page != 3 || res.Score != 300 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestCaptureNoFinger(t *testing.T) {
	d := newFake(func(cmd byte, payload []byte) (byte, []byte) {
		if cmd == CmdGetImage {
			return ConfirmNoFinger, nil
		}
		return defaultHandler(cmd, payload)
	})

	detected, confirm, err := d.Capture()
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if detected || confirm != ConfirmNoFinger {
		t.Fatalf("detected=%v confirm=%#02x, want false/0x02", detected, confirm)
	}
}

func TestConcurrentCommands(t *testing.T) {
	d, _ := newFakeT(defaultHandler)
	const goroutines, iterations = 8, 25
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			for i := 0; i < iterations; i++ {
				p, err := d.ReadSysParams()
				if err != nil {
					errs <- err
					return
				}
				if p.Capacity != 100 {
					errs <- ErrShortReply
					return
				}
			}
			errs <- nil
		}()
	}
	for g := 0; g < goroutines; g++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent ReadSysParams: %v", err)
		}
	}
}

func TestInitContextCanceled(t *testing.T) {
	d, _ := newFakeT(defaultHandler)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := d.InitContext(ctx, time.Second); err == nil {
		t.Fatal("InitContext with a canceled context returned nil")
	}
}

func TestSessionBlocksOneShotUntilCancel(t *testing.T) {
	d, _ := newFakeT(defaultHandler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.StartAutoEnrollContext(ctx, 0, 2, 0); err != nil {
		t.Fatalf("StartAutoEnrollContext: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := d.ReadSysParams()
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("one-shot did not block during a session: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	if err := d.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("one-shot after Cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("one-shot did not resume after Cancel")
	}
}

func TestSessionContextCancel(t *testing.T) {
	d, _ := newFakeT(defaultHandler)
	ctx, cancel := context.WithCancel(context.Background())
	if err := d.StartAutoEnrollContext(ctx, 0, 2, 0); err != nil {
		t.Fatalf("StartAutoEnrollContext: %v", err)
	}
	cancel()

	// Either Next reports the context error or the watchdog ended the session.
	_, _ = d.NextEnrollEvent(50 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		_, err := d.ReadSysParams()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ReadSysParams after session cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session was not released after context cancel")
	}
}
