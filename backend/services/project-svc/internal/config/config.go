// Package config loads service configuration from env vars.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServiceName string
	HTTPAddr    string
	DSN         string
	JWTSecret   string
	LogLevel    string
	ReadTimeout time.Duration
}

func Load() (*Config, error) {
	c := &Config{
		ServiceName: getEnv("SERVICE_NAME", "project-svc"),
		HTTPAddr:    getEnv("HTTP_ADDR", ":8081"),
		DSN:         getEnv("DB_DSN", "postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", ""),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}

	if c.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	if v := os.Getenv("HTTP_READ_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP_READ_TIMEOUT: %w", err)
		}
		c.ReadTimeout = d
	} else {
		c.ReadTimeout = 30 * time.Second
	}

	if v := os.Getenv("SERVICE_NAME"); v != "" {
		c.ServiceName = v
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