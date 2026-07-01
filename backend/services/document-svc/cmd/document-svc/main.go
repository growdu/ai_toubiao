// document-svc is the BidWriter document management service.
//
// Responsibilities:
//   - Multipart file upload with SHA256 dedup
//   - Document metadata CRUD with optimistic locking
//   - Storage abstraction (local filesystem / MinIO / S3)
//   - Tenant-scoped access (ADR-0001)
//
// Endpoints:
//   POST   /api/v1/documents       (multipart/form-data: file, project_id, kind)
//   POST   /api/v1/documents/json  (application/json — metadata-only)
//   GET    /api/v1/documents?project_id=&limit=&cursor=
//   GET    /api/v1/documents/{id}
//   GET    /api/v1/documents/{id}/content
//   PATCH  /api/v1/documents/{id}  (version required)
//   DELETE /api/v1/documents/{id}
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

	"github.com/bidwriter/services/document-svc/internal/api"
	"github.com/bidwriter/services/document-svc/internal/config"
	"github.com/bidwriter/services/document-svc/internal/middleware"
	"github.com/bidwriter/services/document-svc/internal/service"
	"github.com/bidwriter/services/document-svc/internal/store"
	"github.com/bidwriter/services/document-svc/internal/storage"
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

	var st storage.Storage
	switch cfg.StorageKind {
	case "local":
		st, err = storage.NewLocal(cfg.StorageDSN)
		if err != nil {
			return fmt.Errorf("storage init: %w", err)
		}
	case "minio", "s3":
		st, err = storage.NewS3(
			ctx,
			cfg.S3Endpoint,
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			cfg.S3Bucket,
			cfg.S3Region,
			cfg.S3UseSSL,
		)
		if err != nil {
			return fmt.Errorf("storage init: %w", err)
		}
		log.Info("object storage backend initialised",
			slog.String("kind", cfg.StorageKind),
			slog.String("endpoint", cfg.S3Endpoint),
			slog.String("bucket", cfg.S3Bucket),
			slog.String("region", cfg.S3Region),
			slog.Bool("use_ssl", cfg.S3UseSSL),
		)
	default:
		return fmt.Errorf("unknown STORAGE_KIND: %s", cfg.StorageKind)
	}

	h := &api.Handlers{
		Store:    store.New(pool),
		Storage:  st,
		Parser:   service.NewParserService(store.New(pool), st, log, cfg.RouterURL),
		Exporter: service.NewExporterService(store.New(pool), st, log),
		Log:      log,
	}

	// In production, apply auth middleware (JWT verification).
	// For now, the api-gateway handles auth and trusts X-Tenant-ID / X-User-ID headers.
	router := h.Routes()
	handler := middleware.RequestID(middleware.Recover(log)(middleware.Logger(log)(router)))

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  180 * time.Second,
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