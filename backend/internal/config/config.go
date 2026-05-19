package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string
	Port        string
	Environment string
	LogLevel    string
	MaxUploadMB int64
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://yuno:yuno@localhost:5432/reconciliation?sslmode=disable"),
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		MaxUploadMB: getEnvInt64("MAX_UPLOAD_MB", 50),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}
