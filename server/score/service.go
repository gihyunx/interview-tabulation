package score

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gihyunx/interview-tabulation/internal/apierr"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errNotFound = errors.New("not found")

type service struct {
	pool *pgxpool.Pool
	bus  *event.Bus
}

func (s *service) changed(interviewID string) {
	s.bus.Publish(event.Event{Topic: "scores", InterviewID: interviewID})
}

func assigned(ctx context.Context, q pgx.Tx, userID, interviewID string) (assignment, error) {
	a, err := getAssignment(ctx, q, userID, interviewID)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, apierr.Forbidden("You are not assigned to this interview.")
	}
	return a, err
}

func (s *service) board(ctx context.Context, userID, interviewID string) (board, error) {
	var b board
	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		a, err := assigned(ctx, tx, userID, interviewID)
		if err != nil {
			return err
		}
		b.JudgeID = a.JudgeID
		b.Candidates, err = loadBoard(ctx, tx, interviewID, a.JudgeID, a.Mode == "one_on_one", nil)
		return err
	})
	return b, err
}

func validate(scores []scoreValue, max map[string]int16) error {
	if len(scores) != len(max) {
		return apierr.Invalid(fmt.Sprintf("Score every criterion: expected %d scores, got %d.", len(max), len(scores)))
	}
	seen := map[string]bool{}
	for _, sc := range scores {
		m, ok := max[sc.CriterionID]
		switch {
		case !ok:
			return apierr.Invalid("A score is for a criterion that is not in this interview's rubric.")
		case seen[sc.CriterionID]:
			return apierr.Invalid("A criterion is scored twice.")
		case sc.Value < 0 || sc.Value > m:
			return apierr.Invalid(fmt.Sprintf("Each score must be between 0 and the criterion's maximum (%d).", m))
		}
		seen[sc.CriterionID] = true
	}
	return nil
}

// submit saves a judge's scores for a candidate (insert or update) in one
// transaction. The candidate row lock makes two simultaneous 1-on-1 submits
// run one after the other, so only the first wins.
func (s *service) submit(ctx context.Context, userID, interviewID, candidateID string, scores []scoreValue) (boardCandidate, error) {
	var out boardCandidate
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		a, err := assigned(ctx, tx, userID, interviewID)
		if err != nil {
			return err
		}
		if err := lockCandidate(ctx, tx, candidateID); errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		} else if err != nil {
			return err
		}
		max, err := maxPoints(ctx, tx, a.RubricID)
		if err != nil {
			return err
		}
		if err := validate(scores, max); err != nil {
			return err
		}
		oneOnOne := a.Mode == "one_on_one"
		if oneOnOne {
			other, err := otherJudgeName(ctx, tx, interviewID, a.JudgeID, candidateID)
			if err != nil {
				return err
			}
			if other != "" {
				return apierr.Conflict("Already scored by " + other + ".")
			}
		}
		if err := upsertScores(ctx, tx, a.JudgeID, candidateID, scores); err != nil {
			return err
		}
		cs, err := loadBoard(ctx, tx, interviewID, a.JudgeID, oneOnOne, &candidateID)
		if err != nil {
			return err
		}
		out = cs[0]
		return nil
	})
	if err != nil {
		return boardCandidate{}, err
	}
	slog.Info("scores submitted", "user_id", userID, "interview_id", interviewID, "candidate_id", candidateID)
	s.changed(interviewID)
	return out, nil
}

// clear deletes one judge's scores for one candidate, which also frees the
// candidate for other 1-on-1 judges.
func (s *service) clear(ctx context.Context, actorID, interviewID, judgeID, candidateID string) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockCandidate(ctx, tx, candidateID); errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		} else if err != nil {
			return err
		}
		n, err := deleteSubmission(ctx, tx, interviewID, judgeID, candidateID)
		if err != nil {
			return err
		}
		if n == 0 {
			return errNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	slog.Info("submission cleared", "actor_id", actorID, "interview_id", interviewID, "judge_id", judgeID, "candidate_id", candidateID)
	s.changed(interviewID)
	return nil
}
