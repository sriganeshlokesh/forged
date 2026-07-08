package revision_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/sriganeshlokesh/forged/application/revision"
	"github.com/sriganeshlokesh/forged/application/revision/mocks"
	"github.com/sriganeshlokesh/forged/domain/model"
)

// summaryAction is a valid rewrite_field action targeting the summary section.
func summaryAction() *model.SuggestionAction {
	return &model.SuggestionAction{
		Type:   model.ActionRewriteField,
		Target: model.RevisionTarget{Section: "summary", Field: "summary", ItemID: ""},
	}
}

func TestInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		in      *revision.Input
		wantErr error
	}{
		{
			name: "valid summary input",
			in: &revision.Input{
				JobDescription: "Software engineer at Acme",
				Suggestion: model.Suggestion{
					Action: summaryAction(),
				},
				TargetField:   "summary",
				TargetContent: "<p>Experienced Go engineer</p>",
			},
			wantErr: nil,
		},
		{
			name: "empty job description",
			in: &revision.Input{
				JobDescription: "  ",
				Suggestion:     model.Suggestion{Action: summaryAction()},
				TargetField:    "summary",
				TargetContent:  "<p>content</p>",
			},
			wantErr: model.ErrEmptyJobDescription,
		},
		{
			name: "nil action",
			in: &revision.Input{
				JobDescription: "Software engineer",
				Suggestion:     model.Suggestion{Action: nil},
				TargetField:    "summary",
				TargetContent:  "<p>content</p>",
			},
			wantErr: model.ErrUnknownAction,
		},
		{
			name: "wrong action type",
			in: &revision.Input{
				JobDescription: "Software engineer",
				Suggestion: model.Suggestion{
					Action: &model.SuggestionAction{
						Type:   "delete_field",
						Target: model.RevisionTarget{Section: "summary", Field: "summary", ItemID: ""},
					},
				},
				TargetField:   "summary",
				TargetContent: "<p>content</p>",
			},
			wantErr: model.ErrUnknownAction,
		},
		{
			name: "invalid target combo (experience bullets without itemID)",
			in: &revision.Input{
				JobDescription: "Software engineer",
				Suggestion: model.Suggestion{
					Action: &model.SuggestionAction{
						Type:   model.ActionRewriteField,
						Target: model.RevisionTarget{Section: "experience", Field: "bullets", ItemID: ""},
					},
				},
				TargetField:   "bullets",
				TargetContent: "<ul><li>Did things</li></ul>",
			},
			wantErr: model.ErrUnknownAction,
		},
		{
			name: "field mismatch",
			in: &revision.Input{
				JobDescription: "Software engineer",
				Suggestion: model.Suggestion{
					Action: &model.SuggestionAction{
						Type:   model.ActionRewriteField,
						Target: model.RevisionTarget{Section: "experience", Field: "bullets", ItemID: "job-1"},
					},
				},
				TargetField:   "summary",
				TargetContent: "<p>content</p>",
			},
			wantErr: model.ErrTargetMismatch,
		},
		{
			name: "empty target content",
			in: &revision.Input{
				JobDescription: "Software engineer",
				Suggestion:     model.Suggestion{Action: summaryAction()},
				TargetField:    "summary",
				TargetContent:  "   ",
			},
			wantErr: model.ErrEmptyTargetContent,
		},
		{
			name: "target content exceeds 20000 bytes",
			in: &revision.Input{
				JobDescription: "Software engineer",
				Suggestion:     model.Suggestion{Action: summaryAction()},
				TargetField:    "summary",
				TargetContent:  strings.Repeat("a", 20001),
			},
			wantErr: model.ErrTargetMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestExecute_HappyPath(t *testing.T) {
	reviser := mocks.NewMockResumeReviser(t)
	uc := revision.NewUseCase(reviser)

	originalContent := "<p>original summary</p>"
	rawDraft := "<p>rewritten summary</p>"
	// SanitizeHTML of rawDraft should keep it as-is (p is allowed, no unsafe tags).
	expectedAfter := "<p>rewritten summary</p>"

	action := summaryAction()
	in := &revision.Input{
		JobDescription: "Software engineer at Acme",
		Suggestion: model.Suggestion{
			Text:   "Improve clarity",
			Action: action,
		},
		TargetField:   "summary",
		TargetContent: originalContent,
	}

	reviser.EXPECT().
		Revise(mock.Anything, mock.MatchedBy(func(spec model.RevisionSpec) bool {
			return spec.Feedback == ""
		})).
		Return(rawDraft, "tightened wording", nil)

	out, err := uc.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.GetStatus() != "ok" {
		t.Errorf("expected status 'ok', got %q", out.GetStatus())
	}

	result, ok := out.(*revision.Output)
	if !ok {
		t.Fatalf("expected *revision.Output, got %T", out)
	}
	if len(result.Revision.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Revision.Changes))
	}

	ch := result.Revision.Changes[0]
	if ch.Before != originalContent {
		t.Errorf("Before: expected %q, got %q", originalContent, ch.Before)
	}
	if ch.After != expectedAfter {
		t.Errorf("After: expected %q, got %q", expectedAfter, ch.After)
	}
	if ch.Target != action.Target {
		t.Errorf("Target: expected %+v, got %+v", action.Target, ch.Target)
	}
	if ch.Rationale != "tightened wording" {
		t.Errorf("Rationale: expected %q, got %q", "tightened wording", ch.Rationale)
	}
	if result.Revision.Warnings == nil {
		t.Error("Warnings must be non-nil")
	}
	if len(result.Revision.Warnings) != 0 {
		t.Errorf("expected empty Warnings, got %v", result.Revision.Warnings)
	}
}

func TestExecute_SanitizesDraft(t *testing.T) {
	reviser := mocks.NewMockResumeReviser(t)
	uc := revision.NewUseCase(reviser)

	// draft contains unsafe tags; sanitizer should strip script/div but keep p content.
	rawDraft := "<div><p>clean text</p><script>alert(1)</script></div>"

	reviser.EXPECT().
		Revise(mock.Anything, mock.MatchedBy(func(spec model.RevisionSpec) bool {
			return spec.Feedback == ""
		})).
		Return(rawDraft, "rationale", nil)

	in := &revision.Input{
		JobDescription: "Software engineer",
		Suggestion:     model.Suggestion{Text: "Improve", Action: summaryAction()},
		TargetField:    "summary",
		TargetContent:  "<p>original text</p>",
	}

	out, err := uc.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := out.(*revision.Output)
	after := result.Revision.Changes[0].After

	if strings.Contains(after, "script") {
		t.Errorf("After must not contain 'script', got: %q", after)
	}
	if strings.Contains(after, "alert") {
		t.Errorf("After must not contain 'alert', got: %q", after)
	}
	if strings.Contains(after, "div") {
		t.Errorf("After must not contain 'div', got: %q", after)
	}
	if !strings.Contains(after, "<p>clean text</p>") {
		t.Errorf("After must contain '<p>clean text</p>', got: %q", after)
	}
}

func TestExecute_ReviserErrorPassthrough(t *testing.T) {
	reviser := mocks.NewMockResumeReviser(t)
	uc := revision.NewUseCase(reviser)

	reviser.EXPECT().
		Revise(mock.Anything, mock.Anything).
		Return("", "", fmt.Errorf("%w: boom", model.ErrRevisionFailed)).
		Once()

	in := &revision.Input{
		JobDescription: "Software engineer",
		Suggestion:     model.Suggestion{Text: "Improve", Action: summaryAction()},
		TargetField:    "summary",
		TargetContent:  "<p>original content</p>",
	}

	_, err := uc.Execute(context.Background(), in)
	if !errors.Is(err, model.ErrRevisionFailed) {
		t.Fatalf("expected errors.Is(err, ErrRevisionFailed), got %v", err)
	}
}

func TestExecute_RetrySuccess(t *testing.T) {
	reviser := mocks.NewMockResumeReviser(t)
	uc := revision.NewUseCase(reviser)

	// First call: draft introduces a new number (500%) — will fail CheckNoNewNumbers.
	// Original content has no numbers, so "500%" is new.
	reviser.EXPECT().
		Revise(mock.Anything, mock.MatchedBy(func(spec model.RevisionSpec) bool {
			return spec.Feedback == ""
		})).
		Return("<p>grew revenue by 500%</p>", "rationale", nil).
		Once()

	// Second call (retry): clean draft with no new numbers.
	reviser.EXPECT().
		Revise(mock.Anything, mock.MatchedBy(func(spec model.RevisionSpec) bool {
			return spec.Feedback != ""
		})).
		Return("<p>grew revenue significantly</p>", "tightened wording", nil).
		Once()

	in := &revision.Input{
		JobDescription: "Software engineer",
		Suggestion:     model.Suggestion{Text: "Improve summary", Action: summaryAction()},
		TargetField:    "summary",
		TargetContent:  "<p>grew revenue</p>",
	}

	out, err := uc.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}

	result := out.(*revision.Output)
	after := result.Revision.Changes[0].After
	// sanitized clean draft
	if after != "<p>grew revenue significantly</p>" {
		t.Errorf("expected clean draft after retry, got %q", after)
	}
}

func TestExecute_DoubleGuardrailFailure(t *testing.T) {
	reviser := mocks.NewMockResumeReviser(t)
	uc := revision.NewUseCase(reviser)

	// Both calls return a draft that introduces a new number.
	reviser.EXPECT().
		Revise(mock.Anything, mock.MatchedBy(func(spec model.RevisionSpec) bool {
			return spec.Feedback == ""
		})).
		Return("<p>managed 17 people</p>", "rationale", nil).
		Once()

	reviser.EXPECT().
		Revise(mock.Anything, mock.MatchedBy(func(spec model.RevisionSpec) bool {
			return spec.Feedback != ""
		})).
		Return("<p>managed 17 people</p>", "rationale", nil).
		Once()

	in := &revision.Input{
		JobDescription: "Software engineer",
		Suggestion:     model.Suggestion{Text: "Improve", Action: summaryAction()},
		TargetField:    "summary",
		TargetContent:  "<p>managed people</p>",
	}

	_, err := uc.Execute(context.Background(), in)
	if !errors.Is(err, model.ErrRevisionFailed) {
		t.Fatalf("expected errors.Is(err, ErrRevisionFailed), got %v", err)
	}
}
