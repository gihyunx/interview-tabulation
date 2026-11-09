package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sampleRubric = "Mock Interview Rubric"

var sampleInterviews = []struct {
	title, description, mode string
	judges                   []string
}{
	{"1-on-1 Interview", "One judge interviews one candidate at a time.", "one_on_one", []string{"judge1", "judge2", "judge3"}},
	{"Panel Interview", "A panel of judges interviews each candidate.", "panel", []string{"judge4", "judge5", "judge6"}},
}

var sampleCriteria = []struct {
	title, description string
	points             int
}{
	{"Attendance and Punctuality", "Attends the interview and arrives on time.", 10},
	{"Appearance and Professional Etiquette", "Wears proper attire and demonstrates proper courtesy and respect.", 20},
	{"Confidence and Professionalism", "Maintains eye contact, posture, composure, and professional behavior.", 20},
	{"Communication Proficiency", "Speaks clearly with proper grammar and pronunciation. Can answer questions in an organized and logical manner.", 25},
	{"Overall Knowledge and Response Quality", "Demonstrates knowledge of the position, provides relevant answers, and shows critical thinking skills.", 25},
}

var sampleCandidates = [][2]string{
	{"Juan", "Dela Cruz"}, {"Maria", "Santos"}, {"Jose", "Reyes"}, {"Ana", "Garcia"}, {"Miguel", "Mendoza"},
	{"Sofia", "Torres"}, {"Carlo", "Ramos"}, {"Isabel", "Flores"}, {"Paolo", "Villanueva"}, {"Bianca", "Castillo"},
}

// seedSample adds judges (password "password"), the rubric, the interviews with
// their judges and sample candidates to a sandbox database. Safe to re-run:
// existing judges are kept and the rest is skipped if the rubric exists.
func seedSample(ctx context.Context, pool *pgxpool.Pool) error {
	if name := pool.Config().ConnConfig.Database; !strings.HasSuffix(name, "_sandbox") {
		return fmt.Errorf("sample data goes only in a sandbox database, got %q (use mise -E sandbox)", name)
	}

	for _, iv := range sampleInterviews {
		for _, j := range iv.judges {
			if err := user.SeedUser(ctx, pool, auth.RoleJudge, j, "password", "Judge", strings.TrimPrefix(j, "judge")); err != nil {
				return err
			}
		}
	}

	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM rubric WHERE name = @name)`,
			pgx.NamedArgs{"name": sampleRubric}).Scan(&exists); err != nil {
			return err
		}
		if exists {
			slog.Info("seed: sample data already exists")
			return nil
		}
		return insertSample(ctx, tx)
	})
}

func insertSample(ctx context.Context, tx pgx.Tx) error {
	var rubricID string
	if err := tx.QueryRow(ctx, `INSERT INTO rubric (name) VALUES (@name) RETURNING rubric_id`,
		pgx.NamedArgs{"name": sampleRubric}).Scan(&rubricID); err != nil {
		return err
	}
	for i, c := range sampleCriteria {
		if _, err := tx.Exec(ctx, `
				INSERT INTO criterion (rubric_id, title, description, max_points, position)
				VALUES (@rubric_id, @title, @description, @max_points, @position)`, pgx.NamedArgs{
			"rubric_id": rubricID, "title": c.title, "description": c.description,
			"max_points": c.points, "position": i,
		}); err != nil {
			return err
		}
	}

	for _, iv := range sampleInterviews {
		if _, err := tx.Exec(ctx, `
			WITH i AS (
				INSERT INTO interview (rubric_id, title, description, mode, position)
				VALUES (@rubric_id, @title, @description, @mode,
					(SELECT COALESCE(MAX(position) + 1, 0) FROM interview))
				RETURNING interview_id
			)
			INSERT INTO judge (user_id, interview_id)
			SELECT u.user_id, i.interview_id FROM users u, i WHERE u.username = ANY(@judges::text[])`,
			pgx.NamedArgs{
				"rubric_id": rubricID, "title": iv.title, "description": iv.description,
				"mode": iv.mode, "judges": iv.judges,
			}); err != nil {
			return err
		}
	}

	first := make([]string, len(sampleCandidates))
	last := make([]string, len(sampleCandidates))
	for i, c := range sampleCandidates {
		first[i], last[i] = c[0], c[1]
	}
	if _, err := tx.Exec(ctx, `
			INSERT INTO candidate (number, first_name, last_name)
			SELECT (SELECT COALESCE(MAX(number), 0) FROM candidate) + n, f, l
			FROM unnest(@first::text[], @last::text[]) WITH ORDINALITY AS t(f, l, n)`,
		pgx.NamedArgs{"first": first, "last": last}); err != nil {
		return err
	}

	slog.Info("seed: sample data added", "candidates", len(sampleCandidates))
	return nil
}
