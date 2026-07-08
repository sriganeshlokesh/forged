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
				len(r.Experience) == 1 && r.Experience[0].Company == "Acme" &&
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
			},
		}, nil)

	a := llmats.New(engine, slog.Default())
	eval, err := a.Evaluate(context.Background(), "Go engineer", &model.Resume{
		Summary:     "<p>dev</p>",
		Experience:  []model.Experience{{Company: "Acme", Role: "SWE", Bullets: "<ul><li>x</li></ul>"}},
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
	if len(eval.Suggestions) != 1 || eval.Suggestions[0].Text != "fix" ||
		eval.Suggestions[0].Section != "skills" || eval.Suggestions[0].EstimatedLift != 3 {
		t.Errorf("unexpected suggestions: %+v", eval.Suggestions)
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
