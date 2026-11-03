// Package apitest has helpers for HTTP integration tests.
package apitest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const Secret = "test-secret"

var Config = config.Config{JWTSecret: Secret}

// New serves the routes that setup registers under /v1.
func New(setup func(api huma.API)) *httptest.Server {
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("Test API", "1.0.0")
	cfg.CreateHooks = nil
	setup(huma.NewGroup(humago.New(mux, cfg), "/v1"))
	return httptest.NewServer(mux)
}

// Token signs a login token. The user need not exist in the database.
func Token(t *testing.T, userID string, role auth.Role, expiresAt time.Time) string {
	t.Helper()
	claims := auth.Claims{Subject: userID, ExpiresAt: jwt.NewNumericDate(expiresAt), Role: role}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(Secret))
	require.NoError(t, err)
	return s
}

func AdminToken(t *testing.T) string {
	return Token(t, uuid.NewString(), auth.RoleAdmin, time.Now().Add(time.Hour))
}

func JudgeToken(t *testing.T, userID string) string {
	return Token(t, userID, auth.RoleJudge, time.Now().Add(time.Hour))
}

// Do sends a JSON request with the token as the session cookie.
func Do(t *testing.T, srv *httptest.Server, method, path, token string, body any) *http.Response {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&b).Encode(body))
	}
	req, err := http.NewRequest(method, srv.URL+path, &b)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func Decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&v))
	return v
}

// Detail returns the message of a Huma error response.
func Detail(t *testing.T, resp *http.Response) string {
	t.Helper()
	return Decode[struct{ Detail string }](t, resp).Detail
}
