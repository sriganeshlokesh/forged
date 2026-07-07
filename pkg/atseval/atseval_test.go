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

const validEvaluationJSON = `{
	"score": 55,
	"summary": "Decent match.",
	"dimensions": [
		{"key": "skills_match", "label": "Skills match", "score": 20, "max": 35, "evidence": "Go, chi"},
		{"key": "experience_relevance", "label": "Experience relevance", "score": 15, "max": 30, "evidence": "Backend roles"},
		{"key": "impact_evidence", "label": "Impact and evidence", "score": 12, "max": 20, "evidence": "Quantified bullets"},
		{"key": "education_extras", "label": "Education and extras", "score": 8, "max": 15, "evidence": "CS degree"}
	],
	"strengths": ["Strong Go background"],
	"gaps": ["No Kubernetes"],
	"suggestions": ["Quantify the Acme bullet"]
}`

func chatBody(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]any{"content": content}}},
	})
	return string(b)
}

func testResume() Resume {
	return Resume{
		Summary: "<p>Backend engineer</p>",
		Experience: []Experience{{
			Company: "Acme", Role: "SWE", Present: true,
			Bullets: "<ul><li>Built X</li><li>Led Y</li></ul>",
		}},
		SkillGroups: []SkillGroup{{Label: "Languages", Items: []string{"Go"}}},
	}
}

func newTestEvaluator(url string) *Evaluator {
	return New(Options{BaseURL: url, APIKey: "test-key", Model: "test-model"})
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name string
		// responses returned by the fake server per call, as (status, body) pairs
		responses []struct {
			status int
			body   string
		}
		wantErr   error
		wantScore int
		wantCalls int
	}{
		{
			name: "success first try",
			responses: []struct {
				status int
				body   string
			}{{200, chatBody(validEvaluationJSON)}},
			wantScore: 55,
			wantCalls: 1,
		},
		{
			name: "markdown-fenced JSON parses via extraction",
			responses: []struct {
				status int
				body   string
			}{{200, chatBody("```json\n" + validEvaluationJSON + "\n```")}},
			wantScore: 55,
			wantCalls: 1,
		},
		{
			name: "malformed JSON then retry success",
			responses: []struct {
				status int
				body   string
			}{
				{200, chatBody("sorry, here is prose with no braces")},
				{200, chatBody(validEvaluationJSON)},
			},
			wantScore: 55,
			wantCalls: 2,
		},
		{
			name: "malformed JSON twice → ErrBadResponse",
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
		{
			name: "json_schema rejected → json_object fallback succeeds",
			responses: []struct {
				status int
				body   string
			}{
				{400, `{"error":{"message":"response_format json_schema not supported"}}`},
				{200, chatBody(validEvaluationJSON)},
			},
			wantScore: 55,
			wantCalls: 2,
		},
		{
			name: "server error → ErrProvider",
			responses: []struct {
				status int
				body   string
			}{{500, "boom"}},
			wantErr:   ErrProvider,
			wantCalls: 1,
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

			eval, err := newTestEvaluator(srv.URL).Evaluate(context.Background(), "Go engineer", testResume())

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if eval.Score != tt.wantScore {
				t.Errorf("expected score %d, got %d", tt.wantScore, eval.Score)
			}
			if calls != tt.wantCalls {
				t.Errorf("expected %d calls, got %d", tt.wantCalls, calls)
			}
			if eval.Strengths == nil || eval.Gaps == nil || eval.Suggestions == nil {
				t.Error("expected non-nil slices in evaluation")
			}
		})
	}
}

func TestEvaluate_PromptContents(t *testing.T) {
	var gotBody chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("expected bearer auth header, got %q", got)
		}
		_, _ = w.Write([]byte(chatBody(validEvaluationJSON)))
	}))
	defer srv.Close()

	if _, err := newTestEvaluator(srv.URL).Evaluate(context.Background(), "Senior Go engineer with Kubernetes", testResume()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotBody.Messages) != 2 {
		t.Fatalf("expected system+user messages, got %d", len(gotBody.Messages))
	}
	user := gotBody.Messages[1].Content
	for _, want := range []string{"Senior Go engineer with Kubernetes", "Acme", "- Built X", "- Led Y", "Backend engineer"} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt missing %q:\n%s", want, user)
		}
	}
	if strings.Contains(user, "<ul>") || strings.Contains(user, "<p>") {
		t.Errorf("user prompt still contains HTML tags:\n%s", user)
	}
}

func TestParseEvaluation_ClampsAndRecomputes(t *testing.T) {
	raw := `{
		"score": 99,
		"summary": "s",
		"dimensions": [
			{"key": "a", "label": "A", "score": 50, "max": 35, "evidence": ""},
			{"key": "b", "label": "B", "score": -5, "max": 30, "evidence": ""}
		],
		"strengths": null, "gaps": null, "suggestions": null
	}`
	eval, err := parseEvaluation(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Dimensions[0].Score != 35 {
		t.Errorf("expected clamp to max 35, got %d", eval.Dimensions[0].Score)
	}
	if eval.Dimensions[1].Score != 0 {
		t.Errorf("expected clamp to 0, got %d", eval.Dimensions[1].Score)
	}
	if eval.Score != 35 {
		t.Errorf("expected recomputed score 35, got %d", eval.Score)
	}
	if eval.Strengths == nil || eval.Gaps == nil || eval.Suggestions == nil {
		t.Error("expected nil slices normalized to empty")
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain text untouched", "no tags here", "no tags here"},
		{"paragraphs", "<p>one</p><p>two</p>", "one\ntwo"},
		{"list items become bullets", "<ul><li>first</li><li>second</li></ul>", "- first\n- second"},
		{"inline formatting stripped", "<p>a <strong>bold</strong> word</p>", "a bold word"},
		{"self-closing br", "line<br/>break", "line\nbreak"},
		{"empty tag does not panic", "a<>b", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripHTML(tt.in); got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
