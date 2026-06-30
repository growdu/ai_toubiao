package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bidwriter/services/router-svc/internal/model"
)

// CallLogBatcher buffers CallLog entries and flushes them in batches to the
// router_call_logs table. It runs in the background and is safe for concurrent
// Add() calls.
//
// Flush triggers:
//   - interval timer (default 5s)
//   - buffer reaches MaxBuffer entries
//   - explicit Flush() call (used by tests and on graceful shutdown)
type CallLogBatcher struct {
	pool       *pgxpool.Pool
	maxBuffer  int
	interval   time.Duration
	mu         sync.Mutex
	buf        []model.CallLog
	stopCh     chan struct{}
	flushOnce  sync.Once
	wg         sync.WaitGroup
	onFlushErr func(error)
}

// NewCallLogBatcher builds and starts a batcher. Pass nil pool to disable
// persistence (logs are still buffered and dropped on shutdown — useful in
// pure unit tests).
func NewCallLogBatcher(pool *pgxpool.Pool, interval time.Duration, maxBuffer int) *CallLogBatcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if maxBuffer <= 0 {
		maxBuffer = 100
	}
	b := &CallLogBatcher{
		pool:      pool,
		maxBuffer: maxBuffer,
		interval:  interval,
		stopCh:    make(chan struct{}),
		onFlushErr: func(err error) { log.Printf("[router] batcher flush error: %v", err) },
	}
	b.wg.Add(1)
	go b.loop()
	return b
}

// Add queues a log entry, flushing immediately if the buffer is full.
func (b *CallLogBatcher) Add(log model.CallLog) {
	b.mu.Lock()
	b.buf = append(b.buf, log)
	full := len(b.buf) >= b.maxBuffer
	b.mu.Unlock()
	if full {
		// best-effort flush; error callback handles logging
		_ = b.Flush()
	}
}

// Flush writes all buffered entries to the DB and clears the buffer.
func (b *CallLogBatcher) Flush() error {
	b.mu.Lock()
	if len(b.buf) == 0 {
		b.mu.Unlock()
		return nil
	}
	batch := b.buf
	b.buf = nil
	b.mu.Unlock()

	if b.pool == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := b.pool.Begin(ctx)
	if err != nil {
		if b.onFlushErr != nil {
			b.onFlushErr(err)
		}
		return err
	}

	const sql = `
		INSERT INTO router_call_logs (
			tenant_id, workflow_id, step_id, task, provider, model,
			prompt_tokens, completion_tokens, latency_ms, cost_usd,
			cache_hit, fallback_used, attempt, error, metadata, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`
	for _, e := range batch {
		var metaBytes []byte
		if e.Metadata != nil {
			metaBytes, _ = json.Marshal(e.Metadata)
		} else {
			metaBytes = []byte("{}")
		}
		if _, err := tx.Exec(ctx, sql,
			e.TenantID, e.WorkflowID, e.StepID, string(e.Task), e.Provider, e.Model,
			e.PromptTokens, e.CompletionTokens, e.LatencyMs, e.CostUSD,
			e.CacheHit, e.FallbackUsed, e.Attempt, nullableString(e.Error), metaBytes, e.CreatedAt,
		); err != nil {
			_ = tx.Rollback(ctx)
			if b.onFlushErr != nil {
				b.onFlushErr(fmt.Errorf("insert call log: %w", err))
			}
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		if b.onFlushErr != nil {
			b.onFlushErr(err)
		}
		return err
	}
	return nil
}

// Pending returns the current buffer size (for tests/health).
func (b *CallLogBatcher) Pending() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buf)
}

// Stop flushes remaining entries and shuts down the background goroutine.
func (b *CallLogBatcher) Stop() {
	b.flushOnce.Do(func() {
		close(b.stopCh)
		b.wg.Wait()
		_ = b.Flush()
	})
}

func (b *CallLogBatcher) loop() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			_ = b.Flush()
		}
	}
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}