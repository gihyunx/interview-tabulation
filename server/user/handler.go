package user

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/internal/apitag"
	"github.com/gihyunx/interview-tabulation/internal/auth"
)

type handler struct {
	api          huma.API
	svc          *service
	jwtSecret    string
	secureCookie bool
}

func newHandler(api huma.API, svc *service, jwtSecret string, secureCookie bool) *handler {
	return &handler{api: api, svc: svc, jwtSecret: jwtSecret, secureCookie: secureCookie}
}

func (h *handler) setupRoutes() {
	anyUser := huma.Middlewares{auth.RequireAuth(h.api, h.jwtSecret)}
	adminOnly := huma.Middlewares{auth.RequireAuth(h.api, h.jwtSecret, auth.RoleAdmin)}

	huma.Register(h.api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/auth/login",
		Tags:        []string{apitag.Authentication},
		Summary:     "Log in",
		Description: "Checks username and password and sets the session cookie.",
	}, h.login)

	huma.Register(h.api, huma.Operation{
		OperationID: "logout",
		Method:      http.MethodPost,
		Path:        "/auth/logout",
		Tags:        []string{apitag.Authentication},
		Summary:     "Log out",
		Description: "Clears the session cookie.",
	}, h.logout)

	huma.Register(h.api, huma.Operation{
		OperationID: "me",
		Method:      http.MethodGet,
		Path:        "/auth/me",
		Tags:        []string{apitag.Authentication},
		Summary:     "Current user",
		Middlewares: anyUser,
	}, h.me)

	huma.Register(h.api, huma.Operation{
		OperationID: "list-users",
		Method:      http.MethodGet,
		Path:        "/users",
		Tags:        []string{apitag.Users},
		Summary:     "List users",
		Middlewares: adminOnly,
	}, h.list)

	huma.Register(h.api, huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		Path:          "/users",
		Tags:          []string{apitag.Users},
		Summary:       "Create user",
		DefaultStatus: http.StatusCreated,
		Middlewares:   adminOnly,
	}, h.create)

	huma.Register(h.api, huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPatch,
		Path:        "/users/{id}",
		Tags:        []string{apitag.Users},
		Summary:     "Update user",
		Middlewares: adminOnly,
	}, h.update)

	huma.Register(h.api, huma.Operation{
		OperationID:   "delete-user",
		Method:        http.MethodDelete,
		Path:          "/users/{id}",
		Tags:          []string{apitag.Users},
		Summary:       "Delete user",
		Description:   "Also removes the user's judge assignments. Fails with 409 if the user has scores.",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   adminOnly,
	}, h.delete)

	huma.Register(h.api, huma.Operation{
		OperationID:   "reset-user-password",
		Method:        http.MethodPost,
		Path:          "/users/{id}/password",
		Tags:          []string{apitag.Users},
		Summary:       "Reset user password",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   adminOnly,
	}, h.resetPassword)
}

type userOutput struct {
	Body user
}

type loginInput struct {
	Body struct {
		Username string `json:"username" minLength:"1"`
		Password string `json:"password" minLength:"1"`
	}
}

type loginOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      user
}

func (h *handler) login(ctx context.Context, in *loginInput) (*loginOutput, error) {
	u, token, err := h.svc.login(ctx, in.Body.Username, in.Body.Password)
	if errors.Is(err, errBadLogin) {
		return nil, huma.Error401Unauthorized("Wrong username or password.")
	}
	if err != nil {
		return nil, err
	}
	return &loginOutput{SetCookie: auth.Cookie(token, h.secureCookie), Body: u}, nil
}

type logoutOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

func (h *handler) logout(context.Context, *struct{}) (*logoutOutput, error) {
	return &logoutOutput{SetCookie: auth.Cookie("", h.secureCookie)}, nil
}

func (h *handler) me(ctx context.Context, _ *struct{}) (*userOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	u, err := h.svc.get(ctx, claims.Subject)
	if errors.Is(err, errNotFound) {
		// Token outlives a deleted user.
		return nil, huma.Error401Unauthorized("Your account no longer exists.")
	}
	if err != nil {
		return nil, err
	}
	return &userOutput{Body: u}, nil
}

type listOutput struct {
	Body []user
}

func (h *handler) list(ctx context.Context, _ *struct{}) (*listOutput, error) {
	users, err := h.svc.list(ctx)
	if err != nil {
		return nil, err
	}
	return &listOutput{Body: users}, nil
}

type createUserInput struct {
	Body userCreateInput
}

func (h *handler) create(ctx context.Context, in *createUserInput) (*userOutput, error) {
	u, err := h.svc.create(ctx, in.Body)
	if err != nil {
		return nil, mapErr(err)
	}
	return &userOutput{Body: u}, nil
}

type updateUserInput struct {
	ID   string `path:"id" format:"uuid"`
	Body userUpdateInput
}

func (h *handler) update(ctx context.Context, in *updateUserInput) (*userOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	u, err := h.svc.update(ctx, claims.Subject, in.ID, in.Body)
	if err != nil {
		return nil, mapErr(err)
	}
	return &userOutput{Body: u}, nil
}

type idInput struct {
	ID string `path:"id" format:"uuid"`
}

func (h *handler) delete(ctx context.Context, in *idInput) (*struct{}, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	if err := h.svc.delete(ctx, claims.Subject, in.ID); err != nil {
		return nil, mapErr(err)
	}
	return nil, nil
}

type resetPasswordInput struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Password string `json:"password" minLength:"8" maxLength:"72"`
	}
}

func (h *handler) resetPassword(ctx context.Context, in *resetPasswordInput) (*struct{}, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	if err := h.svc.resetPassword(ctx, claims.Subject, in.ID, in.Body.Password); err != nil {
		return nil, mapErr(err)
	}
	return nil, nil
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, errNotFound):
		return huma.Error404NotFound("User not found.")
	case errors.Is(err, errUsernameTaken):
		return huma.Error409Conflict("That username is already taken.")
	case errors.Is(err, errHasScores):
		return huma.Error409Conflict("This user already has scores and cannot be deleted.")
	case errors.Is(err, errSelf):
		return huma.Error409Conflict("You cannot do this to your own account.")
	default:
		return err
	}
}
