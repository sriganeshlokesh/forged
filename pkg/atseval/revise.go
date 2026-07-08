package atseval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// RevisionContext carries optional display context about the entry being
// rewritten (e.g. the company and role of an experience item).
type RevisionContext struct {
	Company string
	Role    string
	Name    string
}

// RevisionRequest is the input for a single-field rewrite.
type RevisionRequest struct {
	JobDescription string
	SuggestionText string
	ActionType     string
	Field          string
	Content        string
	Context        RevisionContext
	Feedback       string
}

// RevisionResult is the rewritten field content plus a one-sentence rationale.
type RevisionResult struct {
	After     string
	Rationale string
}

const reviseSystemPrompt = `You rewrite exactly one resume field. Rules:
1. Output the same structural kind you received — bullet list in (<ul><li>…), bullet list out; paragraph in, paragraph out. Use only these tags: p, ul, ol, li, strong, em, br.
2. NEVER add numbers, metrics, percentages, team sizes, dates, tool names, or achievements that are not present in the original text. You may rephrase, reorder, tighten, and strengthen verbs.
3. Weave in terminology from the job description only where the original content already supports it truthfully.
4. Keep roughly the original length (±40%).
5. Return JSON only: {"after": "...", "rationale": "..."} — rationale is one sentence describing what you changed.`

const revisionSchema = `{
  "type": "json_schema",
  "json_schema": {
    "name": "resume_revision",
    "strict": true,
    "schema": {
      "type": "object",
      "additionalProperties": false,
      "required": ["after", "rationale"],
      "properties": {
        "after": {"type": "string"},
        "rationale": {"type": "string"}
      }
    }
  }
}`

func reviseUserPrompt(req RevisionRequest) string {
	var b strings.Builder
	b.WriteString("# Job description\n")
	b.WriteString(strings.TrimSpace(req.JobDescription))
	b.WriteString("\n\n# Suggestion to apply\n")
	b.WriteString(strings.TrimSpace(req.SuggestionText))
	b.WriteString("\n\n# Field being rewritten (")
	b.WriteString(req.Field)
	ctx := contextParts(req.Context)
	if ctx != "" {
		b.WriteString(", context: ")
		b.WriteString(ctx)
	}
	b.WriteString(")\n")
	b.WriteString(req.Content)
	if req.Feedback != "" {
		b.WriteString("\n\n# Previous attempt was rejected — fix this violation\n")
		b.WriteString(req.Feedback)
	}
	return b.String()
}

func contextParts(c RevisionContext) string {
	parts := make([]string, 0, 3)
	if c.Company != "" {
		parts = append(parts, c.Company)
	}
	if c.Role != "" {
		parts = append(parts, c.Role)
	}
	if c.Name != "" {
		parts = append(parts, c.Name)
	}
	return strings.Join(parts, ", ")
}

// Revise rewrites one resume field per req and returns the revised content.
func (e *Evaluator) Revise(ctx context.Context, req RevisionRequest) (*RevisionResult, error) {
	messages := []chatMessage{
		{Role: "system", Content: reviseSystemPrompt},
		{Role: "user", Content: reviseUserPrompt(req)},
	}

	content, status, err := e.chat(ctx, messages, revisionSchema)
	if err != nil && status >= 400 && status < 500 {
		// Provider may not support json_schema; fall back to json_object
		// (the shape is also described in the system prompt).
		content, _, err = e.chat(ctx, messages, jsonObjectFormat)
	}
	if err != nil {
		return nil, err
	}

	result, parseErr := parseRevision(content)
	if parseErr != nil {
		// One retry with an explicit reminder for models that wrapped the
		// JSON in prose or markdown fences.
		retry := append(messages,
			chatMessage{Role: "assistant", Content: content},
			chatMessage{Role: "user", Content: "Your previous reply was not valid JSON. Return ONLY the JSON object described in the instructions, with no markdown fences or commentary."},
		)
		content, _, err = e.chat(ctx, retry, jsonObjectFormat)
		if err != nil {
			return nil, err
		}
		result, parseErr = parseRevision(content)
		if parseErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrBadResponse, parseErr)
		}
	}
	return result, nil
}

// parseRevision parses the model's JSON reply into a RevisionResult.
func parseRevision(content string) (*RevisionResult, error) {
	var out struct {
		After     string `json:"after"`
		Rationale string `json:"rationale"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &out); err != nil {
		return nil, fmt.Errorf("parse revision JSON: %w", err)
	}
	if strings.TrimSpace(out.After) == "" {
		return nil, errors.New("revision JSON has empty after")
	}
	return &RevisionResult{After: out.After, Rationale: out.Rationale}, nil
}
