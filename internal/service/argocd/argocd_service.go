package argocd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"devops/internal/models/infrastructure"
	approvalRepo "devops/internal/modules/approval/repository"
	infraRepo "devops/internal/modules/infrastructure/repository"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

const ChangeRequestApprovalRecordOffset uint = 2000000000

type Service struct {
	instRepo         *infraRepo.ArgoCDInstanceRepository
	appRepo          *infraRepo.ArgoCDApplicationRepository
	repoRepo         *infraRepo.GitOpsRepoRepository
	changeRepo       *infraRepo.GitOpsChangeRequestRepository
	envPolicyRepo    *approvalRepo.EnvAuditPolicyRepository
	sonarBindingRepo *infraRepo.SonarQubeBindingRepository
}

func NewService(instRepo *infraRepo.ArgoCDInstanceRepository, appRepo *infraRepo.ArgoCDApplicationRepository, repoRepo *infraRepo.GitOpsRepoRepository, changeRepo *infraRepo.GitOpsChangeRequestRepository) *Service {
	return &Service{instRepo: instRepo, appRepo: appRepo, repoRepo: repoRepo, changeRepo: changeRepo}
}

func (s *Service) SetEnvPolicyRepo(repo *approvalRepo.EnvAuditPolicyRepository) {
	s.envPolicyRepo = repo
}

func (s *Service) SetSonarBindingRepo(repo *infraRepo.SonarQubeBindingRepository) {
	s.sonarBindingRepo = repo
}

type DashboardSummary struct {
	InstanceTotal       int64 `json:"instance_total"`
	InstanceActive      int64 `json:"instance_active"`
	AppTotal            int64 `json:"app_total"`
	AppSynced           int64 `json:"app_synced"`
	AppOutOfSync        int64 `json:"app_out_of_sync"`
	AppHealthy          int64 `json:"app_healthy"`
	AppDegraded         int64 `json:"app_degraded"`
	AppDrifted          int64 `json:"app_drifted"`
	AppAutoSync         int64 `json:"app_auto_sync"`
	RepoTotal           int64 `json:"repo_total"`
	RepoSyncEnabled     int64 `json:"repo_sync_enabled"`
	ChangeRequestOpen   int64 `json:"change_request_open"`
	ChangeRequestDraft  int64 `json:"change_request_draft"`
	ChangeRequestFailed int64 `json:"change_request_failed"`
}

type CreateChangeRequestInput struct {
	GitOpsRepoID        uint   `json:"gitops_repo_id"`
	ArgoCDApplicationID *uint  `json:"argocd_application_id"`
	ApplicationID       *uint  `json:"application_id"`
	ApplicationName     string `json:"application_name"`
	Env                 string `json:"env"`
	PipelineID          *uint  `json:"pipeline_id"`
	PipelineRunID       *uint  `json:"pipeline_run_id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	FilePath            string `json:"file_path"`
	ImageRepository     string `json:"image_repository"`
	ImageTag            string `json:"image_tag"`
	HelmChartPath       string `json:"helm_chart_path"`
	HelmValuesPath      string `json:"helm_values_path"`
	HelmReleaseName     string `json:"helm_release_name"`
	Replicas            int    `json:"replicas"`
	CPURequest          string `json:"cpu_request"`
	CPULimit            string `json:"cpu_limit"`
	MemoryRequest       string `json:"memory_request"`
	MemoryLimit         string `json:"memory_limit"`
	TargetBranch        string `json:"target_branch"`
}

type ChangeRequestPrecheck struct {
	CanCreate bool                        `json:"can_create"`
	Policy    *ChangeRequestPolicySummary `json:"policy,omitempty"`
	Checks    []ChangeRequestPrecheckItem `json:"checks"`
}

type ChangeRequestPolicySummary struct {
	EnvName             string `json:"env_name"`
	RequireApproval     bool   `json:"require_approval"`
	RequireChain        bool   `json:"require_chain"`
	RequireCodeReview   bool   `json:"require_code_review"`
	RequireTestPass     bool   `json:"require_test_pass"`
	RequireDeployWindow bool   `json:"require_deploy_window"`
}

type ChangeRequestPrecheckItem struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Detail   string `json:"detail,omitempty"`
}

// --- Instance CRUD ---

func (s *Service) ListInstances() ([]infrastructure.ArgoCDInstance, error) {
	list, err := s.instRepo.List()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].AuthToken = maskToken(list[i].AuthToken)
	}
	return list, nil
}

func (s *Service) GetInstance(id uint) (*infrastructure.ArgoCDInstance, error) {
	inst, err := s.instRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	inst.AuthToken = ""
	return inst, nil
}

func (s *Service) CreateInstance(inst *infrastructure.ArgoCDInstance) error {
	if inst.AuthToken != "" {
		enc, err := encryptToken(inst.AuthToken)
		if err != nil {
			return fmt.Errorf("加密 Token 失败: %w", err)
		}
		inst.AuthToken = enc
	}
	return s.instRepo.Create(inst)
}

func (s *Service) UpdateInstance(inst *infrastructure.ArgoCDInstance) error {
	old, err := s.instRepo.GetByID(inst.ID)
	if err == nil && old != nil {
		inst.CreatedAt = old.CreatedAt
		inst.CreatedBy = old.CreatedBy
	}

	if inst.AuthToken == "" {
		if old != nil {
			inst.AuthToken = old.AuthToken
		}
	} else {
		enc, err := encryptToken(inst.AuthToken)
		if err != nil {
			return fmt.Errorf("加密 Token 失败: %w", err)
		}
		inst.AuthToken = enc
	}
	return s.instRepo.Update(inst)
}

func (s *Service) DeleteInstance(id uint) error {
	return s.instRepo.Delete(id)
}

func (s *Service) TestConnection(id uint) error {
	inst, err := s.instRepo.GetByID(id)
	if err != nil {
		return err
	}
	token := decryptTokenWithLegacyFallback(inst.AuthToken)
	client := NewArgoCDClient(inst.ServerURL, token, inst.Insecure)
	return client.TestConnection()
}

func (s *Service) DashboardSummary(projectID *uint) (*DashboardSummary, error) {
	summary := &DashboardSummary{}

	instances, err := s.instRepo.List()
	if err != nil {
		return nil, err
	}
	summary.InstanceTotal = int64(len(instances))
	for _, inst := range instances {
		if strings.EqualFold(inst.Status, "active") {
			summary.InstanceActive++
		}
	}

	appSummary, err := s.appRepo.Summary(projectID)
	if err != nil {
		return nil, err
	}
	summary.AppTotal = appSummary.Total
	summary.AppSynced = appSummary.Synced
	summary.AppOutOfSync = appSummary.OutOfSync
	summary.AppHealthy = appSummary.Healthy
	summary.AppDegraded = appSummary.Degraded
	summary.AppDrifted = appSummary.Drifted
	summary.AppAutoSync = appSummary.AutoSync

	_, repoTotal, err := s.repoRepo.List(projectID, 1, 1)
	if err != nil {
		return nil, err
	}
	summary.RepoTotal = repoTotal

	repoSyncEnabled, err := s.repoRepo.CountSyncEnabled()
	if err != nil {
		return nil, err
	}
	summary.RepoSyncEnabled = repoSyncEnabled

	if s.changeRepo != nil {
		if summary.ChangeRequestOpen, err = s.changeRepo.CountByStatus(projectID, "open"); err != nil {
			return nil, err
		}
		if summary.ChangeRequestDraft, err = s.changeRepo.CountByStatus(projectID, "draft"); err != nil {
			return nil, err
		}
		if summary.ChangeRequestFailed, err = s.changeRepo.CountByStatus(projectID, "failed"); err != nil {
			return nil, err
		}
	}

	return summary, nil
}

// --- Application Management ---

func (s *Service) ListApplications(f infraRepo.ArgoCDAppFilter, page, pageSize int) ([]infrastructure.ArgoCDApplication, int64, error) {
	return s.appRepo.List(f, page, pageSize)
}

func (s *Service) GetApplication(id uint) (*infrastructure.ArgoCDApplication, error) {
	return s.appRepo.GetByID(id)
}

// SyncFromArgoCD 从 Argo CD 拉取应用列表并同步到本地
func (s *Service) SyncFromArgoCD(instanceID uint) (int, error) {
	inst, err := s.instRepo.GetByID(instanceID)
	if err != nil {
		return 0, fmt.Errorf("实例不存在: %w", err)
	}
	token := decryptTokenWithLegacyFallback(inst.AuthToken)
	client := NewArgoCDClient(inst.ServerURL, token, inst.Insecure)

	apps, err := client.ListApplications()
	if err != nil {
		return 0, fmt.Errorf("获取应用列表失败: %w", err)
	}

	existing, err := s.appRepo.ListByInstance(instanceID)
	if err != nil {
		return 0, fmt.Errorf("查询本地 Argo CD 应用失败: %w", err)
	}
	existingByName := make(map[string]*infrastructure.ArgoCDApplication, len(existing))
	for i := range existing {
		existingByName[existing[i].Name] = &existing[i]
	}

	count := 0
	for _, argoApp := range apps {
		syncPolicy := "manual"
		if argoApp.Spec.SyncPolicy != nil && argoApp.Spec.SyncPolicy.Automated != nil {
			syncPolicy = "auto"
		}
		driftDetected := argoApp.Status.Sync.Status == "OutOfSync"

		found := existingByName[argoApp.Metadata.Name]

		if found != nil {
			found.SyncStatus = argoApp.Status.Sync.Status
			found.HealthStatus = argoApp.Status.Health.Status
			found.SyncPolicy = syncPolicy
			found.DriftDetected = driftDetected
			found.RepoURL = argoApp.Spec.Source.RepoURL
			found.RepoPath = argoApp.Spec.Source.Path
			found.TargetRevision = argoApp.Spec.Source.TargetRevision
			found.DestServer = argoApp.Spec.Destination.Server
			found.DestNamespace = argoApp.Spec.Destination.Namespace
			if argoApp.Status.OperationState != nil && argoApp.Status.OperationState.FinishedAt != "" {
				if t, err := time.Parse(time.RFC3339, argoApp.Status.OperationState.FinishedAt); err == nil {
					found.LastSyncAt = &t
				}
			}
			_ = s.appRepo.Update(found)
		} else {
			newApp := &infrastructure.ArgoCDApplication{
				ArgoCDInstanceID: instanceID,
				Name:             argoApp.Metadata.Name,
				Project:          argoApp.Spec.Project,
				RepoURL:          argoApp.Spec.Source.RepoURL,
				RepoPath:         argoApp.Spec.Source.Path,
				TargetRevision:   argoApp.Spec.Source.TargetRevision,
				DestServer:       argoApp.Spec.Destination.Server,
				DestNamespace:    argoApp.Spec.Destination.Namespace,
				SyncStatus:       argoApp.Status.Sync.Status,
				HealthStatus:     argoApp.Status.Health.Status,
				SyncPolicy:       syncPolicy,
				DriftDetected:    driftDetected,
			}
			_ = s.appRepo.Create(newApp)
			existingByName[newApp.Name] = newApp
		}
		count++
	}
	return count, nil
}

// TriggerSync 触发指定应用同步
func (s *Service) TriggerSync(appID uint) error {
	app, err := s.appRepo.GetByID(appID)
	if err != nil {
		return fmt.Errorf("应用不存在: %w", err)
	}
	inst, err := s.instRepo.GetByID(app.ArgoCDInstanceID)
	if err != nil {
		return fmt.Errorf("Argo CD 实例不存在: %w", err)
	}
	token := decryptTokenWithLegacyFallback(inst.AuthToken)
	client := NewArgoCDClient(inst.ServerURL, token, inst.Insecure)
	if err := client.SyncApplication(app.Name); err != nil {
		return fmt.Errorf("同步失败: %w", err)
	}
	now := time.Now()
	app.LastSyncAt = &now
	app.SyncStatus = "Synced"
	app.DriftDetected = false
	_ = s.appRepo.Update(app)
	return nil
}

// GetResourceTree 获取应用资源树
func (s *Service) GetResourceTree(appID uint) ([]ResourceNode, error) {
	app, err := s.appRepo.GetByID(appID)
	if err != nil {
		return nil, fmt.Errorf("应用不存在: %w", err)
	}
	inst, err := s.instRepo.GetByID(app.ArgoCDInstanceID)
	if err != nil {
		return nil, fmt.Errorf("Argo CD 实例不存在: %w", err)
	}
	token := decryptTokenWithLegacyFallback(inst.AuthToken)
	client := NewArgoCDClient(inst.ServerURL, token, inst.Insecure)
	return client.GetResourceTree(app.Name)
}

// --- GitOps Repo ---

func (s *Service) ListRepos(projectID *uint, page, pageSize int) ([]infrastructure.GitOpsRepo, int64, error) {
	list, total, err := s.repoRepo.List(projectID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	for i := range list {
		list[i].AuthCredential = maskToken(list[i].AuthCredential)
	}
	return list, total, nil
}

func (s *Service) GetRepo(id uint) (*infrastructure.GitOpsRepo, error) {
	repo, err := s.repoRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	repo.AuthCredential = ""
	return repo, nil
}

func (s *Service) CreateRepo(repo *infrastructure.GitOpsRepo) error {
	if repo.AuthCredential != "" {
		enc, err := encryptToken(repo.AuthCredential)
		if err != nil {
			return fmt.Errorf("加密凭证失败: %w", err)
		}
		repo.AuthCredential = enc
	}
	return s.repoRepo.Create(repo)
}

func (s *Service) UpdateRepo(repo *infrastructure.GitOpsRepo) error {
	old, err := s.repoRepo.GetByID(repo.ID)
	if err == nil && old != nil {
		repo.CreatedAt = old.CreatedAt
		repo.CreatedBy = old.CreatedBy
	}
	if repo.AuthCredential == "" {
		if old != nil {
			repo.AuthCredential = old.AuthCredential
		}
	} else {
		enc, err := encryptToken(repo.AuthCredential)
		if err != nil {
			return fmt.Errorf("加密凭证失败: %w", err)
		}
		repo.AuthCredential = enc
	}
	return s.repoRepo.Update(repo)
}

func decryptTokenWithLegacyFallback(value string) string {
	plaintext, err := decryptToken(value)
	if err != nil {
		return value
	}
	return plaintext
}

func (s *Service) DeleteRepo(id uint) error {
	return s.repoRepo.Delete(id)
}

// --- GitOps Change Request ---

func (s *Service) ListChangeRequests(projectID *uint, page, pageSize int) ([]infrastructure.GitOpsChangeRequest, int64, error) {
	return s.changeRepo.List(projectID, page, pageSize)
}

func (s *Service) GetChangeRequest(id uint) (*infrastructure.GitOpsChangeRequest, error) {
	return s.changeRepo.GetByID(id)
}

func (s *Service) GetChangeRequestByApprovalInstanceID(approvalInstanceID uint) (*infrastructure.GitOpsChangeRequest, error) {
	return s.changeRepo.GetByApprovalInstanceID(approvalInstanceID)
}

func (s *Service) PrecheckChangeRequest(input *CreateChangeRequestInput) (*ChangeRequestPrecheck, error) {
	precheck := &ChangeRequestPrecheck{
		CanCreate: true,
		Checks:    make([]ChangeRequestPrecheckItem, 0, 4),
	}
	if input.GitOpsRepoID == 0 {
		precheck.CanCreate = false
		precheck.Checks = append(precheck.Checks, ChangeRequestPrecheckItem{
			Key:      "gitops_repo",
			Name:     "GitOps 仓库",
			Required: true,
			Passed:   false,
			Message:  "必须选择 GitOps 仓库",
		})
		return precheck, nil
	}

	repo, err := s.repoRepo.GetByID(input.GitOpsRepoID)
	if err != nil {
		return nil, fmt.Errorf("GitOps 仓库不存在: %w", err)
	}
	envName := strings.TrimSpace(input.Env)
	if envName == "" {
		envName = strings.TrimSpace(repo.Env)
	}
	appName := strings.TrimSpace(input.ApplicationName)
	if appName == "" {
		appName = strings.TrimSpace(repo.ApplicationName)
	}

	precheck.Checks = append(precheck.Checks, ChangeRequestPrecheckItem{
		Key:      "repo_access",
		Name:     "仓库接入",
		Required: true,
		Passed:   true,
		Message:  fmt.Sprintf("已选择 GitOps 仓库 `%s`，默认分支 `%s`", repo.Name, repo.Branch),
	})

	if s.envPolicyRepo == nil || envName == "" {
		return precheck, nil
	}

	policy, err := s.envPolicyRepo.GetByEnvName(envName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return precheck, nil
		}
		return nil, err
	}

	precheck.Policy = &ChangeRequestPolicySummary{
		EnvName:             policy.EnvName,
		RequireApproval:     policy.RequireApproval,
		RequireChain:        policy.RequireChain,
		RequireCodeReview:   policy.RequireCodeReview,
		RequireTestPass:     policy.RequireTestPass,
		RequireDeployWindow: policy.RequireDeployWindow,
	}

	if policy.RequireCodeReview {
		passed := detectGitProvider(repo.RepoURL) == "gitlab" && strings.EqualFold(repo.AuthType, "token")
		item := ChangeRequestPrecheckItem{
			Key:      "code_review",
			Name:     "代码评审门禁",
			Required: true,
			Passed:   passed,
		}
		if passed {
			item.Message = "当前仓库支持以 GitLab Merge Request 方式发起评审"
			item.Detail = "已满足生成 MR 并挂接审批链的技术前提"
		} else {
			item.Message = "环境策略要求代码评审，但当前仓库未满足 GitLab + Token 的 MR 交付前提"
			item.Detail = "请使用支持 Merge Request 的 GitLab Token 仓库"
			precheck.CanCreate = false
		}
		precheck.Checks = append(precheck.Checks, item)
	}

	if policy.RequireTestPass {
		item := ChangeRequestPrecheckItem{
			Key:      "test_pass",
			Name:     "测试质量门禁",
			Required: true,
			Passed:   false,
		}
		switch {
		case s.sonarBindingRepo == nil:
			item.Message = "未配置 SonarQube 绑定仓储，无法校验质量门"
			precheck.CanCreate = false
		case appName == "":
			item.Message = "环境策略要求测试通过，但当前变更未关联应用名称"
			item.Detail = "请补充应用名称后重试"
			precheck.CanCreate = false
		default:
			binding, bindErr := s.sonarBindingRepo.GetByAppName(appName)
			if bindErr != nil {
				if errors.Is(bindErr, gorm.ErrRecordNotFound) {
					item.Message = fmt.Sprintf("应用 `%s` 未绑定 SonarQube 项目，无法验证质量门", appName)
					precheck.CanCreate = false
				} else {
					return nil, bindErr
				}
			} else {
				status := strings.ToUpper(strings.TrimSpace(binding.QualityGateStatus))
				item.Detail = fmt.Sprintf("Sonar 项目 `%s` 当前质量门状态: %s", binding.SonarProjectKey, fallbackText(status, "UNKNOWN"))
				if status == "OK" || status == "PASS" || status == "PASSED" || status == "SUCCESS" {
					item.Passed = true
					item.Message = "质量门通过"
				} else {
					item.Message = "质量门未通过，当前环境策略不允许创建变更请求"
					precheck.CanCreate = false
				}
			}
		}
		precheck.Checks = append(precheck.Checks, item)
	}

	return precheck, nil
}

func (s *Service) CreateChangeRequest(ctx context.Context, input *CreateChangeRequestInput, createdBy *uint) (*infrastructure.GitOpsChangeRequest, error) {
	if input.GitOpsRepoID == 0 {
		return nil, fmt.Errorf("请选择 GitOps 仓库")
	}
	if strings.TrimSpace(input.FilePath) == "" {
		return nil, fmt.Errorf("请输入清单文件路径")
	}
	if strings.TrimSpace(input.ImageRepository) == "" {
		return nil, fmt.Errorf("请输入镜像仓库地址")
	}
	if strings.TrimSpace(input.ImageTag) == "" {
		return nil, fmt.Errorf("请输入镜像标签")
	}

	repo, err := s.repoRepo.GetByID(input.GitOpsRepoID)
	if err != nil {
		return nil, fmt.Errorf("GitOps 仓库不存在: %w", err)
	}

	targetBranch := strings.TrimSpace(input.TargetBranch)
	if targetBranch == "" {
		targetBranch = repo.Branch
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = fmt.Sprintf("chore(gitops): update %s to %s", strings.TrimSpace(input.ApplicationName), strings.TrimSpace(input.ImageTag))
	}

	item := &infrastructure.GitOpsChangeRequest{
		GitOpsRepoID:        repo.ID,
		ArgoCDApplicationID: input.ArgoCDApplicationID,
		ApplicationID:       input.ApplicationID,
		ApplicationName:     strings.TrimSpace(input.ApplicationName),
		Env:                 strings.TrimSpace(input.Env),
		PipelineID:          input.PipelineID,
		PipelineRunID:       input.PipelineRunID,
		Title:               title,
		Description:         strings.TrimSpace(input.Description),
		FilePath:            strings.TrimSpace(strings.TrimPrefix(input.FilePath, "/")),
		ImageRepository:     strings.TrimSpace(input.ImageRepository),
		ImageTag:            strings.TrimSpace(input.ImageTag),
		HelmChartPath:       strings.TrimSpace(strings.TrimPrefix(input.HelmChartPath, "/")),
		HelmValuesPath:      strings.TrimSpace(strings.TrimPrefix(input.HelmValuesPath, "/")),
		HelmReleaseName:     strings.TrimSpace(input.HelmReleaseName),
		Replicas:            input.Replicas,
		CPURequest:          strings.TrimSpace(input.CPURequest),
		CPULimit:            strings.TrimSpace(input.CPULimit),
		MemoryRequest:       strings.TrimSpace(input.MemoryRequest),
		MemoryLimit:         strings.TrimSpace(input.MemoryLimit),
		TargetBranch:        targetBranch,
		Status:              "draft",
		ApprovalStatus:      "none",
		AutoMergeStatus:     "manual",
		CreatedBy:           createdBy,
		Provider:            detectGitProvider(repo.RepoURL),
	}

	item.SourceBranch = buildChangeRequestBranch(item)

	if s.changeRepo == nil {
		return nil, fmt.Errorf("变更请求仓库未初始化")
	}
	if err := s.changeRepo.Create(item); err != nil {
		return nil, err
	}

	if err := s.submitChangeRequestToProvider(ctx, repo, item); err != nil {
		if isNoopGitOpsChangeError(err) {
			item.Status = "skipped"
			item.AutoMergeStatus = "skipped"
		} else {
			item.Status = "failed"
			item.AutoMergeStatus = "failed"
		}
		item.ErrorMessage = err.Error()
		_ = s.changeRepo.Update(item)
		return item, nil
	}

	item.Status = "open"
	item.ErrorMessage = ""
	if err := s.changeRepo.Update(item); err != nil {
		return nil, err
	}
	return item, nil
}

func BuildChangeRequestApprovalRecordID(changeRequestID uint) uint {
	return ChangeRequestApprovalRecordOffset + changeRequestID
}

func ResolveChangeRequestIDFromApprovalRecord(recordID uint) (uint, bool) {
	if recordID < ChangeRequestApprovalRecordOffset {
		return 0, false
	}
	return recordID - ChangeRequestApprovalRecordOffset, true
}

func (s *Service) AttachApproval(changeRequestID uint, instanceID uint, chainID uint, chainName string) error {
	item, err := s.changeRepo.GetByID(changeRequestID)
	if err != nil {
		return err
	}
	item.ApprovalInstanceID = &instanceID
	item.ApprovalChainID = &chainID
	item.ApprovalChainName = chainName
	item.ApprovalStatus = "pending"
	item.AutoMergeStatus = "pending"
	return s.changeRepo.Update(item)
}

func (s *Service) NotifyApprovalPending(ctx context.Context, changeRequestID uint) error {
	item, err := s.changeRepo.GetByID(changeRequestID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(item.MergeRequestIID) == "" {
		return nil
	}
	if err := s.addMergeRequestComment(ctx, item, fmt.Sprintf("GitOps 变更请求已进入审批流程，审批实例 #%d。审批通过后平台将自动尝试合并该 MR。", derefUint(item.ApprovalInstanceID))); err != nil {
		return err
	}
	return s.updateMergeRequestLabels(ctx, item, []string{"gitops-change", "approval-pending"}, []string{"approval-approved", "approval-rejected", "approval-cancelled", "auto-merge-success", "auto-merge-failed"})
}

func (s *Service) HandleApprovalApproved(ctx context.Context, approvalRecordID uint) error {
	changeRequestID, ok := ResolveChangeRequestIDFromApprovalRecord(approvalRecordID)
	if !ok {
		return nil
	}
	item, err := s.changeRepo.GetByID(changeRequestID)
	if err != nil {
		return err
	}

	now := time.Now()
	item.ApprovalStatus = "approved"
	item.ApprovalFinishedAt = &now
	_ = s.addMergeRequestComment(ctx, item, "审批已通过，平台开始自动合并该 GitOps 变更请求。")
	_ = s.updateMergeRequestLabels(ctx, item, []string{"gitops-change", "approval-approved"}, []string{"approval-pending", "approval-rejected", "approval-cancelled"})

	if strings.TrimSpace(item.MergeRequestIID) == "" {
		item.AutoMergeStatus = "skipped"
		return s.changeRepo.Update(item)
	}

	if err := s.mergeApprovedChangeRequest(ctx, item); err != nil {
		item.AutoMergeStatus = "failed"
		item.Status = "open"
		item.ErrorMessage = err.Error()
		_ = s.addMergeRequestComment(ctx, item, "审批已通过，但平台自动合并失败，请人工检查。错误信息: "+err.Error())
		_ = s.updateMergeRequestLabels(ctx, item, []string{"auto-merge-failed"}, []string{"auto-merge-success"})
		_ = s.changeRepo.Update(item)
		return err
	}

	item.AutoMergeStatus = "success"
	item.AutoMergedAt = &now
	item.Status = "merged"
	item.ErrorMessage = ""
	_ = s.addMergeRequestComment(ctx, item, "审批已通过，平台已自动合并该 GitOps 变更请求。")
	_ = s.updateMergeRequestLabels(ctx, item, []string{"auto-merge-success"}, []string{"auto-merge-failed"})
	return s.changeRepo.Update(item)
}

func (s *Service) HandleApprovalRejected(ctx context.Context, approvalRecordID uint, reason string) error {
	changeRequestID, ok := ResolveChangeRequestIDFromApprovalRecord(approvalRecordID)
	if !ok {
		return nil
	}
	item, err := s.changeRepo.GetByID(changeRequestID)
	if err != nil {
		return err
	}
	now := time.Now()
	item.ApprovalStatus = "rejected"
	item.ApprovalFinishedAt = &now
	item.AutoMergeStatus = "skipped"
	item.Status = "rejected"
	if strings.TrimSpace(reason) != "" {
		item.ErrorMessage = reason
	}
	_ = s.addMergeRequestComment(ctx, item, "审批已拒绝，平台不会自动合并该 GitOps 变更请求。原因: "+fallbackText(reason, "未填写"))
	_ = s.updateMergeRequestLabels(ctx, item, []string{"approval-rejected"}, []string{"approval-pending", "approval-approved", "approval-cancelled", "auto-merge-success", "auto-merge-failed"})
	return s.changeRepo.Update(item)
}

func (s *Service) HandleApprovalCancelled(ctx context.Context, approvalRecordID uint, reason string) error {
	changeRequestID, ok := ResolveChangeRequestIDFromApprovalRecord(approvalRecordID)
	if !ok {
		return nil
	}
	item, err := s.changeRepo.GetByID(changeRequestID)
	if err != nil {
		return err
	}
	now := time.Now()
	item.ApprovalStatus = "cancelled"
	item.ApprovalFinishedAt = &now
	item.AutoMergeStatus = "skipped"
	item.Status = "cancelled"
	if strings.TrimSpace(reason) != "" {
		item.ErrorMessage = reason
	}
	_ = s.addMergeRequestComment(ctx, item, "审批已取消，平台不会自动合并该 GitOps 变更请求。原因: "+fallbackText(reason, "未填写"))
	_ = s.updateMergeRequestLabels(ctx, item, []string{"approval-cancelled"}, []string{"approval-pending", "approval-approved", "approval-rejected", "auto-merge-success", "auto-merge-failed"})
	return s.changeRepo.Update(item)
}

func detectGitProvider(repoURL string) string {
	lowerURL := strings.ToLower(repoURL)
	switch {
	case strings.Contains(lowerURL, "gitlab.com"), strings.Contains(lowerURL, "gitlab"):
		return "gitlab"
	case strings.Contains(lowerURL, "github.com"):
		return "github"
	case strings.Contains(lowerURL, "gitee.com"):
		return "gitee"
	default:
		return "custom"
	}
}

func buildChangeRequestBranch(item *infrastructure.GitOpsChangeRequest) string {
	name := strings.ToLower(strings.TrimSpace(item.ApplicationName))
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	if name == "" {
		name = "app"
	}
	return fmt.Sprintf("gitops/%s/%d", name, time.Now().Unix())
}

func (s *Service) submitChangeRequestToProvider(ctx context.Context, repo *infrastructure.GitOpsRepo, item *infrastructure.GitOpsChangeRequest) error {
	switch detectGitProvider(repo.RepoURL) {
	case "gitlab":
		return s.submitGitLabChangeRequest(ctx, repo, item)
	default:
		return fmt.Errorf("当前仅支持 GitLab 仓库自动创建变更请求")
	}
}

func (s *Service) submitGitLabChangeRequest(ctx context.Context, repo *infrastructure.GitOpsRepo, item *infrastructure.GitOpsChangeRequest) error {
	if strings.TrimSpace(repo.AuthType) != "token" {
		return fmt.Errorf("当前仅支持使用 Token 凭证的 GitLab GitOps 仓库")
	}

	token, err := decryptToken(repo.AuthCredential)
	if err != nil {
		return fmt.Errorf("解密 GitOps 仓库凭证失败: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("GitOps 仓库缺少可用 Token")
	}

	baseURL, projectPath, err := parseGitLabRepositoryURL(repo.RepoURL)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	if err := createGitLabBranch(ctx, client, baseURL, projectPath, item.SourceBranch, item.TargetBranch, token); err != nil {
		return err
	}

	currentContent, lastCommitID, err := getGitLabFileContent(ctx, client, baseURL, projectPath, item.FilePath, item.TargetBranch, token)
	if err != nil {
		return err
	}

	updatedContent, err := renderGitOpsFileContent(currentContent, item)
	if err != nil {
		return err
	}
	if updatedContent == currentContent {
		return fmt.Errorf("未检测到镜像内容变化，请确认仓库文件与镜像仓库地址匹配")
	}

	commitSHA, err := updateGitLabFile(ctx, client, baseURL, projectPath, item.FilePath, item.SourceBranch, token, updatedContent, buildCommitMessage(item), lastCommitID)
	if err != nil {
		return err
	}
	item.LastCommitSHA = commitSHA
	hasDiff, err := gitLabBranchHasDiff(ctx, client, baseURL, projectPath, item.TargetBranch, item.SourceBranch, token)
	if err != nil {
		return err
	}
	if !hasDiff {
		_ = deleteGitLabBranch(ctx, client, baseURL, projectPath, item.SourceBranch, token)
		return fmt.Errorf("GitOps 清单已是目标镜像版本，无需重复创建变更请求")
	}

	mrIID, mrURL, err := createGitLabMergeRequest(ctx, client, baseURL, projectPath, token, item)
	if err != nil {
		return err
	}
	item.MergeRequestIID = mrIID
	item.MergeRequestURL = mrURL
	return nil
}

func parseGitLabRepositoryURL(repoURL string) (string, string, error) {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("当前仅支持 HTTP/HTTPS GitLab 仓库地址")
	}

	projectPath := strings.Trim(strings.TrimSuffix(parsed.Path, ".git"), "/")
	if projectPath == "" {
		return "", "", fmt.Errorf("GitLab 仓库路径为空")
	}

	host := parsed.Host
	if strings.TrimSpace(os.Getenv("MYSQL_HOST")) == "mysql" {
		switch strings.ToLower(parsed.Hostname()) {
		case "localhost", "127.0.0.1", "::1":
			host = "gitlab"
			if parsed.Port() != "" {
				host += ":" + parsed.Port()
			}
		}
	}

	return fmt.Sprintf("%s://%s", parsed.Scheme, host), projectPath, nil
}

func replaceImageTag(content, imageRepository, imageTag string) (string, error) {
	escapedRepo := regexp.QuoteMeta(strings.TrimSpace(imageRepository))
	pattern := regexp.MustCompile(`(?m)^(\s*image\s*:\s*["']?)` + escapedRepo + `(?::[^"'\s]+)?(["']?\s*)$`)
	replaced := pattern.ReplaceAllString(content, "${1}"+imageRepository+":"+imageTag+"${2}")
	if replaced != content {
		return preserveTrailingNewline(content, replaced), nil
	}

	fallback := regexp.MustCompile(`(?m)^(\s*image\s*:\s*)` + escapedRepo + `(?::\S+)?\s*$`)
	replaced = fallback.ReplaceAllString(content, "${1}"+imageRepository+":"+imageTag)
	if replaced != content {
		return preserveTrailingNewline(content, replaced), nil
	}
	return "", fmt.Errorf("在目标文件中未找到镜像 `%s` 的 image 行", imageRepository)
}

func renderGitOpsFileContent(content string, item *infrastructure.GitOpsChangeRequest) (string, error) {
	if item == nil {
		return "", fmt.Errorf("GitOps 变更请求为空")
	}
	if shouldUpdateHelmValues(item) {
		updated, err := updateHelmValues(content, item)
		if err == nil {
			return updated, nil
		}
		if isLikelyHelmValuesPath(item.FilePath) || strings.TrimSpace(item.HelmValuesPath) != "" {
			return "", err
		}
	}
	return replaceImageTag(content, item.ImageRepository, item.ImageTag)
}

func shouldUpdateHelmValues(item *infrastructure.GitOpsChangeRequest) bool {
	if item == nil {
		return false
	}
	if isLikelyHelmValuesPath(item.FilePath) || strings.TrimSpace(item.HelmValuesPath) != "" || strings.TrimSpace(item.HelmChartPath) != "" || strings.TrimSpace(item.HelmReleaseName) != "" {
		return true
	}
	return item.Replicas > 0 || strings.TrimSpace(item.CPURequest) != "" || strings.TrimSpace(item.CPULimit) != "" || strings.TrimSpace(item.MemoryRequest) != "" || strings.TrimSpace(item.MemoryLimit) != ""
}

func isLikelyHelmValuesPath(filePath string) bool {
	filePath = strings.ToLower(strings.TrimSpace(filePath))
	return strings.HasSuffix(filePath, ".yaml") && strings.Contains(filePath, "values") ||
		strings.HasSuffix(filePath, ".yml") && strings.Contains(filePath, "values")
}

func updateHelmValues(content string, item *infrastructure.GitOpsChangeRequest) (string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return "", fmt.Errorf("解析 Helm values 文件失败: %w", err)
	}
	doc := rootContentNode(&root)
	if doc == nil {
		return "", fmt.Errorf("Helm values 文件为空")
	}
	if doc.Kind != yaml.MappingNode {
		return "", fmt.Errorf("Helm values 顶层必须是 YAML 对象")
	}

	setStringValue(doc, []string{"image", "repository"}, strings.TrimSpace(item.ImageRepository))
	setStringValue(doc, []string{"image", "tag"}, strings.TrimSpace(item.ImageTag))
	if item.Replicas > 0 {
		setIntValue(doc, []string{"replicaCount"}, item.Replicas)
		if mappingValue(doc, "replicas") != nil {
			setIntValue(doc, []string{"replicas"}, item.Replicas)
		}
	}
	if strings.TrimSpace(item.CPURequest) != "" {
		setStringValue(doc, []string{"resources", "requests", "cpu"}, strings.TrimSpace(item.CPURequest))
	}
	if strings.TrimSpace(item.CPULimit) != "" {
		setStringValue(doc, []string{"resources", "limits", "cpu"}, strings.TrimSpace(item.CPULimit))
	}
	if strings.TrimSpace(item.MemoryRequest) != "" {
		setStringValue(doc, []string{"resources", "requests", "memory"}, strings.TrimSpace(item.MemoryRequest))
	}
	if strings.TrimSpace(item.MemoryLimit) != "" {
		setStringValue(doc, []string{"resources", "limits", "memory"}, strings.TrimSpace(item.MemoryLimit))
	}

	var builder strings.Builder
	encoder := yaml.NewEncoder(&builder)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		_ = encoder.Close()
		return "", fmt.Errorf("渲染 Helm values 文件失败: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("渲染 Helm values 文件失败: %w", err)
	}
	return preserveTrailingNewline(content, builder.String()), nil
}

func rootContentNode(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			root.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		}
		return root.Content[0]
	}
	return root
}

func setStringValue(root *yaml.Node, keys []string, value string) {
	if value == "" {
		return
	}
	node := ensureMappingPath(root, keys[:len(keys)-1])
	setMappingValue(node, keys[len(keys)-1], &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

func setIntValue(root *yaml.Node, keys []string, value int) {
	if value <= 0 {
		return
	}
	node := ensureMappingPath(root, keys[:len(keys)-1])
	setMappingValue(node, keys[len(keys)-1], &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)})
}

func ensureMappingPath(root *yaml.Node, keys []string) *yaml.Node {
	node := root
	if node.Kind != yaml.MappingNode {
		node.Kind = yaml.MappingNode
		node.Tag = "!!map"
		node.Content = nil
	}
	for _, key := range keys {
		next := mappingValue(node, key)
		if next == nil {
			next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			appendMappingValue(node, key, next)
		}
		if next.Kind != yaml.MappingNode {
			next.Kind = yaml.MappingNode
			next.Tag = "!!map"
			next.Value = ""
			next.Content = nil
		}
		node = next
	}
	return node
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	appendMappingValue(node, key, value)
}

func appendMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

func preserveTrailingNewline(original, updated string) string {
	originalHasLF := strings.HasSuffix(original, "\n")
	updatedHasLF := strings.HasSuffix(updated, "\n")
	switch {
	case originalHasLF && !updatedHasLF:
		return updated + "\n"
	case !originalHasLF && updatedHasLF:
		return strings.TrimSuffix(updated, "\n")
	default:
		return updated
	}
}

func buildCommitMessage(item *infrastructure.GitOpsChangeRequest) string {
	return fmt.Sprintf("chore(gitops): update %s to %s", item.ImageRepository, item.ImageTag)
}

func createGitLabBranch(ctx context.Context, client *http.Client, baseURL, projectPath, sourceBranch, targetBranch, token string) error {
	form := url.Values{}
	form.Set("branch", sourceBranch)
	form.Set("ref", targetBranch)
	_, statusCode, err := gitLabRequest(ctx, client, http.MethodPost, baseURL,
		fmt.Sprintf("/api/v4/projects/%s/repository/branches", url.PathEscape(projectPath)), token, form, nil)
	if err != nil && statusCode == http.StatusBadRequest && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

func getGitLabFileContent(ctx context.Context, client *http.Client, baseURL, projectPath, filePath, ref, token string) (string, string, error) {
	endpoint := fmt.Sprintf("/api/v4/projects/%s/repository/files/%s?ref=%s",
		url.PathEscape(projectPath), url.PathEscape(filePath), url.QueryEscape(ref))
	body, _, err := gitLabRequest(ctx, client, http.MethodGet, baseURL, endpoint, token, nil, nil)
	if err != nil {
		return "", "", fmt.Errorf("读取 GitLab 文件失败: %w", err)
	}

	var resp struct {
		Content      string `json:"content"`
		Encoding     string `json:"encoding"`
		LastCommitID string `json:"last_commit_id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", fmt.Errorf("解析 GitLab 文件内容失败: %w", err)
	}

	if resp.Encoding != "base64" {
		return "", "", fmt.Errorf("当前仅支持 base64 编码文件内容")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
	if err != nil {
		return "", "", fmt.Errorf("解码 GitLab 文件内容失败: %w", err)
	}
	return string(decoded), resp.LastCommitID, nil
}

func updateGitLabFile(ctx context.Context, client *http.Client, baseURL, projectPath, filePath, branch, token, content, commitMessage, lastCommitID string) (string, error) {
	payload := map[string]string{
		"branch":         branch,
		"content":        content,
		"commit_message": commitMessage,
	}
	if lastCommitID != "" {
		payload["last_commit_id"] = lastCommitID
	}

	endpoint := fmt.Sprintf("/api/v4/projects/%s/repository/files/%s", url.PathEscape(projectPath), url.PathEscape(filePath))
	body, _, err := gitLabRequest(ctx, client, http.MethodPut, baseURL, endpoint, token, nil, payload)
	if err != nil {
		return "", fmt.Errorf("更新 GitLab 文件失败: %w", err)
	}

	var resp struct {
		CommitID string `json:"commit_id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析 GitLab 提交结果失败: %w", err)
	}
	return resp.CommitID, nil
}

func gitLabBranchHasDiff(ctx context.Context, client *http.Client, baseURL, projectPath, fromRef, toRef, token string) (bool, error) {
	endpoint := fmt.Sprintf("/api/v4/projects/%s/repository/compare?from=%s&to=%s",
		url.PathEscape(projectPath), url.QueryEscape(strings.TrimSpace(fromRef)), url.QueryEscape(strings.TrimSpace(toRef)))
	body, _, err := gitLabRequest(ctx, client, http.MethodGet, baseURL, endpoint, token, nil, nil)
	if err != nil {
		return false, fmt.Errorf("校验 GitLab 分支差异失败: %w", err)
	}

	var resp struct {
		CompareSameRef bool `json:"compare_same_ref"`
		Diffs          []struct {
			NewPath string `json:"new_path"`
		} `json:"diffs"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("解析 GitLab 分支差异失败: %w", err)
	}
	if resp.CompareSameRef {
		return false, nil
	}
	return len(resp.Diffs) > 0, nil
}

func deleteGitLabBranch(ctx context.Context, client *http.Client, baseURL, projectPath, branch, token string) error {
	_, statusCode, err := gitLabRequest(ctx, client, http.MethodDelete, baseURL,
		fmt.Sprintf("/api/v4/projects/%s/repository/branches/%s", url.PathEscape(projectPath), url.PathEscape(strings.TrimSpace(branch))),
		token, nil, nil)
	if err != nil && statusCode != http.StatusNotFound {
		return fmt.Errorf("删除 GitLab 临时分支失败: %w", err)
	}
	return nil
}

func createGitLabMergeRequest(ctx context.Context, client *http.Client, baseURL, projectPath, token string, item *infrastructure.GitOpsChangeRequest) (string, string, error) {
	payload := map[string]interface{}{
		"source_branch":        item.SourceBranch,
		"target_branch":        item.TargetBranch,
		"title":                item.Title,
		"description":          item.Description,
		"remove_source_branch": true,
	}
	body, _, err := gitLabRequest(ctx, client, http.MethodPost, baseURL,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests", url.PathEscape(projectPath)), token, nil, payload)
	if err != nil {
		return "", "", fmt.Errorf("创建 GitLab 合并请求失败: %w", err)
	}

	var resp struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", fmt.Errorf("解析 GitLab 合并请求结果失败: %w", err)
	}
	return fmt.Sprintf("%d", resp.IID), resp.WebURL, nil
}

func (s *Service) addMergeRequestComment(ctx context.Context, item *infrastructure.GitOpsChangeRequest, body string) error {
	if item == nil || strings.TrimSpace(item.MergeRequestIID) == "" || strings.TrimSpace(body) == "" {
		return nil
	}
	repo, err := s.repoRepo.GetByID(item.GitOpsRepoID)
	if err != nil {
		return err
	}
	switch detectGitProvider(repo.RepoURL) {
	case "gitlab":
		return s.addGitLabMergeRequestComment(ctx, repo, item.MergeRequestIID, body)
	default:
		return nil
	}
}

func (s *Service) updateMergeRequestLabels(ctx context.Context, item *infrastructure.GitOpsChangeRequest, addLabels []string, removeLabels []string) error {
	if item == nil || strings.TrimSpace(item.MergeRequestIID) == "" {
		return nil
	}
	repo, err := s.repoRepo.GetByID(item.GitOpsRepoID)
	if err != nil {
		return err
	}
	switch detectGitProvider(repo.RepoURL) {
	case "gitlab":
		return s.updateGitLabMergeRequestLabels(ctx, repo, item.MergeRequestIID, addLabels, removeLabels)
	default:
		return nil
	}
}

func (s *Service) mergeApprovedChangeRequest(ctx context.Context, item *infrastructure.GitOpsChangeRequest) error {
	repo, err := s.repoRepo.GetByID(item.GitOpsRepoID)
	if err != nil {
		return err
	}

	switch detectGitProvider(repo.RepoURL) {
	case "gitlab":
		return s.mergeGitLabChangeRequest(ctx, repo, item)
	default:
		return fmt.Errorf("当前仅支持 GitLab 仓库自动合并变更请求")
	}
}

func (s *Service) mergeGitLabChangeRequest(ctx context.Context, repo *infrastructure.GitOpsRepo, item *infrastructure.GitOpsChangeRequest) error {
	if strings.TrimSpace(repo.AuthType) != "token" {
		return fmt.Errorf("当前仅支持使用 Token 凭证的 GitLab GitOps 仓库自动合并")
	}
	token, err := decryptToken(repo.AuthCredential)
	if err != nil {
		return fmt.Errorf("解密 GitOps 仓库凭证失败: %w", err)
	}
	baseURL, projectPath, err := parseGitLabRepositoryURL(repo.RepoURL)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	if err := validateGitLabMergeRequest(ctx, client, baseURL, projectPath, strings.TrimSpace(token), item.MergeRequestIID); err != nil {
		return err
	}
	endpoint := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%s/merge", url.PathEscape(projectPath), url.PathEscape(item.MergeRequestIID))
	payload := map[string]interface{}{
		"should_remove_source_branch": true,
	}
	_, _, err = gitLabRequest(ctx, client, http.MethodPut, baseURL, endpoint, strings.TrimSpace(token), nil, payload)
	if err != nil {
		return fmt.Errorf("自动合并 GitLab MR 失败: %w", err)
	}
	return nil
}

func (s *Service) addGitLabMergeRequestComment(ctx context.Context, repo *infrastructure.GitOpsRepo, mergeRequestIID, body string) error {
	token, err := decryptToken(repo.AuthCredential)
	if err != nil {
		return fmt.Errorf("解密 GitOps 仓库凭证失败: %w", err)
	}
	baseURL, projectPath, err := parseGitLabRepositoryURL(repo.RepoURL)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	payload := map[string]string{"body": body}
	_, _, err = gitLabRequest(ctx, client, http.MethodPost, baseURL,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/%s/notes", url.PathEscape(projectPath), url.PathEscape(mergeRequestIID)),
		strings.TrimSpace(token), nil, payload)
	if err != nil {
		return fmt.Errorf("写入 GitLab MR 评论失败: %w", err)
	}
	return nil
}

func validateGitLabMergeRequest(ctx context.Context, client *http.Client, baseURL, projectPath, token, mergeRequestIID string) error {
	body, _, err := gitLabRequest(ctx, client, http.MethodGet, baseURL,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/%s", url.PathEscape(projectPath), url.PathEscape(mergeRequestIID)),
		token, nil, nil)
	if err != nil {
		return fmt.Errorf("获取 GitLab MR 状态失败: %w", err)
	}

	var resp struct {
		State               string `json:"state"`
		Draft               bool   `json:"draft"`
		WorkInProgress      bool   `json:"work_in_progress"`
		MergeStatus         string `json:"merge_status"`
		DetailedMergeStatus string `json:"detailed_merge_status"`
		HasConflicts        bool   `json:"has_conflicts"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("解析 GitLab MR 状态失败: %w", err)
	}

	if !strings.EqualFold(resp.State, "opened") {
		return fmt.Errorf("MR 当前状态为 `%s`，无法自动合并", resp.State)
	}
	if resp.Draft || resp.WorkInProgress {
		return fmt.Errorf("MR 仍为草稿状态，无法自动合并")
	}
	if resp.HasConflicts {
		return fmt.Errorf("MR 存在冲突，无法自动合并")
	}

	detailedStatus := strings.ToLower(strings.TrimSpace(resp.DetailedMergeStatus))
	switch detailedStatus {
	case "", "mergeable", "can_be_merged", "unchecked":
	default:
		return fmt.Errorf("MR 当前不可合并，详细状态为 `%s`", resp.DetailedMergeStatus)
	}

	mergeStatus := strings.ToLower(strings.TrimSpace(resp.MergeStatus))
	switch mergeStatus {
	case "", "can_be_merged", "unchecked":
		return nil
	case "cannot_be_merged":
		return fmt.Errorf("MR 当前不可合并")
	default:
		return nil
	}
}

func (s *Service) updateGitLabMergeRequestLabels(ctx context.Context, repo *infrastructure.GitOpsRepo, mergeRequestIID string, addLabels []string, removeLabels []string) error {
	token, err := decryptToken(repo.AuthCredential)
	if err != nil {
		return fmt.Errorf("解密 GitOps 仓库凭证失败: %w", err)
	}
	baseURL, projectPath, err := parseGitLabRepositoryURL(repo.RepoURL)
	if err != nil {
		return err
	}
	payload := map[string]interface{}{}
	if labels := uniqueNonEmpty(addLabels); len(labels) > 0 {
		payload["add_labels"] = strings.Join(labels, ",")
	}
	if labels := uniqueNonEmpty(removeLabels); len(labels) > 0 {
		payload["remove_labels"] = strings.Join(labels, ",")
	}
	if len(payload) == 0 {
		return nil
	}
	client := &http.Client{Timeout: 15 * time.Second}
	_, _, err = gitLabRequest(ctx, client, http.MethodPut, baseURL,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/%s", url.PathEscape(projectPath), url.PathEscape(mergeRequestIID)),
		strings.TrimSpace(token), nil, payload)
	if err != nil {
		return fmt.Errorf("更新 GitLab MR 标签失败: %w", err)
	}
	return nil
}

func uniqueNonEmpty(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func isNoopGitOpsChangeError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.TrimSpace(err.Error())
	return strings.Contains(msg, "无需重复创建变更请求") ||
		strings.Contains(msg, "未检测到镜像内容变化")
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func derefUint(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

func gitLabRequest(ctx context.Context, client *http.Client, method, baseURL, endpoint, token string, form url.Values, jsonBody interface{}) ([]byte, int, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	if jsonBody != nil {
		raw, err := json.Marshal(jsonBody)
		if err != nil {
			return nil, 0, err
		}
		body = strings.NewReader(string(raw))
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+endpoint, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return nil, resp.StatusCode, fmt.Errorf("GitLab API 返回 %d: %s", resp.StatusCode, msg)
	}
	return respBody, resp.StatusCode, nil
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) > 8 {
		return token[:4] + strings.Repeat("*", 8) + token[len(token)-4:]
	}
	return "****"
}
