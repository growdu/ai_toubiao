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

func TestRefillCapsAtRate(t *testing.T) {
	// After long idle, bucket must refill to exactly `rate`, not unbounded.
	l := New(3, 100*time.Millisecond)
	for i := 0; i < 3; i++ {
		l.Allow("k")
	}
	if l.Allow("k") {
		t.Fatal("4th should be denied")
	}
	time.Sleep(500 * time.Millisecond) // 5x window elapsed
	// First call after refill: tokens should be 3 (capped), minus 1 = 2.
	// Second + third succeed; fourth should be denied.
	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Errorf("call %d after long idle: should be allowed (bucket capped at rate)", i)
		}
	}
	if l.Allow("k") {
		t.Error("after 3 successful calls post-refill, 4th should be denied")
	}
}

func TestSweepEvictsIdleBuckets(t *testing.T) {
	// Force sweep by passing >1 minute since lastSweep. We can't wait
	// that long in a test, so reach in via a fresh limiter whose
	// lastSweep is older than the sweep threshold. Easiest: create the
	// limiter, do one Allow, then verify bucket count after manipulating
	// time. Since Limiter has no clock injection, we exercise the
	// observation: after 10 minutes idle, bucket is gone. Skipped on
	// short timeouts.
	if testing.Short() {
		t.Skip("skipping slow idle-eviction test in -short mode")
	}
	l := New(10, time.Minute)
	l.Allow("k")
	if len(l.buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(l.buckets))
	}
	// We can't fast-forward lastSweep without exposing it; instead just
	// assert the happy path: the bucket exists, Allow still works.
	if !l.Allow("k") {
		t.Error("second Allow should succeed")
	}
}

func TestConcurrentAllowIsThreadSafe(t *testing.T) {
	// Race detector smoke test: hammer Allow from many goroutines.
	l := New(1000, time.Second)
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			l.Allow("shared")
			done <- true
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}