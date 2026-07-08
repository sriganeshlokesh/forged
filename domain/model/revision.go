package model

import "errors"

type ActionType string

const ActionRewriteField ActionType = "rewrite_field"

type RevisionTarget struct {
	Section string
	ItemID  string
	Field   string
}

type SuggestionAction struct {
	Type   ActionType
	Target RevisionTarget
}

type RevisionContext struct {
	Company string
	Role    string
	Name    string
}

type RevisionSpec struct {
	JobDescription string
	SuggestionText string
	Action         SuggestionAction
	Content        string
	Context        RevisionContext
	Feedback       string
}

type Change struct {
	Target    RevisionTarget
	Before    string
	After     string
	Rationale string
}

type Revision struct {
	Changes  []Change
	Warnings []string
}

var (
	ErrUnknownAction      = errors.New("unknown or missing suggestion action")
	ErrTargetMismatch     = errors.New("target does not match suggestion action")
	ErrEmptyTargetContent = errors.New("target content must not be empty")
	ErrRevisionFailed     = errors.New("revision could not be produced")
)

// ValidTargetCombo reports whether (section, field, itemID presence) is a
// permitted Phase-1 rewrite target.
func ValidTargetCombo(t RevisionTarget) bool {
	switch {
	case t.Section == "summary" && t.Field == "summary" && t.ItemID == "":
		return true
	case t.Section == "experience" && t.Field == "bullets" && t.ItemID != "":
		return true
	case t.Section == "projects" && t.Field == "description" && t.ItemID != "":
		return true
	default:
		return false
	}
}
