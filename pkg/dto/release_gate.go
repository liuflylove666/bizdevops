package dto

import "time"

type ReleaseGateSummaryDTO struct {
	ReleaseID    uint                   `json:"release_id"`
	Status       string                 `json:"status"` // pass/warn/block
	Blocked      bool                   `json:"blocked"`
	CanPublish   bool                   `json:"can_publish"`
	BlockReasons []string               `json:"block_reasons"`
	WarnReasons  []string               `json:"warn_reasons"`
	NextAction   string                 `json:"next_action"`
	EvaluatedAt  time.Time              `json:"evaluated_at"`
	Items        []ReleaseGateResultDTO `json:"items"`
}

type ReleaseGateResultDTO struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Status      string         `json:"status"` // pass/warn/block/skip
	Severity    string         `json:"severity"`
	Policy      string         `json:"policy"` // required/advisory/manual
	Blocker     bool           `json:"blocker"`
	Message     string         `json:"message"`
	Detail      map[string]any `json:"detail,omitempty"`
	EvaluatedAt time.Time      `json:"evaluated_at"`
}
