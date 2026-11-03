package interview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
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

func create(t *testing.T, tok, rubricID, mode string, judges ...string) interview {
	t.Helper()
	resp := apitest.Do(t, srv, "POST", "/v1/interviews", tok, map[string]any{
		"title": "T", "description": "", "mode": mode, "rubric_id": rubricID, "judge_user_ids": judges,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	return apitest.Decode[interview](t, resp)
}

func TestCreateAssignsJudges(t *testing.T) {
	tok := apitest.AdminToken(t)
	rid, _ := fx.Rubric(t, 100)
	j1, j2 := fx.User(t, "judge"), fx.User(t, "judge")

	got := create(t, tok, rid, "panel", j1, j2)
	assert.Len(t, got.Judges, 2)
	assert.Equal(t, rid, got.RubricID)

	t.Run("admin user cannot be a judge", func(t *testing.T) {
		resp := apitest.Do(t, srv, "POST", "/v1/interviews", tok, map[string]any{
			"title": "T", "description": "", "mode": "panel", "rubric_id": rid, "judge_user_ids": []string{fx.User(t, "admin")},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
	t.Run("unknown rubric", func(t *testing.T) {
		resp := apitest.Do(t, srv, "POST", "/v1/interviews", tok, map[string]any{
			"title": "T", "description": "", "mode": "panel", "rubric_id": uuid.NewString(),
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
	t.Run("bad mode", func(t *testing.T) {
		resp := apitest.Do(t, srv, "POST", "/v1/interviews", tok, map[string]any{
			"title": "T", "description": "", "mode": "solo", "rubric_id": rid,
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("replace judges", func(t *testing.T) {
		resp := apitest.Do(t, srv, "PUT", "/v1/interviews/"+got.ID+"/judges", tok, map[string]any{"user_ids": []string{j2}})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		out := apitest.Decode[interview](t, resp)
		require.Len(t, out.Judges, 1)
		assert.Equal(t, j2, out.Judges[0].UserID)
	})
}

func TestScoredInterviewIsProtected(t *testing.T) {
	tok := apitest.AdminToken(t)
	rid, crits := fx.Rubric(t, 100)
	other, _ := fx.Rubric(t, 100)
	uid := fx.User(t, "judge")
	iv := create(t, tok, rid, "one_on_one", uid)
	fx.Score(t, iv.Judges[0].JudgeID, fx.Candidate(t), crits[0], 50)

	t.Run("rubric cannot change", func(t *testing.T) {
		resp := apitest.Do(t, srv, "PATCH", "/v1/interviews/"+iv.ID, tok, map[string]any{"rubric_id": other})
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
	t.Run("title can", func(t *testing.T) {
		resp := apitest.Do(t, srv, "PATCH", "/v1/interviews/"+iv.ID, tok, map[string]any{"title": "Renamed", "rubric_id": rid})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		out := apitest.Decode[interview](t, resp)
		assert.Equal(t, "Renamed", out.Title)
		assert.True(t, out.HasScores)
		assert.Equal(t, 1, out.Judges[0].Scored)
	})
	t.Run("scored judge cannot be removed", func(t *testing.T) {
		resp := apitest.Do(t, srv, "PUT", "/v1/interviews/"+iv.ID+"/judges", tok, map[string]any{"user_ids": []string{}})
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
	t.Run("cannot be deleted, nothing removed", func(t *testing.T) {
		assert.Equal(t, http.StatusConflict, apitest.Do(t, srv, "DELETE", "/v1/interviews/"+iv.ID, tok, nil).StatusCode)
		var n int
		require.NoError(t, fx.Pool.QueryRow(context.Background(), `SELECT count(*) FROM judge WHERE interview_id = $1`, iv.ID).Scan(&n))
		assert.Equal(t, 1, n)
	})
}

func TestDeleteUnscoredRemovesAssignments(t *testing.T) {
	tok := apitest.AdminToken(t)
	rid, _ := fx.Rubric(t, 100)
	iv := create(t, tok, rid, "panel", fx.User(t, "judge"))
	assert.Equal(t, http.StatusNoContent, apitest.Do(t, srv, "DELETE", "/v1/interviews/"+iv.ID, tok, nil).StatusCode)
	var n int
	require.NoError(t, fx.Pool.QueryRow(context.Background(), `SELECT count(*) FROM judge WHERE interview_id = $1`, iv.ID).Scan(&n))
	assert.Zero(t, n)
}

func TestReorder(t *testing.T) {
	tok := apitest.AdminToken(t)
	rid, _ := fx.Rubric(t, 100)
	a, b := create(t, tok, rid, "panel"), create(t, tok, rid, "panel")

	all := func() []string {
		resp := apitest.Do(t, srv, "GET", "/v1/interviews", tok, nil)
		var ids []string
		for _, i := range apitest.Decode[[]interview](t, resp) {
			ids = append(ids, i.ID)
		}
		return ids
	}
	existing := all()

	reversed := make([]string, len(existing))
	for i, id := range existing {
		reversed[len(existing)-1-i] = id
	}
	require.Equal(t, http.StatusNoContent, apitest.Do(t, srv, "PUT", "/v1/interviews/order", tok, map[string]any{"ids": reversed}).StatusCode)
	assert.Equal(t, reversed, all())

	assert.Equal(t, http.StatusUnprocessableEntity,
		apitest.Do(t, srv, "PUT", "/v1/interviews/order", tok, map[string]any{"ids": []string{a.ID}}).StatusCode, "missing ids")
	assert.Equal(t, http.StatusUnprocessableEntity,
		apitest.Do(t, srv, "PUT", "/v1/interviews/order", tok, map[string]any{"ids": append(slices.Clip(reversed), b.ID)}).StatusCode, "duplicate id")
	assert.Equal(t, reversed, all(), "failed reorder changes nothing")
}

func TestMyInterviews(t *testing.T) {
	tok := apitest.AdminToken(t)
	rid, crits := fx.Rubric(t, 40, 60)
	uid := fx.User(t, "judge")
	iv := create(t, tok, rid, "panel", uid)
	create(t, tok, rid, "panel", fx.User(t, "judge"))

	resp := apitest.Do(t, srv, "GET", "/v1/me/interviews", apitest.JudgeToken(t, uid), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := apitest.Decode[[]myInterview](t, resp)
	require.Len(t, got, 1)
	assert.Equal(t, iv.ID, got[0].ID)
	require.Len(t, got[0].Rubric.Criteria, 2)
	assert.Equal(t, crits[0], got[0].Rubric.Criteria[0].ID)
}

func TestAdminOnly(t *testing.T) {
	judge := apitest.JudgeToken(t, uuid.NewString())
	id := uuid.NewString()
	for _, r := range []struct{ m, p string }{
		{"GET", "/v1/interviews"}, {"POST", "/v1/interviews"}, {"PATCH", "/v1/interviews/" + id},
		{"DELETE", "/v1/interviews/" + id}, {"PUT", "/v1/interviews/" + id + "/judges"}, {"PUT", "/v1/interviews/order"},
	} {
		assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, r.m, r.p, judge, map[string]any{}).StatusCode, r.m+" "+r.p)
		assert.Equal(t, http.StatusUnauthorized, apitest.Do(t, srv, r.m, r.p, "", map[string]any{}).StatusCode, r.m+" "+r.p)
	}
	assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, "GET", "/v1/me/interviews", apitest.AdminToken(t), nil).StatusCode)
}
