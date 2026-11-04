// Package apierr carries a status and a message a judge or admin can read,
// from services to handlers.
package apierr

import (
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type Error struct {
	Status int
	Msg    string
}

func (e *Error) Error() string { return e.Msg }

func Conflict(msg string) error  { return &Error{http.StatusConflict, msg} }
func Invalid(msg string) error   { return &Error{http.StatusUnprocessableEntity, msg} }
func Forbidden(msg string) error { return &Error{http.StatusForbidden, msg} }

// Huma turns an *Error into a Huma error; anything else is returned as is.
func Huma(err error) error {
	if e, ok := errors.AsType[*Error](err); ok {
		return huma.NewError(e.Status, e.Msg)
	}
	return err
}
