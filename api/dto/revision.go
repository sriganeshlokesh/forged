package dto

import "github.com/sriganeshlokesh/forged/domain/model"

// RevisionContextDTO carries optional display context about the entry being rewritten.
type RevisionContextDTO struct {
	Company string `json:"company,omitempty"`
	Role    string `json:"role,omitempty"`
	Name    string `json:"name,omitempty"`
}

// RevisionTargetSliceDTO is the exact field content the client asks to rewrite.
type RevisionTargetSliceDTO struct {
	Field   string             `json:"field"`
	Content string             `json:"content"`
	Context RevisionContextDTO `json:"context"`
}

// RevisionRequest is the JSON request body for POST /v1/revisions.
type RevisionRequest struct {
	JobDescription string                 `json:"job_description"`
	Suggestion     SuggestionDTO          `json:"suggestion"`
	Target         RevisionTargetSliceDTO `json:"target"`
}

// ChangeDTO is one applied edit in a revision response.
type ChangeDTO struct {
	Target    ActionTargetDTO `json:"target"`
	Before    string          `json:"before"`
	After     string          `json:"after"`
	Rationale string          `json:"rationale"`
}

// RevisionResponse is the JSON response body for POST /v1/revisions.
type RevisionResponse struct {
	Status   string      `json:"status"`
	Changes  []ChangeDTO `json:"changes"`
	Warnings []string    `json:"warnings"`
}

// ToModel maps a SuggestionDTO to a domain model.Suggestion (nil-safe Action).
func (s SuggestionDTO) ToModel() model.Suggestion {
	sug := model.Suggestion{
		Text:          s.Text,
		Section:       s.Section,
		Dimension:     s.Dimension,
		EstimatedLift: s.EstimatedLift,
	}
	if s.Action != nil {
		sug.Action = &model.SuggestionAction{
			Type: model.ActionType(s.Action.Type),
			Target: model.RevisionTarget{
				Section: s.Action.Target.Section,
				ItemID:  s.Action.Target.ItemID,
				Field:   s.Action.Target.Field,
			},
		}
	}
	return sug
}

// ToModel maps a RevisionContextDTO to the domain type.
func (c RevisionContextDTO) ToModel() model.RevisionContext {
	return model.RevisionContext{
		Company: c.Company,
		Role:    c.Role,
		Name:    c.Name,
	}
}

// NewRevisionResponse maps a domain revision into the response DTO,
// normalizing nil slices to empty ones so JSON emits [] instead of null.
func NewRevisionResponse(status string, rev *model.Revision) RevisionResponse {
	changes := make([]ChangeDTO, 0, len(rev.Changes))
	for _, c := range rev.Changes {
		changes = append(changes, ChangeDTO{
			Target: ActionTargetDTO{
				Section: c.Target.Section,
				ItemID:  c.Target.ItemID,
				Field:   c.Target.Field,
			},
			Before:    c.Before,
			After:     c.After,
			Rationale: c.Rationale,
		})
	}
	warnings := rev.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return RevisionResponse{Status: status, Changes: changes, Warnings: warnings}
}
