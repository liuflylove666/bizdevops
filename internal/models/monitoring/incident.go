// Package monitoring
//
// incident.go: 生产事故（Incident）持久化模型（v2.1）。
//
// 与 AlertEvent 的区别：
//   - AlertEvent 是来自 Prometheus/Grafana/AWS 的标准化事件流（瞬时）
//   - Incident 是 OnCall 团队认定的"生产事故"（持久化、可关联到发布）
//
// 用途：
//   - MTTR 真实数据源（DORA Sprint 4）
//   - 复盘审计与发布关联
package monitoring

import "time"

// Incident 生产事故。
type Incident struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	Title           string `gorm:"size:200;not null" json:"title"`
	Description     string `gorm:"type:text" json:"description"`
	ApplicationID   *uint  `gorm:"index:idx_incident_app" json:"application_id"`
	ApplicationName string `gorm:"size:100;index:idx_incident_app_name" json:"application_name"`
	Env             string `gorm:"size:30;index:idx_incident_env;default:'prod'" json:"env"`

	// 严重等级 P0(critical) > P1(high) > P2(medium) > P3(low)
	Severity string `gorm:"size:10;index:idx_incident_severity;not null;default:'P2'" json:"severity"`
	// 状态机: open → mitigated → resolved （或直接 open → resolved）
	Status string `gorm:"size:20;index:idx_incident_status;not null;default:'open'" json:"status"`

	DetectedAt  time.Time  `gorm:"not null;index:idx_incident_detected" json:"detected_at"`
	MitigatedAt *time.Time `json:"mitigated_at"`
	ResolvedAt  *time.Time `gorm:"index:idx_incident_resolved" json:"resolved_at"`

	// 来源与关联
	Source           string `gorm:"size:30;default:'manual'" json:"source"` // manual / alert / release_failure
	ReleaseID        *uint  `gorm:"index:idx_incident_release" json:"release_id"`
	AlertFingerprint string `gorm:"size:100;index:idx_incident_fingerprint" json:"alert_fingerprint"`

	// 复盘
	PostmortemURL string `gorm:"size:500" json:"postmortem_url"`
	RootCause     string `gorm:"type:text" json:"root_cause"`

	// 操作人
	CreatedBy      uint   `json:"created_by"`
	CreatedByName  string `gorm:"size:100" json:"created_by_name"`
	ResolvedBy     *uint  `json:"resolved_by"`
	ResolvedByName string `gorm:"size:100" json:"resolved_by_name"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (Incident) TableName() string { return "incidents" }

// 状态枚举
const (
	IncidentStatusOpen      = "open"
	IncidentStatusMitigated = "mitigated"
	IncidentStatusResolved  = "resolved"
)

// 严重等级枚举
const (
	IncidentSeverityP0 = "P0" // 全站宕机
	IncidentSeverityP1 = "P1" // 核心功能不可用
	IncidentSeverityP2 = "P2" // 部分功能受影响
	IncidentSeverityP3 = "P3" // 体验问题
)

// 来源枚举
const (
	IncidentSourceManual          = "manual"
	IncidentSourceAlert           = "alert"
	IncidentSourceReleaseFailure  = "release_failure"
)

// IsValidSeverity 校验严重等级。
func IsValidSeverity(s string) bool {
	switch s {
	case IncidentSeverityP0, IncidentSeverityP1, IncidentSeverityP2, IncidentSeverityP3:
		return true
	}
	return false
}

// IsValidStatus 校验状态。
func IsValidStatus(s string) bool {
	switch s {
	case IncidentStatusOpen, IncidentStatusMitigated, IncidentStatusResolved:
		return true
	}
	return false
}

// MTTRMinutes 返回从发现到 resolved 的分钟数；未 resolved 返回 0。
func (i *Incident) MTTRMinutes() float64 {
	if i.ResolvedAt == nil {
		return 0
	}
	return i.ResolvedAt.Sub(i.DetectedAt).Minutes()
}
