// Package config loads workflow-svc configuration.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	ServiceName  string
	HTTPAddr     string
	DSN          string
	RouterURL    string
	KnowledgeURL string
	DocumentURL  string
	AuditURL     string
}

func Load() (*Config, error) {
	c := &Config{
		ServiceName:  getEnv("SERVICE_NAME", "workflow-svc"),
		HTTPAddr:     getEnv("HTTP_ADDR", ":9083"),
		DSN:          getEnv("DB_DSN", "postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable"),
		RouterURL:    getEnv("ROUTER_URL", "http://localhost:8083"),
		KnowledgeURL: getEnv("KNOWLEDGE_URL", "http://localhost:8086"),
		DocumentURL:  getEnv("DOCUMENT_URL", "http://localhost:8082"),
		AuditURL:     getEnv("AUDIT_URL", "http://localhost:8087"),
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