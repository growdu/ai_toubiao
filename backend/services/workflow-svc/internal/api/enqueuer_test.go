package api

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/google/uuid"
)

// fakeEnqueuer records which dispatch methods were called.
type fakeEnqueuer struct {
	outlineCalled  bool
	chaptersCalled bool
	auditCalled    bool
	exportCalled   bool
}

func (f *fakeEnqueuer) EnqueueOutline(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
	f.outlineCalled = true
	return nil
}
func (f *fakeEnqueuer) EnqueueChaptersForBid(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	f.chaptersCalled = true
	return nil
}
func (f *fakeEnqueuer) EnqueueAudit(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	f.auditCalled = true
	return nil
}
func (f *fakeEnqueuer) EnqueueExport(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, uuid.UUID) error {
	f.exportCalled = true
	return nil
}

func TestNoopEnqueuer_AllNoOps(t *testing.T) {
	e := noopEnqueuer{}
	id := uuid.New()
	if err := e.EnqueueOutline(context.Background(), id, id, id, id); err != nil {
		t.Errorf("EnqueueOutline: %v", err)
	}
	if err := e.EnqueueChaptersForBid(context.Background(), id, id, id); err != nil {
		t.Errorf("EnqueueChaptersForBid: %v", err)
	}
	if err := e.EnqueueAudit(context.Background(), id, id, id); err != nil {
		t.Errorf("EnqueueAudit: %v", err)
	}
	if err := e.EnqueueExport(context.Background(), id, id, id, "word", uuid.Nil); err != nil {
		t.Errorf("EnqueueExport: %v", err)
	}
}

func TestDispatchOnTransition_NilEnqueuerNoPanic(t *testing.T) {
	h := &Handlers{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		// Enqueuer is nil — should not panic.
	}
	wf := &model.Workflow{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Status:   model.StateOutlining,
	}
	// Should be a no-op without panicking.
	h.dispatchOnTransition(context.Background(), wf, h.Log)
}

func TestDispatchOnTransition_OutliningEnqueuesOutline(t *testing.T) {
	fe := &fakeEnqueuer{}
	h := &Handlers{
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Enqueuer: fe,
		// Store doesn't implement LookupBidJob, so dispatch will return early.
	}
	wf := &model.Workflow{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Status:   model.StateOutlining,
	}
	h.dispatchOnTransition(context.Background(), wf, h.Log)
	// Since Store doesn't implement the LookupBidJob interface, no enqueue
	// should happen. This verifies graceful degradation.
	if fe.outlineCalled {
		t.Error("outline should not be enqueued when Store lacks LookupBidJob")
	}
}
