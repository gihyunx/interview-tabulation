package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/user"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	sample := flag.Bool("sample", false, "also add the sample rubric, interview and candidates (sandbox only)")
	flag.Parse()
	_ = godotenv.Load()

	cfg := config.NewConfig()
	if cfg.AdminUsername == "" || len(cfg.AdminPassword) < 8 || cfg.AdminFirstName == "" || cfg.AdminLastName == "" {
		slog.Error("set ADMIN_USERNAME, ADMIN_PASSWORD (8+ chars), ADMIN_FIRST_NAME and ADMIN_LAST_NAME")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := user.SeedUser(ctx, pool, auth.RoleAdmin, cfg.AdminUsername, cfg.AdminPassword, cfg.AdminFirstName, cfg.AdminLastName); err != nil {
		slog.Error("seed", "error", err)
		pool.Close()
		os.Exit(1)
	}

	if *sample {
		if err := seedSample(ctx, pool); err != nil {
			slog.Error("seed sample", "error", err)
			pool.Close()
			os.Exit(1)
		}
	}
}
