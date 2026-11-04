package realtime

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/apitag"
	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/internal/event"
)

const heartbeat = 15 * time.Second

// Setup registers GET /events. Streams end when done is closed (server
// shutdown) or when the subscriber falls behind; the browser then reconnects
// and refetches everything.
func Setup(api huma.API, cfg config.Config, bus *event.Bus, done <-chan struct{}) {
	sse.Register(api, huma.Operation{
		OperationID: "events",
		Method:      http.MethodGet,
		Path:        "/events",
		Tags:        []string{apitag.Realtime},
		Summary:     "Server-sent events",
		Description: "Invalidation hints: which topic changed. Refetch the matching GET endpoints.",
		Middlewares: huma.Middlewares{auth.RequireAuth(api, cfg.JWTSecret)},
	}, map[string]any{"message": event.Event{}}, func(ctx context.Context, _ *struct{}, send sse.Sender) {
		sub := bus.Subscribe()
		defer bus.Unsubscribe(sub)

		// Flush the headers so the browser sees the stream as open.
		if send.Comment("connected") != nil {
			return
		}
		tick := time.NewTicker(heartbeat)
		defer tick.Stop()

		for {
			var err error
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-sub.Lost:
				return
			case e := <-sub.C:
				err = send.Data(e)
			case <-tick.C:
				err = send.Comment("ping")
			}
			if err != nil {
				return
			}
		}
	})
}
