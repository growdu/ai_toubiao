// Package middleware contains HTTP middleware.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bidwriter/shared/pkg/logger"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// RequestID ensures every request has an X-Request-ID (generates one if missing).
// It also propagates tenant_id from the gateway-injected X-Tenant-ID header
// into the context (since downstream services trust this header per ADR-0001).
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", rid)

		ctx := logger.WithRequest(r.Context(), rid)
		// Propagate tenant_id from gateway header (trusted internal boundary).
		// Validate it's a valid UUID; ignore if malformed.
		if tid := r.Header.Get("X-Tenant-ID"); tid != "" {
			if _, err := uuid.Parse(tid); err == nil {
				ctx = tenant.WithTenant(ctx, tid)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Recover catches panics, logs them, and returns 500 to the client
// instead of crashing the goroutine silently.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						slog.Any("panic", rec),
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
						slog.String("stack", string(debug.Stack())),
					)
					http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"服务器内部错误"}}`,
						http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Logger logs every request with timing.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, r)
			log.Info("http",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.status),
				slog.Duration("dur", time.Since(start)),
				slog.String("rid", logger.RequestIDFrom(r.Context())),
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(s int) {
	r.status = s
	r.ResponseWriter.WriteHeader(s)
}