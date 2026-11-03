package rubric

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/internal/apitest"
	"github.com/gihyunx/interview-tabulation/internal/dbtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	srv *httptest.Server
	fx  dbtest.Fixtures
)

func TestMain(m *testing.M) {
	db := dbtest.New(context.Background())
	fx = dbtest.Fixtures{Pool: db.Pool}
	srv = apitest.New(func(api huma.API) { Setup(api, db.Pool, apitest.Config, nil) })
	code := m.Run()
	srv.Close()
	db.Close()
	os.Exit(code)
}

func crit(title string, points int) map[string]any {
	return map[string]any{"title": title, "description": "", "max_points": points}
}

func TestPointsMustTotal100(t *testing.T) {
	tok := apitest.AdminToken(t)
	resp := apitest.Do(t, srv, "POST", "/v1/rubrics", tok, map[string]any{"name": "R", "criteria": []any{crit("A", 60), crit("B", 30)}})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	resp = apitest.Do(t, srv, "POST", "/v1/rubrics", tok, map[string]any{"name": "R", "criteria": []any{crit("A", 60), crit("B", 40)}})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	got := apitest.Decode[rubric](t, resp)
	require.Len(t, got.Criteria, 2)
	assert.Equal(t, "A", got.Criteria[0].Title)
	assert.False(t, got.Locked)
}

func TestUnlockedRubricEditsFreely(t *testing.T) {
	tok := apitest.AdminToken(t)
	id, crits := fx.Rubric(t, 50, 50)
	body := map[string]any{"name": "New name", "criteria": []any{
		map[string]any{"id": crits[1], "title": "Second first", "description": "d", "max_points": 40},
		crit("Brand new", 60),
	}}
	resp := apitest.Do(t, srv, "PATCH", "/v1/rubrics/"+id, tok, body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := apitest.Decode[rubric](t, resp)
	require.Len(t, got.Criteria, 2, "first criterion removed")
	assert.Equal(t, crits[1], got.Criteria[0].ID, "kept id")
	assert.Equal(t, int16(40), got.Criteria[0].MaxPoints)
	assert.Equal(t, "New name", got.Name)

	body["criteria"] = []any{map[string]any{"id": uuid.NewString(), "title": "x", "description": "", "max_points": 100}}
	assert.Equal(t, http.StatusUnprocessableEntity, apitest.Do(t, srv, "PATCH", "/v1/rubrics/"+id, tok, body).StatusCode, "foreign criterion id")
}

func TestLockAfterScoring(t *testing.T) {
	tok := apitest.AdminToken(t)
	rid, crits := fx.Rubric(t, 50, 50)
	iid := fx.Interview(t, rid, "panel")
	fx.Score(t, fx.Judge(t, fx.User(t, "judge"), iid), fx.Candidate(t), crits[0], 10)

	keep := func(i int, points int, title string) map[string]any {
		return map[string]any{"id": crits[i], "title": title, "description": "", "max_points": points}
	}
	patch := func(criteria ...any) int {
		return apitest.Do(t, srv, "PATCH", "/v1/rubrics/"+rid, tok, map[string]any{"name": "R", "criteria": criteria}).StatusCode
	}

	assert.Equal(t, http.StatusConflict, patch(keep(0, 70, "a"), keep(1, 30, "b")), "points change")
	assert.Equal(t, http.StatusConflict, patch(keep(0, 100, "a")), "remove criterion")
	assert.Equal(t, http.StatusConflict, patch(keep(0, 50, "a"), keep(1, 30, "b"), crit("new", 20)), "add criterion")

	resp := apitest.Do(t, srv, "PATCH", "/v1/rubrics/"+rid, tok, map[string]any{"name": "Renamed", "criteria": []any{keep(1, 50, "b2"), keep(0, 50, "a2")}})
	require.Equal(t, http.StatusOK, resp.StatusCode, "title, name and order stay editable")
	got := apitest.Decode[rubric](t, resp)
	assert.True(t, got.Locked)
	assert.Equal(t, crits[1], got.Criteria[0].ID, "reordered")

	assert.Equal(t, http.StatusConflict, apitest.Do(t, srv, "DELETE", "/v1/rubrics/"+rid, tok, nil).StatusCode)
}

func TestDeleteGuards(t *testing.T) {
	tok := apitest.AdminToken(t)

	t.Run("used by an interview", func(t *testing.T) {
		rid, _ := fx.Rubric(t, 100)
		fx.Interview(t, rid, "panel")
		resp := apitest.Do(t, srv, "DELETE", "/v1/rubrics/"+rid, tok, nil)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Contains(t, apitest.Detail(t, resp), "used by")
	})

	t.Run("unused rubric and criteria are removed", func(t *testing.T) {
		rid, _ := fx.Rubric(t, 100)
		assert.Equal(t, http.StatusNoContent, apitest.Do(t, srv, "DELETE", "/v1/rubrics/"+rid, tok, nil).StatusCode)
		assert.Equal(t, http.StatusNotFound, apitest.Do(t, srv, "GET", "/v1/rubrics/"+rid, tok, nil).StatusCode)
	})
}

func TestRoles(t *testing.T) {
	judge := apitest.JudgeToken(t, uuid.NewString())
	assert.Equal(t, http.StatusOK, apitest.Do(t, srv, "GET", "/v1/rubrics", judge, nil).StatusCode, "judges may read")
	assert.Equal(t, http.StatusUnauthorized, apitest.Do(t, srv, "GET", "/v1/rubrics", "", nil).StatusCode)
	for _, r := range []struct{ m, p string }{
		{"POST", "/v1/rubrics"}, {"PATCH", "/v1/rubrics/" + uuid.NewString()}, {"DELETE", "/v1/rubrics/" + uuid.NewString()},
	} {
		assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, r.m, r.p, judge, map[string]any{}).StatusCode, r.m)
	}
}
