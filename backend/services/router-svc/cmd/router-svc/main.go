// router-svc — bidwriter AI routing service.
//
// Responsibilities:
//   - Resolve task → provider/model via the YAML route table
//   - Fall back to cheaper/secondary providers on failure
//   - Enforce per-tenant monthly USD budgets
//   - Cache identical prompts to reduce cost
//   - Emit structured call logs (batched into router_call_logs)
//
// Port: 8085 (overridable via PORT).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bidwriter/services/router-svc/internal/api"
	"github.com/bidwriter/services/router-svc/internal/config"
	"github.com/bidwriter/services/router-svc/internal/middleware"
	"github.com/bidwriter/services/router-svc/internal/provider"
	"github.com/bidwriter/services/router-svc/internal/router"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	log.Printf("router-svc starting (port=%s, dsn=%s)", cfg.Port, cfg.DSN())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database (optional — service degrades gracefully without it).
	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		pcfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("invalid DATABASE_URL: %v", err)
		}
		pcfg.MaxConns = 10
		pcfg.MinConns = 1
		pcfg.MaxConnLifetime = 30 * time.Minute
		pcfg.HealthCheckPeriod = 30 * time.Second

		pool, err = pgxpool.NewWithConfig(ctx, pcfg)
		if err != nil {
			log.Printf("warning: could not connect to db at startup: %v (continuing without DB)", err)
		} else {
			log.Printf("connected to postgres (max=%d)", pcfg.MaxConns)
			defer pool.Close()
		}
	}

	// Provider registry.
	registry := buildRegistry(cfg)
	log.Printf("registered providers: %v", registry.Names())

	// Routes — try YAML, fall back to defaults.
	routes := router.DefaultRoutes()
	if _, err := os.Stat(cfg.RoutesConfigPath); err == nil {
		parsed, err := router.LoadRoutesConfig(cfg.RoutesConfigPath)
		if err != nil {
			log.Printf("warning: failed to load %s (%v); using defaults", cfg.RoutesConfigPath, err)
		} else {
			routes = parsed
			log.Printf("loaded %d routes from %s", len(routes.Routes), cfg.RoutesConfigPath)
		}
	}
	routesStore := router.NewInMemoryRoutesStore(routes)

	// Cache + budget.
	cache := router.NewLRUCache(cfg.CacheMaxEntries)
	budget := router.NewBudgetChecker()

	// Call-log batcher (only useful when DB is connected).
	batcher := router.NewCallLogBatcher(pool, cfg.BatcherInterval, cfg.BatcherMaxBuffer)
	defer batcher.Stop()

	// Build the routing core.
	r := router.New(registry, routesStore, cache, budget, batcher.Add)

	// HTTP layer.
	handlers := &api.Handlers{
		Router:      r,
		Registry:    registry,
		RoutesStore: routesStore,
		Budget:      budget,
		Cache:       cache,
		Batcher:     batcher,
		Pool:        pool,
	}

	// Middleware stack: recovery → request-id → logger → CORS → JWT.
	handler := middleware.Chain(
		handlers.Routes(),
		middleware.Recovery,
		middleware.RequestID,
		middleware.Logger,
		middleware.CORS,
		middleware.JWTAuth(cfg.RequireAuth, cfg.JWTSecret),
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      300 * time.Second, // AI calls can be slow
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutdown signal received")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
		cancel()
	}()

	log.Printf("listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	log.Println("router-svc stopped")
}

// buildRegistry constructs the provider registry from environment variables.
// Real provider URLs/keys are opt-in via env vars; otherwise a mock is
// registered so the service always has at least one routable target.
func buildRegistry(cfg config.Config) provider.Registry {
	providers := map[string]provider.Provider{}

	// Always-available mock provider (dev/test).
	if cfg.AllowMockProvider {
		providers["mock"] = provider.NewMockProvider()
	}

	// Anthropic
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		baseURL := os.Getenv("ANTHROPIC_BASE_URL")
		providers["anthropic"] = provider.NewAnthropicProvider(
			key, baseURL,
			provider.Pricing{InputPerMTokensUSD: 3.0, OutputPerMTokensUSD: 15.0},
		)
	}

	// OpenAI
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		baseURL := os.Getenv("OPENAI_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		providers["openai"] = provider.NewOpenAICompatible(
			"openai", baseURL, key,
			provider.Pricing{InputPerMTokensUSD: 2.5, OutputPerMTokensUSD: 10.0},
		)
	}

	// DeepSeek (OpenAI-compatible)
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		baseURL := os.Getenv("DEEPSEEK_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}
		providers["deepseek"] = provider.NewOpenAICompatible(
			"deepseek", baseURL, key,
			provider.Pricing{InputPerMTokensUSD: 0.14, OutputPerMTokensUSD: 0.28},
		)
	}

	// Ollama (local, no API key)
	if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
		providers["ollama"] = provider.NewOpenAICompatible(
			"ollama", baseURL, "",
			provider.Pricing{InputPerMTokensUSD: 0.0, OutputPerMTokensUSD: 0.0},
		)
	}

	return provider.NewMapRegistry(providers)
}