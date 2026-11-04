package user

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func do(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()
	var r bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&r).Encode(body))
	}
	req, err := http.NewRequest(method, testServer.URL+path, &r)
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

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&v))
	return v
}

func TestLogin(t *testing.T) {
	u := seedUser(t, auth.RoleJudge)

	t.Run("ok sets cookie, username is case-insensitive", func(t *testing.T) {
		resp := do(t, "POST", "/v1/auth/login", "", map[string]string{"username": " " + u.Username + " ", "password": "password123"})
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var c *http.Cookie
		for _, x := range resp.Cookies() {
			if x.Name == auth.CookieName {
				c = x
			}
		}
		require.NotNil(t, c)
		assert.True(t, c.HttpOnly)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite)

		me := do(t, "GET", "/v1/auth/me", c.Value, nil)
		require.Equal(t, http.StatusOK, me.StatusCode)
		assert.Equal(t, u.ID, decode[user](t, me).ID)
	})

	t.Run("bad password and unknown user get the same 401", func(t *testing.T) {
		for _, body := range []map[string]string{
			{"username": u.Username, "password": "nope"},
			{"username": "ghost", "password": "password123"},
		} {
			resp := do(t, "POST", "/v1/auth/login", "", body)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		}
	})
}

func TestLogoutClearsCookie(t *testing.T) {
	resp := do(t, "POST", "/v1/auth/logout", "", nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	var cleared bool
	for _, c := range resp.Cookies() {
		cleared = cleared || (c.Name == auth.CookieName && c.Value == "" && c.MaxAge < 0)
	}
	assert.True(t, cleared)
}

func TestTokenRejected(t *testing.T) {
	u := seedUser(t, auth.RoleAdmin)
	for name, token := range map[string]string{
		"missing": "",
		"garbage": "not-a-jwt",
		"expired": makeToken(t, u.ID, u.Role, time.Now().Add(-time.Minute)),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, do(t, "GET", "/v1/auth/me", token, nil).StatusCode)
		})
	}

	t.Run("deleted user", func(t *testing.T) {
		gone := makeToken(t, uuid.NewString(), auth.RoleAdmin, time.Now().Add(time.Hour))
		assert.Equal(t, http.StatusUnauthorized, do(t, "GET", "/v1/auth/me", gone, nil).StatusCode)
	})
}

func TestAdminRoutesRequireAdmin(t *testing.T) {
	judge := seedUser(t, auth.RoleJudge)
	id := uuid.NewString()
	routes := []struct {
		method, path string
		body         any
	}{
		{"GET", "/v1/users", nil},
		{"POST", "/v1/users", map[string]any{"username": "abc", "password": "password123", "role": "judge", "first_name": "A", "last_name": "B"}},
		{"PATCH", "/v1/users/" + id, map[string]any{"first_name": "X"}},
		{"DELETE", "/v1/users/" + id, nil},
		{"POST", "/v1/users/" + id + "/password", map[string]any{"password": "password123"}},
	}
	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, do(t, r.method, r.path, "", r.body).StatusCode)
			assert.Equal(t, http.StatusForbidden, do(t, r.method, r.path, tokenFor(t, judge), r.body).StatusCode)
		})
	}
}

func TestCreateUpdateList(t *testing.T) {
	admin := seedUser(t, auth.RoleAdmin)
	tok := tokenFor(t, admin)
	name := "Judge." + uuid.NewString()[:8]

	resp := do(t, "POST", "/v1/users", tok, map[string]any{
		"username": name, "password": "password123", "role": "judge", "first_name": "Ana", "last_name": "Cruz",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created := decode[user](t, resp)
	assert.Equal(t, "judge."+name[6:], created.Username, "username lowercased")

	t.Run("duplicate username is 409, case-insensitive", func(t *testing.T) {
		resp := do(t, "POST", "/v1/users", tok, map[string]any{
			"username": name, "password": "password123", "role": "judge", "first_name": "B", "last_name": "C",
		})
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("validation", func(t *testing.T) {
		resp := do(t, "POST", "/v1/users", tok, map[string]any{
			"username": "ok-name", "password": "short", "role": "judge", "first_name": "B", "last_name": "C",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
		resp = do(t, "POST", "/v1/users", tok, map[string]any{
			"username": "ok-name", "password": "password123", "role": "boss", "first_name": "B", "last_name": "C",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("update", func(t *testing.T) {
		resp := do(t, "PATCH", "/v1/users/"+created.ID, tok, map[string]any{"first_name": "Anna", "role": "admin"})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		got := decode[user](t, resp)
		assert.Equal(t, "Anna", got.FirstName)
		assert.Equal(t, "Cruz", got.LastName)
		assert.Equal(t, auth.RoleAdmin, got.Role)
		assert.True(t, got.UpdatedAt.After(created.UpdatedAt))
	})

	t.Run("cannot change own role", func(t *testing.T) {
		resp := do(t, "PATCH", "/v1/users/"+admin.ID, tok, map[string]any{"role": "judge"})
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("unknown id is 404", func(t *testing.T) {
		resp := do(t, "PATCH", "/v1/users/"+uuid.NewString(), tok, map[string]any{"first_name": "X"})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("list has no password hash", func(t *testing.T) {
		resp := do(t, "GET", "/v1/users", tok, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var raw []map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))
		assert.GreaterOrEqual(t, len(raw), 2)
		for _, u := range raw {
			assert.NotContains(t, u, "password_hash")
		}
	})
}

func TestResetPassword(t *testing.T) {
	admin := seedUser(t, auth.RoleAdmin)
	judge := seedUser(t, auth.RoleJudge)

	resp := do(t, "POST", "/v1/users/"+judge.ID+"/password", tokenFor(t, admin), map[string]any{"password": "brand-new-pass"})
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	login := func(pw string) int {
		return do(t, "POST", "/v1/auth/login", "", map[string]string{"username": judge.Username, "password": pw}).StatusCode
	}
	assert.Equal(t, http.StatusUnauthorized, login("password123"))
	assert.Equal(t, http.StatusOK, login("brand-new-pass"))

	resp = do(t, "POST", "/v1/users/"+uuid.NewString()+"/password", tokenFor(t, admin), map[string]any{"password": "brand-new-pass"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// seedAssignment gives userID a judge row on a fresh interview and returns the judge id.
func seedAssignment(t *testing.T, userID string) string {
	t.Helper()
	ctx := context.Background()
	var rubricID, interviewID, judgeID string
	require.NoError(t, testPool.QueryRow(ctx, `INSERT INTO rubric (name) VALUES ('R') RETURNING rubric_id`).Scan(&rubricID))
	require.NoError(t, testPool.QueryRow(ctx, `
		INSERT INTO interview (rubric_id, title, description, mode, position)
		VALUES ($1, 'I', '', 'panel', 0) RETURNING interview_id`, rubricID).Scan(&interviewID))
	require.NoError(t, testPool.QueryRow(ctx, `
		INSERT INTO judge (user_id, interview_id) VALUES ($1, $2) RETURNING judge_id`, userID, interviewID).Scan(&judgeID))
	return judgeID
}

func TestDelete(t *testing.T) {
	admin := seedUser(t, auth.RoleAdmin)
	tok := tokenFor(t, admin)
	ctx := context.Background()

	count := func(q string, args ...any) (n int) {
		require.NoError(t, testPool.QueryRow(ctx, q, args...).Scan(&n))
		return
	}

	t.Run("unscored user: assignments removed too", func(t *testing.T) {
		u := seedUser(t, auth.RoleJudge)
		seedAssignment(t, u.ID)

		resp := do(t, "DELETE", "/v1/users/"+u.ID, tok, nil)
		require.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Zero(t, count(`SELECT count(*) FROM users WHERE user_id = $1`, u.ID))
		assert.Zero(t, count(`SELECT count(*) FROM judge WHERE user_id = $1`, u.ID))
	})

	t.Run("scored user: 409 and nothing removed", func(t *testing.T) {
		u := seedUser(t, auth.RoleJudge)
		judgeID := seedAssignment(t, u.ID)

		var candidateID, criterionID string
		require.NoError(t, testPool.QueryRow(ctx, `INSERT INTO candidate (first_name, last_name) VALUES ('C', 'D') RETURNING candidate_id`).Scan(&candidateID))
		require.NoError(t, testPool.QueryRow(ctx, `
			INSERT INTO criterion (rubric_id, title, description, max_points, position)
			SELECT rubric_id, 'T', '', 10, 0 FROM interview JOIN judge USING (interview_id) WHERE judge_id = $1
			RETURNING criterion_id`, judgeID).Scan(&criterionID))
		_, err := testPool.Exec(ctx, `INSERT INTO score (judge_id, candidate_id, criterion_id, value) VALUES ($1, $2, $3, 5)`,
			judgeID, candidateID, criterionID)
		require.NoError(t, err)

		resp := do(t, "DELETE", "/v1/users/"+u.ID, tok, nil)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, 1, count(`SELECT count(*) FROM users WHERE user_id = $1`, u.ID))
		assert.Equal(t, 1, count(`SELECT count(*) FROM judge WHERE user_id = $1`, u.ID), "assignment rolled back")
	})

	t.Run("cannot delete yourself", func(t *testing.T) {
		assert.Equal(t, http.StatusConflict, do(t, "DELETE", "/v1/users/"+admin.ID, tok, nil).StatusCode)
	})

	t.Run("unknown id is 404", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, do(t, "DELETE", "/v1/users/"+uuid.NewString(), tok, nil).StatusCode)
	})
}

func TestSeedAdminIsIdempotent(t *testing.T) {
	name := fmt.Sprintf("seed-%s", uuid.NewString()[:8])
	for range 2 {
		require.NoError(t, SeedUser(context.Background(), testPool, auth.RoleAdmin, name, "password123", "A", "B"))
	}
	var n int
	require.NoError(t, testPool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE username = $1 AND role = 'admin'`, name).Scan(&n))
	assert.Equal(t, 1, n)
}
