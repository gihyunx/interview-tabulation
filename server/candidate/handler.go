package candidate

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
	tags := []string{apitag.Candidates}

	huma.Register(h.api, huma.Operation{
		OperationID: "list-candidates", Method: http.MethodGet, Path: "/candidates",
		Tags: tags, Summary: "List candidates", Middlewares: anyUser,
	}, h.list)
	huma.Register(h.api, huma.Operation{
		OperationID: "create-candidate", Method: http.MethodPost, Path: "/candidates",
		Tags: tags, Summary: "Create candidate", DefaultStatus: http.StatusCreated, Middlewares: admin,
	}, h.create)
	huma.Register(h.api, huma.Operation{
		OperationID: "update-candidate", Method: http.MethodPatch, Path: "/candidates/{id}",
		Tags: tags, Summary: "Update candidate", Middlewares: admin,
	}, h.update)
	huma.Register(h.api, huma.Operation{
		OperationID: "delete-candidate", Method: http.MethodDelete, Path: "/candidates/{id}",
		Tags: tags, Summary: "Delete candidate", DefaultStatus: http.StatusNoContent,
		Description: "Fails with 409 if the candidate has scores.", Middlewares: admin,
	}, h.delete)
	huma.Register(h.api, huma.Operation{
		OperationID: "import-candidates", Method: http.MethodPost, Path: "/candidates/import",
		Tags: tags, Summary: "Import candidates from CSV",
		Description: "Header must be `first_name,last_name`. With dry_run=true nothing is saved and row errors are returned in the body.",
		Middlewares: admin,
	}, h.importCSV)
}

type candidateOutput struct{ Body candidate }

type listOutput struct{ Body []candidate }

func (h *handler) list(ctx context.Context, _ *struct{}) (*listOutput, error) {
	cs, err := list(ctx, h.svc.pool)
	return &listOutput{Body: cs}, err
}

func (h *handler) create(ctx context.Context, in *struct{ Body nameInput }) (*candidateOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	c, err := h.svc.create(ctx, claims.Subject, in.Body)
	if err != nil {
		return nil, mapErr(err)
	}
	return &candidateOutput{Body: c}, nil
}

func (h *handler) update(ctx context.Context, in *struct {
	ID   string `path:"id" format:"uuid"`
	Body candidateUpdateInput
}) (*candidateOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	c, err := h.svc.update(ctx, claims.Subject, in.ID, in.Body)
	if err != nil {
		return nil, mapErr(err)
	}
	return &candidateOutput{Body: c}, nil
}

func (h *handler) delete(ctx context.Context, in *struct {
	ID string `path:"id" format:"uuid"`
}) (*struct{}, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	return nil, mapErr(h.svc.delete(ctx, claims.Subject, in.ID))
}

type importInput struct {
	DryRun  bool `query:"dry_run"`
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" contentType:"text/csv,application/vnd.ms-excel,text/plain" required:"true"`
	}]
}

type importOutput struct {
	Body struct {
		DryRun bool        `json:"dry_run"`
		Rows   []importRow `json:"rows"`
		Errors int         `json:"errors"`
	}
}

func (h *handler) importCSV(ctx context.Context, in *importInput) (*importOutput, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	rows, err := h.svc.importCSV(ctx, claims.Subject, in.RawBody.Data().File, in.DryRun)
	if err != nil {
		return nil, mapErr(err)
	}
	out := &importOutput{}
	out.Body.DryRun, out.Body.Rows = in.DryRun, rows
	for _, r := range rows {
		if r.Error != "" {
			out.Body.Errors++
		}
	}
	return out, nil
}

func mapErr(err error) error {
	err = apierr.Huma(err)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNotFound):
		return huma.Error404NotFound("Candidate not found.")
	case errors.Is(err, errHasScores):
		return huma.Error409Conflict("This candidate already has scores and cannot be deleted. Clear their scores first.")
	default:
		return err
	}
}
