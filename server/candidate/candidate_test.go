package candidate

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/internal/apitest"
	"github.com/gihyunx/interview-tabulation/internal/auth"
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

func count(t *testing.T) (n int) {
	require.NoError(t, fx.Pool.QueryRow(context.Background(), `SELECT count(*) FROM candidate`).Scan(&n))
	return
}

func upload(t *testing.T, csv, query string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="c.csv"`)
	h.Set("Content-Type", "text/csv")
	part, err := w.CreatePart(h)
	require.NoError(t, err)
	_, _ = part.Write([]byte(csv))
	require.NoError(t, w.Close())

	req, err := http.NewRequest("POST", srv.URL+"/v1/candidates/import"+query, &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: apitest.AdminToken(t)})
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

type importBody struct {
	DryRun bool `json:"dry_run"`
	Rows   []importRow
	Errors int
}

func TestImport(t *testing.T) {
	before := count(t)

	t.Run("dry run saves nothing and reports row errors", func(t *testing.T) {
		resp := upload(t, "first_name,last_name\nAna,Cruz\n,Reyes\n", "?dry_run=true")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		got := apitest.Decode[importBody](t, resp)
		assert.True(t, got.DryRun)
		assert.Equal(t, 1, got.Errors)
		require.Len(t, got.Rows, 2)
		assert.Equal(t, "Cruz", got.Rows[0].LastName)
		assert.NotEmpty(t, got.Rows[1].Error)
		assert.Equal(t, before, count(t))
	})

	t.Run("row error blocks the whole import", func(t *testing.T) {
		assert.Equal(t, http.StatusUnprocessableEntity, upload(t, "first_name,last_name\nAna,Cruz\n,Reyes\n", "").StatusCode)
		assert.Equal(t, before, count(t))
	})

	t.Run("bad header", func(t *testing.T) {
		assert.Equal(t, http.StatusUnprocessableEntity, upload(t, "name,surname\nAna,Cruz\n", "").StatusCode)
		assert.Equal(t, http.StatusUnprocessableEntity, upload(t, "name,surname\nAna,Cruz\n", "?dry_run=true").StatusCode)
		assert.Equal(t, before, count(t))
	})

	t.Run("header only", func(t *testing.T) {
		assert.Equal(t, http.StatusUnprocessableEntity, upload(t, "first_name,last_name\n", "").StatusCode)
	})

	t.Run("valid file creates all rows", func(t *testing.T) {
		resp := upload(t, "\xef\xbb\xbfFirst_Name , last_name\n José , Rizal \nAna,Cruz\n", "")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, before+2, count(t))
		var first string
		require.NoError(t, fx.Pool.QueryRow(context.Background(), `SELECT first_name FROM candidate WHERE last_name = 'Rizal'`).Scan(&first))
		assert.Equal(t, "José", first, "trimmed, accents kept")
	})

	t.Run("number column", func(t *testing.T) {
		before := count(t)
		resp := upload(t, "number,first_name,last_name\n5001,Ana,Num\nx,B,C\n5001,D,E\n0,F,G\n", "?dry_run=true")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		got := apitest.Decode[importBody](t, resp)
		assert.Equal(t, 3, got.Errors, "bad number, duplicate in file, zero")
		assert.Equal(t, 5001, got.Rows[0].Number)

		resp = upload(t, "number,first_name,last_name\n5002,Ana,Num\n5001,Ben,Num\n", "")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, before+2, count(t))
		var n int
		require.NoError(t, fx.Pool.QueryRow(context.Background(), `SELECT number FROM candidate WHERE first_name = 'Ben' AND last_name = 'Num'`).Scan(&n))
		assert.Equal(t, 5001, n)

		resp = upload(t, "number,first_name,last_name\n5001,Taken,Num\n", "?dry_run=true")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, 1, apitest.Decode[importBody](t, resp).Errors, "number already in the database")
		assert.Equal(t, http.StatusUnprocessableEntity, upload(t, "number,first_name,last_name\n5001,Taken,Num\n", "").StatusCode)
		assert.Equal(t, before+2, count(t))
	})
}

func TestCRUDAndDeleteGuard(t *testing.T) {
	tok := apitest.AdminToken(t)
	resp := apitest.Do(t, srv, "POST", "/v1/candidates", tok, map[string]any{"first_name": "Maria", "last_name": "Clara"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	c := apitest.Decode[candidate](t, resp)

	resp = apitest.Do(t, srv, "PATCH", "/v1/candidates/"+c.ID, tok, map[string]any{"last_name": "Santos"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Santos", apitest.Decode[candidate](t, resp).LastName)

	t.Run("numbers", func(t *testing.T) {
		resp := apitest.Do(t, srv, "POST", "/v1/candidates", tok, map[string]any{"first_name": "Next", "last_name": "One"})
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		next := apitest.Decode[candidate](t, resp)
		assert.Equal(t, c.Number+1, next.Number, "defaults to highest + 1")

		assert.Equal(t, http.StatusConflict, apitest.Do(t, srv, "PATCH", "/v1/candidates/"+next.ID, tok, map[string]any{"number": c.Number}).StatusCode)
		assert.Equal(t, http.StatusConflict, apitest.Do(t, srv, "POST", "/v1/candidates", tok, map[string]any{"number": c.Number, "first_name": "A", "last_name": "B"}).StatusCode)
		resp = apitest.Do(t, srv, "PATCH", "/v1/candidates/"+next.ID, tok, map[string]any{"number": 9000})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, 9000, apitest.Decode[candidate](t, resp).Number)
		assert.Equal(t, http.StatusUnprocessableEntity, apitest.Do(t, srv, "PATCH", "/v1/candidates/"+next.ID, tok, map[string]any{"number": 0}).StatusCode)
	})

	assert.Equal(t, http.StatusUnprocessableEntity, apitest.Do(t, srv, "POST", "/v1/candidates", tok, map[string]any{"first_name": "", "last_name": "X"}).StatusCode)
	assert.Equal(t, http.StatusNotFound, apitest.Do(t, srv, "PATCH", "/v1/candidates/"+uuid.NewString(), tok, map[string]any{"last_name": "X"}).StatusCode)

	t.Run("scored candidate cannot be deleted", func(t *testing.T) {
		rid, crits := fx.Rubric(t, 100)
		jid := fx.Judge(t, fx.User(t, "judge"), fx.Interview(t, rid, "panel"))
		scored := fx.Candidate(t)
		fx.Score(t, jid, scored, crits[0], 5)
		assert.Equal(t, http.StatusConflict, apitest.Do(t, srv, "DELETE", "/v1/candidates/"+scored, tok, nil).StatusCode)
		assert.Equal(t, http.StatusNoContent, apitest.Do(t, srv, "DELETE", "/v1/candidates/"+c.ID, tok, nil).StatusCode)
	})
}

func TestRoles(t *testing.T) {
	judge := apitest.JudgeToken(t, uuid.NewString())
	assert.Equal(t, http.StatusOK, apitest.Do(t, srv, "GET", "/v1/candidates", judge, nil).StatusCode, "judges may read")
	assert.Equal(t, http.StatusUnauthorized, apitest.Do(t, srv, "GET", "/v1/candidates", "", nil).StatusCode)
	for _, r := range []struct{ m, p string }{
		{"POST", "/v1/candidates"}, {"PATCH", "/v1/candidates/" + uuid.NewString()}, {"DELETE", "/v1/candidates/" + uuid.NewString()},
	} {
		assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, r.m, r.p, judge, map[string]any{}).StatusCode, r.m)
	}
}
