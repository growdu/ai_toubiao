// notify-svc handles notifications via email, DingTalk, and WeCom.
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

	"github.com/bidwriter/services/notify-svc/internal/api"
	"github.com/bidwriter/services/notify-svc/internal/config"
	"github.com/bidwriter/services/notify-svc/internal/middleware"
	"github.com/bidwriter/services/notify-svc/internal/service"
	"github.com/bidwriter/services/notify-svc/internal/store"
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
	cfg := config.Load()

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
	notifier := service.NewNotifier(
		service.NewSMTPDialer(service.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     env("SMTP_FROM", "noreply@bidwriter.local"),
		}),
		service.NewHTTPTransport(10*time.Second),
		env("SMTP_FROM", "noreply@bidwriter.local"),
		env("SMTP_SUBJECT_PREFIX", "[bidwriter] "),
	)
	svc := service.NewNotifyService(s, notifier)
	h := &api.Handlers{Service: svc, Log: log}
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

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireEnvOrLog reads a required env var and returns an error
// instead of panicking. It is not currently called by the startup path
// because notify-svc uses env-with-fallback for every setting, but it
// stays here as a safe seam for future strictness — callers that need
// hard-fail behaviour should do:
//
//     val, err := requireEnvOrLog("FOO")
//     if err != nil { return fmt.Errorf("config: %w", err) }
//
// We deliberately do NOT panic in this helper because config loaders
// that panic during request-time (rather than startup) take the whole
// process down for what is usually a single bad request.
func requireEnvOrLog(key string) (string, error) {
	if v := os.Getenv(key); v != "" {
		return v, nil
	}
	return "", errors.New("required env var not set: " + key)
}
