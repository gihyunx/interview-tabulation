package user

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Setup(api huma.API, pool *pgxpool.Pool, cfg config.Config, bus *event.Bus) {
	svc := &service{pool: pool, jwtSecret: cfg.JWTSecret, bus: bus}
	newHandler(api, svc, cfg.JWTSecret, cfg.AppEnv == "production").setupRoutes()
}

// SeedUser creates a user for seeding. It does nothing if the username exists.
func SeedUser(ctx context.Context, pool *pgxpool.Pool, role auth.Role, username, password, firstName, lastName string) error {
	svc := &service{pool: pool}
	_, err := svc.create(ctx, userCreateInput{
		Username:  username,
		Password:  password,
		Role:      role,
		FirstName: firstName,
		LastName:  lastName,
	})
	if errors.Is(err, errUsernameTaken) {
		slog.Info("seed: user already exists", "username", username)
		return nil
	}
	return err
}
