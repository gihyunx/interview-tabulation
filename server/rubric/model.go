package rubric

import "time"

type criterion struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MaxPoints   int16  `json:"max_points"`
	Position    int16  `json:"position"`
}

type rubric struct {
	ID        string    `json:"id" db:"rubric_id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	// Locked: some score uses this rubric, so criteria and points are frozen.
	Locked bool `json:"locked" db:"locked"`
	// UsedBy lists the titles of interviews using this rubric.
	UsedBy   []string    `json:"used_by" db:"used_by"`
	Criteria []criterion `json:"criteria" db:"criteria"`
}

type criterionInput struct {
	// ID is empty for a new criterion.
	ID          string `json:"id,omitempty" format:"uuid"`
	Title       string `json:"title" minLength:"1" maxLength:"200"`
	Description string `json:"description" maxLength:"1000"`
	MaxPoints   int16  `json:"max_points" minimum:"1" maximum:"100"`
}

type saveInput struct {
	Name     string           `json:"name" minLength:"1" maxLength:"200"`
	Criteria []criterionInput `json:"criteria" minItems:"1" maxItems:"50"`
}
