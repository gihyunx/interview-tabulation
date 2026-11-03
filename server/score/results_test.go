package score

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/gihyunx/interview-tabulation/internal/apitest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fill scores every criterion of the interview for the judge so the total is total (spread over the first criterion only).
func (w world) fill(t *testing.T, judge int, candidateID string, total int) {
	t.Helper()
	fx.Score(t, w.judgeIDs[judge], candidateID, w.criteria[0], total)
	fx.Score(t, w.judgeIDs[judge], candidateID, w.criteria[1], 0)
	fx.Score(t, w.judgeIDs[judge], candidateID, w.criteria[2], 0)
}

func results(t *testing.T, ids ...string) Results {
	t.Helper()
	q := url.Values{}
	for _, id := range ids {
		q.Add("interview", id)
	}
	resp := apitest.Do(t, srv, "GET", "/v1/results?"+q.Encode(), apitest.AdminToken(t), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return apitest.Decode[Results](t, resp)
}

func find(t *testing.T, r Results, id string) ResultCandidate {
	t.Helper()
	for _, c := range r.Candidates {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("candidate %s missing", id)
	return ResultCandidate{}
}

func TestAverageOnlyWhenAllTotalsAreIn(t *testing.T) {
	one := newWorld(t, "one_on_one", 2)
	panel := newWorld(t, "panel", 3)
	test := newWorld(t, "one_on_one", 1) // a test run, never selected
	a, b, c := fx.Candidate(t), fx.Candidate(t), fx.Candidate(t)

	one.fill(t, 0, a, 60)
	panel.fill(t, 0, a, 90)
	panel.fill(t, 1, a, 80)
	panel.fill(t, 2, a, 70)
	test.fill(t, 0, a, 0)

	one.fill(t, 1, b, 50)
	for i := range 2 {
		panel.fill(t, i, c, 100)
	}

	r := results(t, one.interviewID, panel.interviewID)
	assert.Equal(t, 4, r.Expected)

	ca := find(t, r, a)
	assert.Len(t, ca.Submissions, 4, "unselected interview is ignored")
	assert.Equal(t, 300, ca.Sum)
	assert.True(t, ca.Complete)
	require.NotNil(t, ca.Average)
	assert.InDelta(t, 75, *ca.Average, 0.001)

	cb := find(t, r, b)
	assert.False(t, cb.Complete)
	assert.Nil(t, cb.Average, "pending")
	require.Len(t, cb.Submissions, 1)
	assert.Equal(t, 50, cb.Submissions[0].Total)

	cc := find(t, r, c)
	assert.Nil(t, cc.Average, "2 of 3 panel judges")
	assert.Len(t, cc.Submissions, 2)

	assert.GreaterOrEqual(t, r.FullyScored, 1)
	for _, i := range r.Interviews {
		assert.Equal(t, i.ID == one.interviewID || i.ID == panel.interviewID, i.Selected, i.Title)
	}

	t.Run("submission detail", func(t *testing.T) {
		s := ca.Submissions[0]
		assert.Equal(t, one.interviewID, s.InterviewID)
		assert.Len(t, s.Values, 3)
		assert.False(t, s.SubmittedAt.IsZero())
	})
}

func TestTwoInterviewsOfTheSameModeBothCount(t *testing.T) {
	x, y := newWorld(t, "one_on_one", 1), newWorld(t, "one_on_one", 1)
	cand := fx.Candidate(t)
	x.fill(t, 0, cand, 80)

	r := results(t, x.interviewID, y.interviewID)
	assert.Equal(t, 2, r.Expected)
	assert.Nil(t, find(t, r, cand).Average)

	y.fill(t, 0, cand, 60)
	got := find(t, results(t, x.interviewID, y.interviewID), cand)
	require.NotNil(t, got.Average)
	assert.InDelta(t, 70, *got.Average, 0.001)
}

func TestDefaultSelectionIsLatestOfEachMode(t *testing.T) {
	newWorld(t, "one_on_one", 1)
	newWorld(t, "panel", 1)
	one, panel := newWorld(t, "one_on_one", 1), newWorld(t, "panel", 2)

	r := results(t)
	var picked []string
	for _, i := range r.Interviews {
		if i.Selected {
			picked = append(picked, i.ID)
		}
	}
	assert.ElementsMatch(t, []string{one.interviewID, panel.interviewID}, picked)
	assert.Equal(t, 3, r.Expected)
}

func TestResultsGuards(t *testing.T) {
	assert.Equal(t, http.StatusUnprocessableEntity,
		apitest.Do(t, srv, "GET", "/v1/results?interview="+uuid.NewString(), apitest.AdminToken(t), nil).StatusCode)
	assert.Equal(t, http.StatusForbidden,
		apitest.Do(t, srv, "GET", "/v1/results", apitest.JudgeToken(t, uuid.NewString()), nil).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, apitest.Do(t, srv, "GET", "/v1/results", "", nil).StatusCode)
}
