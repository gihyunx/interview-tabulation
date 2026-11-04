package interview

import (
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type mode string

const (
	modeOneOnOne mode = "one_on_one"
	modePanel    mode = "panel"
)

func (mode) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{Type: "string", Enum: []any{string(modeOneOnOne), string(modePanel)}}
}

type judgeInfo struct {
	JudgeID   string `json:"judge_id"`
	UserID    string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	// Scored is how many candidates this judge has submitted.
	Scored int `json:"scored"`
}

type interview struct {
	ID          string      `json:"id" db:"interview_id"`
	RubricID    string      `json:"rubric_id" db:"rubric_id"`
	RubricName  string      `json:"rubric_name" db:"rubric_name"`
	Title       string      `json:"title" db:"title"`
	Description string      `json:"description" db:"description"`
	Mode        mode        `json:"mode" db:"mode"`
	Position    int16       `json:"position" db:"position"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
	HasScores   bool        `json:"has_scores" db:"has_scores"`
	Judges      []judgeInfo `json:"judges" db:"judges"`
}

type interviewCreateInput struct {
	Title       string   `json:"title" minLength:"1" maxLength:"200"`
	Description string   `json:"description" maxLength:"1000"`
	Mode        mode     `json:"mode"`
	RubricID    string   `json:"rubric_id" format:"uuid"`
	JudgeIDs    []string `json:"judge_user_ids,omitempty" doc:"User ids to assign as judges." format:"uuid"`
}

type interviewUpdateInput struct {
	Title       *string `json:"title,omitempty" minLength:"1" maxLength:"200"`
	Description *string `json:"description,omitempty" maxLength:"1000"`
	Mode        *mode   `json:"mode,omitempty"`
	RubricID    *string `json:"rubric_id,omitempty" format:"uuid"`
}

// myInterview is what a judge sees: their assignment plus the rubric to score with.
type myInterview struct {
	ID             string   `json:"id"`
	JudgeID        string   `json:"judge_id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Mode           mode     `json:"mode"`
	CandidateCount int      `json:"candidate_count"`
	Rubric         myRubric `json:"rubric"`
}

type myRubric struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Criteria []myCriterion `json:"criteria"`
}

type myCriterion struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MaxPoints   int16  `json:"max_points"`
}
