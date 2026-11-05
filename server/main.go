package main

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	router.Handle("/v1/", http.NotFoundHandler())
	router.Handle("/", spaHandler(cfg.WebDir))

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

// spaHandler serves the built web app from dir, falling back to 200.html
// so client-side routes load on refresh. Uses the build's precompressed .gz
// files when the client accepts gzip.
func spaHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			path = filepath.Join(dir, "200.html")
		}

		// Hashed filenames never change content, so browsers can keep them.
		if strings.HasPrefix(r.URL.Path, "/_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		w.Header().Add("Vary", "Accept-Encoding")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			if _, err := os.Stat(path + ".gz"); err == nil {
				w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(path)))
				w.Header().Set("Content-Encoding", "gzip")
				path += ".gz"
			}
		}
		http.ServeFile(w, r, path)
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
