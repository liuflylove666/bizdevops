package dto

import "time"

type ApplicationReadinessResponse struct {
	ApplicationID   uint                         `json:"application_id"`
	ApplicationName string                       `json:"application_name"`
	Score           int                          `json:"score"`
	Level           string                       `json:"level"`
	Completed       int                          `json:"completed"`
	Total           int                          `json:"total"`
	Checks          []ApplicationReadinessCheck  `json:"checks"`
	NextActions     []ApplicationReadinessAction `json:"next_actions"`
	GeneratedAt     time.Time                    `json:"generated_at"`
}

type ApplicationReadinessCheck struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Severity    string `json:"severity"`
	Path        string `json:"path,omitempty"`
}

type ApplicationReadinessAction struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Path   string `json:"path"`
	Weight int    `json:"weight"`
}
