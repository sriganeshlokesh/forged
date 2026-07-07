package atseval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Sentinel errors. Check with errors.Is.
var (
	// ErrProvider signals an HTTP or provider-side failure.
	ErrProvider = errors.New("atseval: provider request failed")
	// ErrBadResponse signals that the model's output could not be parsed
	// into a valid evaluation even after a retry.
	ErrBadResponse = errors.New("atseval: model returned an unusable response")
)

// Options configures an Evaluator. BaseURL and Model are required.
// APIKey may be empty for endpoints without auth (e.g. local Ollama).
type Options struct {
	BaseURL string
	APIKey  string
	Model   string
	// Timeout applies per HTTP request when HTTPClient is nil (default 60s).
	Timeout time.Duration
	// HTTPClient overrides the default client (useful for tests/proxies).
	HTTPClient *http.Client
}

// Evaluator scores resumes against job descriptions via an
// OpenAI-compatible chat-completions endpoint.
type Evaluator struct {
	opts   Options
	client *http.Client
}

// New builds an Evaluator from opts.
func New(opts Options) *Evaluator {
	client := opts.HTTPClient
	if client == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Evaluator{opts: opts, client: client}
}

// Evaluate scores resume against jobDescription and returns the rubric result.
func (e *Evaluator) Evaluate(ctx context.Context, jobDescription string, resume Resume) (*Evaluation, error) {
	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt(jobDescription, resume)},
	}

	content, status, err := e.chat(ctx, messages, evaluationSchema)
	if err != nil && status >= 400 && status < 500 {
		// Provider may not support json_schema; fall back to json_object
		// (the schema is also described in the system prompt).
		content, _, err = e.chat(ctx, messages, jsonObjectFormat)
	}
	if err != nil {
		return nil, err
	}

	eval, parseErr := parseEvaluation(content)
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
		eval, parseErr = parseEvaluation(content)
		if parseErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrBadResponse, parseErr)
		}
	}
	return eval, nil
}

// parseEvaluation parses and normalizes the model's JSON reply.
func parseEvaluation(content string) (*Evaluation, error) {
	var out struct {
		Score      int    `json:"score"`
		Summary    string `json:"summary"`
		Dimensions []struct {
			Key      string `json:"key"`
			Label    string `json:"label"`
			Score    int    `json:"score"`
			Max      int    `json:"max"`
			Evidence string `json:"evidence"`
		} `json:"dimensions"`
		Strengths   []string `json:"strengths"`
		Gaps        []string `json:"gaps"`
		Suggestions []string `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &out); err != nil {
		return nil, fmt.Errorf("parse evaluation JSON: %w", err)
	}
	if len(out.Dimensions) == 0 {
		return nil, errors.New("evaluation JSON has no dimensions")
	}

	eval := &Evaluation{
		Summary:     out.Summary,
		Dimensions:  make([]Dimension, 0, len(out.Dimensions)),
		Strengths:   emptyIfNil(out.Strengths),
		Gaps:        emptyIfNil(out.Gaps),
		Suggestions: emptyIfNil(out.Suggestions),
	}
	sum := 0
	for _, d := range out.Dimensions {
		if d.Max <= 0 {
			return nil, fmt.Errorf("dimension %q has invalid max %d", d.Key, d.Max)
		}
		score := clamp(d.Score, 0, d.Max)
		sum += score
		eval.Dimensions = append(eval.Dimensions, Dimension{
			Key:      d.Key,
			Label:    d.Label,
			Score:    score,
			Max:      d.Max,
			Evidence: d.Evidence,
		})
	}
	// The overall score must equal the dimension sum; trust the sum.
	eval.Score = clamp(sum, 0, 100)
	return eval, nil
}

// extractJSON tolerates markdown fences and surrounding prose by slicing
// from the first '{' to the last '}'.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
