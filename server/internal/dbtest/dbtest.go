package dbtest

import (
	"context"
	"log/slog"
	"os"

	"github.com/gihyunx/interview-tabulation/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type DB struct {
	Pool *pgxpool.Pool
	URL  string
	ctr  *postgres.PostgresContainer
}

func (d *DB) Close() {
	d.Pool.Close()
	if err := testcontainers.TerminateContainer(d.ctr); err != nil {
		slog.Error(err.Error())
	}
}

func New(ctx context.Context) *DB {
	os.Setenv("APP_ENV", "test")

	ctr, err := postgres.Run(
		ctx,
		"postgres:18.6",
		postgres.WithDatabase("tabulation_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		panic(err)
	}

	connStr, err := ctr.ConnectionString(ctx)
	if err != nil {
		panic(err)
	}

	if err := migrations.Migrate(ctx, connStr); err != nil {
		panic(err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		panic(err)
	}

	return &DB{Pool: pool, URL: connStr, ctr: ctr}
}
