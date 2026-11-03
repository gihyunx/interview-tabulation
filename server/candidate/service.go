package candidate

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gihyunx/interview-tabulation/internal/apierr"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/gihyunx/interview-tabulation/internal/pgerr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errNotFound  = errors.New("candidate not found")
	errHasScores = errors.New("candidate has scores")
)

type service struct {
	pool *pgxpool.Pool
	bus  *event.Bus
}

func (s *service) changed() { s.bus.Publish(event.Event{Topic: "candidates"}) }

func (s *service) create(ctx context.Context, actorID string, in nameInput) (candidate, error) {
	in.FirstName, in.LastName = strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName)
	if in.FirstName == "" || in.LastName == "" {
		return candidate{}, apierr.Invalid("name is empty")
	}
	c, err := insert(ctx, s.pool, in)
	if err != nil {
		return candidate{}, err
	}
	slog.Info("candidate created", "actor_id", actorID, "candidate_id", c.ID)
	s.changed()
	return c, nil
}

func (s *service) update(ctx context.Context, actorID, id string, in candidateUpdateInput) (candidate, error) {
	c, err := update(ctx, s.pool, id, in)
	if errors.Is(err, pgx.ErrNoRows) {
		return candidate{}, errNotFound
	}
	if err != nil {
		return candidate{}, err
	}
	slog.Info("candidate updated", "actor_id", actorID, "candidate_id", id)
	s.changed()
	return c, nil
}

func (s *service) delete(ctx context.Context, actorID, id string) error {
	n, err := remove(ctx, s.pool, id)
	if pgerr.IsForeignKeyViolation(err) {
		return errHasScores
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotFound
	}
	slog.Info("candidate deleted", "actor_id", actorID, "candidate_id", id)
	s.changed()
	return nil
}

// parseCSV reads a `first_name,last_name` file. Row errors are returned per
// row; a bad header is a 422.
func parseCSV(r io.Reader) ([]importRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	head, err := cr.Read()
	if err != nil {
		return nil, apierr.Invalid("file is empty or unreadable")
	}
	if len(head) != 2 ||
		strings.ToLower(strings.TrimSpace(strings.TrimPrefix(head[0], "\xef\xbb\xbf"))) != "first_name" ||
		strings.ToLower(strings.TrimSpace(head[1])) != "last_name" {
		return nil, apierr.Invalid("the header must be exactly first_name,last_name")
	}

	var rows []importRow
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		row := importRow{}
		var pe *csv.ParseError
		switch {
		case errors.As(err, &pe):
			row.Line, row.Error = pe.Line, "could not read this line"
		case err != nil:
			return nil, err
		case len(rec) != 2:
			row.Line, _ = cr.FieldPos(0)
			row.Error = "expected 2 columns"
		default:
			row.Line, _ = cr.FieldPos(0)
			row.FirstName, row.LastName = strings.TrimSpace(rec[0]), strings.TrimSpace(rec[1])
			if row.FirstName == "" || row.LastName == "" {
				row.Error = "first and last name are required"
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, apierr.Invalid("no candidates in file")
	}
	return rows, nil
}

// importCSV returns the parsed rows. Unless dryRun, it saves them all in one
// transaction, or none if any row has an error.
func (s *service) importCSV(ctx context.Context, actorID string, r io.Reader, dryRun bool) ([]importRow, error) {
	rows, err := parseCSV(r)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return rows, nil
	}
	for _, row := range rows {
		if row.Error != "" {
			return rows, apierr.Invalid(fmt.Sprintf("line %d: %s", row.Line, row.Error))
		}
	}
	if err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return insertMany(ctx, tx, rows) }); err != nil {
		return nil, err
	}
	slog.Info("candidates imported", "actor_id", actorID, "count", len(rows))
	s.changed()
	return rows, nil
}
