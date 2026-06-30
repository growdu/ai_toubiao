// Package logger wraps slog with project conventions.
// All services should use this package for structured logging.
package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

// New returns a slog.Logger configured for JSON output to stdout.
// Add common attributes via With() at startup.
func New(serviceName string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})
	return slog.New(h).With(
		slog.String("service", serviceName),
	)
}

// WithRequest stores request_id in context.
func WithRequest(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, requestID)
}

// RequestIDFrom retrieves request_id from context.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}