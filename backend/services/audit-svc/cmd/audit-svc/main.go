// audit-svc is the BidWriter compliance & quality audit service.
//
// Responsibilities:
//   - Audit bid content for compliance violations (废标项扫描)
//   - Cross-chapter consistency checks
//   - Chapter-level quality review
//   - Generate audit reports with severity levels
//
// Endpoints:
//   POST /api/v1/audit/bidjobs/{id}/report    — trigger audit for a bid job
//   GET  /api/v1/audit/bidjobs/{id}/report    — get audit report
//   POST /api/v1/audit/bidjobs/{id}/resolve   — resolve an audit issue
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

	"github.com/bidwriter/services/audit-svc/internal/api"
	"github.com/bidwriter/services/audit-svc/internal/config"
	"github.com/bidwriter/services/audit-svc/internal/middleware"
	"github.com/bidwriter/services/audit-svc/internal/service"
	"github.com/bidwriter/services/audit-svc/internal/store"
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

	s := store.New(pool)
	chapterAuditor := service.NewChapterAuditor(s)
	crossAuditor := service.NewCrossAuditor(s)
	rejectionChecker := service.NewRejectionChecker(s)

	h := &api.Handlers{
		Store:            s,
		ChapterAuditor:   chapterAuditor,
		CrossAuditor:     crossAuditor,
		RejectionChecker: rejectionChecker,
		Log:              log,
	}

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
