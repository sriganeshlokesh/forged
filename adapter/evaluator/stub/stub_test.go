package stub_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sriganeshlokesh/forged/adapter/evaluator/stub"
	"github.com/sriganeshlokesh/forged/domain/model"
)

const wantRationale = "stub reviser: deterministic edit for local development"

func TestStubReviser_BulletsInsertedBeforeClosingUL(t *testing.T) {
	s := stub.NewStubReviser()
	spec := model.RevisionSpec{
		Action:         model.SuggestionAction{Target: model.RevisionTarget{Field: "bullets"}},
		Content:        "<ul><li>one</li></ul>",
		SuggestionText: "add metrics",
	}
	after, rationale, err := s.Revise(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rationale != wantRationale {
		t.Errorf("rationale = %q, want %q", rationale, wantRationale)
	}
	marker := "<li>[stub edit] add metrics</li></ul>"
	if !strings.Contains(after, marker) {
		t.Errorf("expected result to contain %q, got %q", marker, after)
	}
	if !strings.HasPrefix(after, "<ul><li>one</li>") {
		t.Errorf("expected result to start with '<ul><li>one</li>', got %q", after)
	}
}

func TestStubReviser_BulletsNoClosingULFallback(t *testing.T) {
	s := stub.NewStubReviser()
	spec := model.RevisionSpec{
		Action:         model.SuggestionAction{Target: model.RevisionTarget{Field: "bullets"}},
		Content:        "<li>one</li>",
		SuggestionText: "add metrics",
	}
	after, rationale, err := s.Revise(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rationale != wantRationale {
		t.Errorf("rationale = %q, want %q", rationale, wantRationale)
	}
	want := "<li>one</li><li>[stub edit] add metrics</li>"
	if after != want {
		t.Errorf("expected %q, got %q", want, after)
	}
}

func TestStubReviser_SummaryAppendsP(t *testing.T) {
	s := stub.NewStubReviser()
	spec := model.RevisionSpec{
		Action:         model.SuggestionAction{Target: model.RevisionTarget{Field: "summary"}},
		Content:        "<p>original</p>",
		SuggestionText: "add metrics",
	}
	after, rationale, err := s.Revise(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rationale != wantRationale {
		t.Errorf("rationale = %q, want %q", rationale, wantRationale)
	}
	want := "<p>original</p><p>[stub edit] add metrics</p>"
	if after != want {
		t.Errorf("expected %q, got %q", want, after)
	}
}

func TestStubReviser_DescriptionAppendsP(t *testing.T) {
	s := stub.NewStubReviser()
	spec := model.RevisionSpec{
		Action:         model.SuggestionAction{Target: model.RevisionTarget{Field: "description"}},
		Content:        "<p>original</p>",
		SuggestionText: "add metrics",
	}
	after, rationale, err := s.Revise(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rationale != wantRationale {
		t.Errorf("rationale = %q, want %q", rationale, wantRationale)
	}
	want := "<p>original</p><p>[stub edit] add metrics</p>"
	if after != want {
		t.Errorf("expected %q, got %q", want, after)
	}
}

func TestStubReviser_ExactRationale(t *testing.T) {
	s := stub.NewStubReviser()
	cases := []struct {
		name  string
		field string
	}{
		{"bullets with ul", "bullets"},
		{"summary", "summary"},
		{"description", "description"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := model.RevisionSpec{
				Action:         model.SuggestionAction{Target: model.RevisionTarget{Field: tc.field}},
				Content:        "<ul><li>x</li></ul>",
				SuggestionText: "add metrics",
			}
			_, rationale, err := s.Revise(context.Background(), spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rationale != wantRationale {
				t.Errorf("rationale = %q, want %q", rationale, wantRationale)
			}
		})
	}
}
