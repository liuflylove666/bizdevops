// Package dto
//
// timeline.go: 事件时间线聚合项（E4-03 / obs.incident_timeline）。
package dto

import "time"

// TimelineItemKind 时间线条目类型。
const (
	TimelineKindIncident    = "incident"
	TimelineKindChangeEvent = "change_event"
	TimelineKindRelease     = "release"
	TimelineKindAlert       = "alert"
	TimelineKindApproval    = "approval"
)

// TimelineItem 统一时间线节点，前端按 at 降序渲染。
type TimelineItem struct {
	Kind     string         `json:"kind"`
	ID       uint           `json:"id"`
	At       time.Time      `json:"at"`
	Title    string         `json:"title"`
	Summary  string         `json:"summary,omitempty"`
	Status   string         `json:"status,omitempty"`
	Severity string         `json:"severity,omitempty"`
	Env      string         `json:"env,omitempty"`
	Ref      string         `json:"ref"` // 前端路由 path，不含 host
	Meta     map[string]any `json:"meta,omitempty"`
}

// TimelineResponse GET /observability/timeline 响应体。
type TimelineResponse struct {
	Items     []TimelineItem `json:"items"`
	From      time.Time      `json:"from"`
	To        time.Time      `json:"to"`
	Truncated bool           `json:"truncated"` // 是否因 limit 截断
}
