// Package evaluation implements the resume-evaluation use case.
package evaluation

import (
	"context"
	"strings"

	"github.com/sriganeshlokesh/forged/application/core"
	"github.com/sriganeshlokesh/forged/domain/model"
)

// ResumeEvaluator is the dependency this use case needs: something that
// can score a resume against a job description. Declared here, at the
// consumer, so implementations (LLM adapter, stub) satisfy it implicitly.
type ResumeEvaluator interface {
	Evaluate(ctx context.Context, jobDescription string, resume *model.Resume) (*model.Evaluation, error)
}

// Input holds the parameters for a resume evaluation request.
// It implements core.Input.
type Input struct {
	JobDescription string
	Resume         *model.Resume
}

// Validate checks that the job description is non-empty and the resume has content.
// It returns domain sentinel errors so the handler can map them to specific error codes.
func (i *Input) Validate() error {
	if strings.TrimSpace(i.JobDescription) == "" {
		return model.ErrEmptyJobDescription
	}
	if i.Resume == nil || i.Resume.IsEmpty() {
		return model.ErrEmptyResume
	}
	return nil
}

// Output carries the evaluation result.
// It implements core.Output.
type Output struct {
	Evaluation *model.Evaluation
}

// GetStatus implements core.Output.
func (o *Output) GetStatus() string {
	return "ok"
}

// UseCase implements core.UseCase for resume evaluation.
type UseCase struct {
	evaluator ResumeEvaluator
}

// NewUseCase constructs a UseCase with the given evaluator.
func NewUseCase(ev ResumeEvaluator) *UseCase {
	return &UseCase{evaluator: ev}
}

// Execute validates the input and delegates evaluation to the injected port.
func (uc *UseCase) Execute(ctx context.Context, in core.Input) (core.Output, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	req := in.(*Input)
	eval, err := uc.evaluator.Evaluate(ctx, req.JobDescription, req.Resume)
	if err != nil {
		return nil, err
	}
	return &Output{Evaluation: eval}, nil
}
