package auth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/internal/logger"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	CookieName = "access_token"
	TokenTTL   = 12 * time.Hour
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleJudge Role = "judge"
)

func (Role) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{
		Type: "string",
		Enum: []any{string(RoleAdmin), string(RoleJudge)},
	}
}

type Claims struct {
	jwt.RegisteredClaims

	Role Role `json:"role"`
}

type claimsKeyType struct{}

var claimsKey = claimsKeyType{}

// ClaimsFrom returns the claims set by [RequireAuth]. ok is false on routes
// that did not use it.
func ClaimsFrom(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	if !ok {
		return Claims{}, false
	}
	return *c, true
}

func NewToken(secret, userID string, role Role) (string, error) {
	claims := Claims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenTTL)),
		Role:      role,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Cookie builds the session cookie. Pass an empty token to clear it.
func Cookie(token string, secure bool) http.Cookie {
	c := http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if token == "" {
		c.MaxAge = -1
	} else {
		c.MaxAge = int(TokenTTL.Seconds())
	}
	return c
}

// RequireAuth rejects requests without a valid token (401), and with a role
// not in roles (403). Empty roles allows any logged-in user.
func RequireAuth(api huma.API, secret string, roles ...Role) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		cookie, err := huma.ReadCookie(ctx, CookieName)
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "Please log in.")
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(cookie.Value, claims, func(*jwt.Token) (any, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid || claims.Subject == "" {
			logger.Logger(ctx.Context()).Warn("auth: invalid token", "error", err, "path", ctx.URL().Path)
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "Your session is invalid or expired. Please log in again.")
			return
		}

		if len(roles) > 0 && !slices.Contains(roles, claims.Role) {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "You are not allowed to do this.")
			return
		}

		logger.AnnotateUser(ctx.Context(), claims.Subject)
		next(huma.WithValue(ctx, claimsKey, claims))
	}
}

func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

var ErrBadPassword = errors.New("wrong password")

func CheckPassword(password, hash string) error {
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return ErrBadPassword
	}
	return nil
}
