package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Environment, Port, DatabaseURL, WebOrigin, SessionSecret, StoragePath string
	Locale, Timezone, LiveProvider, BillingProvider, LogLevel             string
	MaxUploadBytes                                                        int64
}

func Load() (Config, error) {
	maxMB, err := strconv.ParseInt(env("MAX_UPLOAD_MB", "50"), 10, 64)
	if err != nil || maxMB < 1 || maxMB > 500 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_MB must be between 1 and 500")
	}
	c := Config{
		Environment: env("APP_ENV", "development"), Port: env("API_PORT", "8080"),
		DatabaseURL: env("DATABASE_URL", "postgres://fluenthub:fluenthub@localhost:5432/fluenthub?sslmode=disable"),
		WebOrigin:   env("WEB_ORIGIN", "http://localhost:5173"), SessionSecret: env("SESSION_SECRET", "change-me"),
		StoragePath: env("STORAGE_PATH", "./data/storage"), MaxUploadBytes: maxMB * 1024 * 1024,
		Locale: env("DEFAULT_LOCALE", "pt-BR"), Timezone: env("DEFAULT_TIMEZONE", "America/Sao_Paulo"),
		LiveProvider: env("LIVE_CLASS_PROVIDER", "mock"), BillingProvider: env("BILLING_PROVIDER", "mock"), LogLevel: env("LOG_LEVEL", "info"),
	}
	if c.Environment == "production" && c.SessionSecret == "change-me" {
		return Config{}, fmt.Errorf("SESSION_SECRET must be changed in production")
	}
	return c, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
