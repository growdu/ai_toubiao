package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// ChapterPayload is the task payload for chapter content generation.
type ChapterPayload struct {
	WorkflowID   uuid.UUID `json:"workflow_id"`
	BidJobID    uuid.UUID `json:"bid_job_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	ChapterID   uuid.UUID `json:"chapter_id"`
	ChapterTitle string   `json:"chapter_title"`
}

// ChapterWorker processes chapter content generation tasks.
type ChapterWorker struct {
	log *slog.Logger
}

// NewChapterWorker creates a new chapter worker.
func NewChapterWorker(log *slog.Logger) *ChapterWorker {
	return &ChapterWorker{log: log}
}

// Process handles the chapter content generation task.
func (w *ChapterWorker) Process(ctx context.Context, task *asynq.Task) error {
	var payload ChapterPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	w.log.Info("chapter: generating content",
		slog.String("chapter_id", payload.ChapterID.String()),
		slog.String("chapter_title", payload.ChapterTitle))

	// TODO: Implement actual chapter generation
	// 1. Load chapter spec from database
	// 2. Call knowledge-svc to retrieve relevant evidence (RAG)
	// 3. Call router-svc with TaskContentGen to generate content (Markdown)
	// 4. Parse Markdown for chart placeholders [!figure:xxx type=mermaid ...]
	// 5. Write content to chapter_contents table
	// 6. If charts found, enqueue illustration tasks

	w.log.Info("chapter: content generated",
		slog.String("chapter_id", payload.ChapterID.String()),
		slog.String("chapter_title", payload.ChapterTitle))

	return nil
}

// EnqueueChapter enqueues a chapter generation task.
func EnqueueChapter(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID, chapterID uuid.UUID, chapterTitle string) error {
	payload := ChapterPayload{
		WorkflowID:   workflowID,
		BidJobID:    bidJobID,
		TenantID:    tenantID,
		ChapterID:   chapterID,
		ChapterTitle: chapterTitle,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskChapterGenerate, data)
	_, err = client.EnqueueContext(ctx, task, asynq.MaxRetry(3), asynq.Timeout(30*60*1000))
	return err
}

// EnqueueChapters enqueues multiple chapter generation tasks (batch).
func EnqueueChapters(ctx context.Context, client *asynq.Client, chapters []ChapterPayload) error {
	for _, ch := range chapters {
		if err := EnqueueChapter(ctx, client, ch.WorkflowID, ch.BidJobID, ch.TenantID, ch.ChapterID, ch.ChapterTitle); err != nil {
			return err
		}
	}
	return nil
}
