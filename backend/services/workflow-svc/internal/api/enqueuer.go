package api

import (
	"context"
	"log/slog"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/google/uuid"
)

// Enqueuer is the seam for dispatching async pipeline tasks. The concrete
// implementation wraps an Asynq client; tests inject a no-op to verify
// handler logic without Redis.
type Enqueuer interface {
	// EnqueueOutline dispatches the outline-generation task.
	EnqueueOutline(ctx context.Context, workflowID, bidJobID, tenantID, documentID uuid.UUID) error
	// EnqueueChaptersForBid loads all chapter specs for a bid job and
	// dispatches a content-generation task for each.
	EnqueueChaptersForBid(ctx context.Context, workflowID, bidJobID, tenantID uuid.UUID) error
	// EnqueueAudit dispatches the compliance-audit task.
	EnqueueAudit(ctx context.Context, workflowID, bidJobID, tenantID uuid.UUID) error
	// EnqueueExport dispatches the document-export task.
	EnqueueExport(ctx context.Context, workflowID, bidJobID, tenantID uuid.UUID, format string, templateID uuid.UUID) error
}

// noopEnqueuer does nothing — used when no Asynq client is wired (tests,
// dev mode without Redis).
type noopEnqueuer struct{}

func (noopEnqueuer) EnqueueOutline(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (noopEnqueuer) EnqueueChaptersForBid(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (noopEnqueuer) EnqueueAudit(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (noopEnqueuer) EnqueueExport(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, uuid.UUID) error {
	return nil
}

// dispatchOnTransition enqueues the appropriate async task(s) based on the
// target state of a workflow transition. It is best-effort: enqueue failures
// are logged but do not fail the transition (the user can retry).
func (h *Handlers) dispatchOnTransition(ctx context.Context, wf *model.Workflow, log *slog.Logger) {
	if h.Enqueuer == nil {
		return
	}
	// We need the bid_job_id and document_id from the workflow metadata or
	// a lookup. For now, the workflow's project_id is used to find the
	// associated bid_job. This is a simplification — in production the
	// transition request should carry the bid_job_id.
	bidJobID, documentID := h.lookupBidJob(ctx, wf.ID)
	if bidJobID == uuid.Nil {
		log.Warn("dispatch: no bid_job found for workflow",
			slog.String("workflow_id", wf.ID.String()))
		return
	}

	var err error
	switch wf.Status {
	case model.StateOutlining:
		err = h.Enqueuer.EnqueueOutline(ctx, wf.ID, bidJobID, wf.TenantID, documentID)
	case model.StateGenerating:
		err = h.Enqueuer.EnqueueChaptersForBid(ctx, wf.ID, bidJobID, wf.TenantID)
	case model.StateAuditing:
		err = h.Enqueuer.EnqueueAudit(ctx, wf.ID, bidJobID, wf.TenantID)
	case model.StateExporting:
		err = h.Enqueuer.EnqueueExport(ctx, wf.ID, bidJobID, wf.TenantID, "word", uuid.Nil)
	}
	if err != nil {
		log.Warn("dispatch: enqueue failed",
			slog.String("state", string(wf.Status)),
			slog.Any("error", err))
	}
}

// lookupBidJob finds the bid_job_id and rfp_document_id associated with a
// workflow. Returns uuid.Nil if not found.
func (h *Handlers) lookupBidJob(ctx context.Context, workflowID uuid.UUID) (uuid.UUID, uuid.UUID) {
	backend, ok := h.Store.(interface {
		LookupBidJob(ctx context.Context, workflowID uuid.UUID) (bidJobID, documentID uuid.UUID, err error)
	})
	if !ok {
		return uuid.Nil, uuid.Nil
	}
	bidJobID, documentID, err := backend.LookupBidJob(ctx, workflowID)
	if err != nil {
		return uuid.Nil, uuid.Nil
	}
	return bidJobID, documentID
}
