package interview

import (
	"context"

	"github.com/gihyunx/interview-tabulation/internal/db"
	"github.com/jackc/pgx/v5"
)

const selectInterview = `
	SELECT i.interview_id, i.rubric_id, r.name AS rubric_name, i.title, i.description, i.mode,
		i.position, i.created_at, i.updated_at,
		EXISTS (SELECT 1 FROM score s JOIN judge j USING (judge_id) WHERE j.interview_id = i.interview_id) AS has_scores,
		COALESCE((SELECT jsonb_agg(jsonb_build_object(
				'judge_id', j.judge_id, 'user_id', u.user_id, 'first_name', u.first_name, 'last_name', u.last_name,
				'scored', (SELECT count(DISTINCT s.candidate_id) FROM score s WHERE s.judge_id = j.judge_id))
				ORDER BY u.last_name, u.first_name)
			FROM judge j JOIN users u USING (user_id) WHERE j.interview_id = i.interview_id), '[]') AS judges
	FROM interview i JOIN rubric r USING (rubric_id)`

func list(ctx context.Context, q db.Querier) ([]interview, error) {
	rows, err := q.Query(ctx, selectInterview+` ORDER BY i.position, i.created_at, i.interview_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[interview])
}

func get(ctx context.Context, q db.Querier, id string) (interview, error) {
	rows, err := q.Query(ctx, selectInterview+` WHERE i.interview_id = @id`, pgx.NamedArgs{"id": id})
	if err != nil {
		return interview{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[interview])
}

func lockInterview(ctx context.Context, q db.Querier, id string) error {
	var x string
	return q.QueryRow(ctx, `SELECT interview_id FROM interview WHERE interview_id = @id FOR UPDATE`,
		pgx.NamedArgs{"id": id}).Scan(&x)
}

func insert(ctx context.Context, q db.Querier, in interviewCreateInput) (string, error) {
	var id string
	err := q.QueryRow(ctx, `
		INSERT INTO interview (rubric_id, title, description, mode, position)
		VALUES (@rubric_id, @title, @description, @mode,
			(SELECT COALESCE(MAX(position) + 1, 0) FROM interview))
		RETURNING interview_id`, pgx.NamedArgs{
		"rubric_id": in.RubricID, "title": in.Title, "description": in.Description, "mode": in.Mode,
	}).Scan(&id)
	return id, err
}

func update(ctx context.Context, q db.Querier, id string, in interviewUpdateInput) error {
	_, err := q.Exec(ctx, `
		UPDATE interview SET
			title = COALESCE(@title, title), description = COALESCE(@description, description),
			mode = COALESCE(@mode, mode), rubric_id = COALESCE(@rubric_id, rubric_id), updated_at = NOW()
		WHERE interview_id = @id`, pgx.NamedArgs{
		"id": id, "title": in.Title, "description": in.Description, "mode": in.Mode, "rubric_id": in.RubricID,
	})
	return err
}

func deleteJudges(ctx context.Context, q db.Querier, id string) error {
	_, err := q.Exec(ctx, `DELETE FROM judge WHERE interview_id = @id`, pgx.NamedArgs{"id": id})
	return err
}

func deleteInterview(ctx context.Context, q db.Querier, id string) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM interview WHERE interview_id = @id`, pgx.NamedArgs{"id": id})
	return tag.RowsAffected(), err
}

// countJudgeUsers returns how many of userIDs exist with role judge.
func countJudgeUsers(ctx context.Context, q db.Querier, userIDs []string) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'judge' AND user_id = ANY(@ids::uuid[])`,
		pgx.NamedArgs{"ids": userIDs}).Scan(&n)
	return n, err
}

// removeJudgesExcept fails with a foreign key error if a removed judge has scores.
func removeJudgesExcept(ctx context.Context, q db.Querier, interviewID string, keep []string) error {
	_, err := q.Exec(ctx, `DELETE FROM judge WHERE interview_id = @id AND NOT (user_id = ANY(@keep::uuid[]))`,
		pgx.NamedArgs{"id": interviewID, "keep": keep})
	return err
}

func addJudges(ctx context.Context, q db.Querier, interviewID string, userIDs []string) error {
	_, err := q.Exec(ctx, `
		INSERT INTO judge (user_id, interview_id)
		SELECT u, @id FROM unnest(@ids::uuid[]) AS u
		ON CONFLICT (user_id, interview_id) DO NOTHING`, pgx.NamedArgs{"id": interviewID, "ids": userIDs})
	return err
}

func listIDs(ctx context.Context, q db.Querier) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT interview_id FROM interview ORDER BY interview_id FOR UPDATE`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func setPositions(ctx context.Context, q db.Querier, ids []string) error {
	_, err := q.Exec(ctx, `
		UPDATE interview SET position = o.pos - 1, updated_at = NOW()
		FROM unnest(@ids::uuid[]) WITH ORDINALITY AS o(id, pos)
		WHERE interview.interview_id = o.id`, pgx.NamedArgs{"ids": ids})
	return err
}

func listMine(ctx context.Context, q db.Querier, userID string) ([]myInterview, error) {
	rows, err := q.Query(ctx, `
		SELECT i.interview_id AS id, j.judge_id, i.title, i.description, i.mode,
			(SELECT count(*) FROM candidate)::int AS candidate_count,
			jsonb_build_object('id', r.rubric_id, 'name', r.name, 'criteria', COALESCE((
				SELECT jsonb_agg(jsonb_build_object('id', c.criterion_id, 'title', c.title,
					'description', c.description, 'max_points', c.max_points) ORDER BY c.position)
				FROM criterion c WHERE c.rubric_id = r.rubric_id), '[]')) AS rubric
		FROM judge j JOIN interview i USING (interview_id) JOIN rubric r USING (rubric_id)
		WHERE j.user_id = @user_id
		ORDER BY i.position, i.created_at`, pgx.NamedArgs{"user_id": userID})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[myInterview])
}
