package handle

import (
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

// EvaluationHandler handles the POST /v1/evaluations endpoint.
type EvaluationHandler struct {
	uc     core.UseCase
	logger *slog.Logger
}

// NewEvaluationHandler constructs an EvaluationHandler.
// The constructor accepts *evaluation.UseCase (concrete) so wire has no ambiguity
// when multiple use cases exist; it is stored as the core.UseCase interface.
func NewEvaluationHandler(uc *evaluation.UseCase, logger *slog.Logger) *EvaluationHandler {
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
		if errors.Is(err, model.ErrEmptyJobDescription) || errors.Is(err, model.ErrEmptyResume) {
			writeError(w, &error_code.Error{
				Code: error_code.ErrValidation.Code,
				Msg:  err.Error(),
				HTTP: error_code.ErrValidation.HTTP,
			})
			return
		}
		h.logger.ErrorContext(r.Context(), "evaluation failed", slog.String("error", err.Error()))
		writeError(w, error_code.ErrInternal)
		return
	}

	result := out.(*evaluation.Output)
	writeJSON(w, http.StatusOK, dto.EvaluationResponse{
		Status:      out.GetStatus(),
		Score:       result.Evaluation.Score,
		Summary:     result.Evaluation.Summary,
		Suggestions: result.Evaluation.Suggestions,
	})
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
