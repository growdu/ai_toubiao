// Package middleware contains HTTP cross-cutting concerns used by router-svc.
package middleware

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
)

// statusRecorder wraps ResponseWriter so handlers can set their own status.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Hijack/Pass-through for websockets or flushing endpoints.
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := s.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("hijacker not supported")
}

// Chain wraps handlers with the given middlewares (outermost first).
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// RequestID injects / propagates X-Request-ID.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", rid)
		ctx := context.WithValue(r.Context(), requestIDKey{}, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type requestIDKey struct{}

// RequestIDFrom returns the request ID stamped by the RequestID middleware.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}

// Recovery catches panics and returns a 500 JSON response.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[router] panic: %v\n%s", rec, debug.Stack())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"code":    "INTERNAL",
						"message": "internal server error",
					},
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logger logs request start/end with method, path, status, latency.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		log.Printf("[router] %s %s -> %d (%d bytes) %s",
			r.Method, r.URL.Path, rec.status, rec.bytes, time.Since(start))
	})
}

// CORS sets minimal permissive CORS headers (development-friendly).
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// JWTAuth validates a Bearer JWT using the supplied secret.
//
// Modes:
//   - requireAuth=false: no-op middleware; any caller passes. Used in dev/local.
//   - requireAuth=true + secret=="": misconfiguration (caught at Validate()).
//   - requireAuth=true + secret set: HS256 signature + exp verified; claims
//     attached to request context.
//
// In production, set AUTH_REQUIRED=true and JWT_SECRET=<random>. The api-gateway
// already validates tokens; this layer exists to defend services that are
// occasionally exposed directly (smoke tests, debugging).
func JWTAuth(requireAuth bool, secret string) func(http.Handler) http.Handler {
	if !requireAuth {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]string{"code": "UNAUTHORIZED", "message": "missing bearer token"},
				})
				return
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err := parseJWT(tok, secret)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]string{"code": "UNAUTHORIZED", "message": err.Error()},
				})
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type claimsKey struct{}

// ClaimsFrom returns JWT claims attached by JWTAuth (or nil when running in
// permissive mode).
func ClaimsFrom(ctx context.Context) *Claims {
	v, _ := ctx.Value(claimsKey{}).(*Claims)
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}