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
	"suggestions": [
		{"text": "Quantify the Acme bullet", "section": "experience", "dimension": "impact_evidence", "estimated_lift": 4}
	]
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
	eval, err := parseEvaluation(raw, nil)
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

func TestParseEvaluation_SuggestionNormalization(t *testing.T) {
	// skills_match has 35-28 = 7 points of headroom; overall score 28 → 72 unknown budget.
	raw := `{
		"score": 28,
		"summary": "s",
		"dimensions": [
			{"key": "skills_match", "label": "Skills match", "score": 28, "max": 35, "evidence": ""}
		],
		"strengths": [], "gaps": [],
		"suggestions": [
			{"text": "Add gRPC", "section": "skills", "dimension": "skills_match", "estimated_lift": 5},
			{"text": "Add Kafka", "section": "skills", "dimension": "skills_match", "estimated_lift": 6},
			{"text": "  ", "section": "skills", "dimension": "skills_match", "estimated_lift": 3},
			{"text": "Bad section", "section": "hobbies", "dimension": "unknown_dim", "estimated_lift": -2}
		]
	}`
	eval, err := parseEvaluation(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eval.Suggestions) != 3 {
		t.Fatalf("expected 3 suggestions (blank text dropped), got %d", len(eval.Suggestions))
	}
	if eval.Suggestions[0].EstimatedLift != 5 {
		t.Errorf("first lift: expected 5, got %d", eval.Suggestions[0].EstimatedLift)
	}
	// Second suggestion asked for 6 but only 2 points of skills_match headroom remain.
	if eval.Suggestions[1].EstimatedLift != 2 {
		t.Errorf("second lift: expected cap at 2 (remaining headroom), got %d", eval.Suggestions[1].EstimatedLift)
	}
	last := eval.Suggestions[2]
	if last.Section != "" || last.Dimension != "" {
		t.Errorf("expected invalid section/dimension blanked, got %q/%q", last.Section, last.Dimension)
	}
	if last.EstimatedLift != 0 {
		t.Errorf("expected negative lift clamped to 0, got %d", last.EstimatedLift)
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

// testResumeWithIDs returns a Resume with known item IDs for action target tests.
func testResumeWithIDs() Resume {
	return Resume{
		Summary: "Backend engineer",
		Experience: []Experience{
			{ID: "a1b2c3", Company: "Acme", Role: "SWE", Present: true, Bullets: "Built X"},
			{ID: "d4e5f6", Company: "Beta", Role: "Lead", Present: false, Bullets: "Led Y"},
		},
		Projects: []Project{
			{ID: "p9q8r7", Name: "MyProject", Description: "Cool stuff"},
		},
		SkillGroups: []SkillGroup{{Label: "Languages", Items: []string{"Go"}}},
	}
}

func TestNormalizeActionTarget(t *testing.T) {
	tests := []struct {
		name              string
		target            ActionTarget
		suggestionSection string
		want              ActionTarget
	}{
		{
			name:              "fully specified valid experience action",
			target:            ActionTarget{Section: "experience", ItemID: "a1b2c3", Field: "bullets"},
			suggestionSection: "experience",
			want:              ActionTarget{Section: "experience", ItemID: "a1b2c3", Field: "bullets"},
		},
		{
			name:              "exp: prefix stripped",
			target:            ActionTarget{Section: "experience", ItemID: "exp:a1b2c3", Field: "bullets"},
			suggestionSection: "experience",
			want:              ActionTarget{Section: "experience", ItemID: "a1b2c3", Field: "bullets"},
		},
		{
			name:              "bracketed [exp:UUID] stripped",
			target:            ActionTarget{Section: "", ItemID: "[exp:a1b2c3]", Field: ""},
			suggestionSection: "experience",
			want:              ActionTarget{Section: "experience", ItemID: "a1b2c3", Field: "bullets"},
		},
		{
			name:              "only item_id with exp: prefix, section and field absent",
			target:            ActionTarget{Section: "", ItemID: "exp:a1b2c3", Field: ""},
			suggestionSection: "",
			want:              ActionTarget{Section: "experience", ItemID: "a1b2c3", Field: "bullets"},
		},
		{
			name:              "prj: prefix maps to projects/description",
			target:            ActionTarget{Section: "", ItemID: "prj:p9q8r7", Field: ""},
			suggestionSection: "projects",
			want:              ActionTarget{Section: "projects", ItemID: "p9q8r7", Field: "description"},
		},
		{
			name:              "summary action blanks item_id",
			target:            ActionTarget{Section: "summary", ItemID: "some-stray-id", Field: "summary"},
			suggestionSection: "summary",
			want:              ActionTarget{Section: "summary", ItemID: "", Field: "summary"},
		},
		{
			name:              "edu: prefix maps to education but not an actionSection",
			target:            ActionTarget{Section: "", ItemID: "edu:abc123", Field: ""},
			suggestionSection: "education",
			want:              ActionTarget{Section: "education", ItemID: "abc123", Field: ""},
		},
		{
			name:              "unknown prefix preserved as-is",
			target:            ActionTarget{Section: "experience", ItemID: "foo:bar", Field: "bullets"},
			suggestionSection: "experience",
			want:              ActionTarget{Section: "experience", ItemID: "foo:bar", Field: "bullets"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeActionTarget(tt.target, tt.suggestionSection)
			if got != tt.want {
				t.Errorf("normalizeActionTarget(%+v, %q) = %+v, want %+v", tt.target, tt.suggestionSection, got, tt.want)
			}
		})
	}
}

// evalJSONWithAction builds a complete evaluation JSON with a single suggestion
// that has the given action JSON inline.
func evalJSONWithAction(actionJSON string) string {
	return `{
		"score": 55,
		"summary": "Decent match.",
		"dimensions": [
			{"key": "impact_evidence", "label": "Impact and evidence", "score": 12, "max": 20, "evidence": "Quantified bullets"}
		],
		"strengths": ["Strong Go background"],
		"gaps": ["No Kubernetes"],
		"suggestions": [
			{"text": "Improve bullets", "section": "experience", "dimension": "impact_evidence", "estimated_lift": 4, "action": ` + actionJSON + `}
		]
	}`
}

func TestResolveAction_IntegrationCases(t *testing.T) {
	resume := testResumeWithIDs()
	knownIDs := knownItemIDs(resume)

	tests := []struct {
		name          string
		actionJSON    string
		wantActionNil bool
		wantDropped   bool
		wantItemID    string
		wantSection   string
		wantField     string
	}{
		{
			name:          "fully specified valid action survives",
			actionJSON:    `{"type":"rewrite_field","target":{"section":"experience","item_id":"a1b2c3","field":"bullets"}}`,
			wantActionNil: false,
			wantItemID:    "a1b2c3",
			wantSection:   "experience",
			wantField:     "bullets",
		},
		{
			name:          "exp: prefix stripped and survives",
			actionJSON:    `{"type":"rewrite_field","target":{"section":"experience","item_id":"exp:a1b2c3","field":"bullets"}}`,
			wantActionNil: false,
			wantItemID:    "a1b2c3",
			wantSection:   "experience",
			wantField:     "bullets",
		},
		{
			name:          "bracketed [exp:UUID] survives",
			actionJSON:    `{"type":"rewrite_field","target":{"section":"experience","item_id":"[exp:a1b2c3]","field":"bullets"}}`,
			wantActionNil: false,
			wantItemID:    "a1b2c3",
			wantSection:   "experience",
			wantField:     "bullets",
		},
		{
			name:          "only exp: item_id no section/field inferred",
			actionJSON:    `{"type":"rewrite_field","target":{"section":"","item_id":"exp:a1b2c3","field":""}}`,
			wantActionNil: false,
			wantItemID:    "a1b2c3",
			wantSection:   "experience",
			wantField:     "bullets",
		},
		{
			name:          "prj: prefix maps to projects/description survives",
			actionJSON:    `{"type":"rewrite_field","target":{"section":"","item_id":"prj:p9q8r7","field":""}}`,
			wantActionNil: false,
			wantItemID:    "p9q8r7",
			wantSection:   "projects",
			wantField:     "description",
		},
		{
			name:          "summary with stray item_id blanked and survives",
			actionJSON:    `{"type":"rewrite_field","target":{"section":"summary","item_id":"stray-id","field":"summary"}}`,
			wantActionNil: false,
			wantItemID:    "",
			wantSection:   "summary",
			wantField:     "summary",
		},
		{
			name:          "unknown id after stripping is dropped",
			actionJSON:    `{"type":"rewrite_field","target":{"section":"experience","item_id":"exp:unknown-id-xyz","field":"bullets"}}`,
			wantActionNil: true,
			wantDropped:   true,
		},
		{
			name:          "edu: prefix not an action section is dropped",
			actionJSON:    `{"type":"rewrite_field","target":{"section":"education","item_id":"edu:abc123","field":"degree"}}`,
			wantActionNil: true,
			wantDropped:   true,
		},
		{
			name:          "null action survives with nil action",
			actionJSON:    `null`,
			wantActionNil: true,
			wantDropped:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(chatBody(evalJSONWithAction(tt.actionJSON))))
			}))
			defer srv.Close()

			eval, err := newTestEvaluator(srv.URL).Evaluate(context.Background(), "Go engineer", resume)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(eval.Suggestions) != 1 {
				t.Fatalf("expected 1 suggestion, got %d", len(eval.Suggestions))
			}
			sug := eval.Suggestions[0]
			if tt.wantActionNil {
				if sug.Action != nil {
					t.Errorf("expected nil action, got %+v", sug.Action)
				}
			} else {
				if sug.Action == nil {
					t.Fatal("expected non-nil action, got nil")
				}
				if sug.Action.Target.ItemID != tt.wantItemID {
					t.Errorf("item_id: got %q, want %q", sug.Action.Target.ItemID, tt.wantItemID)
				}
				if sug.Action.Target.Section != tt.wantSection {
					t.Errorf("section: got %q, want %q", sug.Action.Target.Section, tt.wantSection)
				}
				if sug.Action.Target.Field != tt.wantField {
					t.Errorf("field: got %q, want %q", sug.Action.Target.Field, tt.wantField)
				}
			}
			_ = knownIDs // used in the test setup
		})
	}
}
