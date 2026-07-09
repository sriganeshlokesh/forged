package dto

import (
	"regexp"

	"github.com/sriganeshlokesh/forged/domain/model"
)

// itemIDRe accepts short opaque item IDs; anything else is treated as absent.
var itemIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// cleanItemID returns id when it matches the allowed shape, otherwise "".
func cleanItemID(id string) string {
	if itemIDRe.MatchString(id) {
		return id
	}
	return ""
}

// EvaluationRequest is the JSON request body for POST /v1/evaluations.
type EvaluationRequest struct {
	JobDescription string     `json:"job_description"`
	Resume         *ResumeDTO `json:"resume"`
}

// ResumeDTO mirrors the frontend ResumeData contract (all keys snake_case).
// Rich-text fields (Summary, Bullets, Description) are accepted as HTML strings from Tiptap.
type ResumeDTO struct {
	FirstName   string          `json:"first_name"`
	LastName    string          `json:"last_name"`
	Email       string          `json:"email"`
	LinkedIn    string          `json:"linkedin"`
	Phone       string          `json:"phone"`
	Location    string          `json:"location"`
	Summary     string          `json:"summary"`
	Experience  []ExperienceDTO `json:"experience"`
	Projects    []ProjectDTO    `json:"projects"`
	Education   []EducationDTO  `json:"education"`
	SkillGroups []SkillGroupDTO `json:"skill_groups"`
}

// ExperienceDTO is a single work-experience entry.
type ExperienceDTO struct {
	ID         string `json:"id,omitempty"`
	Company    string `json:"company"`
	Role       string `json:"role"`
	Employment string `json:"employment"`
	Start      string `json:"start"`
	End        string `json:"end"`
	Present    bool   `json:"present"`
	Bullets    string `json:"bullets"`
}

// ProjectDTO is a single project entry.
type ProjectDTO struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Link        string   `json:"link"`
	Description string   `json:"description"`
	TechStack   []string `json:"tech_stack"`
}

// EducationDTO is a single education entry.
type EducationDTO struct {
	ID           string               `json:"id,omitempty"`
	School       string               `json:"school"`
	Degree       string               `json:"degree"`
	Start        string               `json:"start"`
	End          string               `json:"end"`
	ExtraDetails []EducationDetailDTO `json:"extra_details"`
}

// EducationDetailDTO is a key-value pair for supplementary education info.
type EducationDetailDTO struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// SkillGroupDTO groups related skills under a shared label.
type SkillGroupDTO struct {
	ID    string   `json:"id,omitempty"`
	Label string   `json:"label"`
	Items []string `json:"items"`
}

// ToModel maps a ResumeDTO to a domain model.Resume.
// Returns nil if the receiver is nil.
func (r *ResumeDTO) ToModel() *model.Resume {
	if r == nil {
		return nil
	}

	resume := &model.Resume{
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Email:     r.Email,
		LinkedIn:  r.LinkedIn,
		Phone:     r.Phone,
		Location:  r.Location,
		Summary:   r.Summary,
	}

	for _, e := range r.Experience {
		resume.Experience = append(resume.Experience, model.Experience{
			ID:         cleanItemID(e.ID),
			Company:    e.Company,
			Role:       e.Role,
			Employment: e.Employment,
			Start:      e.Start,
			End:        e.End,
			Present:    e.Present,
			Bullets:    e.Bullets,
		})
	}

	for _, p := range r.Projects {
		resume.Projects = append(resume.Projects, model.Project{
			ID:          cleanItemID(p.ID),
			Name:        p.Name,
			Link:        p.Link,
			Description: p.Description,
			TechStack:   p.TechStack,
		})
	}

	for _, ed := range r.Education {
		edm := model.Education{
			ID:     cleanItemID(ed.ID),
			School: ed.School,
			Degree: ed.Degree,
			Start:  ed.Start,
			End:    ed.End,
		}
		for _, d := range ed.ExtraDetails {
			edm.ExtraDetails = append(edm.ExtraDetails, model.EducationDetail{
				Label: d.Label,
				Value: d.Value,
			})
		}
		resume.Education = append(resume.Education, edm)
	}

	for _, sg := range r.SkillGroups {
		resume.SkillGroups = append(resume.SkillGroups, model.SkillGroup{
			ID:    cleanItemID(sg.ID),
			Label: sg.Label,
			Items: sg.Items,
		})
	}

	return resume
}

// EvaluationResponse is the JSON response body for POST /v1/evaluations.
type EvaluationResponse struct {
	Status      string          `json:"status"`
	Score       int             `json:"score"`
	Summary     string          `json:"summary"`
	Dimensions  []DimensionDTO  `json:"dimensions"`
	Strengths   []string        `json:"strengths"`
	Gaps        []string        `json:"gaps"`
	Suggestions []SuggestionDTO `json:"suggestions"`
}

// ActionTargetDTO locates the exact resume field a suggestion action rewrites.
type ActionTargetDTO struct {
	Section string `json:"section"`
	ItemID  string `json:"item_id"`
	Field   string `json:"field"`
}

// SuggestionActionDTO is the machine-actionable part of a suggestion.
type SuggestionActionDTO struct {
	Type   string          `json:"type"`
	Target ActionTargetDTO `json:"target"`
}

// SuggestionDTO is one suggested resume edit with its estimated score lift
// and the resume section the frontend should jump to.
type SuggestionDTO struct {
	Text          string               `json:"text"`
	Section       string               `json:"section"`
	Dimension     string               `json:"dimension"`
	EstimatedLift int                  `json:"estimated_lift"`
	Action        *SuggestionActionDTO `json:"action,omitempty"`
}

// DimensionDTO is one scored rubric axis in an evaluation response.
type DimensionDTO struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Score    int    `json:"score"`
	Max      int    `json:"max"`
	Evidence string `json:"evidence"`
}

// NewEvaluationResponse maps a domain evaluation into the response DTO,
// normalizing nil slices to empty ones so JSON emits [] instead of null.
func NewEvaluationResponse(status string, e *model.Evaluation) EvaluationResponse {
	resp := EvaluationResponse{
		Status:      status,
		Score:       e.Score,
		Summary:     e.Summary,
		Dimensions:  make([]DimensionDTO, 0, len(e.Dimensions)),
		Strengths:   emptyIfNil(e.Strengths),
		Gaps:        emptyIfNil(e.Gaps),
		Suggestions: make([]SuggestionDTO, 0, len(e.Suggestions)),
	}
	for _, d := range e.Dimensions {
		resp.Dimensions = append(resp.Dimensions, DimensionDTO{
			Key:      d.Key,
			Label:    d.Label,
			Score:    d.Score,
			Max:      d.Max,
			Evidence: d.Evidence,
		})
	}
	for _, s := range e.Suggestions {
		sug := SuggestionDTO{
			Text:          s.Text,
			Section:       s.Section,
			Dimension:     s.Dimension,
			EstimatedLift: s.EstimatedLift,
		}
		if s.Action != nil {
			sug.Action = &SuggestionActionDTO{
				Type: string(s.Action.Type),
				Target: ActionTargetDTO{
					Section: s.Action.Target.Section,
					ItemID:  s.Action.Target.ItemID,
					Field:   s.Action.Target.Field,
				},
			}
		}
		resp.Suggestions = append(resp.Suggestions, sug)
	}
	return resp
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ErrorResponse is the error envelope used by all API error responses.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
