package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// ExportPayload is the task payload for document export.
type ExportPayload struct {
	WorkflowID uuid.UUID `json:"workflow_id"`
	BidJobID  uuid.UUID `json:"bid_job_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Format    string    `json:"format"` // "word" or "pdf"
	TemplateID uuid.UUID `json:"template_id,omitempty"`
}

// ExportWorker processes document export tasks.
type ExportWorker struct {
	log *slog.Logger
}

// NewExportWorker creates a new export worker.
func NewExportWorker(log *slog.Logger) *ExportWorker {
	return &ExportWorker{log: log}
}

// Process handles the document export task.
func (w *ExportWorker) Process(ctx context.Context, task *asynq.Task) error {
	var payload ExportPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	w.log.Info("export: starting document export",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.String("format", payload.Format))

	// TODO: Implement actual document export
	// 1. Load all chapter_contents for the bid job
	// 2. Load illustrations (charts, images)
	// 3. Call document-svc to assemble Word doc using template
	//    - Load template from template-svc
	//    - Replace placeholders with chapter content
	//    - Embed illustrations
	// 4. For PDF: call LibreOffice headless conversion
	// 5. Upload to S3/MinIO
	// 6. Update workflow state: exporting → done

	w.log.Info("export: document exported successfully",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.String("format", payload.Format))

	return nil
}

// EnqueueExport enqueues a document export task.
func EnqueueExport(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID uuid.UUID, format string, templateID uuid.UUID) error {
	payload := ExportPayload{
		WorkflowID: workflowID,
		BidJobID:  bidJobID,
		TenantID:  tenantID,
		Format:    format,
		TemplateID: templateID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskExport, data)
	_, err = client.EnqueueContext(ctx, task, asynq.MaxRetry(2), asynq.Timeout(30*60*1000))
	return err
}
