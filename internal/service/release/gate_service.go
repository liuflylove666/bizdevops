package release

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbmodel "devops/internal/domain/database/model"
	"devops/internal/models/deploy"
	"devops/internal/models/infrastructure"
	"devops/internal/models/system"
	"devops/internal/types"
	"devops/pkg/dto"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GateService struct {
	db *gorm.DB
}

func NewGateService(db *gorm.DB) *GateService {
	return &GateService{db: db}
}

func (s *GateService) Evaluate(ctx context.Context, releaseID uint, persist bool) (*dto.ReleaseGateSummaryDTO, error) {
	var rel deploy.Release
	if err := s.db.WithContext(ctx).First(&rel, releaseID).Error; err != nil {
		return nil, err
	}

	var items []deploy.ReleaseItem
	if err := s.db.WithContext(ctx).Where("release_id = ?", rel.ID).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	results := []dto.ReleaseGateResultDTO{
		s.changeItemsGate(rel, items, now),
		s.approvalGate(ctx, rel, now),
		s.deployWindowGate(ctx, rel, now),
		s.deployLockGate(ctx, rel, now),
		s.riskGate(rel, now),
		s.securityGate(ctx, rel, items, now),
		s.databaseGate(ctx, items, now),
		s.nacosGate(ctx, rel, items, now),
		s.gitopsPRGate(ctx, rel, now),
		s.argoCDGate(ctx, rel, now),
	}

	summary := buildGateSummary(rel.ID, results, now)
	if persist {
		if err := s.persist(ctx, rel.ID, results, now); err != nil {
			return nil, err
		}
	}
	return summary, nil
}

func (s *GateService) changeItemsGate(rel deploy.Release, items []deploy.ReleaseItem, now time.Time) dto.ReleaseGateResultDTO {
	if len(items) == 0 {
		return blockGate("change_items", "变更项完整性", "change", "high", "required", "发布单尚未关联流水线、配置、数据库或其他变更项", now, map[string]any{
			"count": 0,
		})
	}
	counts := map[string]int{}
	for _, item := range items {
		counts[item.ItemType]++
	}
	return passGate("change_items", "变更项完整性", "change", "已关联 "+strconv.Itoa(len(items))+" 个变更项", now, map[string]any{
		"count": len(items),
		"types": counts,
	})
}

func (s *GateService) approvalGate(ctx context.Context, rel deploy.Release, now time.Time) dto.ReleaseGateResultDTO {
	if rel.Status == deploy.ReleaseStatusDraft {
		return blockGate("approval", "审批状态", "governance", "high", "required", "发布单仍为草稿，需先提交审批", now, nil)
	}
	if rel.Status == deploy.ReleaseStatusRejected {
		return blockGate("approval", "审批状态", "governance", "high", "required", firstNonEmpty(rel.RejectReason, "审批已驳回"), now, nil)
	}
	if rel.ApprovalInstanceID != nil && *rel.ApprovalInstanceID > 0 {
		var approval deploy.ApprovalInstance
		if err := s.db.WithContext(ctx).First(&approval, *rel.ApprovalInstanceID).Error; err == nil {
			switch approval.Status {
			case "approved":
				return passGate("approval", "审批状态", "governance", "审批已通过", now, map[string]any{
					"approval_instance_id": approval.ID,
					"chain_name":           approval.ChainName,
				})
			case "rejected", "cancelled":
				return blockGate("approval", "审批状态", "governance", "high", "required", "审批状态为 "+approval.Status, now, map[string]any{
					"approval_instance_id": approval.ID,
					"chain_name":           approval.ChainName,
				})
			default:
				return blockGate("approval", "审批状态", "governance", "high", "required", "审批仍在处理中", now, map[string]any{
					"approval_instance_id": approval.ID,
					"chain_name":           approval.ChainName,
					"current_node_order":   approval.CurrentNodeOrder,
				})
			}
		}
	}
	if rel.Status == deploy.ReleaseStatusPendingApproval {
		return blockGate("approval", "审批状态", "governance", "high", "required", "等待审批通过", now, nil)
	}
	if rel.ApprovedAt != nil || rel.Status == deploy.ReleaseStatusApproved || rel.Status == deploy.ReleaseStatusPROpened || rel.Status == deploy.ReleaseStatusPRMerged || rel.Status == deploy.ReleaseStatusPublishing || rel.Status == deploy.ReleaseStatusPublished {
		return passGate("approval", "审批状态", "governance", "审批已通过", now, map[string]any{
			"approved_at": rel.ApprovedAt,
		})
	}
	return warnGate("approval", "审批状态", "governance", "medium", "manual", "未发现明确审批实例，请确认该环境是否允许免审批", now, nil)
}

func (s *GateService) deployWindowGate(ctx context.Context, rel deploy.Release, now time.Time) dto.ReleaseGateResultDTO {
	var policy deploy.EnvAuditPolicy
	requiresWindow := false
	if err := s.db.WithContext(ctx).Where("enabled = ? AND env_name = ?", true, rel.Env).First(&policy).Error; err == nil {
		requiresWindow = policy.RequireDeployWindow || policy.AutoRejectOutside
	}

	var windows []deploy.DeployWindow
	q := s.db.WithContext(ctx).Where("enabled = ? AND env = ?", true, rel.Env)
	if rel.ApplicationID != nil && *rel.ApplicationID > 0 {
		q = q.Where("app_id = 0 OR app_id = ?", *rel.ApplicationID)
	} else {
		q = q.Where("app_id = 0")
	}
	if err := q.Find(&windows).Error; err != nil {
		windows = nil
	}
	if len(windows) == 0 {
		if requiresWindow {
			return blockGate("deploy_window", "发布窗口", "governance", "high", "required", "当前环境要求发布窗口，但未配置可用窗口", now, nil)
		}
		return warnGate("deploy_window", "发布窗口", "governance", "low", "advisory", "未配置发布窗口，按观察模式放行", now, nil)
	}
	for _, window := range windows {
		if windowAllows(now, window) {
			return passGate("deploy_window", "发布窗口", "governance", "当前处于允许发布窗口", now, map[string]any{
				"window_id":  window.ID,
				"weekdays":   window.Weekdays,
				"start_time": window.StartTime,
				"end_time":   window.EndTime,
			})
		}
	}
	msg := "当前不在发布窗口内"
	if requiresWindow {
		return blockGate("deploy_window", "发布窗口", "governance", "high", "required", msg, now, map[string]any{"windows": len(windows)})
	}
	return warnGate("deploy_window", "发布窗口", "governance", "medium", "advisory", msg, now, map[string]any{"windows": len(windows)})
}

func (s *GateService) deployLockGate(ctx context.Context, rel deploy.Release, now time.Time) dto.ReleaseGateResultDTO {
	if rel.ApplicationID == nil || *rel.ApplicationID == 0 {
		return warnGate("deploy_lock", "部署锁", "governance", "low", "advisory", "发布单未关联应用，无法判断部署锁", now, nil)
	}
	var locks []deploy.DeployLock
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND env_name = ? AND status = ? AND expires_at > ?", *rel.ApplicationID, rel.Env, "active", now).
		Order("expires_at DESC").
		Find(&locks).Error
	if err != nil || len(locks) == 0 {
		return passGate("deploy_lock", "部署锁", "governance", "无有效部署锁", now, nil)
	}
	lock := locks[0]
	return blockGate("deploy_lock", "部署锁", "governance", "high", "required", "当前应用环境存在有效部署锁", now, map[string]any{
		"lock_id":        lock.ID,
		"locked_by_name": lock.LockedByName,
		"expires_at":     lock.ExpiresAt,
	})
}

func (s *GateService) riskGate(rel deploy.Release, now time.Time) dto.ReleaseGateResultDTO {
	detail := map[string]any{
		"risk_score": rel.RiskScore,
		"risk_level": rel.RiskLevel,
	}
	if rel.RiskScore >= 80 || rel.RiskLevel == "critical" {
		return blockGate("risk", "风险评分", "risk", "critical", "required", "风险评分达到强阻断阈值", now, detail)
	}
	if rel.RiskScore >= 60 || rel.RiskLevel == "high" {
		return warnGate("risk", "风险评分", "risk", "high", "manual", "风险较高，建议确认回滚方案和审批上下文", now, detail)
	}
	return passGate("risk", "风险评分", "risk", "风险评分可接受", now, detail)
}

func (s *GateService) securityGate(ctx context.Context, rel deploy.Release, items []deploy.ReleaseItem, now time.Time) dto.ReleaseGateResultDTO {
	q := s.db.WithContext(ctx).Model(&system.ImageScan{}).
		Where("status = ?", "completed")
	if rel.ApplicationID != nil && *rel.ApplicationID > 0 {
		q = q.Where("application_id = ?", *rel.ApplicationID)
	} else if strings.TrimSpace(rel.ApplicationName) != "" {
		q = q.Where("application_name = ?", rel.ApplicationName)
	} else {
		runIDs := pipelineRunItemIDs(items)
		if len(runIDs) == 0 {
			return warnGate("security_scan", "安全扫描", "security", "low", "advisory", "未关联应用或流水线运行，无法匹配镜像扫描结果", now, nil)
		}
		q = q.Where("pipeline_run_id IN ?", runIDs)
	}
	var scan system.ImageScan
	if err := q.Order("created_at DESC").First(&scan).Error; err != nil {
		return warnGate("security_scan", "安全扫描", "security", "medium", "advisory", "未找到最近一次镜像扫描结果", now, nil)
	}
	detail := map[string]any{
		"scan_id":        scan.ID,
		"image":          scan.Image,
		"risk_level":     scan.RiskLevel,
		"critical_count": scan.CriticalCount,
		"high_count":     scan.HighCount,
		"created_at":     scan.CreatedAt,
	}
	if scan.CriticalCount > 0 || scan.RiskLevel == "critical" {
		return blockGate("security_scan", "安全扫描", "security", "critical", "required", "存在严重镜像漏洞", now, detail)
	}
	if scan.HighCount > 0 || scan.RiskLevel == "high" {
		return warnGate("security_scan", "安全扫描", "security", "high", "manual", "存在高危镜像漏洞", now, detail)
	}
	return passGate("security_scan", "安全扫描", "security", "最近一次镜像扫描无高危阻断项", now, detail)
}

func (s *GateService) databaseGate(ctx context.Context, items []deploy.ReleaseItem, now time.Time) dto.ReleaseGateResultDTO {
	ticketIDs := releaseItemIDsByType(items, deploy.ReleaseItemTypeSQLTicket, deploy.ReleaseItemTypeDatabase)
	if len(ticketIDs) == 0 {
		return skipGate("database_ticket", "数据库工单", "change", "发布单未关联数据库变更", now, nil)
	}
	var tickets []dbmodel.SQLChangeTicket
	if err := s.db.WithContext(ctx).Where("id IN ?", ticketIDs).Find(&tickets).Error; err != nil || len(tickets) == 0 {
		return blockGate("database_ticket", "数据库工单", "change", "high", "required", "发布单关联了数据库变更，但未找到对应工单", now, map[string]any{"ticket_ids": ticketIDs})
	}
	for _, ticket := range tickets {
		if ticket.Status != dbmodel.TicketStatusSucceeded {
			return blockGate("database_ticket", "数据库工单", "change", "high", "required", "存在未执行成功的数据库工单", now, map[string]any{
				"ticket_id": ticket.ID,
				"work_id":   ticket.WorkID,
				"status":    ticket.Status,
			})
		}
	}
	return passGate("database_ticket", "数据库工单", "change", "数据库工单均已执行成功", now, map[string]any{"count": len(tickets)})
}

func (s *GateService) nacosGate(ctx context.Context, rel deploy.Release, items []deploy.ReleaseItem, now time.Time) dto.ReleaseGateResultDTO {
	ids := releaseItemIDsByType(items, deploy.ReleaseItemTypeNacosRelease)
	var releases []deploy.NacosRelease
	q := s.db.WithContext(ctx).Model(&deploy.NacosRelease{})
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	} else {
		q = q.Where("release_id = ?", rel.ID)
	}
	_ = q.Find(&releases).Error
	if len(releases) == 0 {
		return skipGate("nacos_release", "Nacos 配置变更", "change", "发布单未关联 Nacos 配置变更", now, nil)
	}
	for _, item := range releases {
		if item.Status != "published" {
			return warnGate("nacos_release", "Nacos 配置变更", "change", "medium", "manual", "存在未发布的 Nacos 配置变更", now, map[string]any{
				"nacos_release_id": item.ID,
				"status":           item.Status,
				"title":            item.Title,
			})
		}
	}
	return passGate("nacos_release", "Nacos 配置变更", "change", "Nacos 配置变更均已发布", now, map[string]any{"count": len(releases)})
}

func (s *GateService) gitopsPRGate(ctx context.Context, rel deploy.Release, now time.Time) dto.ReleaseGateResultDTO {
	if rel.GitOpsChangeRequestID == nil || *rel.GitOpsChangeRequestID == 0 {
		if rel.Status == deploy.ReleaseStatusApproved || rel.Status == deploy.ReleaseStatusPRMerged || rel.Status == deploy.ReleaseStatusPROpened {
			return blockGate("gitops_pr", "GitOps PR", "gitops", "high", "required", "审批已通过，但尚未生成 GitOps PR", now, nil)
		}
		return warnGate("gitops_pr", "GitOps PR", "gitops", "medium", "manual", "尚未生成 GitOps PR", now, nil)
	}
	var change infrastructure.GitOpsChangeRequest
	if err := s.db.WithContext(ctx).First(&change, *rel.GitOpsChangeRequestID).Error; err != nil {
		return blockGate("gitops_pr", "GitOps PR", "gitops", "high", "required", "GitOps PR 记录不存在", now, map[string]any{"change_request_id": *rel.GitOpsChangeRequestID})
	}
	detail := map[string]any{
		"change_request_id": change.ID,
		"status":            change.Status,
		"approval_status":   change.ApprovalStatus,
		"auto_merge_status": change.AutoMergeStatus,
		"mr_url":            change.MergeRequestURL,
	}
	if change.ErrorMessage != "" || change.Status == "failed" || change.AutoMergeStatus == "failed" {
		return blockGate("gitops_pr", "GitOps PR", "gitops", "high", "required", firstNonEmpty(change.ErrorMessage, "GitOps PR 处理失败"), now, detail)
	}
	if change.Status == "merged" || rel.Status == deploy.ReleaseStatusPRMerged || rel.Status == deploy.ReleaseStatusPublishing || rel.Status == deploy.ReleaseStatusPublished {
		return passGate("gitops_pr", "GitOps PR", "gitops", "GitOps PR 已合并或已进入同步阶段", now, detail)
	}
	return blockGate("gitops_pr", "GitOps PR", "gitops", "high", "required", "GitOps PR 尚未合并", now, detail)
}

func (s *GateService) argoCDGate(ctx context.Context, rel deploy.Release, now time.Time) dto.ReleaseGateResultDTO {
	requiresSyncedHealthy := s.argoRequiresSyncedHealthy(ctx, rel)
	app := s.resolveArgoApp(ctx, rel)
	if app == nil {
		if requiresSyncedHealthy {
			return blockGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "high", "required", "GitOps PR 已合并，但未找到关联 ArgoCD 应用", now, nil)
		}
		return warnGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "medium", "manual", "未找到关联 ArgoCD 应用", now, nil)
	}
	detail := map[string]any{
		"argocd_application_id": app.ID,
		"app_name":              app.Name,
		"sync_status":           app.SyncStatus,
		"health_status":         app.HealthStatus,
		"drift_detected":        app.DriftDetected,
		"last_sync_at":          app.LastSyncAt,
	}
	if app.DriftDetected {
		return blockGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "high", "required", "ArgoCD 检测到漂移", now, detail)
	}
	if app.HealthStatus == "Degraded" || app.HealthStatus == "Missing" {
		return blockGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "high", "required", "ArgoCD 应用健康状态异常", now, detail)
	}
	if app.SyncStatus != "Synced" {
		if requiresSyncedHealthy {
			return blockGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "high", "required", "GitOps PR 已合并，但 ArgoCD 尚未同步到期望状态", now, detail)
		}
		return warnGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "medium", "manual", "ArgoCD 尚未同步到期望状态", now, detail)
	}
	if app.HealthStatus != "" && app.HealthStatus != "Healthy" {
		if requiresSyncedHealthy {
			return blockGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "high", "required", "GitOps PR 已合并，但 ArgoCD 应用尚未达到 Healthy", now, detail)
		}
		return warnGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "medium", "manual", "ArgoCD 应用尚未达到 Healthy", now, detail)
	}
	return passGate("argocd_sync", "ArgoCD Sync / Health / Drift", "gitops", "ArgoCD 已同步且未发现漂移", now, detail)
}

func (s *GateService) argoRequiresSyncedHealthy(ctx context.Context, rel deploy.Release) bool {
	switch rel.Status {
	case deploy.ReleaseStatusPRMerged, deploy.ReleaseStatusPublishing, deploy.ReleaseStatusPublished:
		return true
	}
	if rel.GitOpsChangeRequestID == nil || *rel.GitOpsChangeRequestID == 0 {
		return false
	}
	var change infrastructure.GitOpsChangeRequest
	if err := s.db.WithContext(ctx).First(&change, *rel.GitOpsChangeRequestID).Error; err != nil {
		return false
	}
	return change.Status == "merged"
}

func (s *GateService) resolveArgoApp(ctx context.Context, rel deploy.Release) *infrastructure.ArgoCDApplication {
	if rel.ArgoAppName != "" {
		var app infrastructure.ArgoCDApplication
		if err := s.db.WithContext(ctx).Where("name = ?", rel.ArgoAppName).Order("updated_at DESC").First(&app).Error; err == nil {
			return &app
		}
	}
	if rel.ApplicationID != nil && *rel.ApplicationID > 0 {
		var app infrastructure.ArgoCDApplication
		err := s.db.WithContext(ctx).
			Where("application_id = ? AND (env = ? OR env = '')", *rel.ApplicationID, rel.Env).
			Order(gorm.Expr("CASE WHEN env = ? THEN 0 ELSE 1 END", rel.Env)).
			Order("updated_at DESC").
			First(&app).Error
		if err == nil {
			return &app
		}
	}
	return nil
}

func (s *GateService) persist(ctx context.Context, releaseID uint, results []dto.ReleaseGateResultDTO, now time.Time) error {
	rows := make([]deploy.ReleaseGateResult, 0, len(results))
	for _, item := range results {
		rows = append(rows, deploy.ReleaseGateResult{
			ReleaseID:   releaseID,
			GateKey:     item.Key,
			GateName:    item.Name,
			Category:    item.Category,
			Status:      item.Status,
			Severity:    item.Severity,
			Policy:      item.Policy,
			Blocker:     item.Blocker,
			Message:     item.Message,
			Detail:      types.JSONMap(item.Detail),
			EvaluatedAt: now,
		})
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "release_id"}, {Name: "gate_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"gate_name", "category", "status", "severity", "policy", "blocker", "message", "detail", "evaluated_at", "updated_at",
		}),
	}).Create(&rows).Error
}

func buildGateSummary(releaseID uint, items []dto.ReleaseGateResultDTO, now time.Time) *dto.ReleaseGateSummaryDTO {
	summary := &dto.ReleaseGateSummaryDTO{
		ReleaseID:   releaseID,
		Status:      "pass",
		CanPublish:  true,
		EvaluatedAt: now,
		Items:       items,
	}
	for _, item := range items {
		if item.Blocker {
			summary.Blocked = true
			summary.CanPublish = false
			summary.Status = "block"
			summary.BlockReasons = append(summary.BlockReasons, item.Message)
			continue
		}
		if item.Status == "warn" && summary.Status != "block" {
			summary.Status = "warn"
			summary.WarnReasons = append(summary.WarnReasons, item.Message)
		}
	}
	if len(summary.BlockReasons) > 0 {
		summary.NextAction = summary.BlockReasons[0]
	} else if len(summary.WarnReasons) > 0 {
		summary.NextAction = "确认 Gate 提醒后继续发布"
	} else {
		summary.NextAction = "Gate 已通过，可以继续发布"
	}
	return summary
}

func passGate(key, name, category, msg string, now time.Time, detail map[string]any) dto.ReleaseGateResultDTO {
	return dto.ReleaseGateResultDTO{Key: key, Name: name, Category: category, Status: "pass", Severity: "info", Policy: "required", Message: msg, Detail: detail, EvaluatedAt: now}
}

func warnGate(key, name, category, severity, policy, msg string, now time.Time, detail map[string]any) dto.ReleaseGateResultDTO {
	return dto.ReleaseGateResultDTO{Key: key, Name: name, Category: category, Status: "warn", Severity: severity, Policy: policy, Message: msg, Detail: detail, EvaluatedAt: now}
}

func blockGate(key, name, category, severity, policy, msg string, now time.Time, detail map[string]any) dto.ReleaseGateResultDTO {
	return dto.ReleaseGateResultDTO{Key: key, Name: name, Category: category, Status: "block", Severity: severity, Policy: policy, Blocker: true, Message: msg, Detail: detail, EvaluatedAt: now}
}

func skipGate(key, name, category, msg string, now time.Time, detail map[string]any) dto.ReleaseGateResultDTO {
	return dto.ReleaseGateResultDTO{Key: key, Name: name, Category: category, Status: "skip", Severity: "info", Policy: "advisory", Message: msg, Detail: detail, EvaluatedAt: now}
}

func releaseItemIDsByType(items []deploy.ReleaseItem, types ...string) []uint {
	typeSet := map[string]struct{}{}
	for _, itemType := range types {
		typeSet[itemType] = struct{}{}
	}
	ids := make([]uint, 0)
	for _, item := range items {
		if _, ok := typeSet[item.ItemType]; ok && item.ItemID > 0 {
			ids = append(ids, item.ItemID)
		}
	}
	return ids
}

func pipelineRunItemIDs(items []deploy.ReleaseItem) []uint {
	return releaseItemIDsByType(items, deploy.ReleaseItemTypePipelineRun)
}

func windowAllows(now time.Time, window deploy.DeployWindow) bool {
	if !weekdayAllowed(now, window.Weekdays) {
		return false
	}
	start, err1 := parseClock(window.StartTime)
	end, err2 := parseClock(window.EndTime)
	if err1 != nil || err2 != nil {
		return true
	}
	mins := now.Hour()*60 + now.Minute()
	if start <= end {
		return mins >= start && mins <= end
	}
	return mins >= start || mins <= end
}

func weekdayAllowed(now time.Time, weekdays string) bool {
	if strings.TrimSpace(weekdays) == "" {
		return true
	}
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	needle := strconv.Itoa(weekday)
	for _, part := range strings.Split(weekdays, ",") {
		if strings.TrimSpace(part) == needle {
			return true
		}
	}
	return false
}

func parseClock(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid time")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	return hour*60 + minute, nil
}
