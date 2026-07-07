package evaluation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sriganeshlokesh/forged/application/evaluation"
	"github.com/sriganeshlokesh/forged/domain/model"
)

// fakeEvaluator is a test double for service.IResumeEvaluator.
type fakeEvaluator struct{}

func (f *fakeEvaluator) Evaluate(_ context.Context, _ string, _ *model.Resume) (*model.Evaluation, error) {
	return &model.Evaluation{
		Score:       42,
		Summary:     "fake summary",
		Suggestions: []string{"improve X"},
	}, nil
}

func TestUseCase_Execute(t *testing.T) {
	uc := evaluation.NewUseCase(&fakeEvaluator{})

	nonEmpty := &model.Resume{Summary: "<p>Backend developer</p>"}

	tests := []struct {
		name    string
		in      *evaluation.Input
		wantErr error
		wantOK  bool
	}{
		{
			name:    "empty job description",
			in:      &evaluation.Input{JobDescription: "  ", Resume: nonEmpty},
			wantErr: model.ErrEmptyJobDescription,
		},
		{
			name:    "nil resume",
			in:      &evaluation.Input{JobDescription: "Go engineer", Resume: nil},
			wantErr: model.ErrEmptyResume,
		},
		{
			name:    "empty resume object",
			in:      &evaluation.Input{JobDescription: "Go engineer", Resume: &model.Resume{}},
			wantErr: model.ErrEmptyResume,
		},
		{
			name:   "success",
			in:     &evaluation.Input{JobDescription: "Go engineer", Resume: nonEmpty},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := uc.Execute(context.Background(), tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.GetStatus() != "ok" {
				t.Errorf("expected status 'ok', got %q", out.GetStatus())
			}
		})
	}
}
