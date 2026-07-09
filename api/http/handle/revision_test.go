package handle_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/sriganeshlokesh/forged/api/dto"
	"github.com/sriganeshlokesh/forged/api/http/handle"
	"github.com/sriganeshlokesh/forged/api/http/handle/mocks"
	"github.com/sriganeshlokesh/forged/application/revision"
	"github.com/sriganeshlokesh/forged/domain/model"
)

func validRevisionBody() string {
	return `{
		"job_description": "Go backend engineer",
		"suggestion": {
			"text": "Tighten bullet",
			"section": "experience",
			"dimension": "impact",
			"estimated_lift": 3,
			"action": {
				"type": "rewrite_field",
				"target": {
					"section": "experience",
					"item_id": "a1b2c3",
					"field": "bullets"
				}
			}
		},
		"target": {
			"field": "bullets",
			"content": "<ul><li>old</li></ul>",
			"context": {
				"company": "Acme",
				"role": "SWE",
				"name": ""
			}
		}
	}`
}

func happyRevisionOutput() *revision.Output {
	return &revision.Output{Revision: &model.Revision{
		Changes: []model.Change{{
			Target:    model.RevisionTarget{Section: "experience", ItemID: "a1b2c3", Field: "bullets"},
			Before:    "<ul><li>old</li></ul>",
			After:     "<ul><li>new</li></ul>",
			Rationale: "tightened",
		}},
		Warnings: []string{},
	}}
}

func TestRevisionHandler_Revise_HappyPath(t *testing.T) {
	uc := mocks.NewMockRevisionUseCase(t)
	uc.EXPECT().Execute(mock.Anything, mock.Anything).Return(happyRevisionOutput(), nil)

	h := handle.NewRevisionHandler(uc, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/v1/revisions", bytes.NewBufferString(validRevisionBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Revise(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	rawBody := rec.Body.String()
	if !strings.Contains(rawBody, `"item_id":"a1b2c3"`) {
		t.Errorf("expected raw body to contain %q, got: %s", `"item_id":"a1b2c3"`, rawBody)
	}

	var resp dto.RevisionResponse
	if err := json.NewDecoder(strings.NewReader(rawBody)).Decode(&resp); err != nil {
		t.Fatalf("failed to decode revision response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if len(resp.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(resp.Changes))
	}
	ch := resp.Changes[0]
	if ch.Target.Section != "experience" {
		t.Errorf("expected target.section 'experience', got %q", ch.Target.Section)
	}
	if ch.Target.ItemID != "a1b2c3" {
		t.Errorf("expected target.item_id 'a1b2c3', got %q", ch.Target.ItemID)
	}
	if ch.Target.Field != "bullets" {
		t.Errorf("expected target.field 'bullets', got %q", ch.Target.Field)
	}
	if ch.Before != "<ul><li>old</li></ul>" {
		t.Errorf("unexpected before: %q", ch.Before)
	}
	if ch.After != "<ul><li>new</li></ul>" {
		t.Errorf("unexpected after: %q", ch.After)
	}
	if ch.Rationale != "tightened" {
		t.Errorf("unexpected rationale: %q", ch.Rationale)
	}
	if resp.Warnings == nil {
		t.Error("expected Warnings to be non-nil")
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("expected empty Warnings, got %v", resp.Warnings)
	}
}

func TestRevisionHandler_Revise_Errors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		setup      func(m *mocks.MockRevisionUseCase)
		wantStatus int
		wantCode   int
	}{
		{
			name:       "malformed JSON → 400 code 10001",
			body:       "{not json",
			wantStatus: http.StatusBadRequest,
			wantCode:   10001,
		},
		{
			name:       ">1MB body → 400 code 10001",
			body:       `{"job_description":"` + strings.Repeat("a", (1<<20)+1) + `"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   10001,
		},
		{
			name: "ErrUnknownAction → 400 code 10002",
			body: validRevisionBody(),
			setup: func(m *mocks.MockRevisionUseCase) {
				m.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil, model.ErrUnknownAction)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   10002,
		},
		{
			name: "ErrTargetMismatch → 400 code 10002",
			body: validRevisionBody(),
			setup: func(m *mocks.MockRevisionUseCase) {
				m.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil, model.ErrTargetMismatch)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   10002,
		},
		{
			name: "ErrRevisionFailed → 503 code 30003",
			body: validRevisionBody(),
			setup: func(m *mocks.MockRevisionUseCase) {
				m.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("%w: llm down", model.ErrRevisionFailed))
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   30003,
		},
		{
			name: "unknown error → 500 code 30001",
			body: validRevisionBody(),
			setup: func(m *mocks.MockRevisionUseCase) {
				m.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   30001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := mocks.NewMockRevisionUseCase(t)
			if tt.setup != nil {
				tt.setup(uc)
			}
			h := handle.NewRevisionHandler(uc, slog.Default())

			req := httptest.NewRequest(http.MethodPost, "/v1/revisions", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.Revise(rec, req)

			res := rec.Result()
			defer func() { _ = res.Body.Close() }()

			if res.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, res.StatusCode)
			}

			var errBody dto.ErrorResponse
			if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if errBody.Code != tt.wantCode {
				t.Errorf("expected error code %d, got %d", tt.wantCode, errBody.Code)
			}
		})
	}
}
