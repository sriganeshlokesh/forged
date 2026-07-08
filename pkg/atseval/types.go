package atseval

// Resume is the structured resume input. Rich-text fields (Summary,
// Bullets, Description) may contain HTML; it is stripped when the resume
// is rendered into the evaluation prompt.
type Resume struct {
	FirstName   string
	LastName    string
	Email       string
	LinkedIn    string
	Phone       string
	Location    string
	Summary     string
	Experience  []Experience
	Projects    []Project
	Education   []Education
	SkillGroups []SkillGroup
}

// Experience is a single work-experience entry.
type Experience struct {
	ID         string
	Company    string
	Role       string
	Employment string
	Start      string
	End        string
	Present    bool
	Bullets    string
}

// Project is a single project entry.
type Project struct {
	ID          string
	Name        string
	Link        string
	Description string
	TechStack   []string
}

// Education is a single education entry.
type Education struct {
	ID           string
	School       string
	Degree       string
	Start        string
	End          string
	ExtraDetails []EducationDetail
}

// EducationDetail is a key-value pair for supplementary education info.
type EducationDetail struct {
	Label string
	Value string
}

// SkillGroup groups related skills under a shared label.
type SkillGroup struct {
	ID    string
	Label string
	Items []string
}

// Evaluation is the scored result of comparing a resume to a job description.
type Evaluation struct {
	// Score is the overall match score, 0-100 (sum of dimension scores).
	Score   int
	Summary string
	// Dimensions are the rubric axes with awarded points and evidence.
	Dimensions  []Dimension
	Strengths   []string
	Gaps        []string
	Suggestions []Suggestion
}

// ActionTarget identifies the specific resume field a suggestion action targets.
type ActionTarget struct {
	Section string
	ItemID  string
	Field   string
}

// SuggestionAction specifies a structured edit operation for a suggestion.
type SuggestionAction struct {
	Type   string
	Target ActionTarget
}

// Suggestion is one concrete resume edit, with the resume section to change,
// the rubric dimension it improves, and a realistic estimated score gain.
// Cumulative lifts per dimension never exceed that dimension's remaining
// headroom (max - awarded score); normalization enforces this.
type Suggestion struct {
	Text string
	// Section is the resume section to edit:
	// summary | experience | projects | education | skills ("" when unknown).
	Section string
	// Dimension is the rubric dimension key the edit improves ("" when unknown).
	Dimension string
	// EstimatedLift is the estimated score gain if the edit is applied.
	EstimatedLift int
	// Action is non-nil when the suggestion targets a specific item field for rewriting.
	Action *SuggestionAction
}

// Dimension is one rubric axis with the evidence justifying its score.
type Dimension struct {
	Key      string
	Label    string
	Score    int
	Max      int
	Evidence string
}
