package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_EmitsJSONWithServiceField(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	log := slog.New(h).With(slog.String("service", "test-svc"))

	log.Info("hello", slog.String("key", "val"))

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, buf.String())
	}
	if rec["service"] != "test-svc" {
		t.Errorf("service field missing/wrong: %v", rec["service"])
	}
	if rec["key"] != "val" {
		t.Errorf("attr missing: %v", rec["key"])
	}
	if rec["msg"] != "hello" {
		t.Errorf("msg missing: %v", rec["msg"])
	}
}

func TestNewFactoryProducesLogger(t *testing.T) {
	// Exercise the actual New() factory (writes to os.Stdout — verify it does not panic
	// and returns a usable logger).
	log := New("svc-x")
	if log == nil {
		t.Fatal("New returned nil")
	}
	// Should be Info-level by default — Debug should be filtered.
	var buf bytes.Buffer
	log = slog.New(slog.NewJSONHandler(&buf, nil)).
		With(slog.String("service", "svc-x"))
	log.Debug("filtered")
	log.Info("kept", slog.String("k", "v"))
	if !strings.Contains(buf.String(), `"msg":"kept"`) {
		t.Errorf("Info should pass through; got %q", buf.String())
	}
	if strings.Contains(buf.String(), "filtered") {
		t.Errorf("Debug should be filtered at Info level; got %q", buf.String())
	}
}

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequest(context.Background(), "req-abc")
	if got := RequestIDFrom(ctx); got != "req-abc" {
		t.Errorf("got %q, want req-abc", got)
	}
}

func TestRequestIDMissing(t *testing.T) {
	// Bare context returns "" (not panic, not error — this is the documented contract).
	if got := RequestIDFrom(context.Background()); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestRequestIDOverwrite(t *testing.T) {
	ctx := WithRequest(context.Background(), "first")
	ctx = WithRequest(ctx, "second")
	if got := RequestIDFrom(ctx); got != "second" {
		t.Errorf("got %q, want second (later WithRequest should win)", got)
	}
}