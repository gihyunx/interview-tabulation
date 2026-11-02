package config

import (
	"fmt"
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

func NewConfig() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is not set; copy .env.example to .env")
	}

	return Config{
		Host:           getEnvOrDefault("HOST", "localhost"),
		Port:           getEnvOrDefault("PORT", "8080"),
		DatabaseURL:    dbURL,
		AllowedOrigins: strings.Split(getEnvOrDefault("ALLOWED_ORIGINS", "http://localhost:5173"), ","),
		AppEnv:         getEnvOrDefault("APP_ENV", "development"),
		LogLevel:       getEnvOrDefault("LOG_LEVEL", "info"),
		WebDir:         getEnvOrDefault("WEB_DIR", "../web/build"),
	}, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
