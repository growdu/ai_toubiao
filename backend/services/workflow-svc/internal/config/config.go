// Package config loads workflow-svc configuration.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	ServiceName    string
	HTTPAddr       string
	DSN            string
	RedisAddr      string
	RouterURL      string
	KnowledgeURL   string
	DocumentURL    string
	AuditURL       string
	LibreOfficeBin string
}

func Load() (*Config, error) {
	c := &Config{
		ServiceName:    getEnv("SERVICE_NAME", "workflow-svc"),
		HTTPAddr:       getEnv("HTTP_ADDR", ":9083"),
		DSN:            getEnv("DB_DSN", "postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RouterURL:      getEnv("ROUTER_URL", "http://localhost:8083"),
		KnowledgeURL:   getEnv("KNOWLEDGE_URL", "http://localhost:8086"),
		DocumentURL:    getEnv("DOCUMENT_URL", "http://localhost:8082"),
		AuditURL:       getEnv("AUDIT_URL", "http://localhost:8087"),
		LibreOfficeBin: getEnv("LIBREOFFICE_BIN", ""),
	}
	if c.DSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	if c.RedisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required")
	}
	return c, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
