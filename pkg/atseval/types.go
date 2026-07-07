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
	Name        string
	Link        string
	Description string
	TechStack   []string
}

// Education is a single education entry.
type Education struct {
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
	Suggestions []string
}

// Dimension is one rubric axis with the evidence justifying its score.
type Dimension struct {
	Key      string
	Label    string
	Score    int
	Max      int
	Evidence string
}
