package user

import (
	"time"

	"github.com/gihyunx/interview-tabulation/internal/auth"
)

type user struct {
	ID        string    `json:"id" db:"user_id"`
	Username  string    `json:"username" db:"username"`
	Role      auth.Role `json:"role" db:"role"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type userCreateInput struct {
	Username  string    `json:"username" minLength:"3" maxLength:"50" pattern:"^[A-Za-z0-9._-]+$"`
	Password  string    `json:"password" minLength:"8" maxLength:"72"`
	Role      auth.Role `json:"role"`
	FirstName string    `json:"first_name" minLength:"1" maxLength:"100"`
	LastName  string    `json:"last_name" minLength:"1" maxLength:"100"`
}

type userUpdateInput struct {
	Username  *string    `json:"username,omitempty" minLength:"3" maxLength:"50" pattern:"^[A-Za-z0-9._-]+$"`
	Role      *auth.Role `json:"role,omitempty"`
	FirstName *string    `json:"first_name,omitempty" minLength:"1" maxLength:"100"`
	LastName  *string    `json:"last_name,omitempty" minLength:"1" maxLength:"100"`
}
