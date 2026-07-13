package workers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

// asynqLogger adapts *slog.Logger to the asynq.Logger interface so the
// worker pool emits structured logs instead of the default stderr text.
type asynqLogger struct {
	log *slog.Logger
}

func (l asynqLogger) Debug(args ...any) { l.log.Debug(fmt.Sprint(args...)) }
func (l asynqLogger) Info(args ...any)  { l.log.Info(fmt.Sprint(args...)) }
func (l asynqLogger) Warn(args ...any)  { l.log.Warn(fmt.Sprint(args...)) }
func (l asynqLogger) Error(args ...any) { l.log.Error(fmt.Sprint(args...)) }
func (l asynqLogger) Fatal(args ...any) { l.log.Error(fmt.Sprint(args...)) }

// SetupServer creates and configures the Asynq server for the parser queue.
// Concurrency comes from cfg; the queue map only declares parser-q because
// document-svc only owns parsing — other services own their own queues.
func SetupServer(log *slog.Logger, redisAddr string, concurrency int) *asynq.Server {
	if concurrency <= 0 {
		concurrency = 4
	}
	return asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				QueueParser: 1,
			},
			Logger: asynqLogger{log: log},
		},
	)
}

// Serve starts the Asynq worker server and blocks until ctx is cancelled.
// It uses srv.Start (non-blocking) so the caller's signal handler stays
// in charge of graceful shutdown — srv.Run would install its own.
func Serve(ctx context.Context, log *slog.Logger, parser *ParserWorker, redisAddr string, concurrency int) error {
	srv := SetupServer(log, redisAddr, concurrency)
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskParseDocument, parser.Process)

	log.Info("parser worker: starting Asynq server",
		slog.String("redis_addr", redisAddr),
		slog.Int("concurrency", concurrency),
		slog.String("queue", QueueParser))

	if err := srv.Start(mux); err != nil {
		return fmt.Errorf("start asynq server: %w", err)
	}

	<-ctx.Done()
	log.Info("parser worker: shutting down Asynq server")
	srv.Shutdown()
	return nil
}
