package config

import "os"

type Config struct {
	ServiceName   string
	HTTPAddr     string
	DSN          string
	StorageKind  string
	StoragePath  string
	MinIOEndpoint  string
	MinIOBucket   string
	MinIOAccessKey string
	MinIOSecretKey string
}

func Load() (*Config, error) {
	return &Config{
		ServiceName:   env("SERVICE_NAME", "template-svc"),
		HTTPAddr:      env("HTTP_ADDR", ":8096"),
		DSN:           env("DATABASE_DSN", "postgres://bidwriter:bidwriter@localhost:5432/bidwriter?sslmode=disable"),
		StorageKind:   env("STORAGE_KIND", "local"),
		StoragePath:   env("STORAGE_PATH", "/tmp/template-svc"),
		MinIOEndpoint:   env("MINIO_ENDPOINT", "localhost:9000"),
		MinIOBucket:     env("MINIO_BUCKET", "templates"),
		MinIOAccessKey:  env("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey:  env("MINIO_SECRET_KEY", ""),
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
