package score

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gihyunx/interview-tabulation/internal/db"
	"github.com/jackc/pgx/v5"
)

type assignment struct {
	JudgeID  string
	Mode     string
	RubricID string
}

// getAssignment finds the judge row for a user in an interview.
func getAssignment(ctx context.Context, q db.Querier, userID, interviewID string) (assignment, error) {
	var a assignment
	err := q.QueryRow(ctx, `
		SELECT j.judge_id, i.mode, i.rubric_id
		FROM judge j JOIN interview i USING (interview_id)
		WHERE j.user_id = @user_id AND j.interview_id = @interview_id`,
		pgx.NamedArgs{"user_id": userID, "interview_id": interviewID}).Scan(&a.JudgeID, &a.Mode, &a.RubricID)
	return a, err
}

// lockCandidate serializes submits for one candidate: the second waits for the first to commit.
func lockCandidate(ctx context.Context, q db.Querier, candidateID string) error {
	var x string
	return q.QueryRow(ctx, `SELECT candidate_id FROM candidate WHERE candidate_id = @id FOR UPDATE`,
		pgx.NamedArgs{"id": candidateID}).Scan(&x)
}

func maxPoints(ctx context.Context, q db.Querier, rubricID string) (map[string]int16, error) {
	rows, err := q.Query(ctx, `SELECT criterion_id, max_points FROM criterion WHERE rubric_id = @id`, pgx.NamedArgs{"id": rubricID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int16{}
	for rows.Next() {
		var id string
		var p int16
		if err := rows.Scan(&id, &p); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}

// otherJudgeName returns who else in the interview already scored the candidate ("" if nobody).
func otherJudgeName(ctx context.Context, q db.Querier, interviewID, judgeID, candidateID string) (string, error) {
	var name string
	err := q.QueryRow(ctx, `
		SELECT u.first_name || ' ' || u.last_name
		FROM score s JOIN judge j USING (judge_id) JOIN users u USING (user_id)
		WHERE j.interview_id = @interview_id AND j.judge_id <> @judge_id AND s.candidate_id = @candidate_id
		LIMIT 1`, pgx.NamedArgs{"interview_id": interviewID, "judge_id": judgeID, "candidate_id": candidateID}).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, err
}

func upsertScores(ctx context.Context, q db.Querier, judgeID, candidateID string, scores []scoreValue) error {
	ids := make([]string, len(scores))
	vals := make([]int16, len(scores))
	for i, s := range scores {
		ids[i], vals[i] = s.CriterionID, s.Value
	}
	_, err := q.Exec(ctx, `
		INSERT INTO score (judge_id, candidate_id, criterion_id, value)
		SELECT @judge_id, @candidate_id, t.c, t.v FROM unnest(@criteria::uuid[], @values::int2[]) AS t(c, v)
		ON CONFLICT (judge_id, candidate_id, criterion_id)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		pgx.NamedArgs{"judge_id": judgeID, "candidate_id": candidateID, "criteria": ids, "values": vals})
	return err
}

// loadBoard returns candidates from the judge's point of view. candidateID narrows it to one.
func loadBoard(ctx context.Context, q db.Querier, interviewID, judgeID string, oneOnOne bool, candidateID *string) ([]boardCandidate, error) {
	rows, err := q.Query(ctx, `
		SELECT c.candidate_id, c.number, c.first_name, c.last_name,
			COALESCE((SELECT jsonb_agg(jsonb_build_object('criterion_id', s.criterion_id, 'value', s.value))
				FROM score s WHERE s.judge_id = @judge_id AND s.candidate_id = c.candidate_id), '[]') AS my_scores,
			(SELECT u.first_name || ' ' || u.last_name
				FROM score s JOIN judge j USING (judge_id) JOIN users u USING (user_id)
				WHERE j.interview_id = @interview_id AND j.judge_id <> @judge_id AND s.candidate_id = c.candidate_id
				LIMIT 1) AS other_judge
		FROM candidate c
		WHERE @candidate_id::uuid IS NULL OR c.candidate_id = @candidate_id::uuid
		ORDER BY c.number`,
		pgx.NamedArgs{"interview_id": interviewID, "judge_id": judgeID, "candidate_id": candidateID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []boardCandidate{}
	for rows.Next() {
		var c boardCandidate
		var mine []byte
		var other *string
		if err := rows.Scan(&c.ID, &c.Number, &c.FirstName, &c.LastName, &mine, &other); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(mine, &c.MyScores); err != nil {
			return nil, err
		}
		switch {
		case len(c.MyScores) > 0:
			c.Status = statusScored
			total := 0
			for _, s := range c.MyScores {
				total += int(s.Value)
			}
			c.Total = &total
		case oneOnOne && other != nil:
			c.Status, c.ScoredBy = statusScoredByOther, other
		default:
			c.Status = statusUnscored
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func deleteSubmission(ctx context.Context, q db.Querier, interviewID, judgeID, candidateID string) (int64, error) {
	tag, err := q.Exec(ctx, `
		DELETE FROM score USING judge j
		WHERE score.judge_id = j.judge_id AND j.judge_id = @judge_id AND j.interview_id = @interview_id
			AND score.candidate_id = @candidate_id`,
		pgx.NamedArgs{"judge_id": judgeID, "interview_id": interviewID, "candidate_id": candidateID})
	return tag.RowsAffected(), err
}
