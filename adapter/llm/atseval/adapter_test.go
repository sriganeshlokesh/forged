package atseval_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"

	llmats "github.com/sriganeshlokesh/forged/adapter/llm/atseval"
	"github.com/sriganeshlokesh/forged/adapter/llm/atseval/mocks"
	"github.com/sriganeshlokesh/forged/domain/model"
	"github.com/sriganeshlokesh/forged/pkg/atseval"
)

func TestAdapter_Evaluate_MapsDomainTypes(t *testing.T) {
	engine := mocks.NewMockEngine(t)
	engine.EXPECT().
		Evaluate(mock.Anything, "Go engineer", mock.MatchedBy(func(r atseval.Resume) bool {
			return r.Summary == "<p>dev</p>" &&
				len(r.Experience) == 1 && r.Experience[0].Company == "Acme" && r.Experience[0].ID == "e1" &&
				len(r.SkillGroups) == 1 && r.SkillGroups[0].Items[0] == "Go"
		})).
		Return(&atseval.Evaluation{
			Score:   35,
			Summary: "ok",
			Dimensions: []atseval.Dimension{
				{Key: "skills_match", Label: "Skills match", Score: 35, Max: 35, Evidence: "Go"},
			},
			Strengths: []string{"s"},
			Gaps:      []string{"g"},
			Suggestions: []atseval.Suggestion{
				{Text: "fix", Section: "skills", Dimension: "skills_match", EstimatedLift: 3},
				{Text: "rewrite", Section: "experience", Dimension: "impact_evidence", EstimatedLift: 2,
					Action: &atseval.SuggestionAction{Type: "rewrite_field", Target: atseval.ActionTarget{Section: "experience", ItemID: "e1", Field: "bullets"}}},
			},
		}, nil)

	a := llmats.New(engine, slog.Default())
	eval, err := a.Evaluate(context.Background(), "Go engineer", &model.Resume{
		Summary:     "<p>dev</p>",
		Experience:  []model.Experience{{ID: "e1", Company: "Acme", Role: "SWE", Bullets: "<ul><li>x</li></ul>"}},
		SkillGroups: []model.SkillGroup{{Label: "Languages", Items: []string{"Go"}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Score != 35 {
		t.Errorf("expected score 35, got %d", eval.Score)
	}
	if len(eval.Dimensions) != 1 || eval.Dimensions[0].Key != "skills_match" {
		t.Errorf("unexpected dimensions: %+v", eval.Dimensions)
	}
	if len(eval.Suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(eval.Suggestions))
	}
	if eval.Suggestions[0].Text != "fix" || eval.Suggestions[0].Section != "skills" || eval.Suggestions[0].EstimatedLift != 3 {
		t.Errorf("unexpected suggestion[0]: %+v", eval.Suggestions[0])
	}
	if eval.Suggestions[0].Action != nil {
		t.Errorf("expected suggestion[0].Action to be nil, got %+v", eval.Suggestions[0].Action)
	}
	if eval.Suggestions[1].Action == nil {
		t.Fatal("expected suggestion[1].Action to be non-nil")
	}
	if eval.Suggestions[1].Action.Type != model.ActionRewriteField {
		t.Errorf("expected ActionRewriteField, got %q", eval.Suggestions[1].Action.Type)
	}
	if eval.Suggestions[1].Action.Target.Section != "experience" {
		t.Errorf("expected target section 'experience', got %q", eval.Suggestions[1].Action.Target.Section)
	}
	if eval.Suggestions[1].Action.Target.ItemID != "e1" {
		t.Errorf("expected target item ID 'e1', got %q", eval.Suggestions[1].Action.Target.ItemID)
	}
	if eval.Suggestions[1].Action.Target.Field != "bullets" {
		t.Errorf("expected target field 'bullets', got %q", eval.Suggestions[1].Action.Target.Field)
	}
}

func TestAdapter_Evaluate_WrapsFailures(t *testing.T) {
	engine := mocks.NewMockEngine(t)
	engine.EXPECT().
		Evaluate(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("boom"))

	a := llmats.New(engine, slog.Default())
	_, err := a.Evaluate(context.Background(), "Go engineer", &model.Resume{Summary: "dev"})
	if !errors.Is(err, model.ErrEvaluationFailed) {
		t.Fatalf("expected ErrEvaluationFailed, got %v", err)
	}
}

func TestAdapter_Revise_MapsSpec(t *testing.T) {
	engine := mocks.NewMockEngine(t)
	engine.EXPECT().
		Revise(mock.Anything, mock.MatchedBy(func(req atseval.RevisionRequest) bool {
			return req.JobDescription == "Go engineer JD" &&
				req.SuggestionText == "tighten bullet" &&
				req.ActionType == "rewrite_field" &&
				req.Field == "bullets" &&
				req.Content == "<ul><li>x</li></ul>" &&
				req.Context.Company == "Acme" &&
				req.Context.Role == "SWE" &&
				req.Context.Name == "proj" &&
				req.Feedback == "fix it"
		})).
		Return(&atseval.RevisionResult{After: "<ul><li>better</li></ul>", Rationale: "tightened"}, nil)

	a := llmats.New(engine, slog.Default())
	after, rationale, err := a.Revise(context.Background(), model.RevisionSpec{
		JobDescription: "Go engineer JD",
		SuggestionText: "tighten bullet",
		Action: model.SuggestionAction{
			Type:   model.ActionRewriteField,
			Target: model.RevisionTarget{Section: "experience", ItemID: "e1", Field: "bullets"},
		},
		Content:  "<ul><li>x</li></ul>",
		Context:  model.RevisionContext{Company: "Acme", Role: "SWE", Name: "proj"},
		Feedback: "fix it",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if after != "<ul><li>better</li></ul>" {
		t.Errorf("expected after '<ul><li>better</li></ul>', got %q", after)
	}
	if rationale != "tightened" {
		t.Errorf("expected rationale 'tightened', got %q", rationale)
	}
}

func TestAdapter_Revise_WrapsFailures(t *testing.T) {
	engine := mocks.NewMockEngine(t)
	engine.EXPECT().
		Revise(mock.Anything, mock.Anything).
		Return(nil, errors.New("boom"))

	a := llmats.New(engine, slog.Default())
	_, _, err := a.Revise(context.Background(), model.RevisionSpec{
		Action: model.SuggestionAction{Type: model.ActionRewriteField},
	})
	if !errors.Is(err, model.ErrRevisionFailed) {
		t.Fatalf("expected ErrRevisionFailed, got %v", err)
	}
}
