// Package config loads router-svc runtime configuration from environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime knobs.
type Config struct {
	Port              string
	DatabaseURL       string
	RequireAuth       bool          // when false, router-svc accepts unauthenticated calls (dev/local)
	JWTSecret         string        // HS256 secret; required when RequireAuth=true
	RoutesConfigPath  string        // path to YAML route definitions
	CacheMaxEntries   int           // L1 cache size
	CacheDefaultTTL   time.Duration // default cache TTL when route spec omits it
	BatcherInterval   time.Duration // call-log flush interval
	BatcherMaxBuffer  int           // flush when buffer reaches this size
	ProviderTimeout   time.Duration // default per-call timeout
	AllowMockProvider bool          // expose a deterministic mock provider (dev/test only)
}

// Load reads configuration from the environment, applying sensible defaults.
// Defaults are dev-friendly: AUTH off, mock provider on.
func Load() Config {
	return Config{
		Port:              getEnv("PORT", "8085"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://bidwriter:***@localhost:5432/bidwriter?sslmode=disable"),
		RequireAuth:       getBoolEnv("AUTH_REQUIRED", false),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret-change-me"),
		RoutesConfigPath:  getEnv("ROUTES_CONFIG_PATH", "configs/routes.yaml"),
		CacheMaxEntries:   getIntEnv("CACHE_MAX_ENTRIES", 1024),
		CacheDefaultTTL:   getDurationEnv("CACHE_DEFAULT_TTL", 24*time.Hour),
		BatcherInterval:   getDurationEnv("BATCHER_INTERVAL", 5*time.Second),
		BatcherMaxBuffer:  getIntEnv("BATCHER_MAX_BUFFER", 100),
		ProviderTimeout:   getDurationEnv("PROVIDER_TIMEOUT", 120*time.Second),
		AllowMockProvider: getBoolEnv("ALLOW_MOCK_PROVIDER", true),
	}
}

// DSN returns a redacted DSN suitable for logging.
func (c Config) DSN() string {
	return redactDSN(c.DatabaseURL)
}

// redactDSN replaces the password in a Postgres-style DSN with ***.
// Handles URLs of the form:  scheme://user:password@host:port/db?...
// Returns the input unchanged when no password segment is present.
func redactDSN(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	if at <= 0 {
		return dsn
	}
	// Find the colon that separates user from password.
	// It must appear between the scheme separator ('://') and '@'.
	schemeSep := strings.Index(dsn, "://")
	if schemeSep < 0 {
		return dsn
	}
	// After '://' and before '@', find the last ':' (the user/pass separator).
	head := dsn[schemeSep+3 : at]
	if c := strings.LastIndex(head, ":"); c >= 0 {
		return dsn[:schemeSep+3+c+1] + "***" + dsn[at:]
	}
	return dsn
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getBoolEnv(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// Validate ensures the config is sane enough to boot.
func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.RequireAuth && c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required when AUTH_REQUIRED=true")
	}
	if c.CacheMaxEntries <= 0 {
		return fmt.Errorf("CACHE_MAX_ENTRIES must be > 0")
	}
	if c.BatcherInterval <= 0 {
		return fmt.Errorf("BATCHER_INTERVAL must be > 0")
	}
	return nil
}