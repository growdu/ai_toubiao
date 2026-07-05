package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// AuditPayload is the task payload for compliance audit.
type AuditPayload struct {
	WorkflowID uuid.UUID `json:"workflow_id"`
	BidJobID   uuid.UUID `json:"bid_job_id"`
	TenantID   uuid.UUID `json:"tenant_id"`
}

// AuditTrigger abstracts a single audit call so tests can inject fakes
// without spinning up an HTTP server.
type AuditTrigger interface {
	TriggerSyncAudit(ctx context.Context, bidJobID, tenantID uuid.UUID) (status string, err error)
}

// AutoAdvanceEnqueuer is the minimal enqueue surface the auditor needs to
// trigger the export step. api.AsynqEnqueuer satisfies it.
type AutoAdvanceEnqueuer interface {
	EnqueueExport(ctx context.Context, workflowID, bidJobID, tenantID uuid.UUID, format string, templateID uuid.UUID) error
}

// AutoTransitioner is the same CAS-UPDATE seam used by the progress watcher.
// Both the chapter worker and the auditor worker share it; concrete impl in
// cmd/main.go is *PGTransitioner.
type AutoTransitioner interface {
	Transition(ctx context.Context, workflowID uuid.UUID, fromState, toState model.State, expectedVersion int, actorID uuid.UUID) error
}

// WorkflowReader is what the auditor worker uses to look up the workflow's
// expectedVersion before issuing a CAS UPDATE. *store.Store satisfies it.
type WorkflowReader interface {
	GetWorkflow(ctx context.Context, workflowID uuid.UUID) (version int, err error)
}

// AuditorWorker processes compliance audit tasks.
//
// Lifecycle (auto-advance mode):
//  1. POST audit-svc /api/v1/audit/bidjobs/{id}/report with {"async": false}
//     so the audit completes synchronously and we get the report back.
//  2. On HTTP success + Report.Status == "done", CAS the workflow
//     auditing -> exporting via AutoTransitioner.
//  3. Enqueue the export task via AutoAdvanceEnqueuer.
//
// When the dependencies aren't wired (older cmd/main.go builds, dev mode),
// the worker falls back to a best-effort audit-only behavior, matching the
// legacy contract.
//
// Why we pass async=false: when audit is async, the worker returns before
// the audit finishes and the workflow stays in "auditing" forever because
// nothing else advances it.
type AuditorWorker struct {
	log          *slog.Logger
	cfg          Config
	Trigger      AuditTrigger // optional override for tests; defaults to NewAuditClient(cfg.AuditURL)
	Transitioner AutoTransitioner
	Reader       WorkflowReader
	Enqueuer     AutoAdvanceEnqueuer
	ActorID      uuid.UUID
}

// NewAuditorWorker creates an auditor worker (legacy wire — no auto-advance).
func NewAuditorWorker(log *slog.Logger, cfg Config) *AuditorWorker {
	return &AuditorWorker{log: log, cfg: cfg}
}

// WithTrigger overrides the audit client (used in tests).
func (w *AuditorWorker) WithTrigger(t AuditTrigger) *AuditorWorker {
	w.Trigger = t
	return w
}

// WithAutoAdvance wires the dependencies needed to advance auditing -> exporting
// after a successful audit. cmd/main.go uses this in production.
func (w *AuditorWorker) WithAutoAdvance(t AutoTransitioner, r WorkflowReader, e AutoAdvanceEnqueuer, actor uuid.UUID) *AuditorWorker {
	w.Transitioner = t
	w.Reader = r
	w.Enqueuer = e
	w.ActorID = actor
	return w
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

	// 1. Trigger audit via audit-svc (synchronous).
	trig := w.Trigger
	if trig == nil {
		trig = NewAuditClient(w.cfg.AuditURL)
	}
	reportStatus, err := trig.TriggerSyncAudit(ctx, payload.BidJobID, payload.TenantID)
	if err != nil {
		w.log.Warn("auditor: audit trigger failed", slog.Any("error", err))
		// Don't fail the task — audit may need manual review.
		return nil
	}

	w.log.Info("auditor: compliance audit completed",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.String("report_status", reportStatus))

	// 2. Auto-advance only if we got a terminal report AND dependencies are wired.
	if reportStatus != "done" {
		w.log.Info("auditor: report not done, skipping auto-advance",
			slog.String("report_status", reportStatus))
		return nil
	}
	if w.Transitioner == nil || w.Enqueuer == nil || w.Reader == nil {
		w.log.Info("auditor: auto-advance not wired, legacy mode")
		return nil
	}

	version, err := w.Reader.GetWorkflow(ctx, payload.WorkflowID)
	if err != nil {
		w.log.Warn("auditor: read workflow version failed", slog.Any("error", err))
		return nil
	}
	if err := w.Transitioner.Transition(ctx, payload.WorkflowID,
		model.StateAuditing, model.StateExporting, version, w.ActorID); err != nil {
		w.log.Warn("auditor: transition to exporting failed",
			slog.String("workflow_id", payload.WorkflowID.String()),
			slog.Any("error", err))
		return nil
	}
	if err := w.Enqueuer.EnqueueExport(ctx, payload.WorkflowID,
		payload.BidJobID, payload.TenantID, "word", uuid.Nil); err != nil {
		w.log.Warn("auditor: enqueue export failed",
			slog.String("workflow_id", payload.WorkflowID.String()),
			slog.Any("error", err))
	}
	return nil
}

// EnqueueAudit enqueues an audit task.
func EnqueueAudit(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID uuid.UUID) error {
	payload := AuditPayload{
		WorkflowID: workflowID,
		BidJobID:   bidJobID,
		TenantID:   tenantID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskAudit, data)
	_, err = client.EnqueueContext(ctx, task,
		asynq.MaxRetry(3),
		asynq.Timeout(60*time.Minute),
		asynq.Queue(QueueAuditor))
	return err
}

// ----------------------------------------------------------------------------
// AuditClient — synchronous trigger (defined here because auditor.go owns it)
// ----------------------------------------------------------------------------

// bytesReaderT is a tiny io.Reader over []byte to avoid importing bytes here.
type bytesReaderT []byte

func (b bytesReaderT) Read(p []byte) (int, error) {
	n := copy(p, b)
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

func bytesReader(b []byte) io.Reader { return bytesReaderT(b) }

// TriggerSyncAudit is the synchronous mode audit trigger: explicitly sends
// {"async": false} so audit-svc executes the full pipeline before responding.
// Returns the report.Status string ("done" / "running" / "failed").
func (c *AuditClient) TriggerSyncAudit(ctx context.Context, bidJobID, tenantID uuid.UUID) (string, error) {
	reqBody := map[string]any{"async": false}
	buf, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/audit/bidjobs/"+bidJobID.String()+"/report",
		bytesReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("audit-svc HTTP %d: %s", resp.StatusCode, string(body))
	}
	var wrapper struct {
		Data struct {
			Report struct {
				Status string `json:"status"`
			} `json:"report"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return "", fmt.Errorf("decode audit response: %w", err)
	}
	return wrapper.Data.Report.Status, nil
}
