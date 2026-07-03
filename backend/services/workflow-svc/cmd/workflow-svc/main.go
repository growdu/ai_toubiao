// workflow-svc is the BidWriter bid pipeline workflow orchestrator.
//
// Responsibilities:
//   - Workflow aggregate (one per project, re-runs allowed)
//   - Bid state machine per docs/architecture/state-machine.md
//   - Per-step progress tracking (parsing → outlining → facts → generating → auditing → exporting)
//   - Append-only event log of all transitions
//   - Tenant-scoped access (ADR-0001)
//   - Asynq worker pool for async pipeline execution
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
	"github.com/bidwriter/services/workflow-svc/internal/workers"
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

	// Build the workers config from service config.
	wcfg := workers.Config{
		RedisAddr:    cfg.RedisAddr,
		RouterURL:    cfg.RouterURL,
		KnowledgeURL: cfg.KnowledgeURL,
		DocumentURL:  cfg.DocumentURL,
		AuditURL:     cfg.AuditURL,
	}

	// Create the Asynq client for enqueuing tasks from HTTP handlers.
	asynqClient := workers.NewClient(wcfg)
	defer asynqClient.Close()

	// Try to wire LibreOffice for PDF export.
	pdf := api.NewLibreOfficeConverter(cfg.LibreOfficeBin)
	if !pdf.Available() {
		log.Warn("libreoffice not found; /export/pdf will fall back to docx")
	}

	h := &api.Handlers{
		Store:        store.New(pool),
		Log:          log,
		DocBuilder:   nil, // use default ooxmlBuilder
		PDFConverter: pdf,
		Enqueuer:     api.NewAsynqEnqueuer(asynqClient, pool, log),
		ChapterPool:  pool,
	}
	router := h.Routes()
	handler := middleware.RequestID(middleware.Recover(log)(middleware.Logger(log)(router)))

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Start the Asynq worker server in a goroutine.
	workerErr := make(chan error, 1)
	go func() {
		if err := workers.Serve(ctx, log, pool, wcfg); err != nil {
			workerErr <- fmt.Errorf("workers: %w", err)
		}
	}()

	// Start the HTTP server in a goroutine.
	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", slog.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or a fatal error from either server.
	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case err := <-workerErr:
		return fmt.Errorf("worker server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	// Graceful shutdown: stop accepting new HTTP requests, then wait for
	// in-flight requests to finish. The Asynq server shuts down via the
	// ctx cancellation in workers.Serve.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown error", slog.Any("error", err))
	}
	return nil
}
