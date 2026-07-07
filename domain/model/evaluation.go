package model

import "errors"

// Sentinel errors for resume evaluation domain validation.
// Callers check these with errors.Is.
var (
	ErrEmptyJobDescription = errors.New("job_description must not be empty")
	ErrEmptyResume         = errors.New("resume must contain at least one section")
)

// Evaluation holds the result of evaluating a resume against a job description.
type Evaluation struct {
	Score       int
	Summary     string
	Suggestions []string
}
