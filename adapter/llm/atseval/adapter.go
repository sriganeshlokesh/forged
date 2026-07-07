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

// Adapter implements service.IResumeEvaluator backed by pkg/atseval.
type Adapter struct {
	engine *atseval.Evaluator
	logger *slog.Logger
}

// New builds an Adapter around a configured atseval.Evaluator.
func New(engine *atseval.Evaluator, logger *slog.Logger) *Adapter {
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
		Suggestions: result.Suggestions,
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
	return eval, nil
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
			Name:        p.Name,
			Link:        p.Link,
			Description: p.Description,
			TechStack:   p.TechStack,
		})
	}
	for _, ed := range r.Education {
		edu := atseval.Education{
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
		out.SkillGroups = append(out.SkillGroups, atseval.SkillGroup{Label: sg.Label, Items: sg.Items})
	}
	return out
}
