package config

import "os"

type Config struct {
	ServiceName     string
	HTTPAddr       string
	DSN            string
	SMTPHost       string
	SMTPPort       int
	SMTPUser       string
	SMTPPassword   string
	DingTalkToken  string
	DingTalkSecret string
	WeComWebhookURL string
}

func Load() *Config {
	return &Config{
		ServiceName:     env("SERVICE_NAME", "notify-svc"),
		HTTPAddr:       env("HTTP_ADDR", ":8098"),
		DSN:            env("DATABASE_DSN", "postgres://bidwriter:bidwriter@localhost:5432/bidwriter?sslmode=disable"),
		SMTPHost:       env("SMTP_HOST", "localhost"),
		SMTPPort:       587,
		SMTPUser:       env("SMTP_USER", ""),
		SMTPPassword:   env("SMTP_PASSWORD", ""),
		DingTalkToken:  env("DINGTALK_TOKEN", ""),
		DingTalkSecret: env("DINGTALK_SECRET", ""),
		WeComWebhookURL: env("WECOM_WEBHOOK_URL", ""),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
