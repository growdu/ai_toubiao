// project-svc is the BidWriter project management service.
//
// Responsibilities:
//   - CRUD on Project aggregates
//   - Tenant-scoped access
//   - Soft delete + optimistic locking
//
// Endpoints (full contract in docs/api/rest.md):
//   GET    /healthz
//   GET    /readyz
//   GET    /api/v1/projects
//   POST   /api/v1/projects
//   GET    /api/v1/projects/{id}
//   PATCH  /api/v1/projects/{id}
//   DELETE /api/v1/projects/{id}
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bidwriter/services/project-svc/internal/api"
	"github.com/bidwriter/services/project-svc/internal/auth"
	"github.com/bidwriter/services/project-svc/internal/config"
	"github.com/bidwriter/services/project-svc/internal/middleware"
	"github.com/bidwriter/services/project-svc/internal/store"
	"github.com/bidwriter/shared/pkg/db"
	sharedlogger "github.com/bidwriter/shared/pkg/logger"
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

	log := sharedlogger.New(cfg.ServiceName)
	slog.SetDefault(log)

	log.Info("starting",
		slog.String("addr", cfg.HTTPAddr),
		slog.String("dsn", redactDSN(cfg.DSN)),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Connect to DB
	pool, err := db.New(ctx, db.DefaultConfig(cfg.DSN))
	if err != nil {
		return fmt.Errorf("db connect: %w", err)
	}
	defer pool.Close()
	log.Info("db connected")

	// Wire dependencies
	st := store.New(pool)
	verifier := auth.NewVerifier(cfg.JWTSecret)
	h := &api.Handlers{Store: st, Log: log}

	// Build router
	router := h.Routes(middleware.Auth(verifier))
	handler := middleware.RequestID(middleware.Logger(log)(router))

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Run server
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	log.Info("listening", slog.String("addr", cfg.HTTPAddr))

	select {
	case err := <-serverErr:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("shutdown complete")
	return nil
}

// redactDSN masks the password in a DSN for safe logging.
func redactDSN(dsn string) string {
	// postgres://user:pass@host:port/db -> postgres://user:***@host:port/db
	out := []rune{}
	atIdx := -1
	for i, r := range dsn {
		out = append(out, r)
		if r == '@' {
			atIdx = i
		}
	}
	if atIdx < 0 {
		return dsn
	}
	// Find the scheme://user:pass portion
	colonIdx := -1
	for i := 0; i < atIdx; i++ {
		if out[i] == ':' && i >= 2 && out[i-1] != '/' {
			// skip the "://" colon
			// walk forward from start to find last colon before @
		}
	}
	_ = colonIdx
	// Simple replacement: find "user:pass" between "://" and "@"
	const sep = "://"
	idx := -1
	for i := 0; i+len(sep) <= len(dsn); i++ {
		if dsn[i:i+len(sep)] == sep {
			idx = i + len(sep)
			break
		}
	}
	if idx < 0 {
		return dsn
	}
	at := -1
	for i := idx; i < len(dsn); i++ {
		if dsn[i] == '@' {
			at = i
			break
		}
	}
	if at < 0 {
		return dsn
	}
	return dsn[:idx] + "***" + dsn[at:]
}