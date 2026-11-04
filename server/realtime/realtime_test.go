package realtime

import (
	"bufio"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/internal/apitest"
	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func open(t *testing.T, bus *event.Bus, done chan struct{}) (*http.Response, *bufio.Reader) {
	t.Helper()
	srv := apitest.New(func(api huma.API) { Setup(api, apitest.Config, bus, done) })
	t.Cleanup(srv.Close)

	req, err := http.NewRequest("GET", srv.URL+"/v1/events", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: apitest.AdminToken(t)})
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	return resp, bufio.NewReader(resp.Body)
}

func readUntil(t *testing.T, r *bufio.Reader, want string) {
	t.Helper()
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err, "waiting for %q", want)
		if strings.Contains(line, want) {
			return
		}
	}
}

func TestEventsAreDelivered(t *testing.T) {
	bus := event.NewBus()
	_, r := open(t, bus, make(chan struct{}))
	readUntil(t, r, "connected") // subscribed by now

	bus.Publish(event.Event{Topic: "scores", InterviewID: "abc"})
	readUntil(t, r, `"topic":"scores"`)
}

func TestStreamEndsOnShutdown(t *testing.T) {
	done := make(chan struct{})
	_, r := open(t, event.NewBus(), done)
	readUntil(t, r, "connected")

	close(done)
	finished := make(chan error, 1)
	go func() {
		for {
			if _, err := r.ReadString('\n'); err != nil {
				finished <- err
				return
			}
		}
	}()
	select {
	case err := <-finished:
		assert.Error(t, err, "stream should hit EOF")
	case <-time.After(3 * time.Second):
		t.Fatal("stream still open after shutdown")
	}
}

func TestRequiresLogin(t *testing.T) {
	srv := apitest.New(func(api huma.API) { Setup(api, apitest.Config, event.NewBus(), nil) })
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/events")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
