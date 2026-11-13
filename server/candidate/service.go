package candidate

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gihyunx/interview-tabulation/internal/apierr"
	"github.com/gihyunx/interview-tabulation/internal/db"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/gihyunx/interview-tabulation/internal/pgerr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errNotFound    = errors.New("candidate not found")
	errHasScores   = errors.New("candidate has scores")
	errNumberTaken = errors.New("candidate number taken")
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
	if pgerr.IsUniqueViolation(err) {
		return candidate{}, errNumberTaken
	}
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
	if pgerr.IsUniqueViolation(err) {
		return candidate{}, errNumberTaken
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

func (s *service) renumber(ctx context.Context, actorID string) error {
	if err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return renumber(ctx, tx) }); err != nil {
		return err
	}
	slog.Info("candidates renumbered", "actor_id", actorID)
	s.changed()
	return nil
}

// parseCSV reads a `first_name,last_name` or `number,first_name,last_name`
// file. Row errors are returned per row; a bad header is a 422.
func parseCSV(r io.Reader) ([]importRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	head, err := cr.Read()
	if err != nil {
		return nil, apierr.Invalid("file is empty or unreadable")
	}
	for i := range head {
		head[i] = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(head[i], "\xef\xbb\xbf")))
	}
	header := strings.Join(head, ",")
	if header != "first_name,last_name" && header != "number,first_name,last_name" {
		return nil, apierr.Invalid("the header must be exactly first_name,last_name or number,first_name,last_name")
	}
	numbered := len(head) == 3

	var rows []importRow
	seen := map[int]bool{}
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
		case len(rec) != len(head):
			row.Line, _ = cr.FieldPos(0)
			row.Error = fmt.Sprintf("expected %d columns", len(head))
		default:
			row.Line, _ = cr.FieldPos(0)
			if numbered {
				row.Number, row.Error = parseNumber(rec[0], seen)
				rec = rec[1:]
			}
			row.FirstName, row.LastName = strings.TrimSpace(rec[0]), strings.TrimSpace(rec[1])
			if row.Error == "" && (row.FirstName == "" || row.LastName == "") {
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

// parseNumber reads a candidate number and records it in seen.
func parseNumber(field string, seen map[int]bool) (int, string) {
	n, err := strconv.Atoi(strings.TrimSpace(field))
	switch {
	case err != nil || n < 1:
		return n, "number must be a whole number of 1 or more"
	case seen[n]:
		return n, "number appears twice in the file"
	}
	seen[n] = true
	return n, ""
}

// markTaken flags rows whose number already belongs to a candidate.
func markTaken(ctx context.Context, q db.Querier, rows []importRow) error {
	var nums []int
	for _, r := range rows {
		if r.Number > 0 {
			nums = append(nums, r.Number)
		}
	}
	if len(nums) == 0 {
		return nil
	}
	taken, err := takenNumbers(ctx, q, nums)
	for i := range rows {
		if rows[i].Error == "" && taken[rows[i].Number] {
			rows[i].Error = "another candidate already has this number"
		}
	}
	return err
}

// importCSV returns the parsed rows. Unless dryRun, it saves them all in one
// transaction, or none if any row has an error. Without a number column,
// numbers continue from the highest existing one, in file order.
func (s *service) importCSV(ctx context.Context, actorID string, r io.Reader, dryRun bool) ([]importRow, error) {
	rows, err := parseCSV(r)
	if err != nil {
		return nil, err
	}
	if err := markTaken(ctx, s.pool, rows); err != nil {
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
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return insertMany(ctx, tx, rows) })
	if pgerr.IsUniqueViolation(err) {
		return nil, errNumberTaken
	}
	if err != nil {
		return nil, err
	}
	slog.Info("candidates imported", "actor_id", actorID, "count", len(rows))
	s.changed()
	return rows, nil
}
