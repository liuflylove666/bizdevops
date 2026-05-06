package workspace

import (
	"context"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/models/infrastructure"
	"devops/internal/models/monitoring"
	"devops/pkg/dto"
)

type ActionService struct {
	db *gorm.DB
}

func NewActionService(db *gorm.DB) *ActionService {
	return &ActionService{db: db}
}

func (s *ActionService) List(ctx context.Context, limit int, projectID *uint) (*dto.WorkspaceActionsResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}

	items := make([]dto.ActionItemDTO, 0, limit)
	items = append(items, s.pendingApprovals(ctx, projectID, 10)...)
	items = append(items, s.blockedReleases(ctx, projectID, 12)...)
	items = append(items, s.failedPipelineRuns(ctx, projectID, 8)...)
	items = append(items, s.gitopsDrift(ctx, projectID, 8)...)
	items = append(items, s.openIncidents(ctx, projectID, 8)...)
	items = append(items, s.securityRisks(ctx, projectID, 8)...)

	sort.SliceStable(items, func(i, j int) bool {
		pi, pj := priorityRank(items[i].Priority), priorityRank(items[j].Priority)
		if pi != pj {
			return pi > pj
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}

	summary := map[string]int{}
	groups := map[string][]string{
		"delivery":   {},
		"operations": {},
		"platform":   {},
	}
	for _, item := range items {
		summary[item.Type]++
		summary["total"]++
		for _, group := range actionGroups(item.Type) {
			groups[group] = append(groups[group], item.ID)
		}
	}

	return &dto.WorkspaceActionsResponse{
		Items:   items,
		Summary: summary,
		Groups:  groups,
		Meta: dto.WorkspaceActionsMeta{
			Limit:       limit,
			GeneratedAt: time.Now(),
		},
	}, nil
}

func (s *ActionService) pendingApprovals(ctx context.Context, projectID *uint, limit int) []dto.ActionItemDTO {
	var rows []models.ApprovalInstance
	q := s.db.WithContext(ctx).
		Model(&models.ApprovalInstance{}).
		Where("status = ?", "pending")
	if projectID != nil {
		q = q.Joins("JOIN releases ON releases.approval_instance_id = approval_instances.id").
			Where("releases.project_id = ?", *projectID)
	}
	if err := q.Order("approval_instances.created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil
	}
	items := make([]dto.ActionItemDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.ActionItemDTO{
			ID:          fmt.Sprintf("approval-%d", row.ID),
			Type:        "approval",
			Title:       fmt.Sprintf("审批待处理：%s", firstNonEmpty(row.ChainName, fmt.Sprintf("审批实例 #%d", row.ID))),
			Description: fmt.Sprintf("当前节点 %d，关联记录 #%d", row.CurrentNodeOrder, row.RecordID),
			Status:      row.Status,
			Priority:    "high",
			Path:        fmt.Sprintf("/approval/instances/%d", row.ID),
			ActionLabel: "去审批",
			CreatedAt:   row.CreatedAt,
			SourceID:    row.ID,
			ProjectID:   projectID,
		})
	}
	return items
}

func (s *ActionService) blockedReleases(ctx context.Context, projectID *uint, limit int) []dto.ActionItemDTO {
	var rows []models.Release
	statuses := []string{"pending_approval", "approved", "pr_opened", "pr_merged", "publishing", "failed"}
	q := s.db.WithContext(ctx).
		Where("status IN ?", statuses)
	if projectID != nil {
		q = q.Where("project_id = ?", *projectID)
	}
	if err := q.Order("updated_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil
	}
	items := make([]dto.ActionItemDTO, 0, len(rows))
	for _, row := range rows {
		priority := "medium"
		if row.Env == "prod" || row.RiskLevel == "high" || row.RiskLevel == "critical" || row.Status == "failed" {
			priority = "high"
		}
		items = append(items, dto.ActionItemDTO{
			ID:          fmt.Sprintf("release-%d", row.ID),
			Type:        "release",
			Title:       fmt.Sprintf("发布待推进：%s", row.Title),
			Description: releaseActionDescription(row),
			Status:      row.Status,
			Priority:    priority,
			Application: firstNonEmpty(row.ApplicationName, "-"),
			Env:         row.Env,
			Path:        fmt.Sprintf("/releases/%d", row.ID),
			ActionLabel: "查看发布",
			CreatedAt:   row.UpdatedAt,
			SourceID:    row.ID,
			ProjectID:   row.ProjectID,
		})
	}
	return items
}

func (s *ActionService) failedPipelineRuns(ctx context.Context, projectID *uint, limit int) []dto.ActionItemDTO {
	var rows []models.PipelineRun
	q := s.db.WithContext(ctx).
		Model(&models.PipelineRun{}).
		Where("status IN ?", []string{"failed", "cancelled"})
	if projectID != nil {
		q = q.Joins("JOIN pipelines ON pipelines.id = pipeline_runs.pipeline_id").
			Where("pipelines.project_id = ?", *projectID)
	}
	if err := q.Order("pipeline_runs.created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil
	}
	items := make([]dto.ActionItemDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.ActionItemDTO{
			ID:          fmt.Sprintf("pipeline-run-%d", row.ID),
			Type:        "pipeline_run",
			Title:       fmt.Sprintf("流水线运行失败：%s", firstNonEmpty(row.PipelineName, fmt.Sprintf("流水线 #%d", row.PipelineID))),
			Description: firstNonEmpty(row.GitMessage, fmt.Sprintf("%s / %s / %s", row.ApplicationName, row.Env, row.GitBranch)),
			Status:      row.Status,
			Priority:    "medium",
			Application: row.ApplicationName,
			Env:         row.Env,
			Path:        fmt.Sprintf("/pipeline/%d?run=%d", row.PipelineID, row.ID),
			ActionLabel: "查看运行",
			CreatedAt:   row.CreatedAt,
			SourceID:    row.ID,
			ProjectID:   projectID,
		})
	}
	return items
}

func (s *ActionService) gitopsDrift(ctx context.Context, projectID *uint, limit int) []dto.ActionItemDTO {
	var rows []infrastructure.ArgoCDApplication
	q := s.db.WithContext(ctx).
		Model(&infrastructure.ArgoCDApplication{}).
		Where("drift_detected = ? OR sync_status IN ? OR health_status IN ?", true, []string{"OutOfSync", "Unknown"}, []string{"Degraded", "Missing", "Unknown"})
	if projectID != nil {
		q = q.Joins("JOIN applications ON applications.id = argocd_applications.application_id").
			Where("applications.project_id = ?", *projectID)
	}
	if err := q.Order("argocd_applications.updated_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil
	}
	items := make([]dto.ActionItemDTO, 0, len(rows))
	for _, row := range rows {
		priority := "medium"
		if row.HealthStatus == "Degraded" || row.HealthStatus == "Missing" {
			priority = "high"
		}
		items = append(items, dto.ActionItemDTO{
			ID:          fmt.Sprintf("argocd-app-%d", row.ID),
			Type:        "gitops_drift",
			Title:       fmt.Sprintf("GitOps 状态异常：%s", row.Name),
			Description: fmt.Sprintf("Sync=%s，Health=%s，Drift=%t", row.SyncStatus, row.HealthStatus, row.DriftDetected),
			Status:      firstNonEmpty(row.SyncStatus, row.HealthStatus),
			Priority:    priority,
			Application: row.ApplicationName,
			Env:         row.Env,
			Path:        gitopsActionPath(projectID),
			ActionLabel: "处理漂移",
			CreatedAt:   row.UpdatedAt,
			SourceID:    row.ID,
			ProjectID:   projectID,
		})
	}
	return items
}

func (s *ActionService) openIncidents(ctx context.Context, projectID *uint, limit int) []dto.ActionItemDTO {
	var rows []monitoring.Incident
	q := s.db.WithContext(ctx).
		Model(&monitoring.Incident{}).
		Where("status IN ?", []string{"open", "mitigated"})
	if projectID != nil {
		q = q.Joins("JOIN applications ON applications.id = incidents.application_id").
			Where("applications.project_id = ?", *projectID)
	}
	if err := q.Order("incidents.detected_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil
	}
	items := make([]dto.ActionItemDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.ActionItemDTO{
			ID:          fmt.Sprintf("incident-%d", row.ID),
			Type:        "incident",
			Title:       fmt.Sprintf("事故待处置：%s", row.Title),
			Description: fmt.Sprintf("%s / %s / 发现于 %s", row.Severity, row.Env, row.DetectedAt.Format("2006-01-02 15:04")),
			Status:      row.Status,
			Priority:    incidentPriority(row.Severity),
			Application: row.ApplicationName,
			Env:         row.Env,
			Path:        fmt.Sprintf("/incidents/%d", row.ID),
			ActionLabel: "进入处置",
			CreatedAt:   row.DetectedAt,
			SourceID:    row.ID,
			ProjectID:   projectID,
		})
	}
	return items
}

func (s *ActionService) securityRisks(ctx context.Context, projectID *uint, limit int) []dto.ActionItemDTO {
	var rows []models.ImageScan
	q := s.db.WithContext(ctx).
		Model(&models.ImageScan{}).
		Where("status = ? AND (risk_level IN ? OR critical_count > 0 OR high_count > 0)", "completed", []string{"critical", "high"})
	if projectID != nil {
		q = q.Joins("JOIN applications ON applications.id = image_scans.application_id").
			Where("applications.project_id = ?", *projectID)
	}
	if err := q.Order("image_scans.created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil
	}
	items := make([]dto.ActionItemDTO, 0, len(rows))
	for _, row := range rows {
		priority := "medium"
		if row.RiskLevel == "critical" || row.CriticalCount > 0 {
			priority = "high"
		}
		items = append(items, dto.ActionItemDTO{
			ID:          fmt.Sprintf("image-scan-%d", row.ID),
			Type:        "security_risk",
			Title:       fmt.Sprintf("镜像高风险：%s", row.Image),
			Description: fmt.Sprintf("严重 %d，高危 %d，风险等级 %s", row.CriticalCount, row.HighCount, firstNonEmpty(row.RiskLevel, "-")),
			Status:      row.RiskLevel,
			Priority:    priority,
			Application: row.ApplicationName,
			Path:        "/security/image-scan",
			ActionLabel: "查看风险",
			CreatedAt:   row.CreatedAt,
			SourceID:    row.ID,
			ProjectID:   projectID,
		})
	}
	return items
}

func gitopsActionPath(projectID *uint) string {
	if projectID == nil {
		return "/argocd?tab=apps"
	}
	return fmt.Sprintf("/argocd?tab=apps&project_id=%d", *projectID)
}

func releaseActionDescription(row models.Release) string {
	switch row.Status {
	case "pending_approval":
		return "等待审批通过后才能继续 GitOps PR"
	case "approved":
		return "审批已通过，下一步生成或确认 GitOps PR"
	case "pr_opened":
		return "GitOps PR 已打开，等待合并"
	case "pr_merged":
		return "PR 已合并，等待 Argo CD 同步"
	case "publishing":
		return "发布执行中，需要观察 Argo CD 与运行态"
	case "failed":
		return firstNonEmpty(row.RejectReason, "发布失败，需要查看详情")
	default:
		return "发布待处理"
	}
}

func incidentPriority(severity string) string {
	switch severity {
	case "P0", "P1":
		return "high"
	case "P2":
		return "medium"
	default:
		return "low"
	}
}

func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func actionGroups(actionType string) []string {
	switch actionType {
	case "approval", "release", "pipeline_run":
		return []string{"delivery"}
	case "gitops_drift", "incident":
		return []string{"operations"}
	case "security_risk":
		return []string{"platform"}
	default:
		return []string{"delivery"}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
