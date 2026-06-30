package workers

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupRouter creates and configures the Asynq server with all workers.
func SetupRouter(log *slog.Logger, pool *pgxpool.Pool) *asynq.Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: "localhost:6379"},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				QueuePlanner:   1,
				QueueChapter:   10,
				QueueAuditor:   2,
				QueueExporter:  2,
			},
		},
	)

	_ = log
	_ = pool
	return srv
}

// Serve starts the worker server.
func Serve(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool) error {
	srv := SetupRouter(log, pool)

	// Create mux for routing tasks to queues based on type
	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskOutlineGenerate, func(ctx context.Context, t *asynq.Task) error {
		planner := NewPlannerWorker(log, pool)
		return planner.Process(ctx, t)
	})
	mux.HandleFunc(TaskChapterGenerate, func(ctx context.Context, t *asynq.Task) error {
		chapter := NewChapterWorker(log)
		return chapter.Process(ctx, t)
	})
	mux.HandleFunc(TaskAudit, func(ctx context.Context, t *asynq.Task) error {
		auditor := NewAuditorWorker(log)
		return auditor.Process(ctx, t)
	})
	mux.HandleFunc(TaskExport, func(ctx context.Context, t *asynq.Task) error {
		exporter := NewExportWorker(log)
		return exporter.Process(ctx, t)
	})

	log.Info("workers: starting Asynq server...")
	return srv.Start(mux)
}