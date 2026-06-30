package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// AuditPayload is the task payload for compliance audit.
type AuditPayload struct {
	WorkflowID uuid.UUID `json:"workflow_id"`
	BidJobID  uuid.UUID `json:"bid_job_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
}

// AuditorWorker processes compliance audit tasks.
type AuditorWorker struct {
	log *slog.Logger
}

// NewAuditorWorker creates a new auditor worker.
func NewAuditorWorker(log *slog.Logger) *AuditorWorker {
	return &AuditorWorker{log: log}
}

// Process handles the compliance audit task.
func (w *AuditorWorker) Process(ctx context.Context, task *asynq.Task) error {
	var payload AuditPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	w.log.Info("auditor: starting compliance audit",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.String("bid_job_id", payload.BidJobID.String()))

	// TODO: Implement actual audit
	// 1. Call audit-svc POST /api/v1/audit/bidjobs/:id/report
	// 2. Wait for audit completion (sync or async poll)
	// 3. If critical issues found, mark audit as failed
	// 4. Update workflow state: auditing → exporting (if passed) or failed

	w.log.Info("auditor: compliance audit completed",
		slog.String("workflow_id", payload.WorkflowID.String()))

	return nil
}

// EnqueueAudit enqueues an audit task.
func EnqueueAudit(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID uuid.UUID) error {
	payload := AuditPayload{
		WorkflowID: workflowID,
		BidJobID:  bidJobID,
		TenantID:  tenantID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskAudit, data)
	_, err = client.EnqueueContext(ctx, task, asynq.MaxRetry(3), asynq.Timeout(60*60*1000))
	return err
}
