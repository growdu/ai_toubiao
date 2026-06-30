package ratelimit

import (
	"testing"
	"time"
)

func TestAllowsUpToRate(t *testing.T) {
	l := New(5, time.Minute)
	for i := 0; i < 5; i++ {
		if !l.Allow("k") {
			t.Errorf("call %d: should be allowed", i)
		}
	}
	if l.Allow("k") {
		t.Error("6th call should be denied")
	}
}

func TestSeparateKeys(t *testing.T) {
	l := New(2, time.Minute)
	l.Allow("a")
	l.Allow("a")
	if l.Allow("a") {
		t.Error("a should be limited")
	}
	if !l.Allow("b") {
		t.Error("b should still be allowed (separate bucket)")
	}
}

func TestRefill(t *testing.T) {
	l := New(2, 100*time.Millisecond)
	l.Allow("k")
	l.Allow("k")
	if l.Allow("k") {
		t.Fatal("3rd call should be denied")
	}
	time.Sleep(150 * time.Millisecond)
	if !l.Allow("k") {
		t.Error("after window, call should be allowed")
	}
}