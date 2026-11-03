package score

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

	huma.Register(h.api, huma.Operation{
		OperationID: "interview-board", Method: http.MethodGet, Path: "/interviews/{id}/board",
		Tags: []string{apitag.Scores}, Summary: "Candidates with status and my scores",
		Description: "From the calling judge's point of view. 403 if not assigned to the interview.", Middlewares: judge,
	}, h.board)
	huma.Register(h.api, huma.Operation{
		OperationID: "submit-scores", Method: http.MethodPut, Path: "/interviews/{id}/candidates/{candidateId}/scores",
		Tags: []string{apitag.Scores}, Summary: "Submit or update my scores for a candidate",
		Description: "Idempotent upsert. Every criterion is required. A 1-on-1 candidate already scored by another judge gets 409.",
		Middlewares: judge,
	}, h.submit)
	huma.Register(h.api, huma.Operation{
		OperationID: "clear-submission", Method: http.MethodDelete, Path: "/interviews/{id}/candidates/{candidateId}/scores",
		Tags: []string{apitag.Scores}, Summary: "Clear one judge's scores for a candidate",
		DefaultStatus: http.StatusNoContent, Middlewares: admin,
	}, h.clear)
}

func (h *handler) board(ctx context.Context, in *struct {
	ID string `path:"id" format:"uuid"`
}) (*struct{ Body board }, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	b, err := h.svc.board(ctx, claims.Subject, in.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	return &struct{ Body board }{b}, nil
}

func (h *handler) submit(ctx context.Context, in *struct {
	ID          string `path:"id" format:"uuid"`
	CandidateID string `path:"candidateId" format:"uuid"`
	Body        submitInput
}) (*struct{ Body boardCandidate }, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	c, err := h.svc.submit(ctx, claims.Subject, in.ID, in.CandidateID, in.Body.Scores)
	if err != nil {
		return nil, mapErr(err)
	}
	return &struct{ Body boardCandidate }{c}, nil
}

func (h *handler) clear(ctx context.Context, in *struct {
	ID          string `path:"id" format:"uuid"`
	CandidateID string `path:"candidateId" format:"uuid"`
	Judge       string `query:"judge" format:"uuid" required:"true" doc:"The judge id whose scores to clear."`
}) (*struct{}, error) {
	claims, _ := auth.ClaimsFrom(ctx)
	return nil, mapErr(h.svc.clear(ctx, claims.Subject, in.ID, in.Judge, in.CandidateID))
}

func mapErr(err error) error {
	err = apierr.Huma(err)
	if errors.Is(err, errNotFound) {
		return huma.Error404NotFound("Nothing to find here: the candidate or submission does not exist.")
	}
	return err
}
