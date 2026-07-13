package store

import (
	"context"
	"strings"
	"testing"
)

// These tests do not touch a real database. They cover the auth/
// tenant-context branch that previously panicked, to lock in the
// graceful-error contract after the rewrite. The actual SQL paths
// are exercised by integration tests when DATABASE_URL_TEST_PG is
// set; here we only need to prove the panic is gone.
//
// Why no "happy path" test? pgxpool.Pool is nil here, and pgx
// dereferences it inside QueryRow. We could construct a fake pool
// with miniredis/pgxmock, but that adds a test dep just to prove
// "yes the function eventually called SQL", which is already obvious
// from the implementation. The two tests below cover the bug we
// actually fixed.

func TestStore_GetTenantPlan_NoTenant_ReturnsError(t *testing.T) {
	s := New(nil) // pool is irrelevant — we fail before querying
	_, err := s.GetTenantPlan(context.Background())
	if err == nil {
		t.Fatal("expected error when tenant is missing from context")
	}
	if !strings.Contains(err.Error(), "tenant missing") {
		t.Fatalf("error message should mention tenant missing, got %v", err)
	}
	// The previous implementation used `panic(...)` here, which
	// crashes the whole Go process and surfaces as a stack trace.
	// The new implementation must return a plain Go error so the
	// HTTP layer can translate it to 401/500 cleanly.
	if strings.Contains(err.Error(), "panic") {
		t.Fatalf("panic-like error must not surface: %v", err)
	}
}

func TestStore_UpdateTenantPlan_NoTenant_ReturnsError(t *testing.T) {
	s := New(nil)
	err := s.UpdateTenantPlan(context.Background(), "pro")
	if err == nil {
		t.Fatal("expected error when tenant is missing from context")
	}
	if !strings.Contains(err.Error(), "tenant missing") {
		t.Fatalf("error message should mention tenant missing, got %v", err)
	}
	if strings.Contains(err.Error(), "panic") {
		t.Fatalf("panic-like error must not surface: %v", err)
	}
}
