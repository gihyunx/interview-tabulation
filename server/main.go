package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

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
		return fmt.Errorf("database unreachable (is it running? try `mise run db:start`): %w", err)
	}

	if err := migrations.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           newHandler(cfg, pool),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: SSE streams are long-lived.
	}

	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()

	slog.Info("starting server", "addr", server.Addr)
	if cfg.AppEnv == "production" {
		for _, ip := range lanIPs() {
			slog.Info("reachable on LAN", "url", "http://"+net.JoinHostPort(ip, cfg.Port))
		}
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	slog.Info("server stopped")
	return nil
}

func newHandler(cfg config.Config, pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	api.New(mux, pool)

	if cfg.AppEnv == "production" {
		mux.Handle("/", spaHandler(cfg.WebDir))
	}

	return corsMiddleware(cfg.AllowedOrigins, logger.RequestLogger(mux))
}

// spaHandler serves files from dir and falls back to 200.html for any other
// path. API paths never reach it because Huma routes are more specific; the
// reserved prefixes below 404 instead of returning HTML.
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	reserved := []string{"/v1", "/health", "/docs", "/openapi"}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, p := range reserved {
			if r.URL.Path == p || strings.HasPrefix(r.URL.Path, p+"/") || strings.HasPrefix(r.URL.Path, p+".") {
				http.NotFound(w, r)
				return
			}
		}

		if info, err := os.Stat(filepath.Join(dir, filepath.Clean("/"+r.URL.Path))); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join(dir, "200.html"))
	})
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

func lanIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}

	var ips []string
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
			ips = append(ips, n.IP.String())
		}
	}
	return ips
}
