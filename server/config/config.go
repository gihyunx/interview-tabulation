package config

import (
	"os"
	"strings"
)

type Config struct {
	Host           string
	Port           string
	DatabaseURL    string
	AllowedOrigins []string
	AppEnv         string
	LogLevel       string
	WebDir         string
}

func NewConfig() Config {
	return Config{
		Host:           getEnvOrDefault("HOST", "localhost"),
		Port:           getEnvOrDefault("PORT", "8080"),
		DatabaseURL:    requireEnv("DATABASE_URL"),
		AllowedOrigins: strings.Split(getEnvOrDefault("ALLOWED_ORIGINS", "http://localhost:5173"), ","),
		AppEnv:         getEnvOrDefault("APP_ENV", "development"),
		LogLevel:       getEnvOrDefault("LOG_LEVEL", "info"),
		WebDir:         getEnvOrDefault("WEB_DIR", "../web/build"),
	}
}

func requireEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		panic(key + " not found.")
	}
	return v
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
