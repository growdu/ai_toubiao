package tenant

import (
	"context"
	"errors"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	ctx := WithTenant(context.Background(), "tenant-123")
	got, err := FromContext(ctx)
	if err != nil {
		t.Fatalf("FromContext: %v", err)
	}
	if got != "tenant-123" {
		t.Errorf("got %q, want tenant-123", got)
	}
}

func TestMissing(t *testing.T) {
	_, err := FromContext(context.Background())
	if !errors.Is(err, ErrNoTenant) {
		t.Errorf("got %v, want ErrNoTenant", err)
	}
}

func TestEmpty(t *testing.T) {
	ctx := WithTenant(context.Background(), "")
	_, err := FromContext(ctx)
	if !errors.Is(err, ErrNoTenant) {
		t.Errorf("got %v, want ErrNoTenant", err)
	}
}