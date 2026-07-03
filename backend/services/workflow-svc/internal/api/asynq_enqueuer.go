package api

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bidwriter/services/workflow-svc/internal/workers"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AsynqEnqueuer implements the Enqueuer interface by delegating to the
// workers package's enqueue helpers. It owns an asynq.Client which must
// be closed on shutdown.
type AsynqEnqueuer struct {
	client *asynq.Client
	pool   *pgxpool.Pool
	log    *slog.Logger
}

// NewAsynqEnqueuer builds an enqueuer backed by the given Asynq client
// (connected to Redis) and DB pool (for loading chapter specs).
func NewAsynqEnqueuer(client *asynq.Client, pool *pgxpool.Pool, log *slog.Logger) *AsynqEnqueuer {
	return &AsynqEnqueuer{client: client, pool: pool, log: log}
}

// Close releases the Asynq client connection.
func (e *AsynqEnqueuer) Close() error {
	return e.client.Close()
}

// EnqueueOutline dispatches the outline-generation task.
func (e *AsynqEnqueuer) EnqueueOutline(ctx context.Context, workflowID, bidJobID, tenantID, documentID uuid.UUID) error {
	return workers.EnqueueOutline(ctx, e.client, workflowID, bidJobID, tenantID, documentID)
}

// EnqueueChaptersForBid loads all chapter specs for a bid job and enqueues
// a content-generation task for each one.
func (e *AsynqEnqueuer) EnqueueChaptersForBid(ctx context.Context, workflowID, bidJobID, tenantID uuid.UUID) error {
	rows, err := e.pool.Query(ctx, `
		SELECT id, title, target_word_count, min_word_count
		FROM chapter_specs
		WHERE bid_job_id = $1 AND status = 'planned'
		ORDER BY order_index`, bidJobID)
	if err != nil {
		return fmt.Errorf("load chapter specs: %w", err)
	}
	defer rows.Close()

	var chapters []workers.ChapterPayload
	for rows.Next() {
		var ch workers.ChapterPayload
		if err := rows.Scan(&ch.ChapterID, &ch.ChapterTitle, &ch.TargetWords, &ch.MinWords); err != nil {
			return fmt.Errorf("scan chapter spec: %w", err)
		}
		ch.WorkflowID = workflowID
		ch.BidJobID = bidJobID
		ch.TenantID = tenantID
		chapters = append(chapters, ch)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(chapters) == 0 {
		e.log.Warn("enqueue chapters: no planned chapters found",
			slog.String("bid_job_id", bidJobID.String()))
		return nil
	}

	// Mark all chapters as pending before enqueuing.
	_, _ = e.pool.Exec(ctx, `
		UPDATE chapter_specs SET status = 'pending', updated_at = NOW()
		WHERE bid_job_id = $1 AND status = 'planned'`, bidJobID)

	return workers.EnqueueChapters(ctx, e.client, chapters)
}

// EnqueueChapter dispatches a content-generation task for a single chapter.
func (e *AsynqEnqueuer) EnqueueChapter(ctx context.Context, workflowID, bidJobID, tenantID, chapterID uuid.UUID, chapterTitle string) error {
	return workers.EnqueueChapter(ctx, e.client, workflowID, bidJobID, tenantID, chapterID, chapterTitle)
}

// EnqueueAudit dispatches the compliance-audit task.
func (e *AsynqEnqueuer) EnqueueAudit(ctx context.Context, workflowID, bidJobID, tenantID uuid.UUID) error {
	return workers.EnqueueAudit(ctx, e.client, workflowID, bidJobID, tenantID)
}

// EnqueueExport dispatches the document-export task.
func (e *AsynqEnqueuer) EnqueueExport(ctx context.Context, workflowID, bidJobID, tenantID uuid.UUID, format string, templateID uuid.UUID) error {
	return workers.EnqueueExport(ctx, e.client, workflowID, bidJobID, tenantID, format, templateID)
}
