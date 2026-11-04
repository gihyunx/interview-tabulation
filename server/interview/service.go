package interview

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/gihyunx/interview-tabulation/internal/apierr"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/gihyunx/interview-tabulation/internal/pgerr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errNotFound  = errors.New("interview not found")
	errHasScores = errors.New("interview has scores")
)

type service struct {
	pool *pgxpool.Pool
	bus  *event.Bus
}

func (s *service) changed(id string) {
	s.bus.Publish(event.Event{Topic: "interviews", InterviewID: id})
}

// setJudges replaces the judge set; removing a judge who has scores fails.
func setJudges(ctx context.Context, tx pgx.Tx, interviewID string, userIDs []string) error {
	// Never nil: a NULL array would make the DELETE below match nothing.
	userIDs = append([]string{}, slices.Compact(slices.Sorted(slices.Values(userIDs)))...)
	n, err := countJudgeUsers(ctx, tx, userIDs)
	if err != nil {
		return err
	}
	if n != len(userIDs) {
		return apierr.Invalid("every judge must be an existing user with the judge role")
	}
	if err := removeJudgesExcept(ctx, tx, interviewID, userIDs); err != nil {
		return err
	}
	return addJudges(ctx, tx, interviewID, userIDs)
}

func mapWriteErr(err error) error {
	if pgerr.IsForeignKeyViolation(err) {
		return apierr.Conflict("a judge who already has scores cannot be removed")
	}
	return err
}

func (s *service) create(ctx context.Context, actorID string, in interviewCreateInput) (interview, error) {
	var id string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if id, err = insert(ctx, tx, in); err != nil {
			return err
		}
		return setJudges(ctx, tx, id, in.JudgeIDs)
	})
	if pgerr.IsForeignKeyViolation(err) {
		return interview{}, apierr.Invalid("rubric does not exist")
	}
	if err != nil {
		return interview{}, err
	}
	slog.Info("interview created", "actor_id", actorID, "interview_id", id)
	s.changed(id)
	return get(ctx, s.pool, id)
}

func (s *service) update(ctx context.Context, actorID, id string, in interviewUpdateInput) (interview, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockInterview(ctx, tx, id); errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		} else if err != nil {
			return err
		}
		cur, err := get(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.HasScores {
			if in.RubricID != nil && *in.RubricID != cur.RubricID {
				return apierr.Conflict("the rubric cannot change once the interview has scores")
			}
			if in.Mode != nil && *in.Mode != cur.Mode {
				return apierr.Conflict("the mode cannot change once the interview has scores")
			}
		}
		return update(ctx, tx, id, in)
	})
	if pgerr.IsForeignKeyViolation(err) {
		return interview{}, apierr.Invalid("rubric does not exist")
	}
	if err != nil {
		return interview{}, err
	}
	slog.Info("interview updated", "actor_id", actorID, "interview_id", id)
	s.changed(id)
	return get(ctx, s.pool, id)
}

func (s *service) setJudges(ctx context.Context, actorID, id string, userIDs []string) (interview, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockInterview(ctx, tx, id); errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		} else if err != nil {
			return err
		}
		return setJudges(ctx, tx, id, userIDs)
	})
	if err != nil {
		return interview{}, mapWriteErr(err)
	}
	slog.Info("interview judges set", "actor_id", actorID, "interview_id", id)
	s.changed(id)
	return get(ctx, s.pool, id)
}

func (s *service) delete(ctx context.Context, actorID, id string) error {
	var n int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := deleteJudges(ctx, tx, id); err != nil {
			return err
		}
		var err error
		n, err = deleteInterview(ctx, tx, id)
		return err
	})
	if pgerr.IsForeignKeyViolation(err) {
		return errHasScores
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotFound
	}
	slog.Info("interview deleted", "actor_id", actorID, "interview_id", id)
	s.changed(id)
	return nil
}

// reorder saves position = index. ids must be exactly the existing set.
func (s *service) reorder(ctx context.Context, actorID string, ids []string) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := listIDs(ctx, tx)
		if err != nil {
			return err
		}
		if !slices.Equal(slices.Sorted(slices.Values(ids)), existing) {
			return apierr.Invalid("ids must be exactly the existing interviews, each once")
		}
		return setPositions(ctx, tx, ids)
	})
	if err != nil {
		return err
	}
	slog.Info("interviews reordered", "actor_id", actorID)
	s.changed("")
	return nil
}
