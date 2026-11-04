package user

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/internal/dbtest"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testJWTSecret = "test-secret"

var (
	testPool   *pgxpool.Pool
	testServer *httptest.Server
	testSvc    *service
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	db := dbtest.New(ctx)
	testPool = db.Pool
	testSvc = &service{pool: db.Pool, jwtSecret: testJWTSecret}

	router := http.NewServeMux()
	humaCfg := huma.DefaultConfig("Test API", "1.0.0")
	humaCfg.CreateHooks = nil
	api := humago.New(router, humaCfg)
	Setup(huma.NewGroup(api, "/v1"), db.Pool, config.Config{JWTSecret: testJWTSecret}, nil)
	testServer = httptest.NewServer(router)

	code := m.Run()

	testServer.Close()
	db.Close()
	os.Exit(code)
}

func makeToken(t *testing.T, userID string, role auth.Role, expiresAt time.Time) string {
	t.Helper()
	claims := auth.Claims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		Role:      role,
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// seedUser inserts a user with a unique username and password "password123".
func seedUser(t *testing.T, role auth.Role) user {
	t.Helper()
	u, err := testSvc.create(context.Background(), userCreateInput{
		Username:  "u-" + uuid.NewString(),
		Password:  "password123",
		Role:      role,
		FirstName: "Test",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func tokenFor(t *testing.T, u user) string {
	t.Helper()
	return makeToken(t, u.ID, u.Role, time.Now().Add(time.Hour))
}
