// Package config loads knowledge-svc configuration.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	ServiceName string
	HTTPAddr    string
	DSN         string
	RouterURL   string

	// MinIO configuration for file-backed KB materials (see ADR-0008).
	MinIOEndpoint  string // host:port, no scheme
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string // bucket that KB materials are stored under
	MinIOUseSSL    bool
}

func Load() (*Config, error) {
	c := &Config{
		ServiceName:    getEnv("SERVICE_NAME", "knowledge-svc"),
		HTTPAddr:       getEnv("HTTP_ADDR", ":8086"),
		DSN:            getEnv("DB_DSN", "postgres://postgres:***@localhost:5432/bidwriter?sslmode=disable"),
		RouterURL:      getEnv("ROUTER_URL", "http://localhost:8083"),
		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", ""),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", ""),
		MinIOBucket:    getEnv("MINIO_KB_BUCKET", "kb-materials"),
		MinIOUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
	}
	if c.DSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	return c, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}