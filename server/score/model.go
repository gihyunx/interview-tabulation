package score

import "github.com/danielgtaylor/huma/v2"

type scoreValue struct {
	CriterionID string `json:"criterion_id" format:"uuid"`
	Value       int16  `json:"value" minimum:"0"`
}

type status string

const (
	statusUnscored      status = "unscored"
	statusScored        status = "scored"
	statusScoredByOther status = "scored_by_other"
)

func (status) Schema(r huma.Registry) *huma.Schema {
	return &huma.Schema{Type: "string", Enum: []any{string(statusUnscored), string(statusScored), string(statusScoredByOther)}}
}

type boardCandidate struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Status    status `json:"status"`
	// Total of my scores; set when Status is scored.
	Total *int `json:"total,omitempty"`
	// ScoredBy is the other judge's name when Status is scored_by_other (1-on-1 only).
	ScoredBy *string      `json:"scored_by,omitempty"`
	MyScores []scoreValue `json:"my_scores"`
}

type board struct {
	JudgeID    string           `json:"judge_id"`
	Candidates []boardCandidate `json:"candidates"`
}

type submitInput struct {
	Scores []scoreValue `json:"scores" maxItems:"50"`
}
