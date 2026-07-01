package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("postgres://u:p@localhost:5432/db")
	if cfg.DSN != "postgres://u:p@localhost:5432/db" {
		t.Errorf("DSN not threaded: %q", cfg.DSN)
	}
	if cfg.MaxConns != 20 {
		t.Errorf("MaxConns = %d, want 20", cfg.MaxConns)
	}
	if cfg.MinConns != 2 {
		t.Errorf("MinConns = %d, want 2", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Errorf("MaxConnLifetime = %v, want 1h", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 10*time.Minute {
		t.Errorf("MaxConnIdleTime = %v, want 10m", cfg.MaxConnIdleTime)
	}
}

func TestNew_RejectsBadDSN(t *testing.T) {
	// pgx rejects malformed DSNs without ever opening a socket.
	_, err := New(context.Background(), Config{DSN: "this is not a dsn"})
	if err == nil {
		t.Fatal("want error for bad DSN, got nil")
	}
	if !errors.Is(err, err) && err == nil {
		t.Fatal("unreachable")
	}
	// The error chain must mention either "parse dsn" or "new pool".
	msg := err.Error()
	if !contains(msg, "parse dsn") && !contains(msg, "new pool") {
		t.Errorf("unexpected error: %q", msg)
	}
}

func TestNew_RejectsUnreachableDSN(t *testing.T) {
	// Valid DSN shape but unreachable host: pool.Ping fails.
	// Use a high port that's almost certainly closed. Short context timeout so test is fast.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := New(ctx, Config{
		DSN:      "postgres://nobody:nopass@127.0.0.1:1/nodb?connect_timeout=1",
		MaxConns: 1,
		MinConns: 0,
	})
	if err == nil {
		t.Fatal("want ping failure, got nil")
	}
	if !contains(err.Error(), "ping") {
		t.Errorf("error chain should mention ping: %q", err.Error())
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && stringIndex(haystack, needle) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}