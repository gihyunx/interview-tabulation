package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/gihyunx/interview-tabulation/candidate"
	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/export"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/gihyunx/interview-tabulation/interview"
	"github.com/gihyunx/interview-tabulation/realtime"
	"github.com/gihyunx/interview-tabulation/rubric"
	"github.com/gihyunx/interview-tabulation/score"
	"github.com/gihyunx/interview-tabulation/user"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New builds the Huma API on mux and registers all routes. pool may be nil
// when only the OpenAPI spec is needed (cmd/openapi).
func New(mux *http.ServeMux, pool *pgxpool.Pool, cfg config.Config, shutdown <-chan struct{}) huma.API {
	humaCfg := huma.DefaultConfig("Interview Tabulation API", "0.1.0")
	humaCfg.CreateHooks = nil
	humaCfg.DocsRenderer = huma.DocsRendererScalar

	api := humago.New(mux, humaCfg)
	registerHealth(api, pool)

	v1 := huma.NewGroup(api, "/v1")
	bus := event.NewBus()
	user.Setup(v1, pool, cfg, bus)
	candidate.Setup(v1, pool, cfg, bus)
	rubric.Setup(v1, pool, cfg, bus)
	interview.Setup(v1, pool, cfg, bus)
	score.Setup(v1, pool, cfg, bus)
	export.Setup(v1, pool, cfg)
	realtime.Setup(v1, cfg, bus, shutdown)

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
