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
	"github.com/sriganeshlokesh/forged/application/evaluation"
	"github.com/sriganeshlokesh/forged/domain/model"
)

// EvaluationUseCase is the dependency this handler needs: executing the
// resume-evaluation use case. Declared here, at the consumer, and satisfied
// implicitly by *evaluation.UseCase (bound in adapter/dependency).
type EvaluationUseCase interface {
	Execute(ctx context.Context, in core.Input) (core.Output, error)
}

// EvaluationHandler handles the POST /v1/evaluations endpoint.
type EvaluationHandler struct {
	uc     EvaluationUseCase
	logger *slog.Logger
}

// NewEvaluationHandler constructs an EvaluationHandler.
func NewEvaluationHandler(uc EvaluationUseCase, logger *slog.Logger) *EvaluationHandler {
	return &EvaluationHandler{uc: uc, logger: logger}
}

// Evaluate handles POST /v1/evaluations.
func (h *EvaluationHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var req dto.EvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, error_code.ErrInvalidParams)
		return
	}

	in := &evaluation.Input{
		JobDescription: req.JobDescription,
		Resume:         req.Resume.ToModel(),
	}

	out, err := h.uc.Execute(r.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrEmptyJobDescription) || errors.Is(err, model.ErrEmptyResume):
			writeError(w, &error_code.Error{
				Code: error_code.ErrValidation.Code,
				Msg:  err.Error(),
				HTTP: error_code.ErrValidation.HTTP,
			})
		case errors.Is(err, model.ErrEvaluationFailed):
			h.logger.ErrorContext(r.Context(), "evaluation backend failed", slog.String("error", err.Error()))
			writeError(w, error_code.ErrEvaluation)
		default:
			h.logger.ErrorContext(r.Context(), "evaluation failed", slog.String("error", err.Error()))
			writeError(w, error_code.ErrInternal)
		}
		return
	}

	result := out.(*evaluation.Output)
	writeJSON(w, http.StatusOK, dto.NewEvaluationResponse(out.GetStatus(), result.Evaluation))
}

// writeJSON encodes v as JSON and writes it with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an ErrorResponse derived from an error_code.Error.
func writeError(w http.ResponseWriter, e *error_code.Error) {
	writeJSON(w, e.HTTP, dto.ErrorResponse{
		Code:    e.Code,
		Message: e.Msg,
	})
}
