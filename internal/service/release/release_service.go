package release

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	modelbiz "devops/internal/models/biz"
	"devops/internal/models/deploy"
	"devops/internal/models/infrastructure"
	appRepo "devops/internal/modules/application/repository"
	changelogsvc "devops/internal/service/changelog"
	"devops/internal/types"
	"devops/pkg/dto"

	"gorm.io/gorm"
)

type Service struct {
	repo       *appRepo.ReleaseRepository
	itemRepo   *appRepo.ReleaseItemRepository
	nrRepo     *appRepo.NacosReleaseRepository
	logSvc     *changelogsvc.Service
	riskScorer *RiskScorer
	db         *gorm.DB
}

func NewService(db *gorm.DB, repo *appRepo.ReleaseRepository, itemRepo *appRepo.ReleaseItemRepository, nrRepo *appRepo.NacosReleaseRepository, logSvc *changelogsvc.Service) *Service {
	return &Service{repo: repo, itemRepo: itemRepo, nrRepo: nrRepo, logSvc: logSvc, db: db}
}

// WithRiskScorer 注入风险评分器（v2.0；为可选依赖以保持构造函数兼容）。
func (s *Service) WithRiskScorer(scorer *RiskScorer) *Service {
	s.riskScorer = scorer
	return s
}

func (s *Service) List(f appRepo.ReleaseFilter, page, pageSize int) ([]deploy.Release, int64, error) {
	list, total, err := s.repo.List(f, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if err := s.fillBizLinks(list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *Service) GetByID(id uint) (*deploy.Release, error) {
	rel, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	// 填充关联子项
	items, _ := s.itemRepo.ListByRelease(id)
	for _, item := range items {
		switch item.ItemType {
		case "nacos_release":
			if nr, err := s.nrRepo.GetByID(item.ItemID); err == nil {
				rel.NacosReleases = append(rel.NacosReleases, *nr)
			}
		}
	}
	if err := s.fillBizLink(rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// GetOverview 聚合 Release 主链路状态，避免前端分别探测审批、GitOps、ArgoCD。
func (s *Service) GetOverview(ctx context.Context, id uint) (*dto.ReleaseOverviewDTO, error) {
	rel, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	overview := &dto.ReleaseOverviewDTO{
		ReleaseID: rel.ID,
		Status:    rel.Status,
		Approval:  dto.ReleaseOverviewApproval{Status: "none"},
		GitOps:    dto.ReleaseOverviewGitOps{Status: "none"},
		ArgoCD: dto.ReleaseOverviewArgoCD{
			AppName:    rel.ArgoAppName,
			SyncStatus: rel.ArgoSyncStatus,
		},
	}

	if rel.ApprovalInstanceID != nil && *rel.ApprovalInstanceID > 0 {
		var approval deploy.ApprovalInstance
		if err := s.db.WithContext(ctx).First(&approval, *rel.ApprovalInstanceID).Error; err == nil {
			overview.Approval = dto.ReleaseOverviewApproval{
				InstanceID:       rel.ApprovalInstanceID,
				Status:           approval.Status,
				ChainName:        approval.ChainName,
				CurrentNodeOrder: approval.CurrentNodeOrder,
				StartedAt:        approval.StartedAt,
				FinishedAt:       approval.FinishedAt,
			}
		}
	} else if rel.Status == deploy.ReleaseStatusPendingApproval {
		overview.Approval.Status = "pending"
	} else if rel.ApprovedAt != nil {
		overview.Approval.Status = "approved"
	}

	var change *infrastructure.GitOpsChangeRequest
	if rel.GitOpsChangeRequestID != nil && *rel.GitOpsChangeRequestID > 0 {
		var row infrastructure.GitOpsChangeRequest
		if err := s.db.WithContext(ctx).First(&row, *rel.GitOpsChangeRequestID).Error; err == nil {
			change = &row
			overview.GitOps = dto.ReleaseOverviewGitOps{
				ChangeRequestID: rel.GitOpsChangeRequestID,
				Status:          firstNonEmpty(row.Status, "unknown"),
				MRURL:           row.MergeRequestURL,
				ApprovalStatus:  row.ApprovalStatus,
				AutoMergeStatus: row.AutoMergeStatus,
				ErrorMessage:    row.ErrorMessage,
				UpdatedAt:       &row.UpdatedAt,
			}
		}
	}

	if app := s.resolveReleaseArgoApp(ctx, rel, change); app != nil {
		overview.ArgoCD = dto.ReleaseOverviewArgoCD{
			ApplicationID: &app.ID,
			AppName:       firstNonEmpty(app.Name, rel.ArgoAppName),
			SyncStatus:    firstNonEmpty(app.SyncStatus, rel.ArgoSyncStatus),
			HealthStatus:  app.HealthStatus,
			DriftDetected: app.DriftDetected,
			LastSyncAt:    app.LastSyncAt,
		}
	}

	overview.CurrentStage = releaseOverviewStage(rel, overview)
	items, _ := s.itemRepo.ListByRelease(rel.ID)
	overview.Blocked, overview.BlockedReason, overview.NextAction = releaseOverviewNextAction(rel, overview, len(items) > 0)
	overview.Stages = releaseOverviewStages(rel, overview)
	return overview, nil
}

func (s *Service) Create(rel *deploy.Release) error {
	rel.Status = "draft"
	return s.repo.Create(rel)
}

// CreateFromPipelineRun 将一次成功 CI 运行挂载到 Release 主单。
// 如果 ExistingReleaseID 为空，则创建新的 draft Release；否则把运行追加到已有草稿 Release。
func (s *Service) CreateFromPipelineRun(ctx context.Context, req *dto.CreateReleaseFromPipelineRunRequest, creatorID uint, creatorName string) (*deploy.Release, error) {
	if req == nil || req.PipelineRunID == 0 {
		return nil, fmt.Errorf("pipeline_run_id 不能为空")
	}

	var run deploy.PipelineRun
	if err := s.db.WithContext(ctx).First(&run, req.PipelineRunID).Error; err != nil {
		return nil, fmt.Errorf("流水线运行不存在: %w", err)
	}
	if run.Status != "success" {
		return nil, fmt.Errorf("只有成功的流水线运行才能生成发布单")
	}

	env := firstNonEmpty(req.Env, run.Env, "dev")
	version := firstNonEmpty(req.Version, extractRunVersion(&run), run.GitCommit, fmt.Sprintf("run-%d", run.ID))
	title := firstNonEmpty(req.Title, fmt.Sprintf("%s %s 发布 %s", firstNonEmpty(run.ApplicationName, run.PipelineName, "应用"), env, version))
	description := firstNonEmpty(req.Description, fmt.Sprintf("由流水线 %s 运行 #%d 生成", run.PipelineName, run.ID))
	rolloutStrategy := req.RolloutStrategy
	if rolloutStrategy == "" {
		rolloutStrategy = deploy.RolloutStrategyDirect
	}
	if !deploy.IsValidRolloutStrategy(rolloutStrategy) {
		return nil, fmt.Errorf("rollout_strategy 必须为 direct/canary/blue_green")
	}
	riskLevel := firstNonEmpty(req.RiskLevel, "low")

	var rel deploy.Release
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.ExistingReleaseID > 0 {
			if err := tx.First(&rel, req.ExistingReleaseID).Error; err != nil {
				return fmt.Errorf("发布主单不存在: %w", err)
			}
			if rel.Status != deploy.ReleaseStatusDraft {
				return fmt.Errorf("只能向草稿状态发布单追加流水线运行")
			}
		} else {
			rel = deploy.Release{
				Title:           title,
				ApplicationID:   run.ApplicationID,
				ApplicationName: run.ApplicationName,
				Env:             env,
				Version:         version,
				Description:     description,
				RiskLevel:       riskLevel,
				RolloutStrategy: rolloutStrategy,
				RolloutConfig:   types.JSONMap(req.RolloutConfig),
				CreatedBy:       creatorID,
				CreatedByName:   creatorName,
				Status:          deploy.ReleaseStatusDraft,
			}
			if err := tx.Create(&rel).Error; err != nil {
				return err
			}
		}

		var existing deploy.ReleaseItem
		err := tx.Where("release_id = ? AND item_type = ? AND item_id = ?", rel.ID, deploy.ReleaseItemTypePipelineRun, run.ID).First(&existing).Error
		if err == nil {
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}

		imageRepo, imageTag := extractRunImage(&run)
		payload := types.JSONMap{
			"pipeline_id":              run.PipelineID,
			"pipeline_name":            run.PipelineName,
			"git_branch":               run.GitBranch,
			"git_commit":               run.GitCommit,
			"image":                    run.ScannedImage,
			"image_repository":         imageRepo,
			"image_tag":                imageTag,
			"gitops_change_request_id": run.GitOpsChangeRequestID,
		}
		for key, value := range extractRunGitOpsPayload(&run) {
			payload[key] = value
		}
		item := &deploy.ReleaseItem{
			ReleaseID:  rel.ID,
			ItemType:   deploy.ReleaseItemTypePipelineRun,
			ItemID:     run.ID,
			ItemTitle:  fmt.Sprintf("%s #%d", run.PipelineName, run.ID),
			ItemStatus: run.Status,
			Payload:    payload,
		}
		return tx.Create(item).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(rel.ID)
}

func (s *Service) Update(rel *deploy.Release) error {
	existing, err := s.repo.GetByID(rel.ID)
	if err != nil {
		return fmt.Errorf("发布主单不存在: %w", err)
	}
	if existing.Status != "draft" {
		return fmt.Errorf("只能编辑草稿状态的发布主单")
	}
	rel.CreatedAt = existing.CreatedAt
	rel.CreatedBy = existing.CreatedBy
	rel.CreatedByName = existing.CreatedByName
	rel.Status = existing.Status
	rel.ApprovedBy = existing.ApprovedBy
	rel.ApprovedByName = existing.ApprovedByName
	rel.ApprovedAt = existing.ApprovedAt
	rel.PublishedAt = existing.PublishedAt
	rel.PublishedBy = existing.PublishedBy
	rel.PublishedByName = existing.PublishedByName
	rel.RollbackAt = existing.RollbackAt
	rel.RejectReason = existing.RejectReason
	return s.repo.Update(rel)
}

func (s *Service) Delete(id uint) error {
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("发布主单不存在: %w", err)
	}
	if existing.Status != "draft" {
		return fmt.Errorf("只能删除草稿状态的发布主单")
	}
	_ = s.itemRepo.DeleteByRelease(id)
	return s.repo.Delete(id)
}

// AddItem 添加关联子项
func (s *Service) AddItem(releaseID uint, itemType string, itemID uint, itemTitle string) error {
	rel, err := s.repo.GetByID(releaseID)
	if err != nil {
		return fmt.Errorf("发布主单不存在: %w", err)
	}
	if rel.Status != "draft" {
		return fmt.Errorf("只有草稿状态才能添加关联子项")
	}
	item := &deploy.ReleaseItem{
		ReleaseID: releaseID,
		ItemType:  itemType,
		ItemID:    itemID,
		ItemTitle: itemTitle,
	}
	return s.itemRepo.Create(item)
}

// RemoveItem 移除关联子项
func (s *Service) RemoveItem(itemID uint) error {
	return s.itemRepo.Delete(itemID)
}

// ListItems 查询关联子项
func (s *Service) ListItems(releaseID uint) ([]deploy.ReleaseItem, error) {
	return s.itemRepo.ListByRelease(releaseID)
}

// SubmitForApproval 提交审批
func (s *Service) SubmitForApproval(id uint) (*deploy.Release, error) {
	rel, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布主单不存在: %w", err)
	}
	if rel.Status != "draft" {
		return nil, fmt.Errorf("只有草稿状态才能提交审批")
	}
	items, _ := s.itemRepo.ListByRelease(id)
	if len(items) == 0 {
		return nil, fmt.Errorf("发布主单没有关联任何变更项")
	}
	rel.Status = "pending_approval"
	// v2.0：提交审批时执行风险评分
	if s.riskScorer != nil {
		s.riskScorer.ScoreAndApply(context.Background(), rel, items)
	}
	if err := s.repo.Update(rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// Approve 审批通过
func (s *Service) Approve(id, approverID uint, approverName string) (*deploy.Release, error) {
	rel, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布主单不存在: %w", err)
	}
	if rel.Status != "pending_approval" {
		return nil, fmt.Errorf("只有待审批状态才能审批")
	}
	rel.Status = "approved"
	rel.ApprovedBy = &approverID
	rel.ApprovedByName = approverName
	now := time.Now()
	rel.ApprovedAt = &now
	if err := s.repo.Update(rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// Reject 驳回
func (s *Service) Reject(id, approverID uint, approverName, reason string) (*deploy.Release, error) {
	rel, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布主单不存在: %w", err)
	}
	if rel.Status != "pending_approval" {
		return nil, fmt.Errorf("只有待审批状态才能驳回")
	}
	rel.Status = "rejected"
	rel.ApprovedBy = &approverID
	rel.ApprovedByName = approverName
	rel.RejectReason = reason
	now := time.Now()
	rel.ApprovedAt = &now
	if err := s.repo.Update(rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// Publish 标记为已发布（实际发布动作由各子项独立完成）
func (s *Service) Publish(id, publisherID uint, publisherName string) (*deploy.Release, error) {
	rel, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布主单不存在: %w", err)
	}
	if rel.Status != deploy.ReleaseStatusApproved && rel.Status != deploy.ReleaseStatusPRMerged {
		return nil, fmt.Errorf("只有已审批或 PR 已合并状态才能发布")
	}
	rel.Status = "published"
	rel.PublishedBy = &publisherID
	rel.PublishedByName = publisherName
	now := time.Now()
	rel.PublishedAt = &now
	if err := s.repo.Update(rel); err != nil {
		return nil, err
	}
	if s.logSvc != nil {
		event := &deploy.ChangeEvent{
			EventType:       "release",
			EventID:         rel.ID,
			Title:           rel.Title,
			Description:     rel.Description,
			ApplicationID:   rel.ApplicationID,
			ApplicationName: rel.ApplicationName,
			Env:             rel.Env,
			Status:          rel.Status,
			RiskLevel:       rel.RiskLevel,
			Operator:        publisherName,
			OperatorID:      publisherID,
		}
		if err := s.logSvc.RecordEvent(event); err != nil {
			return nil, err
		}
	}
	return rel, nil
}

func (s *Service) fillBizLinks(list []deploy.Release) error {
	if len(list) == 0 {
		return nil
	}
	releaseIDs := make([]uint, 0, len(list))
	for _, item := range list {
		releaseIDs = append(releaseIDs, item.ID)
	}
	var versions []modelbiz.BizVersion
	if err := s.db.Where("release_id IN ?", uniqueReleaseIDs(releaseIDs)).Find(&versions).Error; err != nil {
		return err
	}
	if len(versions) == 0 {
		return nil
	}
	goalIDs := make([]uint, 0, len(versions))
	versionMap := map[uint]modelbiz.BizVersion{}
	for _, item := range versions {
		versionMap[*item.ReleaseID] = item
		if item.GoalID != nil {
			goalIDs = append(goalIDs, *item.GoalID)
		}
	}
	goalMap := map[uint]string{}
	if len(goalIDs) > 0 {
		var goals []modelbiz.BizGoal
		if err := s.db.Where("id IN ?", uniqueReleaseIDs(goalIDs)).Find(&goals).Error; err != nil {
			return err
		}
		for _, item := range goals {
			goalMap[item.ID] = item.Name
		}
	}
	for i := range list {
		version, ok := versionMap[list[i].ID]
		if !ok {
			continue
		}
		list[i].BizVersionID = &version.ID
		list[i].BizVersionName = version.Name
		if version.GoalID != nil {
			list[i].BizGoalID = version.GoalID
			list[i].BizGoalName = goalMap[*version.GoalID]
		}
	}
	return nil
}

func (s *Service) fillBizLink(rel *deploy.Release) error {
	var version modelbiz.BizVersion
	if err := s.db.Where("release_id = ?", rel.ID).First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	rel.BizVersionID = &version.ID
	rel.BizVersionName = version.Name
	if version.GoalID != nil {
		rel.BizGoalID = version.GoalID
		var goal modelbiz.BizGoal
		if err := s.db.First(&goal, *version.GoalID).Error; err == nil {
			rel.BizGoalName = goal.Name
		}
	}
	return nil
}

func uniqueReleaseIDs(ids []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) resolveReleaseArgoApp(ctx context.Context, rel *deploy.Release, change *infrastructure.GitOpsChangeRequest) *infrastructure.ArgoCDApplication {
	if change != nil && change.ArgoCDApplicationID != nil && *change.ArgoCDApplicationID > 0 {
		var app infrastructure.ArgoCDApplication
		if err := s.db.WithContext(ctx).First(&app, *change.ArgoCDApplicationID).Error; err == nil {
			return &app
		}
	}

	query := s.db.WithContext(ctx).Model(&infrastructure.ArgoCDApplication{})
	if strings.TrimSpace(rel.ArgoAppName) != "" {
		var app infrastructure.ArgoCDApplication
		if err := query.Where("name = ?", rel.ArgoAppName).Order("updated_at DESC").First(&app).Error; err == nil {
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

func releaseOverviewStage(rel *deploy.Release, overview *dto.ReleaseOverviewDTO) string {
	switch rel.Status {
	case deploy.ReleaseStatusDraft:
		return "draft"
	case deploy.ReleaseStatusPendingApproval, deploy.ReleaseStatusApproved, deploy.ReleaseStatusRejected:
		return "approval"
	case deploy.ReleaseStatusPROpened, deploy.ReleaseStatusPRMerged:
		return "gitops"
	case deploy.ReleaseStatusPublishing:
		return "argocd"
	case deploy.ReleaseStatusPublished:
		return "done"
	case deploy.ReleaseStatusFailed, deploy.ReleaseStatusRolledBack:
		return rel.Status
	default:
		if overview.ArgoCD.SyncStatus != "" || overview.ArgoCD.HealthStatus != "" {
			return "argocd"
		}
		return "draft"
	}
}

func releaseOverviewNextAction(rel *deploy.Release, overview *dto.ReleaseOverviewDTO, hasItems bool) (bool, string, string) {
	if overview.GitOps.ErrorMessage != "" {
		return true, overview.GitOps.ErrorMessage, "处理 GitOps 变更失败"
	}
	if overview.ArgoCD.DriftDetected {
		return true, "ArgoCD 检测到漂移", "处理 ArgoCD 漂移"
	}
	if strings.EqualFold(overview.ArgoCD.HealthStatus, "Degraded") || strings.EqualFold(overview.ArgoCD.HealthStatus, "Missing") {
		return true, "ArgoCD 应用健康状态异常", "查看 ArgoCD 应用"
	}

	switch rel.Status {
	case deploy.ReleaseStatusDraft:
		if !hasItems {
			return true, "发布单尚未关联变更项", "添加变更项"
		}
		return false, "", "提交审批"
	case deploy.ReleaseStatusPendingApproval:
		return false, "", "等待审批通过"
	case deploy.ReleaseStatusRejected:
		return true, firstNonEmpty(rel.RejectReason, "审批已驳回"), "修改后重新提交"
	case deploy.ReleaseStatusApproved:
		if overview.GitOps.ChangeRequestID == nil {
			return false, "", "生成 GitOps PR"
		}
		return false, "", "等待 GitOps PR"
	case deploy.ReleaseStatusPROpened:
		return false, "", "等待 PR 合并"
	case deploy.ReleaseStatusPRMerged:
		return false, "", "发布并同步 ArgoCD"
	case deploy.ReleaseStatusPublishing:
		return false, "", "等待 ArgoCD 同步"
	case deploy.ReleaseStatusPublished:
		if !strings.EqualFold(overview.ArgoCD.SyncStatus, "Synced") && overview.ArgoCD.SyncStatus != "" {
			return true, "发布已完成但 ArgoCD 尚未同步", "查看同步状态"
		}
		return false, "", "观察发布结果"
	case deploy.ReleaseStatusFailed:
		return true, "发布失败", "查看失败原因"
	case deploy.ReleaseStatusRolledBack:
		return false, "", "已回滚"
	default:
		return false, "", "查看详情"
	}
}

func releaseOverviewStages(rel *deploy.Release, overview *dto.ReleaseOverviewDTO) []dto.ReleaseOverviewStageDTO {
	order := []struct {
		key   string
		label string
	}{
		{"draft", "变更准备"},
		{"approval", "审批"},
		{"gitops", "GitOps PR"},
		{"argocd", "ArgoCD 同步"},
		{"done", "完成观察"},
	}
	currentRank := 0
	for idx, item := range order {
		if item.key == overview.CurrentStage {
			currentRank = idx
			break
		}
	}
	stages := make([]dto.ReleaseOverviewStageDTO, 0, len(order))
	for idx, item := range order {
		status := "wait"
		if idx < currentRank {
			status = "finish"
		}
		if idx == currentRank {
			status = "process"
		}
		if overview.Blocked && item.key == overview.CurrentStage {
			status = "error"
		}
		message := ""
		switch item.key {
		case "approval":
			message = firstNonEmpty(overview.Approval.Status, "none")
		case "gitops":
			message = firstNonEmpty(overview.GitOps.Status, "none")
		case "argocd":
			message = firstNonEmpty(overview.ArgoCD.SyncStatus, overview.ArgoCD.HealthStatus, "none")
		case "done":
			if rel.PublishedAt != nil {
				message = rel.PublishedAt.Format(time.RFC3339)
			}
		}
		stages = append(stages, dto.ReleaseOverviewStageDTO{
			Key:     item.key,
			Label:   item.label,
			Status:  status,
			Message: message,
		})
	}
	return stages
}

func extractRunVersion(run *deploy.PipelineRun) string {
	if run == nil {
		return ""
	}
	values := runParameters(run)
	for _, key := range []string{"GITOPS_IMAGE_TAG", "IMAGE_TAG", "VERSION", "TAG"} {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	if _, imageTag := splitRunImage(run.ScannedImage); imageTag != "" {
		return imageTag
	}
	if strings.TrimSpace(run.GitCommit) != "" {
		commit := strings.TrimSpace(run.GitCommit)
		if len(commit) > 12 {
			return commit[:12]
		}
		return commit
	}
	return ""
}

func extractRunImage(run *deploy.PipelineRun) (string, string) {
	if run == nil {
		return "", ""
	}
	values := runParameters(run)
	repo := firstNonEmpty(
		values["GITOPS_IMAGE_REPOSITORY"],
		values["IMAGE_REPOSITORY"],
		values["IMAGE_REPO"],
		values["IMAGE_NAME"],
		values["REGISTRY_IMAGE"],
	)
	tag := firstNonEmpty(
		values["GITOPS_IMAGE_TAG"],
		values["IMAGE_TAG"],
		values["VERSION"],
		values["TAG"],
	)
	if imageRepo, imageTag := splitRunImage(run.ScannedImage); imageRepo != "" {
		if repo == "" {
			repo = imageRepo
		}
		if tag == "" {
			tag = imageTag
		}
	}
	if tag == "" && strings.TrimSpace(run.GitCommit) != "" {
		tag = shortRunCommit(run.GitCommit)
	}
	return repo, tag
}

func extractRunGitOpsPayload(run *deploy.PipelineRun) map[string]any {
	out := map[string]any{}
	if run == nil {
		return out
	}
	values := runParameters(run)
	if value := firstNonEmpty(values["GITOPS_REPO_ID"], values["GITOPS_REPOSITORY_ID"]); value != "" {
		out["gitops_repo_id"] = value
	}
	if value := firstNonEmpty(values["GITOPS_FILE_PATH"], values["GITOPS_MANIFEST_PATH"], values["FILE_PATH"]); value != "" {
		out["file_path"] = value
	}
	if value := firstNonEmpty(values["GITOPS_TARGET_BRANCH"], values["TARGET_BRANCH"]); value != "" {
		out["target_branch"] = value
	}
	return out
}

func runParameters(run *deploy.PipelineRun) map[string]string {
	values := map[string]string{}
	if run == nil || strings.TrimSpace(run.ParametersJSON) == "" {
		return values
	}
	_ = json.Unmarshal([]byte(run.ParametersJSON), &values)
	return values
}

func splitRunImage(image string) (string, string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	if strings.Contains(image, "@") {
		parts := strings.SplitN(image, "@", 2)
		return parts[0], ""
	}
	idx := strings.LastIndex(image, ":")
	lastSlash := strings.LastIndex(image, "/")
	if idx > lastSlash && idx > -1 {
		return image[:idx], image[idx+1:]
	}
	return image, ""
}

func shortRunCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
