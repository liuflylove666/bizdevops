// Package release
//
// gitops_pr_service.go: Release 主动触发 GitOps PR 的服务（v2.0）。
//
// 与 internal/service/pipeline/gitops_handoff_service.go 的关系：
//   - GitOpsHandoffService：CI 成功后由流水线侧 自动 单条 推送 PR（细粒度）
//   - GitOpsPRService（本文件）：Release 审批通过后由发布主单 主动 聚合 多个变更项 推送 PR（聚合粒度）
//
// 两者复用同一个 argocd.Service.CreateChangeRequest，但触发位置和粒度不同。
//
// 启用条件：已固定为开启（dry_run 路径用于预览，不受真实 PR 副作用影响）
package release

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	models "devops/internal/models/application"
	"devops/internal/models/deploy"
	"devops/internal/models/infrastructure"
	appRepo "devops/internal/modules/application/repository"
	approvalRepo "devops/internal/modules/approval/repository"
	infraRepo "devops/internal/modules/infrastructure/repository"
	argocdsvc "devops/internal/service/argocd"
	"devops/pkg/dto"
)

// GitOpsPRService 把 Release 聚合根转换成 1~N 条 GitOps 变更请求。
type GitOpsPRService struct {
	db          *gorm.DB
	releaseRepo *appRepo.ReleaseRepository
	itemRepo    *appRepo.ReleaseItemRepository
}

// NewGitOpsPRService 构造服务。
func NewGitOpsPRService(db *gorm.DB, releaseRepo *appRepo.ReleaseRepository, itemRepo *appRepo.ReleaseItemRepository) *GitOpsPRService {
	return &GitOpsPRService{
		db:          db,
		releaseRepo: releaseRepo,
		itemRepo:    itemRepo,
	}
}

// resolvedItem 内部结构，承载从 ReleaseItem + PipelineRun 反推得到的镜像信息。
type resolvedItem struct {
	releaseItem     deploy.ReleaseItem
	pipelineRun     *deploy.PipelineRun
	imageRepo       string
	imageTag        string
	gitopsRepoID    uint
	filePath        string
	targetBranch    string
	helmChartPath   string
	helmValuesPath  string
	helmReleaseName string
	replicas        int
	cpuRequest      string
	cpuLimit        string
	memoryRequest   string
	memoryLimit     string
}

// OpenPR 由 Release 触发一次 GitOps PR 生成。
//
// 前置：
//   - Release 状态必须为 approved（dry_run 也强制此前置，避免误用）
//
// 行为：
//   - 解析 Release 关联的 deployment / pipeline_run 子项
//   - 对每个子项反查 PipelineRun 取镜像信息
//   - 按 application + env 自动匹配 GitOpsRepo
//   - dry_run：返回 FilesChanged 预览，不调用真实 ArgoCD/Git API
//   - 非 dry_run：调用 argocd.CreateChangeRequest 并把首个 ChangeRequestID 回填至 Release，置状态 pr_opened
func (s *GitOpsPRService) OpenPR(ctx context.Context, req *dto.GitOpsPRRequest, operatorID *uint) (*dto.GitOpsPRResponse, error) {
	if req == nil || req.ReleaseID == 0 {
		return nil, errors.New("release_id 必填")
	}

	rel, err := s.releaseRepo.GetByID(req.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("发布主单不存在: %w", err)
	}
	if rel.Status != deploy.ReleaseStatusApproved {
		return nil, fmt.Errorf("仅 approved 状态可触发 GitOps PR，当前状态: %s", rel.Status)
	}

	items, err := s.itemRepo.ListByRelease(rel.ID)
	if err != nil {
		return nil, fmt.Errorf("加载发布子项失败: %w", err)
	}

	resolved, err := s.resolveDeploymentItems(ctx, rel, items)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, errors.New("发布主单未关联任何可用于 GitOps PR 的子项（需要 deployment 或 pipeline_run 类型，且对应应用配置了 GitOps 仓库）")
	}

	branchName := s.deriveBranchName(rel, req.TargetBranch)
	filesChanged := make([]string, 0, len(resolved))
	for _, r := range resolved {
		filesChanged = append(filesChanged, r.filePath)
	}

	resp := &dto.GitOpsPRResponse{
		BranchName:   branchName,
		FilesChanged: filesChanged,
		DryRun:       req.DryRun,
	}

	if req.DryRun {
		resp.Message = fmt.Sprintf("dry-run 完成：将创建 %d 条 GitOps 变更", len(resolved))
		return resp, nil
	}

	// 真实路径：逐条提交 ChangeRequest；首个成功的 ChangeRequestID 回填到 Release
	argocdService := s.newArgoCDService()
	var firstChangeReqID *uint
	prURLs := make([]string, 0, len(resolved))
	for _, r := range resolved {
		input := &argocdsvc.CreateChangeRequestInput{
			GitOpsRepoID:    r.gitopsRepoID,
			ApplicationID:   rel.ApplicationID,
			ApplicationName: rel.ApplicationName,
			Env:             rel.Env,
			Title:           buildChangeRequestTitle(rel, r),
			Description:     buildChangeRequestDescription(rel, r),
			FilePath:        r.filePath,
			ImageRepository: r.imageRepo,
			ImageTag:        r.imageTag,
			TargetBranch:    fallback(req.TargetBranch, r.targetBranch),
			HelmChartPath:   r.helmChartPath,
			HelmValuesPath:  r.helmValuesPath,
			HelmReleaseName: r.helmReleaseName,
			Replicas:        r.replicas,
			CPURequest:      r.cpuRequest,
			CPULimit:        r.cpuLimit,
			MemoryRequest:   r.memoryRequest,
			MemoryLimit:     r.memoryLimit,
		}
		if r.pipelineRun != nil {
			input.PipelineID = &r.pipelineRun.PipelineID
			input.PipelineRunID = &r.pipelineRun.ID
		}
		if req.CommitMsg != "" {
			input.Description = req.CommitMsg + "\n\n" + input.Description
		}

		cr, crErr := argocdService.CreateChangeRequest(ctx, input, operatorID)
		if crErr != nil {
			// 部分失败：尽力上报已创建的，整体返回错误
			resp.Message = fmt.Sprintf("已创建 %d 条变更后失败: %v", len(prURLs), crErr)
			s.applyPartialFailure(rel, firstChangeReqID, resp.Message)
			return resp, crErr
		}
		if cr == nil {
			continue
		}
		if firstChangeReqID == nil {
			id := cr.ID
			firstChangeReqID = &id
		}
		// GitOpsChangeRequest 暂时没有暴露 PR URL 字段，留空待 ADR-0001 补充
		_ = cr
	}

	if firstChangeReqID != nil {
		resp.ChangeRequestID = *firstChangeReqID
		s.applySuccess(rel, *firstChangeReqID)
	}
	resp.Message = fmt.Sprintf("已为发布 #%d 创建 %d 条 GitOps 变更请求", rel.ID, len(resolved))
	return resp, nil
}

// newArgoCDService 构造 argocd 服务（与 gitops_handoff_service 保持一致的依赖装配）。
func (s *GitOpsPRService) newArgoCDService() *argocdsvc.Service {
	instRepo := infraRepo.NewArgoCDInstanceRepository(s.db)
	argoAppRepo := infraRepo.NewArgoCDApplicationRepository(s.db)
	gitopsRepoRepo := infraRepo.NewGitOpsRepoRepository(s.db)
	changeRepo := infraRepo.NewGitOpsChangeRequestRepository(s.db)
	service := argocdsvc.NewService(instRepo, argoAppRepo, gitopsRepoRepo, changeRepo)
	service.SetEnvPolicyRepo(approvalRepo.NewEnvAuditPolicyRepository(s.db))
	service.SetSonarBindingRepo(infraRepo.NewSonarQubeBindingRepository(s.db))
	return service
}

// resolveDeploymentItems 解析 deployment 类子项的镜像与目标仓库。
func (s *GitOpsPRService) resolveDeploymentItems(ctx context.Context, rel *deploy.Release, items []deploy.ReleaseItem) ([]*resolvedItem, error) {
	out := make([]*resolvedItem, 0, len(items))
	for _, item := range items {
		switch item.ItemType {
		case deploy.ReleaseItemTypeDeployment, deploy.ReleaseItemTypePipelineRun:
		default:
			continue
		}

		ri := &resolvedItem{releaseItem: item}

		// 1. 镜像信息：优先取 Payload，其次 PipelineRun.ScannedImage
		if v, ok := stringFromPayload(item.Payload, "image_repository"); ok {
			ri.imageRepo = v
		}
		if v, ok := stringFromPayload(item.Payload, "image_tag"); ok {
			ri.imageTag = v
		}
		if (ri.imageRepo == "" || ri.imageTag == "") && item.ItemID > 0 {
			run, err := s.fetchPipelineRun(ctx, item.ItemID)
			if err == nil && run != nil {
				ri.pipelineRun = run
				if ri.imageRepo == "" || ri.imageTag == "" {
					repo, tag := splitImage(run.ScannedImage)
					if ri.imageRepo == "" {
						ri.imageRepo = repo
					}
					if ri.imageTag == "" {
						if tag != "" {
							ri.imageTag = tag
						} else if run.GitCommit != "" {
							ri.imageTag = shortCommit(run.GitCommit)
						}
					}
				}
			}
		}
		if ri.imageRepo == "" || ri.imageTag == "" {
			return nil, fmt.Errorf("子项 #%d 缺少镜像信息（image_repository/image_tag），且无法从 PipelineRun #%d 反查", item.ID, item.ItemID)
		}

		// 2. GitOps 仓库：优先 Payload.gitops_repo_id，其次按 application+env 匹配
		if v, ok := uintFromPayload(item.Payload, "gitops_repo_id"); ok {
			ri.gitopsRepoID = v
		}
		if v, ok := stringFromPayload(item.Payload, "target_branch"); ok {
			ri.targetBranch = v
		}
		if v, ok := stringFromPayload(item.Payload, "file_path"); ok {
			ri.filePath = v
		}
		applyReleaseItemHelmPayload(ri, item.Payload)
		appEnv := s.matchApplicationEnv(ctx, rel.ApplicationID, rel.ApplicationName, rel.Env)
		applyApplicationEnvToResolvedItem(ri, appEnv)
		if ri.gitopsRepoID == 0 {
			repo, err := s.matchGitOpsRepo(ctx, rel.ApplicationID, rel.ApplicationName, rel.Env, appEnv)
			if err != nil {
				return nil, fmt.Errorf("子项 #%d 自动匹配 GitOps 仓库失败: %w", item.ID, err)
			}
			ri.gitopsRepoID = repo.ID
			if ri.filePath == "" {
				ri.filePath = deriveFilePath(repo, item.Payload)
			}
			if ri.targetBranch == "" {
				ri.targetBranch = strings.TrimSpace(repo.Branch)
			}
			applyRepoDefaultsToResolvedItem(ri, repo, rel.Env)
		} else if ri.filePath == "" {
			gitopsRepoRepo := infraRepo.NewGitOpsRepoRepository(s.db)
			repo, err := gitopsRepoRepo.GetByID(ri.gitopsRepoID)
			if err != nil {
				return nil, fmt.Errorf("子项 #%d 指定的 GitOps 仓库 #%d 不存在: %w", item.ID, ri.gitopsRepoID, err)
			}
			ri.filePath = deriveFilePath(repo, item.Payload)
			if ri.targetBranch == "" {
				ri.targetBranch = strings.TrimSpace(repo.Branch)
			}
			applyRepoDefaultsToResolvedItem(ri, repo, rel.Env)
		}
		if ri.helmValuesPath == "" {
			ri.helmValuesPath = ri.filePath
		}
		if ri.filePath == "" {
			ri.filePath = ri.helmValuesPath
		}

		out = append(out, ri)
	}
	return out, nil
}

func (s *GitOpsPRService) matchApplicationEnv(ctx context.Context, appID *uint, appName, env string) *models.ApplicationEnv {
	if s == nil || s.db == nil || strings.TrimSpace(env) == "" {
		return nil
	}
	if appID != nil && *appID > 0 {
		var appEnv models.ApplicationEnv
		if err := s.db.WithContext(ctx).Where("app_id = ? AND env_name = ?", *appID, strings.TrimSpace(env)).First(&appEnv).Error; err == nil {
			return &appEnv
		}
	}
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return nil
	}
	var app models.Application
	if err := s.db.WithContext(ctx).Where("name = ?", appName).First(&app).Error; err != nil {
		return nil
	}
	var appEnv models.ApplicationEnv
	if err := s.db.WithContext(ctx).Where("app_id = ? AND env_name = ?", app.ID, strings.TrimSpace(env)).First(&appEnv).Error; err != nil {
		return nil
	}
	return &appEnv
}

func applyReleaseItemHelmPayload(ri *resolvedItem, payload map[string]any) {
	if ri == nil {
		return
	}
	if v, ok := stringFromPayload(payload, "helm_chart_path"); ok {
		ri.helmChartPath = v
	}
	if v, ok := stringFromPayload(payload, "helm_values_path"); ok {
		ri.helmValuesPath = v
		if ri.filePath == "" {
			ri.filePath = v
		}
	}
	if v, ok := stringFromPayload(payload, "helm_release_name"); ok {
		ri.helmReleaseName = v
	}
	if v, ok := intFromPayload(payload, "replicas"); ok {
		ri.replicas = v
	}
	if v, ok := stringFromPayload(payload, "cpu_request"); ok {
		ri.cpuRequest = v
	}
	if v, ok := stringFromPayload(payload, "cpu_limit"); ok {
		ri.cpuLimit = v
	}
	if v, ok := stringFromPayload(payload, "memory_request"); ok {
		ri.memoryRequest = v
	}
	if v, ok := stringFromPayload(payload, "memory_limit"); ok {
		ri.memoryLimit = v
	}
}

func applyApplicationEnvToResolvedItem(ri *resolvedItem, appEnv *models.ApplicationEnv) {
	if ri == nil || appEnv == nil {
		return
	}
	if ri.gitopsRepoID == 0 && appEnv.GitOpsRepoID != nil && *appEnv.GitOpsRepoID > 0 {
		ri.gitopsRepoID = *appEnv.GitOpsRepoID
	}
	if ri.targetBranch == "" {
		ri.targetBranch = strings.TrimSpace(appEnv.GitOpsBranch)
	}
	if ri.filePath == "" {
		ri.filePath = strings.TrimSpace(appEnv.HelmValuesPath)
	}
	if ri.helmValuesPath == "" {
		ri.helmValuesPath = strings.TrimSpace(appEnv.HelmValuesPath)
	}
	if ri.helmChartPath == "" {
		ri.helmChartPath = strings.TrimSpace(appEnv.HelmChartPath)
	}
	if ri.helmReleaseName == "" {
		ri.helmReleaseName = strings.TrimSpace(appEnv.HelmReleaseName)
	}
	if ri.replicas <= 0 {
		ri.replicas = appEnv.Replicas
	}
	if ri.cpuRequest == "" {
		ri.cpuRequest = strings.TrimSpace(appEnv.CPURequest)
	}
	if ri.cpuLimit == "" {
		ri.cpuLimit = strings.TrimSpace(appEnv.CPULimit)
	}
	if ri.memoryRequest == "" {
		ri.memoryRequest = strings.TrimSpace(appEnv.MemoryRequest)
	}
	if ri.memoryLimit == "" {
		ri.memoryLimit = strings.TrimSpace(appEnv.MemoryLimit)
	}
}

func applyRepoDefaultsToResolvedItem(ri *resolvedItem, repo *infrastructure.GitOpsRepo, env string) {
	if ri == nil || repo == nil {
		return
	}
	basePath := strings.Trim(strings.TrimSpace(repo.Path), "/")
	if ri.helmChartPath == "" {
		ri.helmChartPath = basePath
	}
	if ri.helmValuesPath == "" {
		ri.helmValuesPath = defaultReleaseHelmValuesPath(basePath, env)
	}
	if ri.filePath == "" || strings.HasSuffix(ri.filePath, "deployment.yaml") {
		ri.filePath = ri.helmValuesPath
	}
}

func defaultReleaseHelmValuesPath(basePath, env string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		env = "values"
	}
	basePath = strings.Trim(strings.TrimSpace(basePath), "/")
	if basePath == "" || basePath == "." {
		return path.Join("values", env+".yaml")
	}
	return path.Join(basePath, "values", env+".yaml")
}

// fetchPipelineRun 反查 PipelineRun（不依赖 service 包，避免循环引用）。
func (s *GitOpsPRService) fetchPipelineRun(ctx context.Context, runID uint) (*deploy.PipelineRun, error) {
	if s.db == nil {
		return nil, errors.New("db 未初始化")
	}
	var run deploy.PipelineRun
	if err := s.db.WithContext(ctx).First(&run, runID).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// matchGitOpsRepo 按 application_id 优先、application_name 兜底自动匹配 GitOps 仓库。
//
// 选择策略：sync_enabled=true 优先，其次按 ID DESC（最新）。
func (s *GitOpsPRService) matchGitOpsRepo(ctx context.Context, appID *uint, appName, env string, appEnv *models.ApplicationEnv) (*infrastructure.GitOpsRepo, error) {
	if appEnv != nil && appEnv.GitOpsRepoID != nil && *appEnv.GitOpsRepoID > 0 {
		repo, err := infraRepo.NewGitOpsRepoRepository(s.db).GetByID(*appEnv.GitOpsRepoID)
		if err != nil {
			return nil, fmt.Errorf("环境绑定的 GitOps 仓库 #%d 不存在: %w", *appEnv.GitOpsRepoID, err)
		}
		return repo, nil
	}
	appName = strings.TrimSpace(appName)
	env = strings.TrimSpace(env)
	if (appID == nil || *appID == 0) && appName == "" {
		return nil, errors.New("application_id/application_name 为空，无法匹配 GitOps 仓库")
	}
	query := s.db.WithContext(ctx).Model(&infrastructure.GitOpsRepo{})
	if appID != nil && *appID > 0 && appName != "" {
		query = query.Where("(application_id = ? OR application_name = ?)", *appID, appName)
	} else if appID != nil && *appID > 0 {
		query = query.Where("application_id = ?", *appID)
	} else {
		query = query.Where("application_name = ?", appName)
	}
	if env != "" {
		query = query.Where("(env = ? OR env = '')", env)
	}
	var repo infrastructure.GitOpsRepo
	if err := query.Order("sync_enabled DESC, id DESC").First(&repo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未匹配到应用 ID `%s` / 名称 `%s` 环境 `%s` 的 GitOps 仓库（请先在 GitOps 仓库管理中创建）", formatUintPtr(appID), appName, env)
		}
		return nil, err
	}
	return &repo, nil
}

// applySuccess 真实 PR 创建成功后，回填 Release 状态。
func (s *GitOpsPRService) applySuccess(rel *deploy.Release, changeRequestID uint) {
	rel.GitOpsChangeRequestID = &changeRequestID
	rel.Status = deploy.ReleaseStatusPROpened
	if err := s.releaseRepo.Update(rel); err != nil {
		// 仅记录，不打断主流程；调用方仍能看到 ChangeRequestID
		_ = err
	}
}

// applyPartialFailure 部分失败时，仍把已创建的首个 ChangeRequestID 回填，避免审计断链。
func (s *GitOpsPRService) applyPartialFailure(rel *deploy.Release, changeRequestID *uint, _ string) {
	if changeRequestID == nil {
		return
	}
	rel.GitOpsChangeRequestID = changeRequestID
	rel.Status = deploy.ReleaseStatusPROpened
	_ = s.releaseRepo.Update(rel)
}

// deriveBranchName 推导 PR 分支名。
func (s *GitOpsPRService) deriveBranchName(rel *deploy.Release, override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	return fmt.Sprintf("release/%s/%d-%s", strings.ToLower(rel.Env), rel.ID, time.Now().Format("20060102-150405"))
}

// ---------- 辅助函数 ----------

// splitImage 把 "registry.example.com/team/app:1.2.3" 切成 ("registry.example.com/team/app", "1.2.3")。
// 不支持 sha256 摘要形式（@sha256:...），返回时 tag 为空，由调用方兜底用 GitCommit。
func splitImage(image string) (string, string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	if strings.Contains(image, "@") {
		parts := strings.SplitN(image, "@", 2)
		return parts[0], ""
	}
	idx := strings.LastIndex(image, ":")
	// idx 必须在最后一个 / 之后才算 tag（防止把端口号当 tag）
	lastSlash := strings.LastIndex(image, "/")
	if idx > lastSlash && idx > -1 {
		return image[:idx], image[idx+1:]
	}
	return image, ""
}

// shortCommit 取前 8 位作为镜像 tag 兜底。
func shortCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}

// deriveFilePath 推导 GitOps 清单文件路径。
func deriveFilePath(repo *infrastructure.GitOpsRepo, payload map[string]any) string {
	if v, ok := stringFromPayload(payload, "file_path"); ok {
		return v
	}
	basePath := strings.Trim(strings.TrimSpace(repo.Path), "/")
	if basePath == "" || basePath == "." {
		return "deployment.yaml"
	}
	return basePath + "/deployment.yaml"
}

// buildChangeRequestTitle 生成 PR 标题。
func buildChangeRequestTitle(rel *deploy.Release, r *resolvedItem) string {
	return fmt.Sprintf("chore(release): %s [#%d] %s", rel.Title, rel.ID, r.imageTag)
}

// buildChangeRequestDescription 生成 PR 描述。
func buildChangeRequestDescription(rel *deploy.Release, r *resolvedItem) string {
	lines := []string{
		fmt.Sprintf("发布主单 #%d %s", rel.ID, rel.Title),
		fmt.Sprintf("应用: %s", fallback(rel.ApplicationName, "-")),
		fmt.Sprintf("环境: %s", fallback(rel.Env, "-")),
		fmt.Sprintf("镜像: %s:%s", r.imageRepo, r.imageTag),
		fmt.Sprintf("策略: %s", fallback(rel.RolloutStrategy, deploy.RolloutStrategyDirect)),
	}
	if rel.JiraIssueKeys != "" {
		lines = append(lines, "Jira: "+rel.JiraIssueKeys)
	}
	if r.pipelineRun != nil {
		lines = append(lines, fmt.Sprintf("PipelineRun: #%d (%s)", r.pipelineRun.ID, r.pipelineRun.GitCommit))
	}
	if rel.Description != "" {
		lines = append(lines, "", rel.Description)
	}
	return strings.Join(lines, "\n")
}

func fallback(value, dft string) string {
	if strings.TrimSpace(value) == "" {
		return dft
	}
	return strings.TrimSpace(value)
}

// stringFromPayload 从 JSONMap 中读取字符串字段。
func stringFromPayload(payload map[string]any, key string) (string, bool) {
	if payload == nil {
		return "", false
	}
	v, ok := payload[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

// uintFromPayload 从 JSONMap 中读取无符号整型字段（兼容 JSON 反序列化的 float64）。
func uintFromPayload(payload map[string]any, key string) (uint, bool) {
	if payload == nil {
		return 0, false
	}
	v, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if n <= 0 {
			return 0, false
		}
		return uint(n), true
	case int:
		if n <= 0 {
			return 0, false
		}
		return uint(n), true
	case int64:
		if n <= 0 {
			return 0, false
		}
		return uint(n), true
	case uint:
		if n == 0 {
			return 0, false
		}
		return n, true
	case string:
		id, err := strconv.ParseUint(strings.TrimSpace(n), 10, 64)
		if err != nil || id == 0 {
			return 0, false
		}
		return uint(id), true
	}
	return 0, false
}

func intFromPayload(payload map[string]any, key string) (int, bool) {
	if payload == nil {
		return 0, false
	}
	v, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if n <= 0 {
			return 0, false
		}
		return int(n), true
	case int:
		if n <= 0 {
			return 0, false
		}
		return n, true
	case int64:
		if n <= 0 {
			return 0, false
		}
		return int(n), true
	case uint:
		if n == 0 {
			return 0, false
		}
		return int(n), true
	case string:
		id, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil || id <= 0 {
			return 0, false
		}
		return id, true
	}
	return 0, false
}

func formatUintPtr(id *uint) string {
	if id == nil || *id == 0 {
		return "-"
	}
	return strconv.FormatUint(uint64(*id), 10)
}
