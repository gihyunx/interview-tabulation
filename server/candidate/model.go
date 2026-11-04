package candidate

import "time"

type candidate struct {
	ID        string    `json:"id" db:"candidate_id"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type nameInput struct {
	FirstName string `json:"first_name" minLength:"1" maxLength:"100"`
	LastName  string `json:"last_name" minLength:"1" maxLength:"100"`
}

type candidateUpdateInput struct {
	FirstName *string `json:"first_name,omitempty" minLength:"1" maxLength:"100"`
	LastName  *string `json:"last_name,omitempty" minLength:"1" maxLength:"100"`
}

type importRow struct {
	Line      int    `json:"line"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Error     string `json:"error,omitempty"`
}
