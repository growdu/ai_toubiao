// Package config loads document-svc configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServiceName string
	HTTPAddr    string
	DSN         string
	StorageKind string // "minio" / "s3" / "local"
	StorageDSN  string // local 模式下的目录;s3/minio 模式可为空

	// S3/MinIO 连接信息(参见 ADR-0008)。仅在 StorageKind == "minio" / "s3" 时必填。
	S3Endpoint   string // host:port,不带 scheme
	S3AccessKey  string
	S3SecretKey  string
	S3Bucket     string // 默认 "bidwriter"
	S3Region     string // 默认 "us-east-1"
	S3UseSSL     bool   // 默认 false (本地 MinIO 多为 HTTP)
	RouterURL    string
	// RedisAddr is the Redis instance used to back the Asynq parser queue.
	// Empty string disables the worker + enqueue (sync parse only).
	RedisAddr    string
	// AsynqConcurrency caps how many parse tasks run at once.
	AsynqConcurrency int
}

func Load() (*Config, error) {
	useSSL, err := getEnvBool("MINIO_USE_SSL", false)
	if err != nil {
		return nil, fmt.Errorf("MINIO_USE_SSL: %w", err)
	}

	c := &Config{
		ServiceName: getEnv("SERVICE_NAME", "document-svc"),
		HTTPAddr:    getEnv("HTTP_ADDR", ":8082"),
		DSN:         getEnv("DB_DSN", "postgres://postgres:***@localhost:5432/bidwriter?sslmode=disable"),
		StorageKind: getEnv("STORAGE_KIND", "local"),
		StorageDSN:  getEnv("STORAGE_DSN", "/tmp/bidwriter-storage"),
		S3Endpoint:  getEnv("MINIO_ENDPOINT", ""),
		S3AccessKey: getEnv("MINIO_ACCESS_KEY", ""),
		S3SecretKey: getEnv("MINIO_SECRET_KEY", ""),
		S3Bucket:    getEnv("MINIO_BUCKET", "bidwriter"),
		S3Region:    getEnv("MINIO_REGION", "us-east-1"),
		S3UseSSL:    useSSL,
		RouterURL:   getEnv("ROUTER_URL", "http://localhost:8083"),
		RedisAddr:   getEnv("REDIS_ADDR", ""),
		AsynqConcurrency: getEnvInt("ASYNQ_CONCURRENCY", 4),
	}
	if c.DSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}

	// 启动期校验:选了对象存储就必须给出 endpoint / bucket(以及密钥)。
	// 提前 fail-fast,避免运行时第一个请求才报错。
	if c.StorageKind == "minio" || c.StorageKind == "s3" {
		if c.S3Endpoint == "" {
			return nil, fmt.Errorf("STORAGE_KIND=%s requires MINIO_ENDPOINT", c.StorageKind)
		}
		if c.S3Bucket == "" {
			return nil, fmt.Errorf("STORAGE_KIND=%s requires MINIO_BUCKET", c.StorageKind)
		}
		if c.S3AccessKey == "" || c.S3SecretKey == "" {
			return nil, fmt.Errorf("STORAGE_KIND=%s requires MINIO_ACCESS_KEY and MINIO_SECRET_KEY", c.StorageKind)
		}
	}
	return c, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvBool(k string, def bool) (bool, error) {
	v := os.Getenv(k)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("parse %s=%q: %w", k, v, err)
	}
	return b, nil
}

func getEnvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		// 出错时回落到默认值而不是 panic —— 启动期 hard-fail 已在 README
		// 的运维约束中说明,但配置文件语法错时不应让容器直接崩。
		return def
	}
	return n
}