// workflow-svc is the BidWriter bid pipeline workflow orchestrator.
//
// Responsibilities:
//   - Workflow aggregate (one per project, re-runs allowed)
//   - Bid state machine per docs/architecture/state-machine.md
//   - Per-step progress tracking (parsing → outlining → facts → generating → auditing → exporting)
//   - Append-only event log of all transitions
//   - Tenant-scoped access (ADR-0001)
//
// Endpoints:
//   POST   /api/v1/workflows                         — create new workflow for a project
//   GET    /api/v1/workflows?project_id=&status=    — list
//   GET    /api/v1/workflows/{id}
//   POST   /api/v1/workflows/{id}/transition        — change state
//   GET    /api/v1/workflows/{id}/steps             — per-step progress
//   GET    /api/v1/workflows/{id}/events            — audit log
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

	"github.com/bidwriter/services/workflow-svc/internal/api"
	"github.com/bidwriter/services/workflow-svc/internal/config"
	"github.com/bidwriter/services/workflow-svc/internal/middleware"
	"github.com/bidwriter/services/workflow-svc/internal/store"
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.New(ctx, db.DefaultConfig(cfg.DSN))
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()

	h := &api.Handlers{Store: store.New(pool), Log: log}
	router := h.Routes()
	handler := middleware.RequestID(middleware.Recover(log)(middleware.Logger(log)(router)))

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	return srv.Shutdown(shutdownCtx)
}