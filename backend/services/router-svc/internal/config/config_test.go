package config_test

import (
	"os"
	"testing"

	"github.com/bidwriter/services/router-svc/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear env so we observe defaults.
	for _, k := range []string{"PORT", "DATABASE_URL", "AUTH_REQUIRED", "JWT_SECRET",
		"ROUTES_CONFIG_PATH", "CACHE_MAX_ENTRIES", "CACHE_DEFAULT_TTL",
		"BATCHER_INTERVAL", "BATCHER_MAX_BUFFER", "PROVIDER_TIMEOUT", "ALLOW_MOCK_PROVIDER"} {
		os.Unsetenv(k)
	}
	cfg := config.Load()

	if cfg.Port != "8085" {
		t.Errorf("port default: %s", cfg.Port)
	}
	if cfg.RequireAuth {
		t.Error("auth should default to false (dev-friendly)")
	}
	if cfg.AllowMockProvider != true {
		t.Error("mock provider should default to true")
	}
	if cfg.CacheMaxEntries != 1024 {
		t.Errorf("cache default: %d", cfg.CacheMaxEntries)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("default validate: %v", err)
	}
}

func TestLoad_Overrides(t *testing.T) {
	os.Setenv("PORT", "9999")
	os.Setenv("AUTH_REQUIRED", "true")
	os.Setenv("JWT_SECRET", "real-secret")
	os.Setenv("CACHE_MAX_ENTRIES", "500")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("AUTH_REQUIRED")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("CACHE_MAX_ENTRIES")
	}()

	cfg := config.Load()
	if cfg.Port != "9999" {
		t.Errorf("port override: %s", cfg.Port)
	}
	if !cfg.RequireAuth {
		t.Error("auth should be true")
	}
	if cfg.JWTSecret != "real-secret" {
		t.Errorf("jwt secret override: %s", cfg.JWTSecret)
	}
	if cfg.CacheMaxEntries != 500 {
		t.Errorf("cache override: %d", cfg.CacheMaxEntries)
	}
}

func TestValidate_AuthRequiredWithoutSecret(t *testing.T) {
	cfg := config.Load()
	cfg.RequireAuth = true
	cfg.JWTSecret = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when AUTH=true with empty secret")
	}
}

func TestValidate_NoDatabase(t *testing.T) {
	cfg := config.Load()
	cfg.DatabaseURL = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when DATABASE_URL missing")
	}
}

func TestDSN_RedactsPassword(t *testing.T) {
	cfg := config.Load()
	cfg.DatabaseURL = "postgres://user:hunter2@db:5432/bidwriter"
	got := cfg.DSN()
	if got != "postgres://user:***@db:5432/bidwriter" {
		t.Errorf("redacted dsn: %s", got)
	}
}