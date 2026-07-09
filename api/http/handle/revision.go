package handle

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/sriganeshlokesh/forged/api/dto"
	"github.com/sriganeshlokesh/forged/api/error_code"
	"github.com/sriganeshlokesh/forged/application/core"
	"github.com/sriganeshlokesh/forged/application/revision"
	"github.com/sriganeshlokesh/forged/domain/model"
)

// RevisionUseCase is the dependency this handler needs.
// Declared here, at the consumer; satisfied implicitly by *revision.UseCase.
type RevisionUseCase interface {
	Execute(ctx context.Context, in core.Input) (core.Output, error)
}

// RevisionHandler handles the POST /v1/revisions endpoint.
type RevisionHandler struct {
	uc     RevisionUseCase
	logger *slog.Logger
}

// NewRevisionHandler constructs a RevisionHandler.
func NewRevisionHandler(uc RevisionUseCase, logger *slog.Logger) *RevisionHandler {
	return &RevisionHandler{uc: uc, logger: logger}
}

// Revise handles POST /v1/revisions.
func (h *RevisionHandler) Revise(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var req dto.RevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, error_code.ErrInvalidParams)
		return
	}

	in := &revision.Input{
		JobDescription: req.JobDescription,
		Suggestion:     req.Suggestion.ToModel(),
		TargetField:    req.Target.Field,
		TargetContent:  req.Target.Content,
		TargetContext:  req.Target.Context.ToModel(),
	}

	out, err := h.uc.Execute(r.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrEmptyJobDescription) ||
			errors.Is(err, model.ErrUnknownAction) ||
			errors.Is(err, model.ErrTargetMismatch) ||
			errors.Is(err, model.ErrEmptyTargetContent):
			writeError(w, &error_code.Error{
				Code: error_code.ErrValidation.Code,
				Msg:  err.Error(),
				HTTP: error_code.ErrValidation.HTTP,
			})
		case errors.Is(err, model.ErrRevisionFailed):
			h.logger.ErrorContext(r.Context(), "revision backend failed", slog.String("error", err.Error()))
			writeError(w, error_code.ErrRevision)
		default:
			h.logger.ErrorContext(r.Context(), "revision failed", slog.String("error", err.Error()))
			writeError(w, error_code.ErrInternal)
		}
		return
	}

	result := out.(*revision.Output)
	writeJSON(w, http.StatusOK, dto.NewRevisionResponse(out.GetStatus(), result.Revision))
}
