package score

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/apierr"
	"github.com/gihyunx/interview-tabulation/internal/apitag"
	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ResultCriterion struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	MaxPoints int16  `json:"max_points"`
}

type ResultJudge struct {
	JudgeID string `json:"judge_id"`
	Name    string `json:"name"`
}

type ResultInterview struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Mode     string            `json:"mode" enum:"one_on_one,panel"`
	Selected bool              `json:"selected"`
	Judges   []ResultJudge     `json:"judges"`
	Criteria []ResultCriterion `json:"criteria"`
}

type SubmissionValue struct {
	CriterionID string `json:"criterion_id"`
	Value       int16  `json:"value"`
}

// Submission is one judge's scores for one candidate.
type Submission struct {
	InterviewID string            `json:"interview_id"`
	JudgeID     string            `json:"judge_id"`
	JudgeName   string            `json:"judge_name"`
	Total       int               `json:"total"`
	Values      []SubmissionValue `json:"values"`
	SubmittedAt time.Time         `json:"submitted_at"`
	EditedAt    time.Time         `json:"edited_at"`
}

type ResultCandidate struct {
	ID          string       `json:"id"`
	FirstName   string       `json:"first_name"`
	LastName    string       `json:"last_name"`
	Submissions []Submission `json:"submissions"`
	// Sum of every judge total in the selected interviews.
	Sum int `json:"sum"`
	// Average is set only when Complete: sum divided by the number of totals.
	Average  *float64 `json:"average,omitempty"`
	Complete bool     `json:"complete"`
}

type Results struct {
	// Interviews lists every interview; Selected marks the graded ones.
	Interviews []ResultInterview `json:"interviews"`
	Candidates []ResultCandidate `json:"candidates"`
	// Expected totals per candidate: 1 per selected 1-on-1, one per judge of each selected panel.
	Expected    int `json:"expected"`
	FullyScored int `json:"fully_scored"`
}

// LoadResults builds the results table for the selected interview ids. With no
// ids, it selects the most recently created interview of each mode. Pass a
// repeatable-read read-only transaction for a consistent snapshot.
func LoadResults(ctx context.Context, q db.Querier, selected []string) (Results, error) {
	var res Results
	byID := map[string]*ResultInterview{}
	createdAt := map[string]time.Time{}
	rubricOf := map[string]string{}

	rows, err := q.Query(ctx, `SELECT interview_id, title, mode, rubric_id, created_at FROM interview ORDER BY position, created_at, interview_id`)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var i ResultInterview
		var rubricID string
		var created time.Time
		if err := rows.Scan(&i.ID, &i.Title, &i.Mode, &rubricID, &created); err != nil {
			return res, err
		}
		i.Judges, i.Criteria = []ResultJudge{}, []ResultCriterion{}
		res.Interviews = append(res.Interviews, i)
		createdAt[i.ID], rubricOf[i.ID] = created, rubricID
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}
	for i := range res.Interviews {
		byID[res.Interviews[i].ID] = &res.Interviews[i]
	}

	if len(selected) == 0 {
		for _, mode := range []string{"one_on_one", "panel"} {
			var latest *ResultInterview
			for i := range res.Interviews {
				c := &res.Interviews[i]
				if c.Mode == mode && (latest == nil || createdAt[c.ID].After(createdAt[latest.ID])) {
					latest = c
				}
			}
			if latest != nil {
				selected = append(selected, latest.ID)
			}
		}
	}
	for _, id := range selected {
		i, ok := byID[id]
		if !ok {
			return res, apierr.Invalid("Unknown interview " + id + ".")
		}
		i.Selected = true
	}

	// Judges and criteria of every interview.
	rows, err = q.Query(ctx, `
		SELECT j.interview_id, j.judge_id, u.first_name || ' ' || u.last_name
		FROM judge j JOIN users u USING (user_id) ORDER BY u.last_name, u.first_name, j.judge_id`)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var interviewID string
		var j ResultJudge
		if err := rows.Scan(&interviewID, &j.JudgeID, &j.Name); err != nil {
			return res, err
		}
		byID[interviewID].Judges = append(byID[interviewID].Judges, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	rows, err = q.Query(ctx, `SELECT rubric_id, criterion_id, title, max_points FROM criterion ORDER BY position`)
	if err != nil {
		return res, err
	}
	crits := map[string][]ResultCriterion{}
	for rows.Next() {
		var rubricID string
		var c ResultCriterion
		if err := rows.Scan(&rubricID, &c.ID, &c.Title, &c.MaxPoints); err != nil {
			return res, err
		}
		crits[rubricID] = append(crits[rubricID], c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}
	for i := range res.Interviews {
		in := &res.Interviews[i]
		if c := crits[rubricOf[in.ID]]; c != nil {
			in.Criteria = c
		}
		if !in.Selected {
			continue
		}
		if in.Mode == "panel" {
			res.Expected += len(in.Judges)
		} else {
			res.Expected++
		}
	}

	// Candidates.
	rows, err = q.Query(ctx, `SELECT candidate_id, first_name, last_name FROM candidate ORDER BY last_name, first_name, candidate_id`)
	if err != nil {
		return res, err
	}
	res.Candidates = []ResultCandidate{}
	cands := map[string]int{}
	for rows.Next() {
		c := ResultCandidate{Submissions: []Submission{}}
		if err := rows.Scan(&c.ID, &c.FirstName, &c.LastName); err != nil {
			return res, err
		}
		cands[c.ID] = len(res.Candidates)
		res.Candidates = append(res.Candidates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	// Scores of the selected interviews, grouped into submissions.
	rows, err = q.Query(ctx, `
		SELECT j.interview_id, j.judge_id, u.first_name || ' ' || u.last_name, s.candidate_id,
			s.criterion_id, s.value, s.created_at, s.updated_at
		FROM score s JOIN judge j USING (judge_id) JOIN users u USING (user_id)
		WHERE j.interview_id = ANY(@ids::uuid[])
		ORDER BY s.created_at, s.score_id`, pgx.NamedArgs{"ids": append([]string{}, selected...)})
	if err != nil {
		return res, err
	}
	type key struct{ judge, candidate string }
	subs := map[key]*Submission{}
	for rows.Next() {
		var interviewID, judgeID, name, candidateID, criterionID string
		var value int16
		var created, updated time.Time
		if err := rows.Scan(&interviewID, &judgeID, &name, &candidateID, &criterionID, &value, &created, &updated); err != nil {
			return res, err
		}
		k := key{judgeID, candidateID}
		sub, ok := subs[k]
		if !ok {
			sub = &Submission{InterviewID: interviewID, JudgeID: judgeID, JudgeName: name, SubmittedAt: created, EditedAt: updated, Values: []SubmissionValue{}}
			subs[k] = sub
		}
		sub.Total += int(value)
		sub.Values = append(sub.Values, SubmissionValue{criterionID, value})
		if updated.After(sub.EditedAt) {
			sub.EditedAt = updated
		}
		if created.Before(sub.SubmittedAt) {
			sub.SubmittedAt = created
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}
	for k, sub := range subs {
		c := &res.Candidates[cands[k.candidate]]
		c.Submissions = append(c.Submissions, *sub)
	}

	order := map[string]int{}
	for i, in := range res.Interviews {
		order[in.ID] = i
	}
	for i := range res.Candidates {
		c := &res.Candidates[i]
		slices.SortFunc(c.Submissions, func(a, b Submission) int {
			if d := order[a.InterviewID] - order[b.InterviewID]; d != 0 {
				return d
			}
			if d := a.SubmittedAt.Compare(b.SubmittedAt); d != 0 {
				return d
			}
			return compareStr(a.JudgeID, b.JudgeID)
		})
		for _, s := range c.Submissions {
			c.Sum += s.Total
		}
		if res.Expected > 0 && len(c.Submissions) == res.Expected {
			c.Complete = true
			avg := float64(c.Sum) / float64(res.Expected)
			c.Average = &avg
			res.FullyScored++
		}
	}
	return res, nil
}

func compareStr(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// ReadResults loads results in one consistent read-only transaction.
func ReadResults(ctx context.Context, pool *pgxpool.Pool, selected []string) (Results, error) {
	var res Results
	err := pgx.BeginTxFunc(ctx, pool, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		var err error
		res, err = LoadResults(ctx, tx, selected)
		return err
	})
	return res, err
}

func setupResults(api huma.API, pool *pgxpool.Pool, cfg config.Config) {
	huma.Register(api, huma.Operation{
		OperationID: "results", Method: http.MethodGet, Path: "/results",
		Tags: []string{apitag.Results}, Summary: "Candidates with judge totals and average",
		Description: "Repeat `interview` to select the graded interviews. With none, the most recently created interview of each mode is used. The average is set only when every expected total is in.",
		Middlewares: huma.Middlewares{auth.RequireAuth(api, cfg.JWTSecret, auth.RoleAdmin)},
	}, func(ctx context.Context, in *struct {
		Interview []string `query:"interview,explode" format:"uuid"`
	}) (*struct{ Body Results }, error) {
		res, err := ReadResults(ctx, pool, in.Interview)
		if err != nil {
			return nil, apierr.Huma(err)
		}
		return &struct{ Body Results }{res}, nil
	})
}
