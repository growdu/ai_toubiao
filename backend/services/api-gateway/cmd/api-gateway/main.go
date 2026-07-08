// api-gateway is the single public entry point for BidWriter.
//
// Responsibilities:
//   - JWT login + refresh
//   - Per-tenant rate limiting
//   - Reverse-proxying /api/v1/* to upstream services
//   - Request ID propagation
//   - CORS for the web frontend
//
// Routing table:
//   POST /api/v1/auth/login         -> handled locally (DB lookup)
//   POST /api/v1/auth/refresh       -> handled locally (TODO: revocation list)
//   /api/v1/projects/*              -> project-svc
//   /api/v1/documents/*             -> document-svc
//   /api/v1/bids/*                  -> workflow-svc (includes /export/{word,pdf})
//   /api/v1/kb/*                    -> knowledge-svc
//   /api/v1/docgen/*               -> docgen-svc
//   /api/v1/audit/*                -> audit-svc
//   /api/v1/templates/*            -> template-svc
//   /api/v1/billing/*              -> billing-svc
//   /api/v1/notifications/*        -> notify-svc
// Note: the public prefix is /api/v1/kb/* (not /api/v1/knowledge/*)
// because knowledge-svc mounts its handlers under /api/v1/kb/* internally
// and api-gateway forwards the path unchanged. Using a matching public
// prefix keeps the path consistent end-to-end and avoids a rewrite seam.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bidwriter/services/api-gateway/internal/auth"
	"github.com/bidwriter/services/api-gateway/internal/config"
	"github.com/bidwriter/services/api-gateway/internal/proxy"
	"github.com/bidwriter/services/api-gateway/internal/ratelimit"
	"github.com/bidwriter/shared/pkg/db"
	"github.com/bidwriter/shared/pkg/httperr"
	sharedlogger "github.com/bidwriter/shared/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log := sharedlogger.New("api-gateway")
	slog.SetDefault(log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.New(ctx, db.DefaultConfig(cfg.DBDSN))
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()
	log.Info("db connected")

	authSvc := auth.New(pool, cfg.JWTSecret, cfg.JWTTTL, cfg.RefreshTTL)
	limiter := ratelimit.New(cfg.RateLimitPerMin, time.Minute)

	mux := http.NewServeMux()

	// ---- Auth endpoints (unauthenticated) ----
	mux.HandleFunc("/api/v1/auth/login", loginHandler(authSvc, log))
	mux.HandleFunc("/api/v1/auth/register", registerHandler(authSvc, log))
	mux.HandleFunc("/api/v1/auth/refresh", refreshHandler(authSvc))

	// ---- Health ----
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	// ---- Protected proxy routes ----
	routes, err := buildRoutes(cfg)
	if err != nil {
		return fmt.Errorf("routes: %w", err)
	}
	proxiedHandler := proxy.New(routes)
	proxyWithAuth := authMiddleware(authSvc, limiter, proxiedHandler, log)

	// Mount under chi for nicer routing
	r := chi.NewRouter()
	r.Use(corsMiddleware)
	r.Handle("/api/v1/auth/*", mux)
	r.HandleFunc("/api/v1/auth/login", loginHandler(authSvc, log))
	r.HandleFunc("/api/v1/auth/register", registerHandler(authSvc, log))
	r.HandleFunc("/api/v1/auth/refresh", refreshHandler(authSvc))
	r.Handle("/healthz", mux)
	r.Handle("/api/v1/projects", proxyWithAuth)
	r.Handle("/api/v1/projects/*", proxyWithAuth)
	r.Handle("/api/v1/documents", proxyWithAuth)
	r.Handle("/api/v1/documents/*", proxyWithAuth)
	r.Handle("/api/v1/bids", proxyWithAuth)
	r.Handle("/api/v1/bids/*", proxyWithAuth)
	r.Handle("/api/v1/kb", proxyWithAuth)
	r.Handle("/api/v1/kb/*", proxyWithAuth)
	r.Handle("/api/v1/docgen", proxyWithAuth)
	r.Handle("/api/v1/docgen/*", proxyWithAuth)
	r.Handle("/api/v1/audit", proxyWithAuth)
	r.Handle("/api/v1/audit/*", proxyWithAuth)
	r.Handle("/api/v1/templates", proxyWithAuth)
	r.Handle("/api/v1/templates/*", proxyWithAuth)
	r.Handle("/api/v1/billing", proxyWithAuth)
	r.Handle("/api/v1/billing/*", proxyWithAuth)
	r.Handle("/api/v1/notifications", proxyWithAuth)
	r.Handle("/api/v1/notifications/*", proxyWithAuth)
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.NotFound(w, req)
	})

	handler := requestIDMiddleware(log, r)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", slog.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return srv.Shutdown(shutdownCtx)
}

// ---- middleware ----

func requestIDMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", rid)
		ctx := sharedlogger.WithRequest(r.Context(), rid)
		start := time.Now()
		next.ServeHTTP(w, r.WithContext(ctx))
		log.Info("http",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Duration("dur", time.Since(start)),
			slog.String("rid", rid),
		)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(svc *auth.Service, limiter *ratelimit.Limiter, next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := sharedlogger.RequestIDFrom(r.Context())
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
		claims, err := svc.Verify(parts[1])
		if err != nil {
			httperr.Unauthorized(w, rid)
			return
		}

		// Rate limit per tenant
		if !limiter.Allow(claims.TenantID) {
			httperr.Write(w, http.StatusTooManyRequests, httperr.CodeRateLimited,
				"请求过于频繁", rid, nil)
			return
		}

		// Propagate user identity to upstream
		r.Header.Set("X-Tenant-ID", claims.TenantID)
		r.Header.Set("X-User-ID", claims.UserID)
		r.Header.Set("X-User-Role", claims.Role)

		next.ServeHTTP(w, r)
	})
}

// ---- handlers ----

// buildRoutes constructs the proxy route table from upstream URLs. The
// function is exported at package scope (not method-scoped) so that
// cmd/api-gateway/main_test.go can exercise it without standing up the full
// server. Adding a new upstream service means: add a Config field, add an
// entry here, and add the matching chi.Handle() line in run().
func buildRoutes(cfg *config.Config) ([]proxy.Route, error) {
	parse := func(envName, raw string) (*url.URL, error) {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", envName, err)
		}
		return u, nil
	}

	projectURL, err := parse("PROJECT_SVC_URL", cfg.ProjectSvcURL)
	if err != nil {
		return nil, err
	}
	documentURL, err := parse("DOCUMENT_SVC_URL", cfg.DocumentSvcURL)
	if err != nil {
		return nil, err
	}
	workflowURL, err := parse("WORKFLOW_SVC_URL", cfg.WorkflowSvcURL)
	if err != nil {
		return nil, err
	}
	knowledgeURL, err := parse("KNOWLEDGE_SVC_URL", cfg.KnowledgeSvcURL)
	if err != nil {
		return nil, err
	}

	docgenURL, err := parse("DOCGEN_SVC_URL", cfg.DocgenSvcURL)
	if err != nil {
		return nil, err
	}
	auditURL, err := parse("AUDIT_SVC_URL", cfg.AuditSvcURL)
	if err != nil {
		return nil, err
	}
	templateURL, err := parse("TEMPLATE_SVC_URL", cfg.TemplateSvcURL)
	if err != nil {
		return nil, err
	}
	billingURL, err := parse("BILLING_SVC_URL", cfg.BillingSvcURL)
	if err != nil {
		return nil, err
	}
	notifyURL, err := parse("NOTIFY_SVC_URL", cfg.NotifySvcURL)
	if err != nil {
		return nil, err
	}

	return []proxy.Route{
		{Prefix: "/api/v1/projects", Upstream: projectURL},
		{Prefix: "/api/v1/documents", Upstream: documentURL},
		{Prefix: "/api/v1/bids", Upstream: workflowURL},
		{Prefix: "/api/v1/kb", Upstream: knowledgeURL},
		{Prefix: "/api/v1/docgen", Upstream: docgenURL},
		{Prefix: "/api/v1/audit", Upstream: auditURL},
		{Prefix: "/api/v1/templates", Upstream: templateURL},
		{Prefix: "/api/v1/billing", Upstream: billingURL},
		{Prefix: "/api/v1/notifications", Upstream: notifyURL},
	}, nil
}

func loginHandler(svc *auth.Service, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rid := sharedlogger.RequestIDFrom(r.Context())
		var req struct {
			TenantSlug string `json:"tenant_slug"`
			Email      string `json:"email"`
			Password   string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.InvalidInput(w, rid, "invalid JSON", nil)
			return
		}
		if req.TenantSlug == "" || req.Email == "" || req.Password == "" {
			httperr.InvalidInput(w, rid, "tenant_slug, email, password are required", nil)
			return
		}

		user, err := svc.Login(r.Context(), req.TenantSlug, req.Email, req.Password)
		if errors.Is(err, auth.ErrInvalidCredentials) {
			httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized,
				"邮箱或密码错误", rid, nil)
			return
		}
		if err != nil {
			log.Error("login failed", slog.String("err", err.Error()))
			httperr.InternalError(w, rid)
			return
		}

		access, refresh, ttl, err := svc.IssueTokens(user)
		if err != nil {
			log.Error("issue tokens", slog.String("err", err.Error()))
			httperr.InternalError(w, rid)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  access,
			"refresh_token": refresh,
			"expires_in":    ttl,
			"token_type":    "Bearer",
			"user": map[string]any{
				"id":    user.ID,
				"email": user.Email,
				"role":  user.Role,
			},
		})
	}
}

// registerHandler handles POST /api/v1/auth/register. On success it
// returns the same shape as loginHandler so the front-end can drop the
// user straight into /bids with no extra round-trip.
func registerHandler(svc *auth.Service, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rid := sharedlogger.RequestIDFrom(r.Context())
		var req auth.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.InvalidInput(w, rid, "invalid JSON: "+err.Error(), nil)
			return
		}
		result, err := svc.Register(r.Context(), req)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrInvalidInput):
				// Surface the wrapped message so the user knows which
				// field is bad; in the future we may want a structured
				// per-field error response.
				httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidInput, err.Error(), rid, nil)
				return
			case errors.Is(err, auth.ErrTenantSlugTaken):
				httperr.Write(w, http.StatusConflict, httperr.CodeAlreadyExists,
					"该工作区标识已被占用，请换一个", rid, nil)
				return
			case errors.Is(err, auth.ErrEmailTaken):
				httperr.Write(w, http.StatusConflict, httperr.CodeAlreadyExists,
					"该邮箱在此工作区已注册，请直接登录或换一个邮箱", rid, nil)
				return
			}
			log.Error("register failed", slog.String("err", err.Error()))
			httperr.InternalError(w, rid)
			return
		}

		access, refresh, ttl, err := svc.IssueTokens(result.User)
		if err != nil {
			log.Error("issue tokens after register", slog.String("err", err.Error()))
			httperr.InternalError(w, rid)
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"access_token":  access,
			"refresh_token": refresh,
			"expires_in":    ttl,
			"token_type":    "Bearer",
			"tenant": map[string]any{
				"id":   result.TenantID,
				"name": result.TenantName,
				"slug": result.TenantSlug,
				"plan": result.Plan,
			},
			"user": map[string]any{
				"id":    result.User.ID,
				"email": result.User.Email,
				"role":  result.User.Role,
			},
		})
	}
}

func refreshHandler(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rid := sharedlogger.RequestIDFrom(r.Context())
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
			httperr.InvalidInput(w, rid, "refresh_token required", nil)
			return
		}

		// For now, accept any refresh token signed by us (verified loosely).
		// In production, maintain a refresh-token table for revocation.
		// Reuse IssueTokens by re-verifying the refresh token.
		// (Implementation simplified: we accept the existing user identity from the refresh claim.)
		_ = svc // TODO proper refresh-token revocation list

		w.WriteHeader(http.StatusNotImplemented)
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": map[string]any{
				"code":    "NOT_IMPLEMENTED",
				"message": "refresh token flow is TODO (need revocation list)",
			},
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}