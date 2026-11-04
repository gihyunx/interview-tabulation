package rubric

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/internal/apierr"
	"github.com/gihyunx/interview-tabulation/internal/apitag"
	"github.com/gihyunx/interview-tabulation/internal/auth"
)

type handler struct {
	api huma.API
	svc *service
	jwt string
}

func (h *handler) setupRoutes() {
	admin := huma.Middlewares{auth.RequireAuth(h.api, h.jwt, auth.RoleAdmin)}
	anyUser := huma.Middlewares{auth.RequireAuth(h.api, h.jwt)}
	tags := []string{apitag.Rubrics}

	huma.Register(h.api, huma.Operation{
		OperationID: "list-rubrics", Method: http.MethodGet, Path: "/rubrics",
		Tags: tags, Summary: "List rubrics", Middlewares: anyUser,
	}, h.list)
	huma.Register(h.api, huma.Operation{
		OperationID: "get-rubric", Method: http.MethodGet, Path: "/rubrics/{id}",
		Tags: tags, Summary: "Get rubric", Middlewares: anyUser,
	}, h.get)
	huma.Register(h.api, huma.Operation{
		OperationID: "create-rubric", Method: http.MethodPost, Path: "/rubrics",
		Tags: tags, Summary: "Create rubric", DefaultStatus: http.StatusCreated,
		Description: "Criteria points must total 100.", Middlewares: admin,
	}, h.create)
	huma.Register(h.api, huma.Operation{
		OperationID: "update-rubric", Method: http.MethodPatch, Path: "/rubrics/{id}",
		Tags: tags, Summary: "Update rubric and its criteria",
		Description: "Criteria are saved together with the rubric, in order. Once a score uses the rubric, criteria cannot be added or removed and points cannot change (409).",
		Middlewares: admin,
	}, h.update)
	huma.Register(h.api, huma.Operation{
		OperationID: "delete-rubric", Method: http.MethodDelete, Path: "/rubrics/{id}",
		Tags: tags, Summary: "Delete rubric", DefaultStatus: http.StatusNoContent,
		Description: "Fails with 409 while an interview uses it.", Middlewares: admin,
	}, h.delete)
}

type idInput struct {
	ID string `path:"id" format:"uuid"`
}

type rubricOutput struct{ Body rubric }

func (h *handler) list(ctx context.Context, _ *struct{}) (*struct{ Body []rubric }, error) {
	rs, err := list(ctx, h.svc.pool)
	return &struct{ Body []rubric }{rs}, err
}

func (h *handler) get(ctx context.Context, in *idInput) (*rubricOutput, error) {
	r, err := h.svc.get(ctx, in.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	return &rubricOutput{r}, nil
}

func (h *handler) create(ctx context.Context, in *struct{ Body saveInput }) (*rubricOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	r, err := h.svc.create(ctx, claims.Subject, in.Body)
	if err != nil {
		return nil, mapErr(err)
	}
	return &rubricOutput{r}, nil
}

func (h *handler) update(ctx context.Context, in *struct {
	ID   string `path:"id" format:"uuid"`
	Body saveInput
}) (*rubricOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	r, err := h.svc.update(ctx, claims.Subject, in.ID, in.Body)
	if err != nil {
		return nil, mapErr(err)
	}
	return &rubricOutput{r}, nil
}

func (h *handler) delete(ctx context.Context, in *idInput) (*struct{}, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	return nil, mapErr(h.svc.delete(ctx, claims.Subject, in.ID))
}

func mapErr(err error) error {
	err = apierr.Huma(err)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNotFound):
		return huma.Error404NotFound("Rubric not found.")
	case errors.Is(err, errHasScores):
		return huma.Error409Conflict("This rubric already has scores and cannot be deleted.")
	default:
		return err
	}
}
