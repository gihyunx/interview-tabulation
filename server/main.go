package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/api"
	"github.com/gihyunx/interview-tabulation/internal/logger"
	"github.com/gihyunx/interview-tabulation/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg := config.NewConfig()

	setupLogger(cfg.LogLevel, cfg.AppEnv)

	if err := run(cfg); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}

	if err := migrations.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}

	router := http.NewServeMux()
	api.New(router, pool, cfg, ctx.Done())

	server := http.Server{
		Addr:    cfg.Host + ":" + cfg.Port,
		Handler: corsMiddleware(cfg.AllowedOrigins, logger.RequestLogger(router)),
	}

	slog.Info("starting server", "port", cfg.Port)

	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	return server.Shutdown(shutdownCtx)
}

func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowed[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setupLogger(level, appEnv string) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(level))); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler
	if appEnv == "production" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(h))
}
