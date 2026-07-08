// Package atseval adapts the self-contained pkg/atseval evaluation engine
// to the domain's IResumeEvaluator port. It is the only place forged types
// and pkg/atseval types meet, keeping the engine extractable into its own
// repository.
package atseval

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sriganeshlokesh/forged/domain/model"
	"github.com/sriganeshlokesh/forged/pkg/atseval"
)

// Engine is what this adapter needs from the evaluation engine.
// Declared here, at the consumer; satisfied implicitly by *atseval.Evaluator.
type Engine interface {
	Evaluate(ctx context.Context, jobDescription string, resume atseval.Resume) (*atseval.Evaluation, error)
	Revise(ctx context.Context, req atseval.RevisionRequest) (*atseval.RevisionResult, error)
}

// Adapter implements the application's ResumeEvaluator backed by pkg/atseval.
type Adapter struct {
	engine Engine
	logger *slog.Logger
}

// New builds an Adapter around a configured evaluation engine.
func New(engine Engine, logger *slog.Logger) *Adapter {
	return &Adapter{engine: engine, logger: logger}
}

// Evaluate maps domain types into the engine and back.
func (a *Adapter) Evaluate(ctx context.Context, jobDescription string, resume *model.Resume) (*model.Evaluation, error) {
	result, err := a.engine.Evaluate(ctx, jobDescription, toEngineResume(resume))
	if err != nil {
		a.logger.ErrorContext(ctx, "llm evaluation failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("%w: %w", model.ErrEvaluationFailed, err)
	}

	eval := &model.Evaluation{
		Score:       result.Score,
		Summary:     result.Summary,
		Dimensions:  make([]model.Dimension, 0, len(result.Dimensions)),
		Strengths:   result.Strengths,
		Gaps:        result.Gaps,
		Suggestions: make([]model.Suggestion, 0, len(result.Suggestions)),
	}
	for _, d := range result.Dimensions {
		eval.Dimensions = append(eval.Dimensions, model.Dimension{
			Key:      d.Key,
			Label:    d.Label,
			Score:    d.Score,
			Max:      d.Max,
			Evidence: d.Evidence,
		})
	}
	for _, s := range result.Suggestions {
		sug := model.Suggestion{
			Text:          s.Text,
			Section:       s.Section,
			Dimension:     s.Dimension,
			EstimatedLift: s.EstimatedLift,
		}
		if s.Action != nil {
			sug.Action = &model.SuggestionAction{
				Type: model.ActionType(s.Action.Type),
				Target: model.RevisionTarget{
					Section: s.Action.Target.Section,
					ItemID:  s.Action.Target.ItemID,
					Field:   s.Action.Target.Field,
				},
			}
		}
		eval.Suggestions = append(eval.Suggestions, sug)
	}
	return eval, nil
}

// Revise maps a domain revision spec into the engine and returns the draft.
func (a *Adapter) Revise(ctx context.Context, spec model.RevisionSpec) (string, string, error) {
	req := atseval.RevisionRequest{
		JobDescription: spec.JobDescription,
		SuggestionText: spec.SuggestionText,
		ActionType:     string(spec.Action.Type),
		Field:          spec.Action.Target.Field,
		Content:        spec.Content,
		Context: atseval.RevisionContext{
			Company: spec.Context.Company,
			Role:    spec.Context.Role,
			Name:    spec.Context.Name,
		},
		Feedback: spec.Feedback,
	}
	result, err := a.engine.Revise(ctx, req)
	if err != nil {
		a.logger.ErrorContext(ctx, "llm revision failed", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%w: %w", model.ErrRevisionFailed, err)
	}
	return result.After, result.Rationale, nil
}

func toEngineResume(r *model.Resume) atseval.Resume {
	out := atseval.Resume{
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Email:     r.Email,
		LinkedIn:  r.LinkedIn,
		Phone:     r.Phone,
		Location:  r.Location,
		Summary:   r.Summary,
	}
	for _, e := range r.Experience {
		out.Experience = append(out.Experience, atseval.Experience{
			ID:         e.ID,
			Company:    e.Company,
			Role:       e.Role,
			Employment: e.Employment,
			Start:      e.Start,
			End:        e.End,
			Present:    e.Present,
			Bullets:    e.Bullets,
		})
	}
	for _, p := range r.Projects {
		out.Projects = append(out.Projects, atseval.Project{
			ID:          p.ID,
			Name:        p.Name,
			Link:        p.Link,
			Description: p.Description,
			TechStack:   p.TechStack,
		})
	}
	for _, ed := range r.Education {
		edu := atseval.Education{
			ID:     ed.ID,
			School: ed.School,
			Degree: ed.Degree,
			Start:  ed.Start,
			End:    ed.End,
		}
		for _, d := range ed.ExtraDetails {
			edu.ExtraDetails = append(edu.ExtraDetails, atseval.EducationDetail{Label: d.Label, Value: d.Value})
		}
		out.Education = append(out.Education, edu)
	}
	for _, sg := range r.SkillGroups {
		out.SkillGroups = append(out.SkillGroups, atseval.SkillGroup{ID: sg.ID, Label: sg.Label, Items: sg.Items})
	}
	return out
}
