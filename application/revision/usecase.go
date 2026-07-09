// Package revision implements the single-field resume revision use case.
package revision

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sriganeshlokesh/forged/application/core"
	"github.com/sriganeshlokesh/forged/domain/model"
	"github.com/sriganeshlokesh/forged/domain/service"
)

// ResumeReviser is the dependency this use case needs.
// Declared here, at the consumer; satisfied implicitly by adapter implementations.
type ResumeReviser interface {
	Revise(ctx context.Context, spec model.RevisionSpec) (draftAfter string, rationale string, err error)
}

// Input holds the parameters for a revision request. It implements core.Input.
type Input struct {
	JobDescription string
	Suggestion     model.Suggestion
	TargetField    string
	TargetContent  string
	TargetContext  model.RevisionContext
}

// Validate checks the input and returns domain sentinel errors for the handler to map.
func (in *Input) Validate() error {
	if strings.TrimSpace(in.JobDescription) == "" {
		return model.ErrEmptyJobDescription
	}
	if in.Suggestion.Action == nil || in.Suggestion.Action.Type != model.ActionRewriteField {
		return model.ErrUnknownAction
	}
	if !model.ValidTargetCombo(in.Suggestion.Action.Target) {
		return model.ErrUnknownAction
	}
	if in.TargetField != in.Suggestion.Action.Target.Field {
		return model.ErrTargetMismatch
	}
	if strings.TrimSpace(in.TargetContent) == "" {
		return model.ErrEmptyTargetContent
	}
	if len(in.TargetContent) > 20_000 {
		return fmt.Errorf("%w: exceeds 20000 bytes", model.ErrTargetMismatch)
	}
	return nil
}

// Output carries the revision result. It implements core.Output.
type Output struct{ Revision *model.Revision }

// GetStatus implements core.Output.
func (o *Output) GetStatus() string { return "ok" }

// UseCase implements core.UseCase for single-field resume revision.
type UseCase struct {
	reviser ResumeReviser
}

// NewUseCase constructs a UseCase with the given reviser.
func NewUseCase(r ResumeReviser) *UseCase {
	return &UseCase{reviser: r}
}

// Execute validates the input, requests a draft from the reviser, applies the
// domain guardrails, and retries once with feedback before failing.
func (uc *UseCase) Execute(ctx context.Context, in core.Input) (core.Output, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	req := in.(*Input)
	action := req.Suggestion.Action

	spec := model.RevisionSpec{
		JobDescription: req.JobDescription,
		SuggestionText: req.Suggestion.Text,
		Action:         *action,
		Content:        req.TargetContent,
		Context:        req.TargetContext,
	}

	draft, rationale, err := uc.reviser.Revise(ctx, spec)
	if err != nil {
		return nil, err // adapter already wraps as ErrRevisionFailed; no double-wrap
	}

	after := service.SanitizeHTML(draft)
	verr := validateDraft(req.TargetField, req.TargetContent, after)
	if verr != nil {
		slog.Default().Warn("revision: guardrail rejected first draft, retrying", "field", req.TargetField, "guardrail", guardrailKind(verr))
		spec.Feedback = verr.Error()
		draft, rationale, err = uc.reviser.Revise(ctx, spec)
		if err != nil {
			return nil, err
		}
		after = service.SanitizeHTML(draft)
		if verr = validateDraft(req.TargetField, req.TargetContent, after); verr != nil {
			slog.Default().Warn("revision: guardrail rejected retry draft, failing", "field", req.TargetField, "guardrail", guardrailKind(verr))
			return nil, fmt.Errorf("%w: %v", model.ErrRevisionFailed, verr)
		}
	}

	return &Output{Revision: &model.Revision{
		Changes: []model.Change{{
			Target:    action.Target,
			Before:    req.TargetContent,
			After:     after,
			Rationale: rationale,
		}},
		Warnings: []string{},
	}}, nil
}

// validateDraft runs the shape and numeric-fact guardrails on a sanitized draft.
func validateDraft(field, before, after string) error {
	if err := service.ValidateShape(field, before, after); err != nil {
		return err
	}
	return service.CheckNoNewNumbers(before, after, nil)
}

// guardrailKind classifies a guardrail failure for content-free logging:
// "numbers" for a fabricated-number rejection, "shape" for a structural one.
// It deliberately avoids logging the offending token, which may echo résumé content.
func guardrailKind(err error) string {
	if err != nil && strings.Contains(err.Error(), "numeric token") {
		return "numbers"
	}
	return "shape"
}
