package rubric

import (
	"context"

	"github.com/gihyunx/interview-tabulation/internal/db"
	"github.com/jackc/pgx/v5"
)

const selectRubric = `
	SELECT r.rubric_id, r.name, r.created_at, r.updated_at,
		EXISTS (
			SELECT 1 FROM score s JOIN criterion c USING (criterion_id) WHERE c.rubric_id = r.rubric_id
		) AS locked,
		COALESCE((SELECT array_agg(i.title ORDER BY i.position, i.created_at)
			FROM interview i WHERE i.rubric_id = r.rubric_id), '{}') AS used_by,
		COALESCE((SELECT jsonb_agg(jsonb_build_object(
				'id', c.criterion_id, 'title', c.title, 'description', c.description,
				'max_points', c.max_points, 'position', c.position) ORDER BY c.position)
			FROM criterion c WHERE c.rubric_id = r.rubric_id), '[]') AS criteria
	FROM rubric r`

func list(ctx context.Context, q db.Querier) ([]rubric, error) {
	rows, err := q.Query(ctx, selectRubric+` ORDER BY r.created_at, r.rubric_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[rubric])
}

func get(ctx context.Context, q db.Querier, id string) (rubric, error) {
	rows, err := q.Query(ctx, selectRubric+` WHERE r.rubric_id = @id`, pgx.NamedArgs{"id": id})
	if err != nil {
		return rubric{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[rubric])
}

func insertRubric(ctx context.Context, q db.Querier, name string) (string, error) {
	var id string
	err := q.QueryRow(ctx, `INSERT INTO rubric (name) VALUES (@name) RETURNING rubric_id`,
		pgx.NamedArgs{"name": name}).Scan(&id)
	return id, err
}

// lockRubric takes a row lock so two saves of one rubric run one after the other.
func lockRubric(ctx context.Context, q db.Querier, id string) error {
	var x string
	return q.QueryRow(ctx, `SELECT rubric_id FROM rubric WHERE rubric_id = @id FOR UPDATE`,
		pgx.NamedArgs{"id": id}).Scan(&x)
}

func renameRubric(ctx context.Context, q db.Querier, id, name string) error {
	_, err := q.Exec(ctx, `UPDATE rubric SET name = @name, updated_at = NOW() WHERE rubric_id = @id`,
		pgx.NamedArgs{"id": id, "name": name})
	return err
}

func insertCriterion(ctx context.Context, q db.Querier, rubricID string, c criterionInput, pos int) error {
	_, err := q.Exec(ctx, `
		INSERT INTO criterion (rubric_id, title, description, max_points, position)
		VALUES (@rubric_id, @title, @description, @max_points, @position)`, pgx.NamedArgs{
		"rubric_id": rubricID, "title": c.Title, "description": c.Description,
		"max_points": c.MaxPoints, "position": pos,
	})
	return err
}

func updateCriterion(ctx context.Context, q db.Querier, rubricID string, c criterionInput, pos int) error {
	_, err := q.Exec(ctx, `
		UPDATE criterion SET title = @title, description = @description, max_points = @max_points,
			position = @position, updated_at = NOW()
		WHERE criterion_id = @id AND rubric_id = @rubric_id`, pgx.NamedArgs{
		"id": c.ID, "rubric_id": rubricID, "title": c.Title, "description": c.Description,
		"max_points": c.MaxPoints, "position": pos,
	})
	return err
}

func deleteCriteriaExcept(ctx context.Context, q db.Querier, rubricID string, keep []string) error {
	_, err := q.Exec(ctx, `DELETE FROM criterion WHERE rubric_id = @rubric_id AND NOT (criterion_id = ANY(@keep::uuid[]))`,
		pgx.NamedArgs{"rubric_id": rubricID, "keep": keep})
	return err
}

func deleteRubric(ctx context.Context, q db.Querier, id string) error {
	if _, err := q.Exec(ctx, `DELETE FROM criterion WHERE rubric_id = @id`, pgx.NamedArgs{"id": id}); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `DELETE FROM rubric WHERE rubric_id = @id`, pgx.NamedArgs{"id": id})
	return err
}
