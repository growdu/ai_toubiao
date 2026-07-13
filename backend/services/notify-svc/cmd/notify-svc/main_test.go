package main

import (
	"strings"
	"testing"
)

// requireEnvOrLog replaces the old `mustEnv` panic helper. The test
// pins the new contract: returns (value, nil) when set, ("", error)
// when missing, and never panics. The error message must mention the
// variable name so operators can fix the deployment from the log.

func TestRequireEnvOrLog_Set(t *testing.T) {
	t.Setenv("NOTIFY_TEST_KEY", "value-123")
	got, err := requireEnvOrLog("NOTIFY_TEST_KEY")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "value-123" {
		t.Fatalf("got %q, want %q", got, "value-123")
	}
}

func TestRequireEnvOrLog_Missing(t *testing.T) {
	t.Setenv("NOTIFY_TEST_KEY", "")
	got, err := requireEnvOrLog("NOTIFY_TEST_KEY")
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
	if got != "" {
		t.Fatalf("value must be empty on error, got %q", got)
	}
	if !strings.Contains(err.Error(), "NOTIFY_TEST_KEY") {
		t.Fatalf("error must mention the variable name, got %q", err.Error())
	}
}

// Critical regression guard: the old helper panicked, which took the
// whole notify-svc process down. The new helper must NOT panic even
// when called from a goroutine that didn't initialise the env.
func TestRequireEnvOrLog_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("requireEnvOrLog must not panic, got: %v", r)
		}
	}()
	t.Setenv("NOTIFY_TEST_KEY", "")
	_, _ = requireEnvOrLog("NOTIFY_TEST_KEY")
}
