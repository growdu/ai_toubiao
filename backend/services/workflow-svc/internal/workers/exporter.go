package workers

import (
	"strings"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ExportPayload is the task payload for document export.
type ExportPayload struct {
	WorkflowID uuid.UUID `json:"workflow_id"`
	BidJobID   uuid.UUID `json:"bid_job_id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	Format     string    `json:"format"` // "word" or "pdf"
	TemplateID uuid.UUID `json:"template_id,omitempty"`
}

// ExportWorker processes document export tasks.
type ExportWorker struct {
	log  *slog.Logger
	pool *pgxpool.Pool
	cfg  Config
}

// NewExportWorker creates a new export worker.
func NewExportWorker(log *slog.Logger, pool *pgxpool.Pool, cfg Config) *ExportWorker {
	return &ExportWorker{log: log, pool: pool, cfg: cfg}
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

	// 1. Load all chapter specs + their latest content from the DB.
	chapters, err := w.loadChapters(ctx, payload.BidJobID)
	if err != nil {
		return fmt.Errorf("load chapters: %w", err)
	}
	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found for bid_job %s", payload.BidJobID)
	}

	// 2. Render the document. Try docgen-svc first (richer formatting +
	// mermaid rendering); fall back to document-svc if it's unavailable.
	var downloadURL string
	if w.cfg.DocgenURL != "" {
		dl, err := w.callDocgenRender(ctx, payload, chapters)
		if err != nil {
			w.log.Warn("export: docgen-svc render failed, falling back to document-svc", slog.Any("error", err))
		} else {
			downloadURL = dl
		}
	}
	if downloadURL == "" {
		dl, err := w.callExportAPI(ctx, payload, chapters)
		if err != nil {
			w.log.Warn("export: document-svc call failed", slog.Any("error", err))
			return fmt.Errorf("export api: %w", err)
		}
		downloadURL = dl
	}

	// 3. Update workflow state: exporting → done.
	if _, err := w.pool.Exec(ctx, `
		UPDATE workflows SET status = 'done', current_step = NULL,
		       finished_at = NOW(), updated_at = NOW()
		WHERE id = $1`, payload.WorkflowID); err != nil {
		w.log.Warn("export: failed to finalize workflow", slog.Any("error", err))
	}

	// Record the transition event.
	_, _ = w.pool.Exec(ctx, `
		INSERT INTO workflow_events (workflow_id, tenant_id, from_state, to_state, actor_id, reason)
		VALUES ($1, $2, 'exporting', 'done', $3, $4)`,
		payload.WorkflowID, payload.TenantID, payload.TenantID,
		fmt.Sprintf("exported %d chapters, url=%s", len(chapters), downloadURL))

	w.log.Info("export: document exported successfully",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.String("format", payload.Format),
		slog.String("download_url", downloadURL),
		slog.Int("chapters", len(chapters)))

	return nil
}

// chapterExportData is the shape document-svc expects per chapter.
type chapterExportData struct {
	SpecID    uuid.UUID `json:"spec_id"`
	Title     string    `json:"title"`
	Level     int       `json:"level"`
	Content   string    `json:"content"`
	SortOrder int       `json:"sort_order"`
}

// loadChapters fetches all chapter specs for a bid job joined with their
// latest content version.
func (w *ExportWorker) loadChapters(ctx context.Context, bidJobID uuid.UUID) ([]chapterExportData, error) {
	rows, err := w.pool.Query(ctx, `
		SELECT cs.id, cs.title, cs.level, cs.order_index,
		       COALESCE(
		         (SELECT cc.content_text
		          FROM chapter_contents cc
		          WHERE cc.chapter_spec_id = cs.id
		          ORDER BY cc.version DESC
		          LIMIT 1), '') AS content
		FROM chapter_specs cs
		WHERE cs.bid_job_id = $1
		ORDER BY cs.order_index`, bidJobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []chapterExportData
	for rows.Next() {
		var ch chapterExportData
		if err := rows.Scan(&ch.SpecID, &ch.Title, &ch.Level, &ch.SortOrder, &ch.Content); err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

// callExportAPI posts chapter data to document-svc and returns the
// download URL for the generated document.
func (w *ExportWorker) callExportAPI(ctx context.Context, payload ExportPayload, chapters []chapterExportData) (string, error) {
	format := payload.Format
	if format == "" {
		format = "word"
	}
	reqBody := map[string]any{
		"bid_job_id": payload.BidJobID,
		"format":     format,
		"chapters":   chapters,
		"title":      "投标文件",
	}
	if payload.TemplateID != uuid.Nil {
		reqBody["template_id"] = payload.TemplateID
	}

	buf, _ := json.Marshal(reqBody)
	url := w.cfg.DocumentURL + "/api/v1/export/document"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", payload.TenantID.String())

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("document-svc HTTP %d: %s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		Data struct {
			DownloadURL string `json:"download_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return "", fmt.Errorf("decode export response: %w", err)
	}
	return wrapper.Data.DownloadURL, nil
}

// callDocgenRender posts chapter data to docgen-svc's /render endpoint
// for rich document assembly (title page, mermaid rendering, better
// formatting). Returns the download URL.
func (w *ExportWorker) callDocgenRender(ctx context.Context, payload ExportPayload, chapters []chapterExportData) (string, error) {
	reqBody := map[string]any{
		"title":   "投标文件",
		"format":  payload.Format,
		"chapters": chapters,
	}
	buf, _ := json.Marshal(reqBody)
	url := w.cfg.DocgenURL + "/api/v1/docgen/render"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", payload.TenantID.String())

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("docgen-svc HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode docgen response: %w", err)
	}
	if result.DownloadURL == "" {
		return "", fmt.Errorf("docgen-svc returned empty download_url")
	}
	// Resolve relative URL to absolute via the docgen-svc base.
	if strings.HasPrefix(result.DownloadURL, "/") {
		result.DownloadURL = w.cfg.DocgenURL + result.DownloadURL
	}
	return result.DownloadURL, nil
}

// EnqueueExport enqueues a document export task.
func EnqueueExport(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID uuid.UUID, format string, templateID uuid.UUID) error {
	payload := ExportPayload{
		WorkflowID: workflowID,
		BidJobID:   bidJobID,
		TenantID:   tenantID,
		Format:     format,
		TemplateID: templateID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskExport, data)
	_, err = client.EnqueueContext(ctx, task,
		asynq.MaxRetry(2),
		asynq.Timeout(30*time.Minute),
		asynq.Queue(QueueExporter))
	return err
}
