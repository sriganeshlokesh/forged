package model

// Resume represents a candidate's resume with structured sections.
// Fields are plain Go types with no JSON tags — DTOs own serialisation.
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

// IsEmpty returns true when the resume contains no meaningful content.
func (r *Resume) IsEmpty() bool {
	return r.Summary == "" &&
		len(r.Experience) == 0 &&
		len(r.Projects) == 0 &&
		len(r.Education) == 0 &&
		len(r.SkillGroups) == 0
}

// Experience represents a single work-experience entry.
type Experience struct {
	Company    string
	Role       string
	Employment string
	Start      string
	End        string
	Present    bool
	Bullets    string
}

// Project represents a single project entry.
type Project struct {
	Name        string
	Link        string
	Description string
	TechStack   []string
}

// Education represents a single education entry.
type Education struct {
	School       string
	Degree       string
	Start        string
	End          string
	ExtraDetails []EducationDetail
}

// EducationDetail is a key-value pair for supplementary education info (e.g. GPA).
type EducationDetail struct {
	Label string
	Value string
}

// SkillGroup groups related skills under a shared label.
type SkillGroup struct {
	Label string
	Items []string
}
