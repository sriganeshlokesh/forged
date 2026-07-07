package evaluation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/sriganeshlokesh/forged/application/evaluation"
	"github.com/sriganeshlokesh/forged/application/evaluation/mocks"
	"github.com/sriganeshlokesh/forged/domain/model"
)

func TestUseCase_Execute(t *testing.T) {
	nonEmpty := &model.Resume{Summary: "<p>Backend developer</p>"}
	evalErr := errors.New("provider exploded")

	tests := []struct {
		name    string
		in      *evaluation.Input
		setup   func(m *mocks.MockResumeEvaluator)
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
			name: "evaluator failure propagates",
			in:   &evaluation.Input{JobDescription: "Go engineer", Resume: nonEmpty},
			setup: func(m *mocks.MockResumeEvaluator) {
				m.EXPECT().
					Evaluate(mock.Anything, "Go engineer", nonEmpty).
					Return(nil, evalErr)
			},
			wantErr: evalErr,
		},
		{
			name: "success",
			in:   &evaluation.Input{JobDescription: "Go engineer", Resume: nonEmpty},
			setup: func(m *mocks.MockResumeEvaluator) {
				m.EXPECT().
					Evaluate(mock.Anything, "Go engineer", nonEmpty).
					Return(&model.Evaluation{Score: 42, Summary: "fake summary"}, nil)
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := mocks.NewMockResumeEvaluator(t)
			if tt.setup != nil {
				tt.setup(evaluator)
			}
			uc := evaluation.NewUseCase(evaluator)

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
