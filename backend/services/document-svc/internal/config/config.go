// Package config loads document-svc configuration.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	ServiceName string
	HTTPAddr    string
	DSN         string
	StorageKind string // "minio" or "local"
	StorageDSN  string // s3://... or /var/lib/...
}

func Load() (*Config, error) {
	c := &Config{
		ServiceName: getEnv("SERVICE_NAME", "document-svc"),
		HTTPAddr:    getEnv("HTTP_ADDR", ":8082"),
		DSN:         getEnv("DB_DSN", "postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable"),
		StorageKind: getEnv("STORAGE_KIND", "local"),
		StorageDSN:  getEnv("STORAGE_DSN", "/tmp/bidwriter-storage"),
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