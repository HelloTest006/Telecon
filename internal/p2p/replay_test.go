package p2p

import "testing"

func TestReplayWindow(t *testing.T) {
	w := NewReplayWindow(8)
	if !w.Accept(0) {
		t.Fatal("0")
	}
	if w.Accept(0) {
		t.Fatal("dup")
	}
	if !w.Accept(1) {
		t.Fatal("1")
	}
	if !w.Accept(5) {
		t.Fatal("5")
	}
}
