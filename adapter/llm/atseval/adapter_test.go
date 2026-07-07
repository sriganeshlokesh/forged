package atseval_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	llmats "github.com/sriganeshlokesh/forged/adapter/llm/atseval"
	"github.com/sriganeshlokesh/forged/domain/model"
	"github.com/sriganeshlokesh/forged/pkg/atseval"
)

func fakeServer(t *testing.T, status int, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": content}}},
		})
		_, _ = w.Write(body)
	}))
}

func TestAdapter_Evaluate_MapsDomainTypes(t *testing.T) {
	srv := fakeServer(t, http.StatusOK, `{
		"score": 40, "summary": "ok",
		"dimensions": [{"key": "skills_match", "label": "Skills match", "score": 40, "max": 35, "evidence": "Go"}],
		"strengths": ["s"], "gaps": ["g"], "suggestions": ["fix"]
	}`)
	defer srv.Close()

	engine := atseval.New(atseval.Options{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	a := llmats.New(engine, slog.Default())

	eval, err := a.Evaluate(context.Background(), "Go engineer", &model.Resume{
		Summary:    "<p>dev</p>",
		Experience: []model.Experience{{Company: "Acme", Role: "SWE", Bullets: "<ul><li>x</li></ul>"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Score != 35 { // clamped to dimension max
		t.Errorf("expected score 35, got %d", eval.Score)
	}
	if len(eval.Dimensions) != 1 || eval.Dimensions[0].Key != "skills_match" {
		t.Errorf("unexpected dimensions: %+v", eval.Dimensions)
	}
	if len(eval.Suggestions) != 1 || eval.Suggestions[0] != "fix" {
		t.Errorf("unexpected suggestions: %+v", eval.Suggestions)
	}
}

func TestAdapter_Evaluate_WrapsFailures(t *testing.T) {
	srv := fakeServer(t, http.StatusInternalServerError, "boom")
	defer srv.Close()

	engine := atseval.New(atseval.Options{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	a := llmats.New(engine, slog.Default())

	_, err := a.Evaluate(context.Background(), "Go engineer", &model.Resume{Summary: "dev"})
	if !errors.Is(err, model.ErrEvaluationFailed) {
		t.Fatalf("expected ErrEvaluationFailed, got %v", err)
	}
}
