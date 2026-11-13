package candidate

import (
	"context"

	"github.com/gihyunx/interview-tabulation/internal/db"
	"github.com/jackc/pgx/v5"
)

const columns = `candidate_id, number, first_name, last_name, created_at, updated_at`

func list(ctx context.Context, q db.Querier) ([]candidate, error) {
	rows, err := q.Query(ctx, `SELECT `+columns+` FROM candidate ORDER BY number`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[candidate])
}

// ponytail: MAX+1 races under concurrent creates (409 on the loser); fine for one admin.
func insert(ctx context.Context, q db.Querier, in nameInput) (candidate, error) {
	rows, err := q.Query(ctx, `
		INSERT INTO candidate (number, first_name, last_name)
		VALUES (COALESCE(@number, (SELECT COALESCE(MAX(number), 0) + 1 FROM candidate)), @first_name, @last_name)
		RETURNING `+columns, pgx.NamedArgs{"number": in.Number, "first_name": in.FirstName, "last_name": in.LastName})
	if err != nil {
		return candidate{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[candidate])
}

// insertMany numbers rows without a number after the highest existing one, in order.
func insertMany(ctx context.Context, q db.Querier, rows []importRow) error {
	num := make([]int, len(rows))
	first := make([]string, len(rows))
	last := make([]string, len(rows))
	for i, r := range rows {
		num[i], first[i], last[i] = r.Number, r.FirstName, r.LastName
	}
	_, err := q.Exec(ctx, `
		INSERT INTO candidate (number, first_name, last_name)
		SELECT COALESCE(NULLIF(num, 0), (SELECT COALESCE(MAX(number), 0) FROM candidate) + n), f, l
		FROM unnest(@num::int[], @first::text[], @last::text[]) WITH ORDINALITY AS t(num, f, l, n)`,
		pgx.NamedArgs{"num": num, "first": first, "last": last})
	return err
}

// takenNumbers returns which of nums already belong to a candidate.
func takenNumbers(ctx context.Context, q db.Querier, nums []int) (map[int]bool, error) {
	rows, err := q.Query(ctx, `SELECT number FROM candidate WHERE number = ANY(@nums)`, pgx.NamedArgs{"nums": nums})
	if err != nil {
		return nil, err
	}
	taken, err := pgx.CollectRows(rows, pgx.RowTo[int])
	out := map[int]bool{}
	for _, n := range taken {
		out[n] = true
	}
	return out, err
}

func update(ctx context.Context, q db.Querier, id string, in candidateUpdateInput) (candidate, error) {
	rows, err := q.Query(ctx, `
		UPDATE candidate SET
			number     = COALESCE(@number, number),
			first_name = COALESCE(@first_name, first_name),
			last_name  = COALESCE(@last_name, last_name),
			updated_at = NOW()
		WHERE candidate_id = @id
		RETURNING `+columns, pgx.NamedArgs{"id": id, "number": in.Number, "first_name": in.FirstName, "last_name": in.LastName})
	if err != nil {
		return candidate{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[candidate])
}

func remove(ctx context.Context, q db.Querier, id string) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM candidate WHERE candidate_id = @id`, pgx.NamedArgs{"id": id})
	return tag.RowsAffected(), err
}

// renumber closes gaps so numbers run 1..N in current order. The unique
// constraint is checked per row, so first shift everything above the old range.
func renumber(ctx context.Context, q db.Querier) error {
	_, err := q.Exec(ctx, `UPDATE candidate SET number = number + (SELECT MAX(number) FROM candidate)`)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		UPDATE candidate c SET number = r.n, updated_at = NOW()
		FROM (SELECT candidate_id, ROW_NUMBER() OVER (ORDER BY number) AS n FROM candidate) r
		WHERE c.candidate_id = r.candidate_id`)
	return err
}
