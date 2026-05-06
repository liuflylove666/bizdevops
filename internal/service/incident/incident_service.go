// Package incident
//
// incident_service.go: 生产事故业务逻辑（v2.1）。
//
// 状态机：
//   open → mitigated → resolved
//   open → resolved（轻量直达，跳过 mitigated）
//
// 设计要点：
//   - Detect 时强制要求 detected_at；未传则取 now
//   - Resolve 自动写 resolved_at + resolved_by/_name
//   - 接入 DORA：resolved 后 MTTR 即可计算
package incident

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"devops/internal/models/monitoring"
	monRepo "devops/internal/modules/monitoring/repository"
)

// Service 事故服务。
type Service struct {
	repo *monRepo.IncidentRepository
}

// NewService 构造服务。
func NewService(repo *monRepo.IncidentRepository) *Service {
	return &Service{repo: repo}
}

// CreateInput 创建参数。
type CreateInput struct {
	Title           string
	Description     string
	ApplicationID   *uint
	ApplicationName string
	Env             string
	Severity        string
	DetectedAt      *time.Time
	Source          string
	ReleaseID       *uint
	AlertFingerprint string
	CreatedBy       uint
	CreatedByName   string
}

// Create 创建一条事故（默认 status=open）。
func (s *Service) Create(in *CreateInput) (*monitoring.Incident, error) {
	if in == nil || strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("title 必填")
	}
	severity := in.Severity
	if severity == "" {
		severity = monitoring.IncidentSeverityP2
	}
	if !monitoring.IsValidSeverity(severity) {
		return nil, fmt.Errorf("severity 不合法: %s", severity)
	}
	env := in.Env
	if env == "" {
		env = "prod"
	}
	source := in.Source
	if source == "" {
		source = monitoring.IncidentSourceManual
	}
	detectedAt := time.Now()
	if in.DetectedAt != nil {
		detectedAt = *in.DetectedAt
	}

	inc := &monitoring.Incident{
		Title:            strings.TrimSpace(in.Title),
		Description:      in.Description,
		ApplicationID:    in.ApplicationID,
		ApplicationName:  in.ApplicationName,
		Env:              env,
		Severity:         severity,
		Status:           monitoring.IncidentStatusOpen,
		DetectedAt:       detectedAt,
		Source:           source,
		ReleaseID:        in.ReleaseID,
		AlertFingerprint: in.AlertFingerprint,
		CreatedBy:        in.CreatedBy,
		CreatedByName:    in.CreatedByName,
	}
	if err := s.repo.Create(inc); err != nil {
		return nil, err
	}
	return inc, nil
}

// Mitigate 标记止血（不改变最终 resolved 状态）。
func (s *Service) Mitigate(id uint, _operatorID uint) (*monitoring.Incident, error) {
	inc, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if inc.Status == monitoring.IncidentStatusResolved {
		return nil, errors.New("已 resolved，不能再 mitigate")
	}
	now := time.Now()
	inc.Status = monitoring.IncidentStatusMitigated
	if inc.MitigatedAt == nil {
		inc.MitigatedAt = &now
	}
	if err := s.repo.Update(inc); err != nil {
		return nil, err
	}
	return inc, nil
}

// ResolveInput 解决参数。
type ResolveInput struct {
	RootCause      string
	PostmortemURL  string
	ResolvedBy     uint
	ResolvedByName string
}

// Resolve 标记彻底解决，写入 resolved_at 与处理人。
func (s *Service) Resolve(id uint, in *ResolveInput) (*monitoring.Incident, error) {
	inc, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if inc.Status == monitoring.IncidentStatusResolved {
		return inc, nil
	}
	now := time.Now()
	inc.Status = monitoring.IncidentStatusResolved
	inc.ResolvedAt = &now
	if in != nil {
		if in.RootCause != "" {
			inc.RootCause = in.RootCause
		}
		if in.PostmortemURL != "" {
			inc.PostmortemURL = in.PostmortemURL
		}
		if in.ResolvedBy > 0 {
			rid := in.ResolvedBy
			inc.ResolvedBy = &rid
			inc.ResolvedByName = in.ResolvedByName
		}
	}
	if inc.MitigatedAt == nil {
		inc.MitigatedAt = &now
	}
	if err := s.repo.Update(inc); err != nil {
		return nil, err
	}
	return inc, nil
}

// List 列表。
func (s *Service) List(f monRepo.IncidentFilter, page, pageSize int) ([]monitoring.Incident, int64, error) {
	return s.repo.List(f, page, pageSize)
}

// GetByID 详情。
func (s *Service) GetByID(id uint) (*monitoring.Incident, error) {
	return s.repo.GetByID(id)
}

// Delete 删除。
func (s *Service) Delete(id uint) error {
	return s.repo.Delete(id)
}

// ListResolvedInWindow 暴露给 DORA aggregator 的辅助查询。
func (s *Service) ListResolvedInWindow(ctx context.Context, env string, from, to time.Time) ([]monitoring.Incident, error) {
	return s.repo.ListResolvedInWindow(ctx, env, from, to)
}

// UpsertFromAlert 依据标准 AlertEvent 创建或更新 Incident（v2.2）。
//
// 行为约定：
//   - event.Status == "firing":
//       - 同 (fingerprint, env) 已有未 resolved 的 Incident → 仅按"就高不就低"提升严重等级
//       - 不存在 → 新建 Incident，source=alert，severity 由 labels.severity 映射
//   - event.Status == "resolved":
//       - 同 (fingerprint, env) 找到未 resolved Incident → 自动 Resolve（resolved_at 取 event.EndsAt 或 now）
//       - 找不到（或已 resolved）→ 无副作用
//
// 返回 (incident, created)：created=true 表示新建，false 表示复用或解决了一条。
func (s *Service) UpsertFromAlert(_ context.Context, event *monitoring.AlertEvent) (*monitoring.Incident, bool, error) {
	if event == nil || event.Fingerprint == "" {
		return nil, false, errors.New("event.fingerprint 必填")
	}
	env := extractEnvFromLabels(event.Labels)
	appName := extractAppFromLabels(event.Labels)
	severity := mapSeverity(event.Level, event.Labels)

	switch strings.ToLower(event.Status) {
	case "resolved", "ok":
		existing, err := s.repo.GetByFingerprint(event.Fingerprint, env)
		if err != nil {
			return nil, false, nil // 未找到即视为无操作
		}
		resolvedAt := time.Now()
		if event.EndsAt != nil {
			resolvedAt = *event.EndsAt
		}
		existing.Status = monitoring.IncidentStatusResolved
		existing.ResolvedAt = &resolvedAt
		if existing.MitigatedAt == nil {
			existing.MitigatedAt = &resolvedAt
		}
		if existing.RootCause == "" {
			existing.RootCause = "（告警自动解决：" + event.Title + "）"
		}
		if err := s.repo.Update(existing); err != nil {
			return nil, false, err
		}
		return existing, false, nil

	default: // firing / active / 空字符串默认视为 firing
		existing, err := s.repo.GetByFingerprint(event.Fingerprint, env)
		if err == nil && existing != nil && existing.ID > 0 {
			// 严重等级就高不就低（P0 > P1 > P2 > P3）
			if severityRank(severity) > severityRank(existing.Severity) {
				existing.Severity = severity
			}
			// 描述追加一行而不是覆盖，便于复盘时看到告警连续出现
			if event.Content != "" {
				existing.Description = appendAlertNote(existing.Description, event)
			}
			if err := s.repo.Update(existing); err != nil {
				return nil, false, err
			}
			return existing, false, nil
		}
		// 新建
		detectedAt := time.Now()
		if !event.StartsAt.IsZero() {
			detectedAt = event.StartsAt
		}
		inc := &monitoring.Incident{
			Title:            truncate(event.Title, 200),
			Description:      event.Content,
			ApplicationName:  appName,
			Env:              env,
			Severity:         severity,
			Status:           monitoring.IncidentStatusOpen,
			DetectedAt:       detectedAt,
			Source:           monitoring.IncidentSourceAlert,
			AlertFingerprint: event.Fingerprint,
			CreatedByName:    "system/alert",
		}
		if err := s.repo.Create(inc); err != nil {
			return nil, false, err
		}
		return inc, true, nil
	}
}

// ---------- helpers: label mapping ----------

// extractEnvFromLabels 从告警标签推断环境，缺省 prod。
func extractEnvFromLabels(labels map[string]string) string {
	if labels == nil {
		return "prod"
	}
	for _, k := range []string{"env", "environment", "stage"} {
		if v, ok := labels[k]; ok && v != "" {
			return v
		}
	}
	return "prod"
}

// extractAppFromLabels 从标签中提取应用名（多源兼容）。
func extractAppFromLabels(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	for _, k := range []string{"application", "app", "service", "service_name", "job"} {
		if v, ok := labels[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// mapSeverity 综合 event.Level + labels.severity 映射到 P0-P3。
//
// 规则：labels.severity 优先级最高（显式指定），其次由 level 自动映射：
//   critical → P0；error → P1；warning → P2；其余 → P3
func mapSeverity(level string, labels map[string]string) string {
	if labels != nil {
		if s := strings.ToUpper(strings.TrimSpace(labels["severity"])); s != "" {
			if monitoring.IsValidSeverity(s) {
				return s
			}
		}
	}
	switch strings.ToLower(level) {
	case "critical", "fatal", "emergency":
		return monitoring.IncidentSeverityP0
	case "error", "high":
		return monitoring.IncidentSeverityP1
	case "warning", "warn", "medium":
		return monitoring.IncidentSeverityP2
	default:
		return monitoring.IncidentSeverityP3
	}
}

// severityRank 严重等级比较用：数值越大越严重。
func severityRank(s string) int {
	switch s {
	case monitoring.IncidentSeverityP0:
		return 4
	case monitoring.IncidentSeverityP1:
		return 3
	case monitoring.IncidentSeverityP2:
		return 2
	case monitoring.IncidentSeverityP3:
		return 1
	}
	return 0
}

func appendAlertNote(existing string, ev *monitoring.AlertEvent) string {
	line := "[" + ev.StartsAt.Format(time.RFC3339) + "] firing: " + ev.Title
	if existing == "" {
		return line
	}
	// 避免无限增长：超过 4KB 截断保留首部
	const maxBody = 4096
	merged := existing + "\n" + line
	if len(merged) > maxBody {
		merged = merged[:maxBody]
	}
	return merged
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
