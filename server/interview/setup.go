package interview

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Setup(api huma.API, pool *pgxpool.Pool, cfg config.Config, bus *event.Bus) {
	h := &handler{api: api, svc: &service{pool: pool, bus: bus}, jwt: cfg.JWTSecret}
	h.setupRoutes()
}
