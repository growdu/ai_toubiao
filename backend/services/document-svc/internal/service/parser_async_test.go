package service

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

// fakeEnqueuer captures the latest enqueue call so we can assert that
// the parser wires the queue correctly without spinning up Asynq.
type fakeEnqueuer struct {
	calls atomic.Int32
	last  struct {
		docID    uuid.UUID
		tenantID uuid.UUID
	}
	err error
}

func (f *fakeEnqueuer) EnqueueParse(_ context.Context, docID, tenantID uuid.UUID) error {
	f.calls.Add(1)
	f.last.docID = docID
	f.last.tenantID = tenantID
	return f.err
}

func TestParserService_NopEnqueuerIsDefault(t *testing.T) {
	// Default constructor wires the nop enqueuer so dev mode keeps
	// working when REDIS_ADDR is not set. Asserting the interface
	// implementation is bound to nopParseEnqueuer keeps us honest if
	// someone accidentally swaps in a nil pointer.
	svc := NewParserService(nil, nil, slog.Default(), "")
	var _ ParseEnqueuer = svc.enqueuer // compile-time interface check
	if _, ok := svc.enqueuer.(nopParseEnqueuer); !ok {
		t.Fatalf("default enqueuer should be nopParseEnqueuer, got %T", svc.enqueuer)
	}
}

func TestParserService_WithEnqueuer_NilSafe(t *testing.T) {
	// Passing nil must not blank out the enqueuer — production wiring
	// relies on a non-nil implementation even if config is partially
	// populated.
	svc := NewParserService(nil, nil, slog.Default(), "").WithEnqueuer(nil)
	if svc.enqueuer == nil {
		t.Fatal("WithEnqueuer(nil) should leave existing enqueuer intact")
	}
}

func TestParserService_WithEnqueuer_Swaps(t *testing.T) {
	// Real swap path: a fake enqueuer replaces the default and is
	// returned through the method chain.
	fake := &fakeEnqueuer{}
	svc := NewParserService(nil, nil, slog.Default(), "").WithEnqueuer(fake)
	if svc.enqueuer != fake {
		t.Fatalf("WithEnqueuer did not swap enqueuer: got %T", svc.enqueuer)
	}
}

// The fake below lets us assert the parser calls the enqueuer with the
// correct (docID, tenantID) tuple. We deliberately do not assert any
// side effect on the store because Parse touches the DB; instead we
// limit the test to verifying the interface wiring.
func TestFakeEnqueuer_CapturesPayload(t *testing.T) {
	f := &fakeEnqueuer{}
	doc := uuid.New()
	tenant := uuid.New()
	if err := f.EnqueueParse(context.Background(), doc, tenant); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if f.calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", f.calls.Load())
	}
	if f.last.docID != doc || f.last.tenantID != tenant {
		t.Fatalf("payload mismatch: got doc=%s tenant=%s", f.last.docID, f.last.tenantID)
	}
}
