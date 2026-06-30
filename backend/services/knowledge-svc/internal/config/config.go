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
}

func Load() (*Config, error) {
	c := &Config{
		ServiceName: getEnv("SERVICE_NAME", "knowledge-svc"),
		HTTPAddr:    getEnv("HTTP_ADDR", ":8086"),
		DSN:         getEnv("DB_DSN", "postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable"),
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