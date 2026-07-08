package atseval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// evaluationSchema is the strict JSON schema for the model's reply, used
// with providers that support response_format json_schema.
const evaluationSchema = `{
  "type": "json_schema",
  "json_schema": {
    "name": "resume_evaluation",
    "strict": true,
    "schema": {
      "type": "object",
      "additionalProperties": false,
      "required": ["score", "summary", "dimensions", "strengths", "gaps", "suggestions"],
      "properties": {
        "score": {"type": "integer"},
        "summary": {"type": "string"},
        "dimensions": {
          "type": "array",
          "items": {
            "type": "object",
            "additionalProperties": false,
            "required": ["key", "label", "score", "max", "evidence"],
            "properties": {
              "key": {"type": "string"},
              "label": {"type": "string"},
              "score": {"type": "integer"},
              "max": {"type": "integer"},
              "evidence": {"type": "string"}
            }
          }
        },
        "strengths": {"type": "array", "items": {"type": "string"}},
        "gaps": {"type": "array", "items": {"type": "string"}},
        "suggestions": {
          "type": "array",
          "items": {
            "type": "object",
            "additionalProperties": false,
            "required": ["text", "section", "dimension", "estimated_lift"],
            "properties": {
              "text": {"type": "string"},
              "section": {"type": "string", "enum": ["summary", "experience", "projects", "education", "skills"]},
              "dimension": {"type": "string", "enum": ["skills_match", "experience_relevance", "impact_evidence", "education_extras"]},
              "estimated_lift": {"type": "integer"}
            }
          }
        }
      }
    }
  }
}`

const jsonObjectFormat = `{"type": "json_object"}`

// chat sends one chat-completions request and returns the assistant
// message content. A non-2xx response is reported with the HTTP status so
// the caller can decide whether to retry with a simpler response_format.
func (e *Evaluator) chat(ctx context.Context, messages []chatMessage, responseFormat string) (string, int, error) {
	body, err := json.Marshal(chatRequest{
		Model:          e.opts.Model,
		Messages:       messages,
		Temperature:    0,
		ResponseFormat: json.RawMessage(responseFormat),
	})
	if err != nil {
		return "", 0, fmt.Errorf("%w: encode request: %w", ErrProvider, err)
	}

	url := strings.TrimSuffix(e.opts.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("%w: build request: %w", ErrProvider, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.opts.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.opts.APIKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", ErrProvider, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("%w: read response: %w", ErrProvider, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", resp.StatusCode, fmt.Errorf("%w: status %d: %s", ErrProvider, resp.StatusCode, truncate(string(raw), 300))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", resp.StatusCode, fmt.Errorf("%w: decode response: %w", ErrProvider, err)
	}
	if len(parsed.Choices) == 0 {
		return "", resp.StatusCode, fmt.Errorf("%w: response contained no choices", ErrProvider)
	}
	return parsed.Choices[0].Message.Content, resp.StatusCode, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
