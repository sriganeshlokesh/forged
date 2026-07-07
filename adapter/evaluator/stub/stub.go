// Package stub provides a placeholder implementation of service.IResumeEvaluator.
// It returns a fixed response so the frontend can build against the real contract
// before an LLM adapter is wired in.
//
// To swap in a real evaluator: add adapter/llm/… and change the wire.Bind in
// adapter/dependency/wire.go — no other files need to change.
package stub

import (
	"context"

	"github.com/sriganeshlokesh/forged/domain/model"
)

// StubEvaluator is a no-op implementation of service.IResumeEvaluator.
type StubEvaluator struct{}

// NewStubEvaluator constructs a StubEvaluator.
func NewStubEvaluator() *StubEvaluator {
	return &StubEvaluator{}
}

// Evaluate returns a fixed placeholder evaluation.
func (s *StubEvaluator) Evaluate(_ context.Context, _ string, _ *model.Resume) (*model.Evaluation, error) {
	return &model.Evaluation{
		Score:       0,
		Summary:     "Evaluation pending — stub response",
		Suggestions: []string{},
	}, nil
}
