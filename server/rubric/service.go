package rubric

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gihyunx/interview-tabulation/internal/apierr"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/gihyunx/interview-tabulation/internal/pgerr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errNotFound  = errors.New("rubric not found")
	errHasScores = errors.New("rubric has scores")
)

type service struct {
	pool *pgxpool.Pool
	bus  *event.Bus
}

func (s *service) changed() { s.bus.Publish(event.Event{Topic: "rubrics"}) }

func validate(in saveInput) error {
	total := 0
	for _, c := range in.Criteria {
		total += int(c.MaxPoints)
	}
	if total != 100 {
		return apierr.Invalid(fmt.Sprintf("criteria points must total 100 (they total %d)", total))
	}
	return nil
}

func (s *service) create(ctx context.Context, actorID string, in saveInput) (rubric, error) {
	if err := validate(in); err != nil {
		return rubric{}, err
	}
	var id string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if id, err = insertRubric(ctx, tx, in.Name); err != nil {
			return err
		}
		for i, c := range in.Criteria {
			if err := insertCriterion(ctx, tx, id, c, i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return rubric{}, err
	}
	slog.Info("rubric created", "actor_id", actorID, "rubric_id", id)
	s.changed()
	return get(ctx, s.pool, id)
}

// update saves name and criteria together. Once a score uses the rubric the
// criteria set and every max_points are frozen; titles, descriptions and
// order still change.
func (s *service) update(ctx context.Context, actorID, id string, in saveInput) (rubric, error) {
	if err := validate(in); err != nil {
		return rubric{}, err
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockRubric(ctx, tx, id); errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		} else if err != nil {
			return err
		}
		cur, err := get(ctx, tx, id)
		if err != nil {
			return err
		}
		existing := map[string]criterion{}
		for _, c := range cur.Criteria {
			existing[c.ID] = c
		}

		keep := make([]string, 0, len(in.Criteria))
		for _, c := range in.Criteria {
			if c.ID == "" {
				if cur.Locked {
					return apierr.Conflict("criteria cannot be added once the rubric has scores")
				}
				continue
			}
			old, ok := existing[c.ID]
			if !ok {
				return apierr.Invalid(fmt.Sprintf("criterion %s is not in this rubric", c.ID))
			}
			if cur.Locked && old.MaxPoints != c.MaxPoints {
				return apierr.Conflict("points cannot change once the rubric has scores")
			}
			keep = append(keep, c.ID)
		}
		if cur.Locked && len(keep) != len(existing) {
			return apierr.Conflict("criteria cannot be removed once the rubric has scores")
		}

		if err := renameRubric(ctx, tx, id, in.Name); err != nil {
			return err
		}
		if err := deleteCriteriaExcept(ctx, tx, id, keep); err != nil {
			return err
		}
		for i, c := range in.Criteria {
			if c.ID == "" {
				err = insertCriterion(ctx, tx, id, c, i)
			} else {
				err = updateCriterion(ctx, tx, id, c, i)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if pgerr.IsForeignKeyViolation(err) {
		return rubric{}, apierr.Conflict("This rubric already has scores, so its criteria and points cannot change.")
	}
	if err != nil {
		return rubric{}, err
	}
	slog.Info("rubric updated", "actor_id", actorID, "rubric_id", id)
	s.changed()
	return get(ctx, s.pool, id)
}

func (s *service) get(ctx context.Context, id string) (rubric, error) {
	r, err := get(ctx, s.pool, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return rubric{}, errNotFound
	}
	return r, err
}

func (s *service) delete(ctx context.Context, actorID, id string) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		r, err := get(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if len(r.UsedBy) > 0 {
			return apierr.Conflict(fmt.Sprintf("used by %s", strings.Join(r.UsedBy, ", ")))
		}
		return deleteRubric(ctx, tx, id)
	})
	if pgerr.IsForeignKeyViolation(err) {
		return errHasScores
	}
	if err != nil {
		return err
	}
	slog.Info("rubric deleted", "actor_id", actorID, "rubric_id", id)
	s.changed()
	return nil
}
