package atseval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
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

// markerPrefixToSection maps the bracketed marker prefixes rendered in the
// prompt ([exp:...], [prj:...], ...) back to the section they imply.
var markerPrefixToSection = map[string]string{
	"exp": "experience", "prj": "projects", "edu": "education", "skill": "skills",
}

// canonicalField is the single rewrite field permitted per action section.
var canonicalField = map[string]string{
	"summary": "summary", "experience": "bullets", "projects": "description",
}

// actionSections are the sections a Phase-1 rewrite_field action may target.
var actionSections = map[string]bool{"summary": true, "experience": true, "projects": true}

// atsevalItemIDRe mirrors the DTO item-id hygiene rule (opaque client ids).
var atsevalItemIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

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

	knownIDs := knownItemIDs(resume)
	eval, parseErr := parseEvaluation(content, knownIDs)
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
		eval, parseErr = parseEvaluation(content, knownIDs)
		if parseErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrBadResponse, parseErr)
		}
	}
	return eval, nil
}

// knownItemIDs returns the set of non-empty IDs across all resume items.
func knownItemIDs(r Resume) map[string]bool {
	ids := make(map[string]bool)
	for _, e := range r.Experience {
		if e.ID != "" {
			ids[e.ID] = true
		}
	}
	for _, p := range r.Projects {
		if p.ID != "" {
			ids[p.ID] = true
		}
	}
	for _, ed := range r.Education {
		if ed.ID != "" {
			ids[ed.ID] = true
		}
	}
	for _, sg := range r.SkillGroups {
		if sg.ID != "" {
			ids[sg.ID] = true
		}
	}
	return ids
}

// validActionTarget reports whether a suggestion action target is a permitted Phase-1 rewrite target.
func validActionTarget(t ActionTarget, knownIDs map[string]bool) bool {
	switch {
	case t.Section == "summary" && t.Field == "summary" && t.ItemID == "":
		return true
	case t.Section == "experience" && t.Field == "bullets" && t.ItemID != "" && knownIDs[t.ItemID]:
		return true
	case t.Section == "projects" && t.Field == "description" && t.ItemID != "" && knownIDs[t.ItemID]:
		return true
	default:
		return false
	}
}

// normalizeActionTarget strips prompt-rendered marker prefixes and brackets
// from an action target, infers missing section/field from context, and
// enforces canonical field values. It does NOT fabricate item IDs.
func normalizeActionTarget(t ActionTarget, suggestionSection string) ActionTarget {
	// Strip surrounding brackets and whitespace from the item id.
	id := strings.Trim(strings.TrimSpace(t.ItemID), "[]")

	// If id contains ":", split on first ":" and check if the left part is a
	// known marker prefix. If so, record the implied section and take the right
	// part as the bare id.
	var prefixSection string
	if idx := strings.Index(id, ":"); idx >= 0 {
		prefix := strings.ToLower(id[:idx])
		if sec, ok := markerPrefixToSection[prefix]; ok {
			prefixSection = sec
			id = id[idx+1:]
			// After stripping a known prefix, reject ids that don't match the
			// opaque-id format so they cleanly fail the knownIDs check.
			if id != "" && !atsevalItemIDRe.MatchString(id) {
				id = ""
			}
		}
		// If the prefix is unknown, leave id as-is (don't strip arbitrary colons).
	}

	// Resolve section: prefer t.Section if it's a valid action section,
	// then fall back to prefixSection, then suggestionSection.
	section := t.Section
	if !actionSections[section] {
		if prefixSection != "" {
			section = prefixSection
		} else if actionSections[suggestionSection] {
			section = suggestionSection
		}
	}

	// Resolve field: use canonical field for the section if known.
	field := t.Field
	if cf, ok := canonicalField[section]; ok && field != cf {
		field = cf
	}

	// Summary actions must have an empty item_id.
	if section == "summary" {
		id = ""
	}

	return ActionTarget{Section: section, ItemID: id, Field: field}
}

// suggestionActionJSON is the shape of an action object in the model's JSON reply.
type suggestionActionJSON struct {
	Type   string `json:"type"`
	Target struct {
		Section string `json:"section"`
		ItemID  string `json:"item_id"`
		Field   string `json:"field"`
	} `json:"target"`
}

// resolveAction normalizes and validates a raw suggestion action. It returns
// (nil, false) when no actionable action is present, (action, false) when a
// valid action survives, and (nil, true) when a present action was dropped.
func resolveAction(raw *suggestionActionJSON, suggestionSection string, knownIDs map[string]bool) (*SuggestionAction, bool) {
	if raw == nil || raw.Type != "rewrite_field" {
		return nil, false
	}
	t := normalizeActionTarget(ActionTarget{
		Section: raw.Target.Section,
		ItemID:  raw.Target.ItemID,
		Field:   raw.Target.Field,
	}, suggestionSection)
	if validActionTarget(t, knownIDs) {
		return &SuggestionAction{Type: raw.Type, Target: t}, false
	}
	return nil, true
}

// parseEvaluation parses and normalizes the model's JSON reply.
// knownIDs is the set of item IDs present in the resume, used to validate suggestion actions.
func parseEvaluation(content string, knownIDs map[string]bool) (*Evaluation, error) {
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
		Suggestions []struct {
			Text          string                `json:"text"`
			Section       string                `json:"section"`
			Dimension     string                `json:"dimension"`
			EstimatedLift int                   `json:"estimated_lift"`
			Action        *suggestionActionJSON `json:"action"`
		} `json:"suggestions"`
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
		Suggestions: []Suggestion{},
	}
	sum := 0
	// headroom tracks the remaining points available per dimension; suggestion
	// lifts are capped against it so displayed gains stay achievable.
	headroom := make(map[string]int, len(out.Dimensions))
	for _, d := range out.Dimensions {
		if d.Max <= 0 {
			return nil, fmt.Errorf("dimension %q has invalid max %d", d.Key, d.Max)
		}
		score := clamp(d.Score, 0, d.Max)
		sum += score
		headroom[d.Key] = d.Max - score
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

	unknownBudget := 100 - eval.Score
	droppedActions := 0
	for _, s := range out.Suggestions {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		section := s.Section
		if !validSections[section] {
			section = ""
		}
		lift := s.EstimatedLift
		if lift < 0 {
			lift = 0
		}
		if budget, known := headroom[s.Dimension]; known {
			lift = clamp(lift, 0, budget)
			headroom[s.Dimension] -= lift
			sug := Suggestion{Text: text, Section: section, Dimension: s.Dimension, EstimatedLift: lift}
			act, dropped := resolveAction(s.Action, s.Section, knownIDs)
			sug.Action = act
			if dropped {
				droppedActions++
			}
			eval.Suggestions = append(eval.Suggestions, sug)
			continue
		}
		lift = clamp(lift, 0, unknownBudget)
		unknownBudget -= lift
		sug := Suggestion{Text: text, Section: section, Dimension: "", EstimatedLift: lift}
		act, dropped := resolveAction(s.Action, s.Section, knownIDs)
		sug.Action = act
		if dropped {
			droppedActions++
		}
		eval.Suggestions = append(eval.Suggestions, sug)
	}
	if droppedActions > 0 {
		slog.Debug("atseval: dropped invalid suggestion actions", "count", droppedActions)
	}
	return eval, nil
}

// validSections are the resume sections the frontend can jump to.
var validSections = map[string]bool{
	"summary":    true,
	"experience": true,
	"projects":   true,
	"education":  true,
	"skills":     true,
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
