package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	appmodel "devops/internal/models/application"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"devops/internal/models"
	"devops/internal/models/infrastructure"
	"devops/pkg/dto"
)

type ReadinessService struct {
	db *gorm.DB
}

func NewReadinessService(db *gorm.DB) *ReadinessService {
	return &ReadinessService{db: db}
}

func (s *ReadinessService) Get(ctx context.Context, appID uint) (*dto.ApplicationReadinessResponse, error) {
	return s.evaluate(ctx, appID)
}

func (s *ReadinessService) Refresh(ctx context.Context, appID uint) (*dto.ApplicationReadinessResponse, error) {
	result, err := s.evaluate(ctx, appID)
	if err != nil {
		return nil, err
	}
	if err := s.persist(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ReadinessService) evaluate(ctx context.Context, appID uint) (*dto.ApplicationReadinessResponse, error) {
	var app models.Application
	if err := s.db.WithContext(ctx).First(&app, appID).Error; err != nil {
		return nil, err
	}

	appPath := fmt.Sprintf("/applications/%d", app.ID)
	checks := []dto.ApplicationReadinessCheck{
		s.checkProfile(app, appPath+"#info"),
		s.checkRepo(ctx, app, appPath+"#repos"),
		s.checkEnvs(ctx, app, appPath+"#envs"),
		s.checkPipelines(ctx, app, appPath+"#delivery"),
		s.checkGitOps(ctx, app, "/argocd?tab=apps"),
		s.checkGovernance(ctx, app, "/approval/env-policies"),
		s.checkRuntime(ctx, app, appPath+"#envs"),
	}

	completed := 0
	actions := make([]dto.ApplicationReadinessAction, 0)
	for _, check := range checks {
		if check.Status == "pass" {
			completed++
			continue
		}
		actions = append(actions, dto.ApplicationReadinessAction{
			Key:    check.Key,
			Title:  check.Title,
			Path:   check.Path,
			Weight: actionWeight(check.Severity),
		})
	}

	total := len(checks)
	score := 0
	if total > 0 {
		score = int(float64(completed) / float64(total) * 100)
	}

	return &dto.ApplicationReadinessResponse{
		ApplicationID:   app.ID,
		ApplicationName: app.DisplayName,
		Score:           score,
		Level:           readinessLevel(score),
		Completed:       completed,
		Total:           total,
		Checks:          checks,
		NextActions:     actions,
		GeneratedAt:     time.Now(),
	}, nil
}

func (s *ReadinessService) persist(ctx context.Context, result *dto.ApplicationReadinessResponse) error {
	if result == nil || len(result.Checks) == 0 {
		return nil
	}
	rows := make([]appmodel.ApplicationReadinessCheck, 0, len(result.Checks))
	for _, check := range result.Checks {
		rows = append(rows, appmodel.ApplicationReadinessCheck{
			ApplicationID: result.ApplicationID,
			CheckKey:      check.Key,
			Title:         check.Title,
			Description:   check.Description,
			Status:        check.Status,
			Severity:      check.Severity,
			Path:          check.Path,
			Score:         result.Score,
			Level:         result.Level,
			CheckedAt:     result.GeneratedAt,
		})
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "application_id"}, {Name: "check_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title",
			"description",
			"status",
			"severity",
			"path",
			"score",
			"level",
			"checked_at",
			"updated_at",
		}),
	}).Create(&rows).Error
}

func (s *ReadinessService) checkProfile(app models.Application, path string) dto.ApplicationReadinessCheck {
	missing := make([]string, 0)
	if strings.TrimSpace(app.Name) == "" {
		missing = append(missing, "应用名称")
	}
	if strings.TrimSpace(app.Team) == "" {
		missing = append(missing, "团队")
	}
	if strings.TrimSpace(app.Owner) == "" {
		missing = append(missing, "负责人")
	}
	if len(missing) == 0 {
		return passCheck("profile", "应用档案", "应用名称、团队和负责人已补齐", path)
	}
	return warnCheck("profile", "补齐应用档案", "缺少："+strings.Join(missing, "、"), "medium", path)
}

func (s *ReadinessService) checkRepo(ctx context.Context, app models.Application, path string) dto.ApplicationReadinessCheck {
	var count int64
	s.db.WithContext(ctx).Model(&models.ApplicationRepoBinding{}).Where("application_id = ?", app.ID).Count(&count)
	if count > 0 || strings.TrimSpace(app.GitRepo) != "" {
		return passCheck("repo", "Git 仓库", "已绑定标准仓库或应用仓库地址", path)
	}
	return warnCheck("repo", "绑定 Git 仓库", "应用尚未绑定标准 Git 仓库，无法稳定创建交付流水线", "high", path)
}

func (s *ReadinessService) checkEnvs(ctx context.Context, app models.Application, path string) dto.ApplicationReadinessCheck {
	var envs []models.ApplicationEnv
	_ = s.db.WithContext(ctx).Where("app_id = ?", app.ID).Find(&envs).Error
	if len(envs) == 0 {
		return warnCheck("envs", "配置环境", "尚未配置 dev/test/staging/prod 等交付环境", "high", path)
	}
	for _, env := range envs {
		if strings.TrimSpace(env.K8sNamespace) != "" && strings.TrimSpace(env.K8sDeployment) != "" {
			return passCheck("envs", "环境配置", fmt.Sprintf("已配置 %d 个环境，至少 1 个环境有运行目标", len(envs)), path)
		}
	}
	return warnCheck("envs", "补齐运行目标", fmt.Sprintf("已配置 %d 个环境，但缺少 namespace/deployment 运行目标", len(envs)), "medium", path)
}

func (s *ReadinessService) checkPipelines(ctx context.Context, app models.Application, path string) dto.ApplicationReadinessCheck {
	var count int64
	q := s.db.WithContext(ctx).Model(&models.Pipeline{})
	q = q.Where("application_id = ? OR application_name = ?", app.ID, app.Name)
	q.Count(&count)
	if count > 0 {
		return passCheck("pipelines", "交付流水线", fmt.Sprintf("已关联 %d 条流水线", count), path)
	}
	return warnCheck("pipelines", "创建交付流水线", "尚未找到关联流水线，无法形成标准 CI/CD 链路", "high", path)
}

func (s *ReadinessService) checkGitOps(ctx context.Context, app models.Application, path string) dto.ApplicationReadinessCheck {
	var repoCount int64
	s.db.WithContext(ctx).Model(&infrastructure.GitOpsRepo{}).
		Where("application_id = ? OR application_name = ?", app.ID, app.Name).
		Count(&repoCount)
	var argoCount int64
	s.db.WithContext(ctx).Model(&infrastructure.ArgoCDApplication{}).
		Where("application_id = ? OR application_name = ?", app.ID, app.Name).
		Count(&argoCount)
	if repoCount > 0 && argoCount > 0 {
		return passCheck("gitops", "GitOps 接入", "已绑定 GitOps 仓库并同步 Argo CD 应用", path)
	}
	if repoCount > 0 || argoCount > 0 {
		return warnCheck("gitops", "补齐 GitOps 接入", "GitOps 仓库或 Argo CD 应用仅部分接入", "medium", path)
	}
	return warnCheck("gitops", "接入 GitOps", "尚未绑定 GitOps 仓库和 Argo CD 应用", "high", path)
}

func (s *ReadinessService) checkGovernance(ctx context.Context, app models.Application, path string) dto.ApplicationReadinessCheck {
	var ruleCount int64
	s.db.WithContext(ctx).Model(&models.ApprovalRule{}).
		Where("enabled = ? AND (app_id = ? OR app_id = 0)", true, app.ID).
		Count(&ruleCount)
	var policyCount int64
	s.db.WithContext(ctx).Model(&models.EnvAuditPolicy{}).
		Where("enabled = ?", true).
		Count(&policyCount)
	var windowCount int64
	s.db.WithContext(ctx).Model(&models.DeployWindow{}).
		Where("enabled = ? AND (app_id = ? OR app_id = 0)", true, app.ID).
		Count(&windowCount)
	if ruleCount > 0 && (policyCount > 0 || windowCount > 0) {
		return passCheck("governance", "发布治理", "已配置审批规则，并具备环境策略或发布窗口", path)
	}
	if ruleCount > 0 || policyCount > 0 || windowCount > 0 {
		return warnCheck("governance", "补齐发布治理", "已有部分治理配置，建议补齐审批、窗口和环境策略", "medium", path)
	}
	return warnCheck("governance", "配置发布治理", "尚未发现审批规则、发布窗口或环境审核策略", "medium", path)
}

func (s *ReadinessService) checkRuntime(ctx context.Context, app models.Application, path string) dto.ApplicationReadinessCheck {
	var envCount int64
	s.db.WithContext(ctx).Model(&models.ApplicationEnv{}).
		Where("app_id = ? AND k8s_cluster_id IS NOT NULL AND k8s_namespace != '' AND k8s_deployment != ''", app.ID).
		Count(&envCount)
	if envCount > 0 {
		return passCheck("runtime", "运行态归属", fmt.Sprintf("已有 %d 个环境配置了集群、命名空间和 Deployment", envCount), path)
	}
	return warnCheck("runtime", "补齐运行态归属", "建议在应用环境中补齐集群、namespace 和 deployment，便于日志、成本、安全聚合", "low", path)
}

func passCheck(key, title, desc, path string) dto.ApplicationReadinessCheck {
	return dto.ApplicationReadinessCheck{Key: key, Title: title, Description: desc, Status: "pass", Severity: "info", Path: path}
}

func warnCheck(key, title, desc, severity, path string) dto.ApplicationReadinessCheck {
	return dto.ApplicationReadinessCheck{Key: key, Title: title, Description: desc, Status: "missing", Severity: severity, Path: path}
}

func readinessLevel(score int) string {
	switch {
	case score >= 90:
		return "ready"
	case score >= 70:
		return "almost_ready"
	case score >= 40:
		return "partial"
	default:
		return "not_ready"
	}
}

func actionWeight(severity string) int {
	switch severity {
	case "high":
		return 100
	case "medium":
		return 60
	default:
		return 30
	}
}
