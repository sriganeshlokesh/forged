// Package stub provides deterministic placeholder implementations of the
// application's evaluator and reviser ports. They return fixed responses so
// the frontend can build against the real contract before an LLM adapter is
// wired in.
//
// To swap in a real implementation: adapter/llm/… satisfies the same
// consumer interfaces and adapter/dependency selects by config — no other
// files need to change.
package stub

import (
	"context"
	"fmt"
	"strings"

	"github.com/sriganeshlokesh/forged/domain/model"
)

// StubEvaluator is a no-op implementation of evaluation.ResumeEvaluator.
type StubEvaluator struct{}

// NewStubEvaluator constructs a StubEvaluator.
func NewStubEvaluator() *StubEvaluator {
	return &StubEvaluator{}
}

// Evaluate returns a fixed placeholder evaluation in the full rich shape
// so the frontend can build against the real contract.
func (s *StubEvaluator) Evaluate(_ context.Context, _ string, _ *model.Resume) (*model.Evaluation, error) {
	return &model.Evaluation{
		Score:   0,
		Summary: "Evaluation pending — stub response",
		Dimensions: []model.Dimension{
			{Key: "skills_match", Label: "Skills match", Score: 0, Max: 35, Evidence: ""},
			{Key: "experience_relevance", Label: "Experience relevance", Score: 0, Max: 30, Evidence: ""},
			{Key: "impact_evidence", Label: "Impact and evidence", Score: 0, Max: 20, Evidence: ""},
			{Key: "education_extras", Label: "Education and extras", Score: 0, Max: 15, Evidence: ""},
		},
		Strengths:   []string{},
		Gaps:        []string{},
		Suggestions: []model.Suggestion{},
	}, nil
}

// StubReviser is a deterministic no-op revision implementation used when no
// LLM API key is configured, so local dev and deploys work without credentials.
// For bullets it inserts a marker <li> before the closing </ul>; for summary
// and description fields it appends a marker <p>.
type StubReviser struct{}

// NewStubReviser constructs a StubReviser.
func NewStubReviser() *StubReviser {
	return &StubReviser{}
}

// Revise applies a deterministic edit so the frontend can exercise the full
// revision flow without an LLM.
func (s *StubReviser) Revise(_ context.Context, spec model.RevisionSpec) (string, string, error) {
	content := spec.Content
	tag := fmt.Sprintf("<li>[stub edit] %s</li>", spec.SuggestionText)
	const rationale = "stub reviser: deterministic edit for local development"

	if spec.Action.Target.Field == "bullets" {
		idx := strings.LastIndex(content, "</ul>")
		if idx >= 0 {
			content = content[:idx] + tag + "</ul>" + content[idx+5:]
		} else {
			content = content + tag
		}
		return content, rationale, nil
	}
	// summary/description default
	content = content + fmt.Sprintf("<p>[stub edit] %s</p>", spec.SuggestionText)
	return content, rationale, nil
}
