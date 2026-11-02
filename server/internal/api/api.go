package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New builds the Huma API on mux and registers all routes. pool may be nil
// when only the OpenAPI spec is needed (cmd/openapi).
func New(mux *http.ServeMux, pool *pgxpool.Pool) huma.API {
	cfg := huma.DefaultConfig("Interview Tabulation API", "0.1.0")
	cfg.CreateHooks = nil
	cfg.DocsRenderer = huma.DocsRendererScalar

	api := humago.New(mux, cfg)
	registerHealth(api, pool)

	// Feature routes mount under huma.NewGroup(api, "/v1").
	return api
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func registerHealth(api huma.API, pool *pgxpool.Pool) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
	}, func(ctx context.Context, _ *struct{}) (*healthOutput, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return nil, huma.Error503ServiceUnavailable("database unavailable")
		}

		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}
