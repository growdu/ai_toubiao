// Package config loads api-gateway configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all settings.
type Config struct {
	HTTPAddr        string
	JWTSecret       string
	JWTTTL          time.Duration
	RefreshTTL      time.Duration
	RateLimitPerMin int
	DBDSN           string

	// Upstream service addresses (set by env, defaults match docker-compose).
	ProjectSvcURL string
	DocumentSvcURL string
	WorkflowSvcURL string
}

func Load() (*Config, error) {
	c := &Config{
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTTTL:          getEnvDuration("JWT_TTL", time.Hour),
		RefreshTTL:      getEnvDuration("REFRESH_TTL", 30*24*time.Hour),
		RateLimitPerMin: getEnvInt("RATE_LIMIT_PER_MIN", 60),
		DBDSN:           getEnv("DB_DSN", "postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable"),
		ProjectSvcURL:   getEnv("PROJECT_SVC_URL", "http://localhost:8081"),
		DocumentSvcURL:  getEnv("DOCUMENT_SVC_URL", "http://localhost:8082"),
		WorkflowSvcURL:  getEnv("WORKFLOW_SVC_URL", "http://localhost:8083"),
	}

	if c.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if v := os.Getenv("JWT_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_TTL: %w", err)
		}
		c.JWTTTL = d
	}
	return c, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getEnvDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}