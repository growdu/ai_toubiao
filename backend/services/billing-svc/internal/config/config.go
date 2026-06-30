package config

import "os"

type Config struct {
	ServiceName string
	HTTPAddr   string
	DSN        string
}

func Load() (*Config, error) {
	return &Config{
		ServiceName: env("SERVICE_NAME", "billing-svc"),
		HTTPAddr:   env("HTTP_ADDR", ":8097"),
		DSN:        env("DATABASE_DSN", "postgres://bidwriter:bidwriter@localhost:5432/bidwriter?sslmode=disable"),
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
