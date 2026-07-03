package workers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupServer creates and configures the Asynq server with all workers.
func SetupServer(log *slog.Logger, pool *pgxpool.Pool, cfg Config) *asynq.Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				QueuePlanner:  1,
				QueueChapter:  10,
				QueueAuditor:  2,
				QueueExporter: 2,
			},
			Logger: asynqLogger{log: log},
		},
	)
	return srv
}

// Serve starts the Asynq worker server and blocks until ctx is cancelled.
// It uses srv.Start (non-blocking) instead of srv.Run (which installs its
// own signal handlers that conflict with the caller's signal handling).
func Serve(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, cfg Config) error {
	srv := SetupServer(log, pool, cfg)

	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskOutlineGenerate, func(ctx context.Context, t *asynq.Task) error {
		planner := NewPlannerWorker(log, pool, cfg)
		return planner.Process(ctx, t)
	})
	mux.HandleFunc(TaskChapterGenerate, func(ctx context.Context, t *asynq.Task) error {
		chapter := NewChapterWorker(log, pool, cfg)
		return chapter.Process(ctx, t)
	})
	mux.HandleFunc(TaskAudit, func(ctx context.Context, t *asynq.Task) error {
		auditor := NewAuditorWorker(log, cfg)
		return auditor.Process(ctx, t)
	})
	mux.HandleFunc(TaskExport, func(ctx context.Context, t *asynq.Task) error {
		exporter := NewExportWorker(log, pool, cfg)
		return exporter.Process(ctx, t)
	})

	log.Info("workers: starting Asynq server",
		slog.String("redis_addr", cfg.RedisAddr),
		slog.String("router_url", cfg.RouterURL))

	// Start is non-blocking; it returns an error if the server can't start.
	if err := srv.Start(mux); err != nil {
		return fmt.Errorf("start asynq server: %w", err)
	}

	// Block until ctx is cancelled, then gracefully shut down.
	<-ctx.Done()
	log.Info("workers: shutting down Asynq server")
	srv.Shutdown()
	return nil
}

// NewClient creates an Asynq client for enqueuing tasks.
func NewClient(cfg Config) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
}

// asynqLogger adapts *slog.Logger to the asynq.Logger interface.
type asynqLogger struct {
	log *slog.Logger
}

func (l asynqLogger) Debug(args ...any) { l.log.Debug(fmt.Sprint(args...)) }
func (l asynqLogger) Info(args ...any)  { l.log.Info(fmt.Sprint(args...)) }
func (l asynqLogger) Warn(args ...any)  { l.log.Warn(fmt.Sprint(args...)) }
func (l asynqLogger) Error(args ...any) { l.log.Error(fmt.Sprint(args...)) }
func (l asynqLogger) Fatal(args ...any) { l.log.Error(fmt.Sprint(args...)) }
