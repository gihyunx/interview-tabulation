package candidate

import (
	"context"

	"github.com/gihyunx/interview-tabulation/internal/db"
	"github.com/jackc/pgx/v5"
)

const columns = `candidate_id, first_name, last_name, created_at, updated_at`

func list(ctx context.Context, q db.Querier) ([]candidate, error) {
	rows, err := q.Query(ctx, `SELECT `+columns+` FROM candidate ORDER BY last_name, first_name, candidate_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[candidate])
}

func insert(ctx context.Context, q db.Querier, in nameInput) (candidate, error) {
	rows, err := q.Query(ctx, `
		INSERT INTO candidate (first_name, last_name) VALUES (@first_name, @last_name)
		RETURNING `+columns, pgx.NamedArgs{"first_name": in.FirstName, "last_name": in.LastName})
	if err != nil {
		return candidate{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[candidate])
}

func insertMany(ctx context.Context, q db.Querier, rows []importRow) error {
	first := make([]string, len(rows))
	last := make([]string, len(rows))
	for i, r := range rows {
		first[i], last[i] = r.FirstName, r.LastName
	}
	_, err := q.Exec(ctx, `
		INSERT INTO candidate (first_name, last_name)
		SELECT * FROM unnest(@first::text[], @last::text[])`, pgx.NamedArgs{"first": first, "last": last})
	return err
}

func update(ctx context.Context, q db.Querier, id string, in candidateUpdateInput) (candidate, error) {
	rows, err := q.Query(ctx, `
		UPDATE candidate SET
			first_name = COALESCE(@first_name, first_name),
			last_name  = COALESCE(@last_name, last_name),
			updated_at = NOW()
		WHERE candidate_id = @id
		RETURNING `+columns, pgx.NamedArgs{"id": id, "first_name": in.FirstName, "last_name": in.LastName})
	if err != nil {
		return candidate{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[candidate])
}

func remove(ctx context.Context, q db.Querier, id string) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM candidate WHERE candidate_id = @id`, pgx.NamedArgs{"id": id})
	return tag.RowsAffected(), err
}
