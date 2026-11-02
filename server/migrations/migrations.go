package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"

	"github.com/pressly/goose/v3"
)

// Embeds this file only so the package compiles before the first .sql exists;
// embed pattern "*" picks up migrations as they are created.
//
//go:embed *
var embedMigrations embed.FS

func Migrate(ctx context.Context, dbURL string) error {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, embedMigrations, goose.WithVerbose(true))
	if errors.Is(err, goose.ErrNoMigrations) {
		return nil // no migrations yet
	}
	if err != nil {
		return err
	}

	_, err = provider.Up(ctx)
	return err
}
