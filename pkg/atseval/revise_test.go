package atseval

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testRevisionRequest() RevisionRequest {
	return RevisionRequest{
		JobDescription: "Senior Go engineer",
		SuggestionText: "Quantify the Acme bullet",
		ActionType:     "rewrite_field",
		Field:          "bullets",
		Content:        "<ul><li>Built X</li></ul>",
		Context:        RevisionContext{Company: "Acme", Role: "SWE"},
	}
}

func TestRevise(t *testing.T) {
	tests := []struct {
		name      string
		responses []struct {
			status int
			body   string
		}
		wantErr   error
		wantAfter string
		wantCalls int
	}{
		{
			name: "happy path",
			responses: []struct {
				status int
				body   string
			}{{200, chatBody(`{"after": "<ul><li>rewritten</li></ul>", "rationale": "tightened verbs"}`)}},
			wantAfter: "<ul><li>rewritten</li></ul>",
			wantCalls: 1,
		},
		{
			name: "json_schema 400 → json_object fallback success",
			responses: []struct {
				status int
				body   string
			}{
				{400, `{"error":{"message":"response_format json_schema not supported"}}`},
				{200, chatBody(`{"after": "<ul><li>rewritten</li></ul>", "rationale": "tightened verbs"}`)},
			},
			wantAfter: "<ul><li>rewritten</li></ul>",
			wantCalls: 2,
		},
		{
			name: "provider 500 → ErrProvider",
			responses: []struct {
				status int
				body   string
			}{{500, "boom"}},
			wantErr:   ErrProvider,
			wantCalls: 1,
		},
		{
			name: "garbage JSON twice → ErrBadResponse",
			responses: []struct {
				status int
				body   string
			}{
				{200, chatBody("not json")},
				{200, chatBody("still not json")},
			},
			wantErr:   ErrBadResponse,
			wantCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				idx := calls
				if idx >= len(tt.responses) {
					idx = len(tt.responses) - 1
				}
				calls++
				w.WriteHeader(tt.responses[idx].status)
				_, _ = w.Write([]byte(tt.responses[idx].body))
			}))
			defer srv.Close()

			result, err := newTestEvaluator(srv.URL).Revise(context.Background(), testRevisionRequest())

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.After != tt.wantAfter {
				t.Errorf("expected After %q, got %q", tt.wantAfter, result.After)
			}
			if calls != tt.wantCalls {
				t.Errorf("expected %d calls, got %d", tt.wantCalls, calls)
			}
		})
	}
}

func TestRevise_FeedbackBlock(t *testing.T) {
	t.Run("feedback present when Feedback set", func(t *testing.T) {
		var gotBody chatRequest
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = w.Write([]byte(chatBody(`{"after": "<ul><li>rewritten</li></ul>", "rationale": "done"}`)))
		}))
		defer srv.Close()

		req := testRevisionRequest()
		req.Feedback = "added a number"
		if _, err := newTestEvaluator(srv.URL).Revise(context.Background(), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(gotBody.Messages) < 2 {
			t.Fatalf("expected at least system+user messages, got %d", len(gotBody.Messages))
		}
		user := gotBody.Messages[1].Content
		if !strings.Contains(user, "# Previous attempt was rejected — fix this violation") {
			t.Errorf("user prompt missing feedback heading:\n%s", user)
		}
		if !strings.Contains(user, "added a number") {
			t.Errorf("user prompt missing feedback text:\n%s", user)
		}
	})

	t.Run("feedback absent when Feedback empty", func(t *testing.T) {
		var gotBody chatRequest
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = w.Write([]byte(chatBody(`{"after": "<ul><li>rewritten</li></ul>", "rationale": "done"}`)))
		}))
		defer srv.Close()

		req := testRevisionRequest()
		req.Feedback = ""
		if _, err := newTestEvaluator(srv.URL).Revise(context.Background(), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(gotBody.Messages) < 2 {
			t.Fatalf("expected at least system+user messages, got %d", len(gotBody.Messages))
		}
		user := gotBody.Messages[1].Content
		if strings.Contains(user, "# Previous attempt was rejected") {
			t.Errorf("user prompt should not contain feedback section when Feedback is empty:\n%s", user)
		}
	})
}

func TestRevise_PromptContents(t *testing.T) {
	var gotBody chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(chatBody(`{"after": "<ul><li>rewritten</li></ul>", "rationale": "tightened verbs"}`)))
	}))
	defer srv.Close()

	if _, err := newTestEvaluator(srv.URL).Revise(context.Background(), testRevisionRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotBody.Messages) != 2 {
		t.Fatalf("expected system+user messages, got %d", len(gotBody.Messages))
	}

	system := gotBody.Messages[0].Content
	if system != reviseSystemPrompt {
		t.Errorf("system message does not equal reviseSystemPrompt:\ngot:  %q\nwant: %q", system, reviseSystemPrompt)
	}

	user := gotBody.Messages[1].Content
	for _, want := range []string{"Senior Go engineer", "Quantify the Acme bullet", "bullets", "Acme, SWE"} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt missing %q:\n%s", want, user)
		}
	}
}
