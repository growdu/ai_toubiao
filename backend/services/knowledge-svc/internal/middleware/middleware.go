// Package middleware provides HTTP middleware for knowledge-svc.
package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/bidwriter/shared/pkg/logger"
	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "rid"

// RequestID middleware extracts or generates a request ID.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), RequestIDKey, rid)
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger middleware logs request details.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := logger.RequestIDFrom(r.Context())
			log.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("rid", rid),
			)
			next.ServeHTTP(w, r)
		})
	}
}

// Recover middleware recovers from panics.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic recovered", slog.Any("panic", p))
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}