package service

import (
	"context"

	"github.com/sriganeshlokesh/forged/domain/model"
)

// IResumeEvaluator is the port for evaluating a resume against a job description.
// Implementations live in adapter/evaluator/; this interface must never import adapter code.
// Swap the stub for an LLM adapter by changing one wire.Bind in adapter/dependency/wire.go.
type IResumeEvaluator interface {
	Evaluate(ctx context.Context, jobDescription string, resume *model.Resume) (*model.Evaluation, error)
}
