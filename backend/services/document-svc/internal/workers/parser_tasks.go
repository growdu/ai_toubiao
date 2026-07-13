// Package workers implements Asynq task workers for the document-svc parser.
//
// The parser is split out of the HTTP request path so that large RFP
// uploads (PDF/Word with hundreds of pages) do not block the caller while
// we extract text + call router-svc for structured LLM extraction.
// Instead, the HTTP handler enqueues a `TaskParseDocument` task on the
// `parser-q` queue, updates the document row to `StatusParsing` immediately
// and returns 202 Accepted. The worker picks the task up, runs the same
// `ParserService.doParse` logic, and writes the result back into
// `document.metadata`.
//
// If REDIS_ADDR is empty the worker goroutine is not started and every
// parse request falls back to inline execution — this keeps local
// development frictionless while still allowing production to opt in.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bidwriter/services/document-svc/internal/service"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// QueueParser is the Asynq queue name for parse tasks.
const QueueParser = "parser"

// TaskParseDocument is the Asynq task type for background document parsing.
const TaskParseDocument = "document:parse"

// ParsePayload is the JSON payload carried by a parse task. We keep it
// minimal — just the document ID and the tenant — because the worker
// re-reads the row from the store so we never work on stale data.
type ParsePayload struct {
	DocumentID uuid.UUID `json:"document_id"`
	TenantID   uuid.UUID `json:"tenant_id"`
}

// Enqueuer is the small surface area ParserService needs from the Asynq
// client. Defining it as an interface keeps ParserService unit-testable
// without spinning up a real Redis.
type Enqueuer interface {
	EnqueueParse(ctx context.Context, docID, tenantID uuid.UUID) error
}

// AsynqEnqueuer implements Enqueuer on top of *asynq.Client.
type AsynqEnqueuer struct {
	client *asynq.Client
}

// NewAsynqEnqueuer wraps an existing asynq client.
func NewAsynqEnqueuer(c *asynq.Client) *AsynqEnqueuer {
	return &AsynqEnqueuer{client: c}
}

// EnqueueParse enqueues a parse task on the parser queue with bounded
// retries and a sensible timeout. It returns the asynq error verbatim
// so the caller (HTTP handler) can decide whether to surface a 503.
func (e *AsynqEnqueuer) EnqueueParse(ctx context.Context, docID, tenantID uuid.UUID) error {
	payload, err := json.Marshal(ParsePayload{
		DocumentID: docID,
		TenantID:   tenantID,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskParseDocument, payload,
		asynq.Queue(QueueParser),
		asynq.MaxRetry(3),
		asynq.Timeout(15*60*1_000_000_000), // 15 minutes in ns
		asynq.TaskID(fmt.Sprintf("parse:%s", docID.String())), // dedup re-runs
	)
	_, err = e.client.EnqueueContext(ctx, task)
	return err
}

// ParserWorker runs the actual parse for a single document task.
type ParserWorker struct {
	parser *service.ParserService
	log    *slog.Logger
}

// NewParserWorker constructs a worker from a pre-built ParserService. The
// pool is held inside ParserService already so we do not pass it again.
func NewParserWorker(parser *service.ParserService, log *slog.Logger) *ParserWorker {
	return &ParserWorker{parser: parser, log: log}
}

// Process unmarshals the payload, calls Parse synchronously and lets asynq
// retry on error. ParserService.Parse already writes the per-document
// status transitions to the store, so failure here is just an error
// string that asynq surfaces in its retry policy.
func (w *ParserWorker) Process(ctx context.Context, task *asynq.Task) error {
	var payload ParsePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		// Bad payload is a permanent failure — no point retrying.
		w.log.Error("parser worker: bad payload", slog.String("err", err.Error()))
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	w.log.Info("parser worker: processing",
		slog.String("doc_id", payload.DocumentID.String()),
		slog.String("tenant_id", payload.TenantID.String()))

	// Force inline=true so Parse blocks until done; the document row
	// has already been updated to StatusParsing by the HTTP handler.
	if _, err := w.parser.Parse(ctx, payload.DocumentID, false); err != nil {
		w.log.Warn("parser worker: parse failed",
			slog.String("doc_id", payload.DocumentID.String()),
			slog.String("err", err.Error()))
		return err
	}

	w.log.Info("parser worker: parse complete",
		slog.String("doc_id", payload.DocumentID.String()))
	return nil
}
