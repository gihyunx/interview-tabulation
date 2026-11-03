package user

import (
	"context"

	"github.com/gihyunx/interview-tabulation/internal/db"
	"github.com/jackc/pgx/v5"
)

const userColumns = `user_id, username, role, first_name, last_name, created_at, updated_at`

func insertUser(ctx context.Context, q db.Querier, in userCreateInput, passwordHash string) (user, error) {
	rows, err := q.Query(ctx, `
		INSERT INTO users (username, password_hash, role, first_name, last_name)
		VALUES (@username, @password_hash, @role, @first_name, @last_name)
		RETURNING `+userColumns, pgx.NamedArgs{
		"username":      in.Username,
		"password_hash": passwordHash,
		"role":          in.Role,
		"first_name":    in.FirstName,
		"last_name":     in.LastName,
	})
	if err != nil {
		return user{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[user])
}

func listUsers(ctx context.Context, q db.Querier) ([]user, error) {
	rows, err := q.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY last_name, first_name, username`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[user])
}

func getUser(ctx context.Context, q db.Querier, id string) (user, error) {
	rows, err := q.Query(ctx, `SELECT `+userColumns+` FROM users WHERE user_id = @id`, pgx.NamedArgs{"id": id})
	if err != nil {
		return user{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[user])
}

// getCredentials returns the user and password hash for a (lowercased) username.
func getCredentials(ctx context.Context, q db.Querier, username string) (user, string, error) {
	type row struct {
		user         `db:",inline"`
		PasswordHash string `db:"password_hash"`
	}
	rows, err := q.Query(ctx, `SELECT `+userColumns+`, password_hash FROM users WHERE username = @username`,
		pgx.NamedArgs{"username": username})
	if err != nil {
		return user{}, "", err
	}
	r, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[row])
	return r.user, r.PasswordHash, err
}

func updateUser(ctx context.Context, q db.Querier, id string, in userUpdateInput) (user, error) {
	rows, err := q.Query(ctx, `
		UPDATE users SET
			username   = COALESCE(@username, username),
			role       = COALESCE(@role, role),
			first_name = COALESCE(@first_name, first_name),
			last_name  = COALESCE(@last_name, last_name),
			updated_at = NOW()
		WHERE user_id = @id
		RETURNING `+userColumns, pgx.NamedArgs{
		"id":         id,
		"username":   in.Username,
		"role":       in.Role,
		"first_name": in.FirstName,
		"last_name":  in.LastName,
	})
	if err != nil {
		return user{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[user])
}

func setPassword(ctx context.Context, q db.Querier, id, passwordHash string) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE users SET password_hash = @hash, updated_at = NOW() WHERE user_id = @id`,
		pgx.NamedArgs{"id": id, "hash": passwordHash})
	return tag.RowsAffected(), err
}

func deleteJudgeAssignments(ctx context.Context, q db.Querier, id string) error {
	_, err := q.Exec(ctx, `DELETE FROM judge WHERE user_id = @id`, pgx.NamedArgs{"id": id})
	return err
}

func deleteUser(ctx context.Context, q db.Querier, id string) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM users WHERE user_id = @id`, pgx.NamedArgs{"id": id})
	return tag.RowsAffected(), err
}
