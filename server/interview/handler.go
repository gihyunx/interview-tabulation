package interview

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
	judge := huma.Middlewares{auth.RequireAuth(h.api, h.jwt, auth.RoleJudge)}
	tags := []string{apitag.Interviews}

	huma.Register(h.api, huma.Operation{
		OperationID: "list-interviews", Method: http.MethodGet, Path: "/interviews",
		Tags: tags, Summary: "List interviews with judges", Middlewares: admin,
	}, h.list)
	huma.Register(h.api, huma.Operation{
		OperationID: "create-interview", Method: http.MethodPost, Path: "/interviews",
		Tags: tags, Summary: "Create interview", DefaultStatus: http.StatusCreated, Middlewares: admin,
	}, h.create)
	huma.Register(h.api, huma.Operation{
		OperationID: "update-interview", Method: http.MethodPatch, Path: "/interviews/{id}",
		Tags: tags, Summary: "Update interview",
		Description: "The rubric and mode cannot change once the interview has scores (409).", Middlewares: admin,
	}, h.update)
	huma.Register(h.api, huma.Operation{
		OperationID: "delete-interview", Method: http.MethodDelete, Path: "/interviews/{id}",
		Tags: tags, Summary: "Delete interview", DefaultStatus: http.StatusNoContent,
		Description: "Removes judge assignments too. Fails with 409 if any judge has scores.", Middlewares: admin,
	}, h.delete)
	huma.Register(h.api, huma.Operation{
		OperationID: "set-interview-judges", Method: http.MethodPut, Path: "/interviews/{id}/judges",
		Tags: tags, Summary: "Replace the judges of an interview", Middlewares: admin,
	}, h.setJudges)
	huma.Register(h.api, huma.Operation{
		OperationID: "reorder-interviews", Method: http.MethodPut, Path: "/interviews/order",
		Tags: tags, Summary: "Save interview order", DefaultStatus: http.StatusNoContent, Middlewares: admin,
	}, h.reorder)
	huma.Register(h.api, huma.Operation{
		OperationID: "list-my-interviews", Method: http.MethodGet, Path: "/me/interviews",
		Tags: tags, Summary: "Interviews I judge, with rubric", Middlewares: judge,
	}, h.listMine)
}

type interviewOutput struct{ Body interview }

func (h *handler) list(ctx context.Context, _ *struct{}) (*struct{ Body []interview }, error) {
	is, err := list(ctx, h.svc.pool)
	return &struct{ Body []interview }{is}, err
}

func (h *handler) listMine(ctx context.Context, _ *struct{}) (*struct{ Body []myInterview }, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	is, err := listMine(ctx, h.svc.pool, claims.Subject)
	return &struct{ Body []myInterview }{is}, err
}

func (h *handler) create(ctx context.Context, in *struct{ Body interviewCreateInput }) (*interviewOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	i, err := h.svc.create(ctx, claims.Subject, in.Body)
	if err != nil {
		return nil, mapErr(err)
	}
	return &interviewOutput{i}, nil
}

func (h *handler) update(ctx context.Context, in *struct {
	ID   string `path:"id" format:"uuid"`
	Body interviewUpdateInput
}) (*interviewOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	i, err := h.svc.update(ctx, claims.Subject, in.ID, in.Body)
	if err != nil {
		return nil, mapErr(err)
	}
	return &interviewOutput{i}, nil
}

func (h *handler) delete(ctx context.Context, in *struct {
	ID string `path:"id" format:"uuid"`
}) (*struct{}, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	return nil, mapErr(h.svc.delete(ctx, claims.Subject, in.ID))
}

func (h *handler) setJudges(ctx context.Context, in *struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		UserIDs []string `json:"user_ids" format:"uuid"`
	}
}) (*interviewOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	i, err := h.svc.setJudges(ctx, claims.Subject, in.ID, in.Body.UserIDs)
	if err != nil {
		return nil, mapErr(err)
	}
	return &interviewOutput{i}, nil
}

func (h *handler) reorder(ctx context.Context, in *struct {
	Body struct {
		IDs []string `json:"ids" format:"uuid"`
	}
}) (*struct{}, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	return nil, mapErr(h.svc.reorder(ctx, claims.Subject, in.Body.IDs))
}

func mapErr(err error) error {
	err = apierr.Huma(err)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNotFound):
		return huma.Error404NotFound("Interview not found.")
	case errors.Is(err, errHasScores):
		return huma.Error409Conflict("This interview already has scores and cannot be deleted.")
	default:
		return err
	}
}
