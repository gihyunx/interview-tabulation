package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/internal/event"
	"github.com/gihyunx/interview-tabulation/internal/pgerr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errBadLogin      = errors.New("wrong username or password")
	errNotFound      = errors.New("user not found")
	errUsernameTaken = errors.New("username already taken")
	errHasScores     = errors.New("user has scores")
	errSelf          = errors.New("cannot change your own account this way")
)

type service struct {
	pool      *pgxpool.Pool
	jwtSecret string
	bus       *event.Bus
}

func (s *service) changed() { s.bus.Publish(event.Event{Topic: "users"}) }

// create lowercases the username, hashes the password and inserts the user.
func (s *service) create(ctx context.Context, in userCreateInput) (user, error) {
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return user{}, err
	}
	u, err := insertUser(ctx, s.pool, in, hash)
	if pgerr.IsUniqueViolation(err) {
		return user{}, errUsernameTaken
	}
	if err != nil {
		return user{}, err
	}
	slog.Info("user created", "user_id", u.ID)
	s.changed()
	return u, nil
}

func (s *service) login(ctx context.Context, username, password string) (user, string, error) {
	u, hash, err := getCredentials(ctx, s.pool, strings.ToLower(strings.TrimSpace(username)))
	if errors.Is(err, pgx.ErrNoRows) {
		// Still burn a bcrypt compare so timing does not reveal valid usernames.
		_ = auth.CheckPassword(password, dummyHash)
		return user{}, "", errBadLogin
	}
	if err != nil {
		return user{}, "", err
	}
	if auth.CheckPassword(password, hash) != nil {
		return user{}, "", errBadLogin
	}
	token, err := auth.NewToken(s.jwtSecret, u.ID, u.Role)
	return u, token, err
}

// dummyHash is a real bcrypt hash, used for the unknown-user path.
var dummyHash = func() string {
	h, err := auth.HashPassword("unknown-user")
	if err != nil {
		panic(err)
	}
	return h
}()

func (s *service) get(ctx context.Context, id string) (user, error) {
	u, err := getUser(ctx, s.pool, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return user{}, errNotFound
	}
	return u, err
}

func (s *service) list(ctx context.Context) ([]user, error) {
	return listUsers(ctx, s.pool)
}

func (s *service) update(ctx context.Context, actorID, id string, in userUpdateInput) (user, error) {
	if in.Username != nil {
		v := strings.ToLower(strings.TrimSpace(*in.Username))
		in.Username = &v
	}
	// Changing your own role could lock the last admin out.
	if in.Role != nil && actorID == id {
		return user{}, errSelf
	}
	u, err := updateUser(ctx, s.pool, id, in)
	if errors.Is(err, pgx.ErrNoRows) {
		return user{}, errNotFound
	}
	if pgerr.IsUniqueViolation(err) {
		return user{}, errUsernameTaken
	}
	if err != nil {
		return user{}, err
	}
	slog.Info("user updated", "actor_id", actorID, "user_id", id)
	s.changed()
	return u, nil
}

func (s *service) resetPassword(ctx context.Context, actorID, id, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	n, err := setPassword(ctx, s.pool, id, hash)
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotFound
	}
	slog.Info("user password reset", "actor_id", actorID, "user_id", id)
	s.changed()
	return nil
}

// delete removes the user and their judge assignments in one transaction.
// Fails with errHasScores (nothing removed) if any assignment has scores.
func (s *service) delete(ctx context.Context, actorID, id string) error {
	if actorID == id {
		return errSelf
	}
	var n int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := deleteJudgeAssignments(ctx, tx, id); err != nil {
			return err
		}
		var err error
		n, err = deleteUser(ctx, tx, id)
		return err
	})
	if pgerr.IsForeignKeyViolation(err) {
		return errHasScores
	}
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if n == 0 {
		return errNotFound
	}
	slog.Info("user deleted", "actor_id", actorID, "user_id", id)
	s.changed()
	return nil
}
