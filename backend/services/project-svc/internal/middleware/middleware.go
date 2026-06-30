// Package middleware contains HTTP middleware.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bidwriter/services/project-svc/internal/auth"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// RequestID ensures every request has an X-Request-ID (generates one if missing).
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", rid)
		ctx := logger.WithRequest(r.Context(), rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom returns the request id from context (used by error helpers).
func RequestIDFrom(ctx context.Context) string { return logger.RequestIDFrom(ctx) }

// Auth validates JWT and populates tenant_id in context.
func Auth(v *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := logger.RequestIDFrom(r.Context())
			h := r.Header.Get("Authorization")
			if h == "" {
				httperr.Unauthorized(w, rid)
				return
			}
			parts := strings.SplitN(h, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				httperr.Unauthorized(w, rid)
				return
			}
			claims, err := v.Verify(parts[1])
			if err != nil {
				httperr.Unauthorized(w, rid)
				return
			}
			ctx := tenant.WithTenant(r.Context(), claims.TenantID)
			ctx = context.WithValue(ctx, userKey{}, claims.UserID)
			ctx = context.WithValue(ctx, roleKey{}, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type userKey struct{}
type roleKey struct{}

// UserID returns the authenticated user ID from context.
func UserID(ctx context.Context) string { v, _ := ctx.Value(userKey{}).(string); return v }

// Role returns the user's role.
func Role(ctx context.Context) string { v, _ := ctx.Value(roleKey{}).(string); return v }

// Logger logs every request with timing and status.
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