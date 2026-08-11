package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment, Port, DatabaseURL, WebOrigin, SessionSecret, StoragePath string
	Locale, Timezone, LiveProvider, BillingProvider, LogLevel             string
	LiveKitURL, LiveKitAPIKey, LiveKitAPISecret                           string
	AsaasBaseURL, AsaasAPIKey                                             string
	AsaasWebhookToken                                                     string
	StorageEncryptionKey, MalwareScanner, ClamAVAddress                   string
	BackupPath                                                            string
	BackupInterval, BackupRetention                                       time.Duration
	BillingReconcileInterval                                              time.Duration
	AllowLegacyPlaintext                                                  bool
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
		LiveKitURL: strings.TrimRight(os.Getenv("LIVEKIT_URL"), "/"), LiveKitAPIKey: os.Getenv("LIVEKIT_API_KEY"), LiveKitAPISecret: os.Getenv("LIVEKIT_API_SECRET"),
		AsaasBaseURL: strings.TrimRight(env("ASAAS_BASE_URL", "https://api-sandbox.asaas.com/v3"), "/"), AsaasAPIKey: os.Getenv("ASAAS_API_KEY"),
		AsaasWebhookToken: os.Getenv("ASAAS_WEBHOOK_TOKEN"), BillingReconcileInterval: durationMinutes("BILLING_RECONCILE_MINUTES", 15),
		StorageEncryptionKey: os.Getenv("STORAGE_ENCRYPTION_KEY"), MalwareScanner: env("MALWARE_SCANNER", "disabled"), ClamAVAddress: env("CLAMAV_ADDRESS", "clamav:3310"),
		BackupPath: env("BACKUP_PATH", "./data/backups"), BackupInterval: durationHours("BACKUP_INTERVAL_HOURS", 24), BackupRetention: durationHours("BACKUP_RETENTION_HOURS", 24*30),
		AllowLegacyPlaintext: env("STORAGE_ALLOW_LEGACY_PLAINTEXT", "false") == "true",
	}
	if c.Environment == "production" && c.SessionSecret == "change-me" {
		return Config{}, fmt.Errorf("SESSION_SECRET must be changed in production")
	}
	if c.LiveProvider != "mock" && c.LiveProvider != "livekit" {
		return Config{}, fmt.Errorf("LIVE_CLASS_PROVIDER must be mock or livekit")
	}
	if c.LiveProvider == "livekit" && (c.LiveKitURL == "" || c.LiveKitAPIKey == "" || len(c.LiveKitAPISecret) < 16) {
		return Config{}, fmt.Errorf("LIVEKIT_URL, LIVEKIT_API_KEY and a strong LIVEKIT_API_SECRET are required for livekit")
	}
	if c.BillingProvider != "mock" && c.BillingProvider != "asaas" {
		return Config{}, fmt.Errorf("BILLING_PROVIDER must be mock or asaas")
	}
	if c.BillingProvider == "asaas" && (c.AsaasAPIKey == "" || len(c.AsaasWebhookToken) < 32 || len(c.AsaasWebhookToken) > 255) {
		return Config{}, fmt.Errorf("ASAAS_API_KEY and ASAAS_WEBHOOK_TOKEN (32-255 characters) are required for asaas")
	}
	if c.MalwareScanner != "disabled" && c.MalwareScanner != "clamav" {
		return Config{}, fmt.Errorf("MALWARE_SCANNER must be disabled or clamav")
	}
	if c.StorageEncryptionKey != "" {
		key, err := base64.StdEncoding.DecodeString(c.StorageEncryptionKey)
		if err != nil || len(key) != 32 {
			return Config{}, fmt.Errorf("STORAGE_ENCRYPTION_KEY must be base64 for exactly 32 bytes")
		}
	}
	if c.Environment == "production" && (c.StorageEncryptionKey == "" || c.MalwareScanner != "clamav") {
		return Config{}, fmt.Errorf("production requires encrypted storage and MALWARE_SCANNER=clamav")
	}
	return c, nil
}

func durationHours(key string, fallback int) time.Duration {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return time.Duration(fallback) * time.Hour
	}
	return time.Duration(value) * time.Hour
}

func durationMinutes(key string, fallback int) time.Duration {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return time.Duration(fallback) * time.Minute
	}
	return time.Duration(value) * time.Minute
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
