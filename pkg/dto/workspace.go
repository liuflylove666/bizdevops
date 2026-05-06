package dto

import "time"

// ActionItemDTO is the unified task shape for the v2.1 workspace.
// It intentionally stays read-only in Sprint 1 so the dashboard can aggregate
// existing work without changing each domain's state machine.
type ActionItemDTO struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	Owner       string     `json:"owner,omitempty"`
	Application string     `json:"application,omitempty"`
	Env         string     `json:"env,omitempty"`
	ProjectID   *uint      `json:"project_id,omitempty"`
	Path        string     `json:"path"`
	ActionLabel string     `json:"action_label"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	SourceID    uint       `json:"source_id"`
}

type WorkspaceActionsResponse struct {
	Items   []ActionItemDTO      `json:"items"`
	Summary map[string]int       `json:"summary"`
	Groups  map[string][]string  `json:"groups"`
	Meta    WorkspaceActionsMeta `json:"meta"`
}

type WorkspaceActionsMeta struct {
	Limit       int       `json:"limit"`
	GeneratedAt time.Time `json:"generated_at"`
}
