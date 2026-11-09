package dbtest

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Fixtures insert rows directly so tests do not depend on other slices.
type Fixtures struct {
	Pool *pgxpool.Pool
}

func (f Fixtures) one(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	require.NoError(t, f.Pool.QueryRow(context.Background(), sql, args...).Scan(&id))
	return id
}

// User inserts a user with a unique username and a dummy password hash.
func (f Fixtures) User(t *testing.T, role string) string {
	return f.one(t, `INSERT INTO users (username, password_hash, role, first_name, last_name)
		VALUES ($1, 'x', $2, 'First', 'Last') RETURNING user_id`, "u-"+uuid.NewString(), role)
}

// Rubric inserts a rubric with criteria of the given max points, in order.
func (f Fixtures) Rubric(t *testing.T, points ...int) (rubricID string, criterionIDs []string) {
	rubricID = f.one(t, `INSERT INTO rubric (name) VALUES ('Rubric') RETURNING rubric_id`)
	for i, p := range points {
		criterionIDs = append(criterionIDs, f.one(t, `
			INSERT INTO criterion (rubric_id, title, description, max_points, position)
			VALUES ($1, $2, '', $3, $4) RETURNING criterion_id`, rubricID, "C"+uuid.NewString()[:4], p, i))
	}
	return rubricID, criterionIDs
}

func (f Fixtures) Interview(t *testing.T, rubricID, mode string) string {
	return f.one(t, `INSERT INTO interview (rubric_id, title, description, mode, position)
		VALUES ($1, $2, '', $3, 0) RETURNING interview_id`, rubricID, "I-"+uuid.NewString()[:4], mode)
}

func (f Fixtures) Judge(t *testing.T, userID, interviewID string) string {
	return f.one(t, `INSERT INTO judge (user_id, interview_id) VALUES ($1, $2) RETURNING judge_id`, userID, interviewID)
}

func (f Fixtures) Candidate(t *testing.T) string {
	return f.one(t, `INSERT INTO candidate (number, first_name, last_name)
		VALUES ((SELECT COALESCE(MAX(number), 0) + 1 FROM candidate), 'Can', $1) RETURNING candidate_id`, "D-"+uuid.NewString()[:4])
}

func (f Fixtures) Score(t *testing.T, judgeID, candidateID, criterionID string, value int) {
	t.Helper()
	_, err := f.Pool.Exec(context.Background(),
		`INSERT INTO score (judge_id, candidate_id, criterion_id, value) VALUES ($1, $2, $3, $4)`,
		judgeID, candidateID, criterionID, value)
	require.NoError(t, err)
}
