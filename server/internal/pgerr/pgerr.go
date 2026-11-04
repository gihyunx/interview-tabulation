package pgerr

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func code(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code
	}
	return ""
}

func IsUniqueViolation(err error) bool { return code(err) == "23505" }

func IsForeignKeyViolation(err error) bool { return code(err) == "23503" }

func IsCheckViolation(err error) bool { return code(err) == "23514" }
