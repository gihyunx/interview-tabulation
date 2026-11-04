package score

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/internal/apitest"
	"github.com/gihyunx/interview-tabulation/internal/dbtest"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	srv *httptest.Server
	fx  dbtest.Fixtures
	bus = event.NewBus()
)

func TestMain(m *testing.M) {
	db := dbtest.New(context.Background())
	fx = dbtest.Fixtures{Pool: db.Pool}
	srv = apitest.New(func(api huma.API) {
		Setup(api, db.Pool, apitest.Config, bus)
	})
	code := m.Run()
	srv.Close()
	db.Close()
	os.Exit(code)
}

// world is one interview with a 30/30/40 rubric and its judges.
type world struct {
	interviewID string
	criteria    []string
	judgeUsers  []string
	judgeIDs    []string
}

func newWorld(t *testing.T, mode string, judges int) world {
	t.Helper()
	rid, crits := fx.Rubric(t, 30, 30, 40)
	w := world{interviewID: fx.Interview(t, rid, mode), criteria: crits}
	for range judges {
		u := fx.User(t, "judge")
		w.judgeUsers = append(w.judgeUsers, u)
		w.judgeIDs = append(w.judgeIDs, fx.Judge(t, u, w.interviewID))
	}
	return w
}

func (w world) body(values ...int) map[string]any {
	var scores []map[string]any
	for i, v := range values {
		scores = append(scores, map[string]any{"criterion_id": w.criteria[i], "value": v})
	}
	return map[string]any{"scores": scores}
}

func (w world) put(t *testing.T, judge int, candidateID string, body any) *http.Response {
	t.Helper()
	return apitest.Do(t, srv, "PUT", fmt.Sprintf("/v1/interviews/%s/candidates/%s/scores", w.interviewID, candidateID),
		apitest.JudgeToken(t, w.judgeUsers[judge]), body)
}

func scoreRows(t *testing.T, judgeID, candidateID string) (n int) {
	require.NoError(t, fx.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM score WHERE judge_id = $1 AND candidate_id = $2`, judgeID, candidateID).Scan(&n))
	return
}

func TestOneOnOneRace(t *testing.T) {
	const n = 8
	w := newWorld(t, "one_on_one", n)
	cand := fx.Candidate(t)

	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { codes[i] = w.put(t, i, cand, w.body(10, 10, 10)).StatusCode })
	}
	wg.Wait()

	wins, conflicts, owner := 0, 0, -1
	for i, c := range codes {
		switch c {
		case http.StatusOK:
			wins, owner = wins+1, i
		case http.StatusConflict:
			conflicts++
		}
	}
	assert.Equal(t, 1, wins, "codes: %v", codes)
	assert.Equal(t, n-1, conflicts, "codes: %v", codes)
	for i, id := range w.judgeIDs {
		want := 0
		if i == owner {
			want = 3
		}
		assert.Equal(t, want, scoreRows(t, id, cand), "judge %d rows", i)
	}
}

func TestPanelAllowsEveryJudge(t *testing.T) {
	w := newWorld(t, "panel", 3)
	cand := fx.Candidate(t)
	for i := range 3 {
		assert.Equal(t, http.StatusOK, w.put(t, i, cand, w.body(10+i, 10, 10)).StatusCode)
	}
	for _, id := range w.judgeIDs {
		assert.Equal(t, 3, scoreRows(t, id, cand))
	}
}

func TestSubmitValidation(t *testing.T) {
	w := newWorld(t, "panel", 1)
	other, otherCrits := fx.Rubric(t, 100)
	_ = other
	cand := fx.Candidate(t)

	cases := map[string]any{
		"missing criterion":   w.body(10, 10),
		"too many":            map[string]any{"scores": append(w.body(1, 1, 1)["scores"].([]map[string]any), map[string]any{"criterion_id": otherCrits[0], "value": 1})},
		"criterion of other":  map[string]any{"scores": []any{map[string]any{"criterion_id": otherCrits[0], "value": 1}, map[string]any{"criterion_id": w.criteria[1], "value": 1}, map[string]any{"criterion_id": w.criteria[2], "value": 1}}},
		"above max":           w.body(31, 0, 0),
		"negative":            w.body(-1, 0, 0),
		"duplicate criterion": map[string]any{"scores": []any{map[string]any{"criterion_id": w.criteria[0], "value": 1}, map[string]any{"criterion_id": w.criteria[0], "value": 1}, map[string]any{"criterion_id": w.criteria[2], "value": 1}}},
		"empty":               w.body(),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusUnprocessableEntity, w.put(t, 0, cand, body).StatusCode)
		})
	}
	assert.Zero(t, scoreRows(t, w.judgeIDs[0], cand), "nothing saved by invalid submits")

	t.Run("boundaries are allowed", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, w.put(t, 0, cand, w.body(0, 30, 40)).StatusCode)
	})
	t.Run("unknown candidate", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, w.put(t, 0, uuid.NewString(), w.body(1, 1, 1)).StatusCode)
	})
}

func TestUpdateAndIdempotentRetry(t *testing.T) {
	w := newWorld(t, "panel", 2)
	cand := fx.Candidate(t)

	first := w.put(t, 0, cand, w.body(10, 10, 10))
	require.Equal(t, http.StatusOK, first.StatusCode)
	assert.Equal(t, 30, *apitest.Decode[boardCandidate](t, first).Total)
	w.put(t, 1, cand, w.body(5, 5, 5))

	var created, updated time.Time
	q := `SELECT min(created_at), max(updated_at) FROM score WHERE judge_id = $1 AND candidate_id = $2`
	require.NoError(t, fx.Pool.QueryRow(context.Background(), q, w.judgeIDs[0], cand).Scan(&created, &updated))

	retry := w.put(t, 0, cand, w.body(10, 10, 10))
	assert.Equal(t, http.StatusOK, retry.StatusCode, "same PUT again")
	assert.Equal(t, 3, scoreRows(t, w.judgeIDs[0], cand), "no duplicate rows")

	time.Sleep(10 * time.Millisecond)
	upd := w.put(t, 0, cand, w.body(30, 30, 40))
	require.Equal(t, http.StatusOK, upd.StatusCode)
	assert.Equal(t, 100, *apitest.Decode[boardCandidate](t, upd).Total)

	var created2, updated2 time.Time
	require.NoError(t, fx.Pool.QueryRow(context.Background(), q, w.judgeIDs[0], cand).Scan(&created2, &updated2))
	assert.Equal(t, created, created2)
	assert.True(t, updated2.After(updated), "updated_at moved")

	var other int
	require.NoError(t, fx.Pool.QueryRow(context.Background(),
		`SELECT sum(value) FROM score WHERE judge_id = $1 AND candidate_id = $2`, w.judgeIDs[1], cand).Scan(&other))
	assert.Equal(t, 15, other, "another judge's scores are untouched")
}

func TestWhoMayScore(t *testing.T) {
	w := newWorld(t, "panel", 1)
	cand := fx.Candidate(t)
	path := fmt.Sprintf("/v1/interviews/%s/candidates/%s/scores", w.interviewID, cand)

	stranger := apitest.JudgeToken(t, fx.User(t, "judge"))
	assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, "PUT", path, stranger, w.body(1, 1, 1)).StatusCode, "not assigned")
	assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, "PUT", path, apitest.AdminToken(t), w.body(1, 1, 1)).StatusCode, "admin")
	assert.Equal(t, http.StatusUnauthorized, apitest.Do(t, srv, "PUT", path, "", w.body(1, 1, 1)).StatusCode)
	assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, "GET", "/v1/interviews/"+w.interviewID+"/board", stranger, nil).StatusCode)
}

func TestBoardStatuses(t *testing.T) {
	w := newWorld(t, "one_on_one", 2)
	mine, theirs, none := fx.Candidate(t), fx.Candidate(t), fx.Candidate(t)
	require.Equal(t, http.StatusOK, w.put(t, 0, mine, w.body(10, 20, 30)).StatusCode)
	require.Equal(t, http.StatusOK, w.put(t, 1, theirs, w.body(1, 1, 1)).StatusCode)

	resp := apitest.Do(t, srv, "GET", "/v1/interviews/"+w.interviewID+"/board", apitest.JudgeToken(t, w.judgeUsers[0]), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	b := apitest.Decode[board](t, resp)
	assert.Equal(t, w.judgeIDs[0], b.JudgeID)
	byID := map[string]boardCandidate{}
	for _, c := range b.Candidates {
		byID[c.ID] = c
	}
	assert.Equal(t, statusScored, byID[mine].Status)
	assert.Equal(t, 60, *byID[mine].Total)
	assert.Len(t, byID[mine].MyScores, 3)
	assert.Equal(t, statusScoredByOther, byID[theirs].Status)
	assert.NotEmpty(t, *byID[theirs].ScoredBy)
	assert.Equal(t, statusUnscored, byID[none].Status)

	t.Run("a scored candidate cannot be taken by another 1-on-1 judge", func(t *testing.T) {
		assert.Equal(t, http.StatusConflict, w.put(t, 0, theirs, w.body(1, 1, 1)).StatusCode)
	})
	t.Run("the same judge may update", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, w.put(t, 0, mine, w.body(11, 20, 30)).StatusCode)
	})
}

func TestPanelBoardIgnoresOtherJudges(t *testing.T) {
	w := newWorld(t, "panel", 2)
	cand := fx.Candidate(t)
	require.Equal(t, http.StatusOK, w.put(t, 1, cand, w.body(1, 1, 1)).StatusCode)
	resp := apitest.Do(t, srv, "GET", "/v1/interviews/"+w.interviewID+"/board", apitest.JudgeToken(t, w.judgeUsers[0]), nil)
	for _, c := range apitest.Decode[board](t, resp).Candidates {
		if c.ID == cand {
			assert.Equal(t, statusUnscored, c.Status)
		}
	}
}

func TestClearSubmission(t *testing.T) {
	w := newWorld(t, "one_on_one", 2)
	cand := fx.Candidate(t)
	require.Equal(t, http.StatusOK, w.put(t, 0, cand, w.body(1, 1, 1)).StatusCode)
	path := fmt.Sprintf("/v1/interviews/%s/candidates/%s/scores?judge=", w.interviewID, cand)

	assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, "DELETE", path+w.judgeIDs[0], apitest.JudgeToken(t, w.judgeUsers[0]), nil).StatusCode, "judge")
	assert.Equal(t, http.StatusConflict, w.put(t, 1, cand, w.body(2, 2, 2)).StatusCode, "still taken")

	admin := apitest.AdminToken(t)
	assert.Equal(t, http.StatusNoContent, apitest.Do(t, srv, "DELETE", path+w.judgeIDs[0], admin, nil).StatusCode)
	assert.Zero(t, scoreRows(t, w.judgeIDs[0], cand))
	assert.Equal(t, http.StatusNotFound, apitest.Do(t, srv, "DELETE", path+w.judgeIDs[0], admin, nil).StatusCode, "already cleared")
	assert.Equal(t, http.StatusOK, w.put(t, 1, cand, w.body(2, 2, 2)).StatusCode, "freed for the other judge")

	other := newWorld(t, "panel", 1)
	assert.Equal(t, http.StatusNotFound, apitest.Do(t, srv, "DELETE", path+other.judgeIDs[0], admin, nil).StatusCode, "judge of another interview")
}

func TestSubmitPublishesEvent(t *testing.T) {
	w := newWorld(t, "panel", 1)
	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)
	require.Equal(t, http.StatusOK, w.put(t, 0, fx.Candidate(t), w.body(1, 1, 1)).StatusCode)
	select {
	case e := <-sub.C:
		assert.Equal(t, event.Event{Topic: "scores", InterviewID: w.interviewID}, e)
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
}
