package main

import "testing"

func TestSplitPIDs(t *testing.T) {
	got := splitPIDs("010C, 0D;0111")
	want := []string{"0C", "0D", "11"}
	if len(got) != len(want) {
		t.Fatalf("splitPIDs = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitPIDs = %v, want %v", got, want)
		}
	}
}

func TestBuildGroups(t *testing.T) {
	groups := buildGroups([]string{"0C", "0D", "11"}, 2)
	if len(groups) != 2 || len(groups[0]) != 2 || len(groups[1]) != 1 {
		t.Fatalf("groups = %v", groups)
	}
}

func TestMakeCommand(t *testing.T) {
	if got := makeCommand([]string{"0C", "0D"}); got != "010C 010D" {
		t.Fatalf("makeCommand = %q", got)
	}
}

func TestResponseMask(t *testing.T) {
	// 0100 response from the car: 41 00 BE 3F B8 13.
	raw := []byte("4100BE3FB813>")
	mask, ok := responseMask(raw, 0x00)
	if !ok {
		t.Fatal("mask not found")
	}
	if mask != 0xBE3FB813 {
		t.Fatalf("mask = %08X", mask)
	}
}

func TestAllExpected(t *testing.T) {
	raw := []byte("410C1AF8410D1E>")
	if !allExpected(raw, []string{"0C", "0D"}) {
		t.Fatal("expected both PIDs")
	}
	if allExpected(raw, []string{"0C", "11"}) {
		t.Fatal("PID 11 should be missing")
	}
}
