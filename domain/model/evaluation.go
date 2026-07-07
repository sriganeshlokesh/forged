package model

import "errors"

// Sentinel errors for resume evaluation domain validation.
// Callers check these with errors.Is.
var (
	ErrEmptyJobDescription = errors.New("job_description must not be empty")
	ErrEmptyResume         = errors.New("resume must contain at least one section")
	// ErrEvaluationFailed signals that the evaluation backend (e.g. an LLM
	// provider) could not produce a result. Handlers map it to a 5xx.
	ErrEvaluationFailed = errors.New("resume evaluation failed")
)

// Evaluation holds the result of evaluating a resume against a job description.
type Evaluation struct {
	Score       int
	Summary     string
	Dimensions  []Dimension
	Strengths   []string
	Gaps        []string
	Suggestions []string
}

// Dimension is one scored axis of an evaluation (e.g. skills match),
// with the evidence that justifies the awarded points.
type Dimension struct {
	Key      string
	Label    string
	Score    int
	Max      int
	Evidence string
}
