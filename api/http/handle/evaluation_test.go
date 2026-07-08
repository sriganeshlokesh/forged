package handle_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/sriganeshlokesh/forged/api/dto"
	"github.com/sriganeshlokesh/forged/api/http/handle"
	"github.com/sriganeshlokesh/forged/api/http/handle/mocks"
	"github.com/sriganeshlokesh/forged/application/evaluation"
	"github.com/sriganeshlokesh/forged/domain/model"
)

func richEvaluationOutput() *evaluation.Output {
	return &evaluation.Output{Evaluation: &model.Evaluation{
		Score:   42,
		Summary: "stub",
		Dimensions: []model.Dimension{
			{Key: "skills_match", Label: "Skills match", Score: 42, Max: 35, Evidence: "Go"},
		},
		Strengths: []string{"s"},
		Gaps:      []string{"g"},
		Suggestions: []model.Suggestion{
			{Text: "Add gRPC", Section: "skills", Dimension: "skills_match", EstimatedLift: 5},
		},
	}}
}

func TestEvaluationHandler_Evaluate(t *testing.T) {
	validBody := `{"job_description":"Go engineer","resume":{"summary":"<p>Backend dev</p>","experience":[{"company":"Acme","role":"SWE","bullets":"<ul><li>Built X</li></ul>"}]}}`

	tests := []struct {
		name       string
		body       string
		setup      func(m *mocks.MockEvaluationUseCase)
		wantStatus int
		wantCode   int // 0 means no error code check
	}{
		{
			name: "happy path — full structured resume",
			body: validBody,
			setup: func(m *mocks.MockEvaluationUseCase) {
				m.EXPECT().Execute(mock.Anything, mock.Anything).Return(richEvaluationOutput(), nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "malformed JSON",
			body:       `{not valid json}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   10001,
		},
		{
			name: "validation failure maps to 10002",
			body: `{"job_description":"","resume":{"summary":"<p>test</p>"}}`,
			setup: func(m *mocks.MockEvaluationUseCase) {
				m.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil, model.ErrEmptyJobDescription)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   10002,
		},
		{
			name: "empty resume maps to 10002",
			body: `{"job_description":"Go engineer","resume":{}}`,
			setup: func(m *mocks.MockEvaluationUseCase) {
				m.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil, model.ErrEmptyResume)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   10002,
		},
		{
			name: "evaluation backend failure maps to 30002/503",
			body: validBody,
			setup: func(m *mocks.MockEvaluationUseCase) {
				m.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil, model.ErrEvaluationFailed)
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   30002,
		},
		{
			name:       "oversized body",
			body:       strings.Repeat("a", 1<<20+1),
			wantStatus: http.StatusBadRequest,
			wantCode:   10001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := mocks.NewMockEvaluationUseCase(t)
			if tt.setup != nil {
				tt.setup(uc)
			}
			h := handle.NewEvaluationHandler(uc, slog.Default())

			req := httptest.NewRequest(http.MethodPost, "/v1/evaluations", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.Evaluate(rec, req)

			res := rec.Result()
			defer func() { _ = res.Body.Close() }()

			if res.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, res.StatusCode)
			}

			ct := res.Header.Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", ct)
			}

			if tt.wantCode != 0 {
				var errBody dto.ErrorResponse
				if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
					t.Fatalf("failed to decode error response: %v", err)
				}
				if errBody.Code != tt.wantCode {
					t.Errorf("expected error code %d, got %d", tt.wantCode, errBody.Code)
				}
				return
			}

			// Happy path: decode as EvaluationResponse.
			var evalResp dto.EvaluationResponse
			if err := json.NewDecoder(res.Body).Decode(&evalResp); err != nil {
				t.Fatalf("failed to decode evaluation response: %v", err)
			}
			if evalResp.Status != "ok" {
				t.Errorf("expected status 'ok', got %q", evalResp.Status)
			}
			if len(evalResp.Suggestions) != 1 ||
				evalResp.Suggestions[0].Section != "skills" ||
				evalResp.Suggestions[0].EstimatedLift != 5 {
				t.Errorf("unexpected suggestions: %+v", evalResp.Suggestions)
			}
			if len(evalResp.Dimensions) != 1 || evalResp.Dimensions[0].Key != "skills_match" {
				t.Errorf("expected one skills_match dimension, got %+v", evalResp.Dimensions)
			}
			if evalResp.Strengths == nil || evalResp.Gaps == nil {
				t.Error("expected Strengths and Gaps to be non-nil slices")
			}
		})
	}
}
