package config

import (
	"os"
	"strconv"
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
	JWTSecret      string
	CookieSecure   bool

	AdminUsername  string
	AdminPassword  string
	AdminFirstName string
	AdminLastName  string
}

func NewConfig() Config {
	appEnv := getEnvOrDefault("APP_ENV", "development")
	return Config{
		Host:           getEnvOrDefault("HOST", "localhost"),
		Port:           getEnvOrDefault("PORT", "8080"),
		DatabaseURL:    requireEnv("DATABASE_URL"),
		AllowedOrigins: strings.Split(getEnvOrDefault("ALLOWED_ORIGINS", "http://localhost:5173"), ","),
		AppEnv:         appEnv,
		LogLevel:       getEnvOrDefault("LOG_LEVEL", "info"),
		WebDir:         getEnvOrDefault("WEB_DIR", "../web/build"),
		JWTSecret:      requireEnv("JWT_SECRET"),
		// Off for plain http over Tailscale, which is already encrypted.
		CookieSecure: getEnvOrDefault("COOKIE_SECURE", strconv.FormatBool(appEnv == "production")) == "true",

		AdminUsername:  os.Getenv("ADMIN_USERNAME"),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
		AdminFirstName: os.Getenv("ADMIN_FIRST_NAME"),
		AdminLastName:  os.Getenv("ADMIN_LAST_NAME"),
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
