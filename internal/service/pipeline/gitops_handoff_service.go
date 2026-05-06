package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/models/infrastructure"
	approvalRepo "devops/internal/modules/approval/repository"
	infraRepo "devops/internal/modules/infrastructure/repository"
	approvalsvc "devops/internal/service/approval"
	argocdsvc "devops/internal/service/argocd"
)

// GitOpsHandoffService 将 CI 成功产物自动交接到 GitOps 变更链路。
type GitOpsHandoffService struct {
	db *gorm.DB
}

func NewGitOpsHandoffService(db *gorm.DB) *GitOpsHandoffService {
	return &GitOpsHandoffService{db: db}
}

func (s *GitOpsHandoffService) HandleSuccessfulRun(ctx context.Context, run *models.PipelineRun, pipeline *models.Pipeline, env map[string]string) (*infrastructure.GitOpsChangeRequest, error) {
	if s == nil || s.db == nil || run == nil || pipeline == nil {
		return nil, nil
	}
	if !isTruthy(env["AUTO_GITOPS_HANDOFF"]) {
		return nil, nil
	}

	argocdService := s.newArgoCDService()
	input, err := s.buildChangeRequestInput(ctx, run, pipeline, env)
	if err != nil {
		return nil, err
	}
	if input == nil {
		return nil, nil
	}

	precheck, err := argocdService.PrecheckChangeRequest(input)
	if err != nil {
		s.upsertDeployRecord(ctx, run, pipeline, input, nil, "failed", err.Error())
		return nil, err
	}
	if precheck != nil && !precheck.CanCreate {
		msg := summarizeGitOpsPrecheckErrors(precheck)
		s.upsertDeployRecord(ctx, run, pipeline, input, nil, "failed", msg)
		return nil, errors.New(msg)
	}

	item, err := argocdService.CreateChangeRequest(ctx, input, pipeline.CreatedBy)
	if err != nil {
		s.updateRunGitOpsHandoff(run.ID, "failed", err.Error(), nil)
		s.upsertDeployRecord(ctx, run, pipeline, input, nil, "failed", err.Error())
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	if !strings.EqualFold(strings.TrimSpace(item.Status), "open") {
		handoffStatus := "failed"
		if strings.EqualFold(strings.TrimSpace(item.Status), "skipped") {
			handoffStatus = "skipped"
		}
		s.updateRunGitOpsHandoff(run.ID, handoffStatus, firstNonEmpty(strings.TrimSpace(item.ErrorMessage), "GitOps 变更请求创建失败"), &item.ID)
		recordStatus := "failed"
		if handoffStatus == "skipped" {
			recordStatus = "success"
		}
		s.upsertDeployRecord(ctx, run, pipeline, input, item, recordStatus, strings.TrimSpace(item.ErrorMessage))
		return item, nil
	}
	s.updateRunGitOpsHandoff(run.ID, "created", "", &item.ID)
	s.upsertDeployRecord(ctx, run, pipeline, input, item, "success", "")

	if err := s.bindApprovalChain(ctx, argocdService, item, input.Env); err != nil {
		item.ErrorMessage = strings.TrimSpace(strings.Trim(item.ErrorMessage+"; 审批链创建失败: "+err.Error(), "; "))
		_ = s.newChangeRepo().Update(item)
		s.updateRunGitOpsHandoff(run.ID, "failed", "GitOps 变更已创建，但审批链挂接失败: "+err.Error(), &item.ID)
		s.upsertDeployRecord(ctx, run, pipeline, input, item, "failed", "GitOps 变更已创建，但审批链挂接失败: "+err.Error())
		return item, fmt.Errorf("GitOps 变更已创建，但审批链挂接失败: %w", err)
	}
	s.updateRunGitOpsHandoff(run.ID, "created", "", &item.ID)
	s.upsertDeployRecord(ctx, run, pipeline, input, item, "success", "")
	return item, nil
}

func (s *GitOpsHandoffService) newArgoCDService() *argocdsvc.Service {
	instRepo := infraRepo.NewArgoCDInstanceRepository(s.db)
	appRepo := infraRepo.NewArgoCDApplicationRepository(s.db)
	repoRepo := infraRepo.NewGitOpsRepoRepository(s.db)
	changeRepo := infraRepo.NewGitOpsChangeRequestRepository(s.db)
	service := argocdsvc.NewService(instRepo, appRepo, repoRepo, changeRepo)
	service.SetEnvPolicyRepo(approvalRepo.NewEnvAuditPolicyRepository(s.db))
	service.SetSonarBindingRepo(infraRepo.NewSonarQubeBindingRepository(s.db))
	return service
}

func (s *GitOpsHandoffService) newChangeRepo() *infraRepo.GitOpsChangeRequestRepository {
	return infraRepo.NewGitOpsChangeRequestRepository(s.db)
}

func (s *GitOpsHandoffService) buildChangeRequestInput(ctx context.Context, run *models.PipelineRun, pipeline *models.Pipeline, env map[string]string) (*argocdsvc.CreateChangeRequestInput, error) {
	appEnv := s.resolveApplicationEnv(ctx, env, pipeline, run)
	repo, err := s.resolveGitOpsRepo(ctx, env, pipeline, run, appEnv)
	if err != nil {
		return nil, err
	}

	app, err := s.resolveArgoCDApplication(ctx, env, repo, pipeline, run, appEnv)
	if err != nil {
		return nil, err
	}

	applicationName := firstNonEmpty(
		strings.TrimSpace(env["APP_NAME"]),
		strings.TrimSpace(env["APPLICATION_NAME"]),
		valueFromApp(app, true),
		strings.TrimSpace(repo.ApplicationName),
		strings.TrimSpace(run.PipelineName),
		strings.TrimSpace(pipeline.Name),
	)
	deployEnv := firstNonEmpty(
		strings.TrimSpace(env["DEPLOY_ENV"]),
		strings.TrimSpace(env["GITOPS_ENV"]),
		valueFromApp(app, false),
		strings.TrimSpace(repo.Env),
	)
	imageRepository := firstNonEmpty(strings.TrimSpace(env["GITOPS_IMAGE_REPOSITORY"]), strings.TrimSpace(env["IMAGE_NAME"]))
	if imageRepository == "" {
		return nil, fmt.Errorf("GitOps 自动交接失败：缺少 IMAGE_NAME 或 GITOPS_IMAGE_REPOSITORY")
	}

	imageTag := firstNonEmpty(strings.TrimSpace(env["GITOPS_IMAGE_TAG"]), strings.TrimSpace(env["IMAGE_TAG"]), shortGitCommit(run.GitCommit))
	if imageTag == "" {
		return nil, fmt.Errorf("GitOps 自动交接失败：缺少 IMAGE_TAG，无法生成镜像版本")
	}

	filePath := firstNonEmpty(strings.TrimSpace(env["GITOPS_FILE_PATH"]), strings.TrimSpace(env["HELM_VALUES_PATH"]))
	if filePath == "" && appEnv != nil {
		filePath = strings.TrimSpace(appEnv.HelmValuesPath)
	}
	if filePath == "" {
		basePath := strings.Trim(strings.TrimSpace(repo.Path), "/")
		if basePath == "" {
			filePath = "deployment.yaml"
		} else {
			filePath = basePath + "/deployment.yaml"
		}
	}

	targetBranch := firstNonEmpty(strings.TrimSpace(env["GITOPS_TARGET_BRANCH"]), valueFromApplicationEnv(appEnv, "gitops_branch"), strings.TrimSpace(repo.Branch))
	title := firstNonEmpty(strings.TrimSpace(env["GITOPS_CHANGE_TITLE"]), fmt.Sprintf("chore(gitops): release %s %s", fallbackText(applicationName, pipeline.Name), imageTag))
	description := strings.TrimSpace(env["GITOPS_CHANGE_DESCRIPTION"])
	if description == "" {
		description = buildAutoHandoffDescription(run, pipeline, applicationName, deployEnv, imageRepository, imageTag)
	}

	input := &argocdsvc.CreateChangeRequestInput{
		GitOpsRepoID:    repo.ID,
		ApplicationName: applicationName,
		Env:             deployEnv,
		PipelineID:      &pipeline.ID,
		PipelineRunID:   &run.ID,
		Title:           title,
		Description:     description,
		FilePath:        filePath,
		ImageRepository: imageRepository,
		ImageTag:        imageTag,
		TargetBranch:    targetBranch,
		HelmChartPath:   firstNonEmpty(strings.TrimSpace(env["HELM_CHART_PATH"]), valueFromApplicationEnv(appEnv, "helm_chart_path")),
		HelmValuesPath:  firstNonEmpty(strings.TrimSpace(env["HELM_VALUES_PATH"]), valueFromApplicationEnv(appEnv, "helm_values_path"), filePath),
		HelmReleaseName: firstNonEmpty(strings.TrimSpace(env["HELM_RELEASE_NAME"]), valueFromApplicationEnv(appEnv, "helm_release_name")),
		Replicas:        firstPositiveInt(parseEnvIntValue(env["HELM_REPLICAS"]), parseEnvIntValue(env["REPLICAS"]), replicasFromApplicationEnv(appEnv)),
		CPURequest:      firstNonEmpty(strings.TrimSpace(env["CPU_REQUEST"]), valueFromApplicationEnv(appEnv, "cpu_request")),
		CPULimit:        firstNonEmpty(strings.TrimSpace(env["CPU_LIMIT"]), valueFromApplicationEnv(appEnv, "cpu_limit")),
		MemoryRequest:   firstNonEmpty(strings.TrimSpace(env["MEMORY_REQUEST"]), valueFromApplicationEnv(appEnv, "memory_request")),
		MemoryLimit:     firstNonEmpty(strings.TrimSpace(env["MEMORY_LIMIT"]), valueFromApplicationEnv(appEnv, "memory_limit")),
	}
	if app != nil {
		input.ArgoCDApplicationID = &app.ID
	}
	_ = s.ensureApplicationContext(ctx, input, repo, app, pipeline, env)
	return input, nil
}

func (s *GitOpsHandoffService) ensureApplicationContext(ctx context.Context, input *argocdsvc.CreateChangeRequestInput, repo *infrastructure.GitOpsRepo, argoApp *infrastructure.ArgoCDApplication, pipeline *models.Pipeline, env map[string]string) error {
	if s == nil || s.db == nil || input == nil {
		return nil
	}
	applicationName := strings.TrimSpace(input.ApplicationName)
	if applicationName == "" {
		return nil
	}
	deployEnv := strings.TrimSpace(input.Env)
	gitRepoURL := firstNonEmpty(
		strings.TrimSpace(env["APP_GIT_REPO"]),
		strings.TrimSpace(env["GIT_REPO_URL"]),
		strings.TrimSpace(env["__GIT_REPO_URL__"]),
		s.lookupPipelineGitRepoURL(ctx, pipeline),
	)
	k8sNamespace := firstNonEmpty(strings.TrimSpace(env["K8S_NAMESPACE"]), valueFromArgoApp(argoApp, "namespace"))
	k8sDeployment := firstNonEmpty(strings.TrimSpace(env["K8S_DEPLOYMENT"]), applicationName)

	var app models.Application
	err := s.db.WithContext(ctx).Where("name = ?", applicationName).First(&app).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		app = models.Application{
			Name:        applicationName,
			DisplayName: applicationName,
			Description: "由流水线 GitOps 自动交接创建",
			GitRepo:     gitRepoURL,
			Language:    firstNonEmpty(strings.TrimSpace(env["APP_LANGUAGE"]), detectLanguageFromImage(input.ImageRepository)),
			Framework:   strings.TrimSpace(env["APP_FRAMEWORK"]),
			Team:        firstNonEmpty(strings.TrimSpace(env["APP_TEAM"]), "platform"),
			Owner:       strings.TrimSpace(env["APP_OWNER"]),
			Status:      "active",
			CreatedBy:   pipeline.CreatedBy,
		}
		if err := s.db.WithContext(ctx).Create(&app).Error; err != nil {
			return err
		}
	} else {
		updates := map[string]any{}
		if strings.TrimSpace(app.DisplayName) == "" {
			updates["display_name"] = applicationName
		}
		if strings.TrimSpace(app.Description) == "" {
			updates["description"] = "由流水线 GitOps 自动交接创建"
		}
		if strings.TrimSpace(app.GitRepo) == "" && gitRepoURL != "" {
			updates["git_repo"] = gitRepoURL
		}
		if strings.TrimSpace(app.Language) == "" {
			if language := firstNonEmpty(strings.TrimSpace(env["APP_LANGUAGE"]), detectLanguageFromImage(input.ImageRepository)); language != "" {
				updates["language"] = language
			}
		}
		if strings.TrimSpace(app.Team) == "" {
			updates["team"] = firstNonEmpty(strings.TrimSpace(env["APP_TEAM"]), "platform")
		}
		if len(updates) > 0 {
			if err := s.db.WithContext(ctx).Model(&app).Updates(updates).Error; err != nil {
				return err
			}
			if err := s.db.WithContext(ctx).First(&app, app.ID).Error; err != nil {
				return err
			}
		}
	}

	if deployEnv != "" {
		if err := s.ensureApplicationEnv(ctx, app.ID, deployEnv, input.TargetBranch, k8sNamespace, k8sDeployment, input, repo, argoApp); err != nil {
			return err
		}
	}
	input.ApplicationID = &app.ID
	if argoApp != nil {
		updates := map[string]any{}
		if argoApp.ApplicationID == nil || *argoApp.ApplicationID == 0 {
			updates["application_id"] = app.ID
		}
		if strings.TrimSpace(argoApp.ApplicationName) == "" {
			updates["application_name"] = applicationName
		}
		if strings.TrimSpace(argoApp.Env) == "" && deployEnv != "" {
			updates["env"] = deployEnv
		}
		if len(updates) > 0 {
			_ = s.db.WithContext(ctx).Model(argoApp).Updates(updates).Error
		}
	}
	if repo != nil {
		updates := map[string]any{}
		if repo.ApplicationID == nil || *repo.ApplicationID == 0 {
			updates["application_id"] = app.ID
		}
		if strings.TrimSpace(repo.ApplicationName) == "" {
			updates["application_name"] = applicationName
		}
		if strings.TrimSpace(repo.Env) == "" && deployEnv != "" {
			updates["env"] = deployEnv
		}
		if len(updates) > 0 {
			_ = s.db.WithContext(ctx).Model(repo).Updates(updates).Error
		}
	}
	return nil
}

func (s *GitOpsHandoffService) ensureApplicationEnv(ctx context.Context, appID uint, envName, branch, k8sNamespace, k8sDeployment string, input *argocdsvc.CreateChangeRequestInput, repo *infrastructure.GitOpsRepo, argoApp *infrastructure.ArgoCDApplication) error {
	gitopsBranch := strings.TrimSpace(branch)
	gitopsPath := ""
	if repo != nil {
		gitopsBranch = firstNonEmpty(gitopsBranch, strings.TrimSpace(repo.Branch), "main")
		gitopsPath = strings.Trim(strings.TrimSpace(repo.Path), "/")
	}
	var gitopsRepoID *uint
	if repo != nil && repo.ID > 0 {
		id := repo.ID
		gitopsRepoID = &id
	}
	var argoAppID *uint
	if argoApp != nil && argoApp.ID > 0 {
		id := argoApp.ID
		argoAppID = &id
	}
	var appEnv models.ApplicationEnv
	err := s.db.WithContext(ctx).Where("app_id = ? AND env_name = ?", appID, envName).First(&appEnv).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		appEnv = models.ApplicationEnv{
			ApplicationID: appID,
			EnvName:       envName,
			Branch:        firstNonEmpty(strings.TrimSpace(branch), "main"),
			GitOpsRepoID:  gitopsRepoID,
			GitOpsBranch:  gitopsBranch,
			GitOpsPath:    gitopsPath,
			K8sNamespace:  k8sNamespace,
			K8sDeployment: k8sDeployment,
			Replicas:      1,
		}
		if input != nil {
			appEnv.ArgoCDApplicationID = argoAppID
			appEnv.HelmChartPath = strings.TrimSpace(input.HelmChartPath)
			appEnv.HelmValuesPath = strings.TrimSpace(input.HelmValuesPath)
			appEnv.HelmReleaseName = strings.TrimSpace(input.HelmReleaseName)
			if input.Replicas > 0 {
				appEnv.Replicas = input.Replicas
			}
			appEnv.CPURequest = strings.TrimSpace(input.CPURequest)
			appEnv.CPULimit = strings.TrimSpace(input.CPULimit)
			appEnv.MemoryRequest = strings.TrimSpace(input.MemoryRequest)
			appEnv.MemoryLimit = strings.TrimSpace(input.MemoryLimit)
		}
		return s.db.WithContext(ctx).Create(&appEnv).Error
	}
	updates := map[string]any{}
	if strings.TrimSpace(appEnv.Branch) == "" && strings.TrimSpace(branch) != "" {
		updates["branch"] = strings.TrimSpace(branch)
	}
	if strings.TrimSpace(appEnv.K8sNamespace) == "" && strings.TrimSpace(k8sNamespace) != "" {
		updates["k8s_namespace"] = strings.TrimSpace(k8sNamespace)
	}
	if strings.TrimSpace(appEnv.K8sDeployment) == "" && strings.TrimSpace(k8sDeployment) != "" {
		updates["k8s_deployment"] = strings.TrimSpace(k8sDeployment)
	}
	if appEnv.GitOpsRepoID == nil && gitopsRepoID != nil {
		updates["gitops_repo_id"] = *gitopsRepoID
	}
	if appEnv.ArgoCDApplicationID == nil && argoAppID != nil {
		updates["argocd_application_id"] = *argoAppID
	}
	if strings.TrimSpace(appEnv.GitOpsBranch) == "" && gitopsBranch != "" {
		updates["gitops_branch"] = gitopsBranch
	}
	if strings.TrimSpace(appEnv.GitOpsPath) == "" && gitopsPath != "" {
		updates["gitops_path"] = gitopsPath
	}
	if input != nil {
		if strings.TrimSpace(appEnv.HelmChartPath) == "" && strings.TrimSpace(input.HelmChartPath) != "" {
			updates["helm_chart_path"] = strings.TrimSpace(input.HelmChartPath)
		}
		if strings.TrimSpace(appEnv.HelmValuesPath) == "" && strings.TrimSpace(input.HelmValuesPath) != "" {
			updates["helm_values_path"] = strings.TrimSpace(input.HelmValuesPath)
		}
		if strings.TrimSpace(appEnv.HelmReleaseName) == "" && strings.TrimSpace(input.HelmReleaseName) != "" {
			updates["helm_release_name"] = strings.TrimSpace(input.HelmReleaseName)
		}
		if appEnv.Replicas <= 0 && input.Replicas > 0 {
			updates["replicas"] = input.Replicas
		}
		if strings.TrimSpace(appEnv.CPURequest) == "" && strings.TrimSpace(input.CPURequest) != "" {
			updates["cpu_request"] = strings.TrimSpace(input.CPURequest)
		}
		if strings.TrimSpace(appEnv.CPULimit) == "" && strings.TrimSpace(input.CPULimit) != "" {
			updates["cpu_limit"] = strings.TrimSpace(input.CPULimit)
		}
		if strings.TrimSpace(appEnv.MemoryRequest) == "" && strings.TrimSpace(input.MemoryRequest) != "" {
			updates["memory_request"] = strings.TrimSpace(input.MemoryRequest)
		}
		if strings.TrimSpace(appEnv.MemoryLimit) == "" && strings.TrimSpace(input.MemoryLimit) != "" {
			updates["memory_limit"] = strings.TrimSpace(input.MemoryLimit)
		}
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&appEnv).Updates(updates).Error
}

func (s *GitOpsHandoffService) lookupPipelineGitRepoURL(ctx context.Context, pipeline *models.Pipeline) string {
	if s == nil || s.db == nil || pipeline == nil || pipeline.GitRepoID == nil || *pipeline.GitRepoID == 0 {
		return ""
	}
	var repo models.GitRepository
	if err := s.db.WithContext(ctx).Select("url").First(&repo, *pipeline.GitRepoID).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(repo.URL)
}

func valueFromArgoApp(app *infrastructure.ArgoCDApplication, field string) string {
	if app == nil {
		return ""
	}
	switch field {
	case "namespace":
		return strings.TrimSpace(app.DestNamespace)
	default:
		return ""
	}
}

func valueFromApplicationEnv(appEnv *models.ApplicationEnv, field string) string {
	if appEnv == nil {
		return ""
	}
	switch field {
	case "gitops_branch":
		return strings.TrimSpace(appEnv.GitOpsBranch)
	case "helm_chart_path":
		return strings.TrimSpace(appEnv.HelmChartPath)
	case "helm_values_path":
		return strings.TrimSpace(appEnv.HelmValuesPath)
	case "helm_release_name":
		return strings.TrimSpace(appEnv.HelmReleaseName)
	case "cpu_request":
		return strings.TrimSpace(appEnv.CPURequest)
	case "cpu_limit":
		return strings.TrimSpace(appEnv.CPULimit)
	case "memory_request":
		return strings.TrimSpace(appEnv.MemoryRequest)
	case "memory_limit":
		return strings.TrimSpace(appEnv.MemoryLimit)
	default:
		return ""
	}
}

func replicasFromApplicationEnv(appEnv *models.ApplicationEnv) int {
	if appEnv == nil || appEnv.Replicas <= 0 {
		return 0
	}
	return appEnv.Replicas
}

func detectLanguageFromImage(image string) string {
	image = strings.ToLower(strings.TrimSpace(image))
	switch {
	case strings.Contains(image, "go"):
		return "go"
	case strings.Contains(image, "node"):
		return "nodejs"
	case strings.Contains(image, "java"):
		return "java"
	case strings.Contains(image, "python"):
		return "python"
	default:
		return ""
	}
}

func (s *GitOpsHandoffService) resolveApplicationEnv(ctx context.Context, env map[string]string, pipeline *models.Pipeline, run *models.PipelineRun) *models.ApplicationEnv {
	if s == nil || s.db == nil {
		return nil
	}
	deployEnv := firstNonEmpty(strings.TrimSpace(env["DEPLOY_ENV"]), strings.TrimSpace(env["GITOPS_ENV"]), strings.TrimSpace(pipeline.Env), strings.TrimSpace(run.Env))
	if deployEnv == "" {
		return nil
	}
	if appID, err := parseEnvUint(env, "APPLICATION_ID"); err == nil && appID > 0 {
		var appEnv models.ApplicationEnv
		if err := s.db.WithContext(ctx).Where("app_id = ? AND env_name = ?", appID, deployEnv).First(&appEnv).Error; err == nil {
			return &appEnv
		}
	}
	if pipeline.ApplicationID != nil && *pipeline.ApplicationID > 0 {
		var appEnv models.ApplicationEnv
		if err := s.db.WithContext(ctx).Where("app_id = ? AND env_name = ?", *pipeline.ApplicationID, deployEnv).First(&appEnv).Error; err == nil {
			return &appEnv
		}
	}
	if run.ApplicationID != nil && *run.ApplicationID > 0 {
		var appEnv models.ApplicationEnv
		if err := s.db.WithContext(ctx).Where("app_id = ? AND env_name = ?", *run.ApplicationID, deployEnv).First(&appEnv).Error; err == nil {
			return &appEnv
		}
	}
	applicationName := firstNonEmpty(strings.TrimSpace(env["APP_NAME"]), strings.TrimSpace(env["APPLICATION_NAME"]), strings.TrimSpace(run.ApplicationName), strings.TrimSpace(pipeline.ApplicationName), strings.TrimSpace(run.PipelineName), strings.TrimSpace(pipeline.Name))
	if applicationName == "" {
		return nil
	}
	var app models.Application
	if err := s.db.WithContext(ctx).Where("name = ?", applicationName).First(&app).Error; err != nil {
		return nil
	}
	var appEnv models.ApplicationEnv
	if err := s.db.WithContext(ctx).Where("app_id = ? AND env_name = ?", app.ID, deployEnv).First(&appEnv).Error; err != nil {
		return nil
	}
	return &appEnv
}

func (s *GitOpsHandoffService) resolveGitOpsRepo(ctx context.Context, env map[string]string, pipeline *models.Pipeline, run *models.PipelineRun, appEnv *models.ApplicationEnv) (*infrastructure.GitOpsRepo, error) {
	repoRepo := infraRepo.NewGitOpsRepoRepository(s.db)
	if repoID, err := parseEnvUint(env, "GITOPS_REPO_ID"); err != nil {
		return nil, fmt.Errorf("GitOps 自动交接失败：GITOPS_REPO_ID 无效: %w", err)
	} else if repoID > 0 {
		repo, getErr := repoRepo.GetByID(repoID)
		if getErr != nil {
			return nil, fmt.Errorf("GitOps 自动交接失败：GitOps 仓库不存在: %w", getErr)
		}
		return repo, nil
	}
	if appEnv != nil && appEnv.GitOpsRepoID != nil && *appEnv.GitOpsRepoID > 0 {
		repo, getErr := repoRepo.GetByID(*appEnv.GitOpsRepoID)
		if getErr != nil {
			return nil, fmt.Errorf("GitOps 自动交接失败：环境绑定的 GitOps 仓库不存在: %w", getErr)
		}
		return repo, nil
	}

	repoName := strings.TrimSpace(env["GITOPS_REPO_NAME"])
	applicationName := firstNonEmpty(strings.TrimSpace(env["APP_NAME"]), strings.TrimSpace(env["APPLICATION_NAME"]), strings.TrimSpace(run.ApplicationName), strings.TrimSpace(pipeline.ApplicationName), run.PipelineName, pipeline.Name)
	deployEnv := firstNonEmpty(strings.TrimSpace(env["DEPLOY_ENV"]), strings.TrimSpace(env["GITOPS_ENV"]), strings.TrimSpace(run.Env), strings.TrimSpace(pipeline.Env))

	query := s.db.WithContext(ctx).Model(&infrastructure.GitOpsRepo{})
	if repoName != "" {
		query = query.Where("name = ?", repoName)
	}
	if pipeline.ApplicationID != nil && *pipeline.ApplicationID > 0 && applicationName != "" {
		query = query.Where("(application_id = ? OR application_name = ?)", *pipeline.ApplicationID, applicationName)
	} else if run.ApplicationID != nil && *run.ApplicationID > 0 && applicationName != "" {
		query = query.Where("(application_id = ? OR application_name = ?)", *run.ApplicationID, applicationName)
	} else if applicationName != "" {
		query = query.Where("application_name = ?", applicationName)
	}
	if deployEnv != "" {
		query = query.Where("(env = ? OR env = '')", deployEnv)
	}

	var repo infrastructure.GitOpsRepo
	if err := query.Order("sync_enabled DESC, id DESC").First(&repo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("GitOps 自动交接失败：未匹配到应用 `%s` 环境 `%s` 的 GitOps 仓库", applicationName, deployEnv)
		}
		return nil, err
	}
	return &repo, nil
}

func (s *GitOpsHandoffService) resolveArgoCDApplication(ctx context.Context, env map[string]string, repo *infrastructure.GitOpsRepo, pipeline *models.Pipeline, run *models.PipelineRun, appEnv *models.ApplicationEnv) (*infrastructure.ArgoCDApplication, error) {
	if appID, err := parseEnvUint(env, "ARGOCD_APPLICATION_ID"); err != nil {
		return nil, fmt.Errorf("GitOps 自动交接失败：ARGOCD_APPLICATION_ID 无效: %w", err)
	} else if appID > 0 {
		return infraRepo.NewArgoCDApplicationRepository(s.db).GetByID(appID)
	}
	if appEnv != nil && appEnv.ArgoCDApplicationID != nil && *appEnv.ArgoCDApplicationID > 0 {
		return infraRepo.NewArgoCDApplicationRepository(s.db).GetByID(*appEnv.ArgoCDApplicationID)
	}

	applicationName := firstNonEmpty(strings.TrimSpace(env["APP_NAME"]), strings.TrimSpace(env["APPLICATION_NAME"]), strings.TrimSpace(repo.ApplicationName), run.PipelineName, pipeline.Name)
	if applicationName == "" {
		return nil, nil
	}
	deployEnv := firstNonEmpty(strings.TrimSpace(env["DEPLOY_ENV"]), strings.TrimSpace(env["GITOPS_ENV"]), strings.TrimSpace(repo.Env))

	query := s.db.WithContext(ctx).Model(&infrastructure.ArgoCDApplication{}).
		Where("(application_name = ? OR name = ?)", applicationName, applicationName)
	if deployEnv != "" {
		query = query.Where("(env = ? OR env = '')", deployEnv)
	}

	var app infrastructure.ArgoCDApplication
	if err := query.Order("id DESC").First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &app, nil
}

func (s *GitOpsHandoffService) bindApprovalChain(ctx context.Context, argocdService *argocdsvc.Service, item *infrastructure.GitOpsChangeRequest, env string) error {
	if item == nil || argocdService == nil {
		return nil
	}

	chainRepo := approvalRepo.NewApprovalChainRepository(s.db)
	nodeRepo := approvalRepo.NewApprovalNodeRepository(s.db)
	instanceRepo := approvalRepo.NewApprovalInstanceRepository(s.db)
	nodeInstanceRepo := approvalRepo.NewApprovalNodeInstanceRepository(s.db)
	actionRepo := approvalRepo.NewApprovalActionRepository(s.db)
	policyRepo := approvalRepo.NewEnvAuditPolicyRepository(s.db)

	chainService := approvalsvc.NewChainService(chainRepo, nodeRepo)
	nodeExecutor := approvalsvc.NewNodeExecutor(nodeInstanceRepo, actionRepo, instanceRepo)
	approverResolver := approvalsvc.NewApproverResolver(s.db)
	instanceService := approvalsvc.NewInstanceService(instanceRepo, nodeInstanceRepo, chainService, nodeExecutor, approverResolver)
	policyService := approvalsvc.NewEnvAuditPolicyService(policyRepo)

	chain, err := matchGitOpsApprovalChain(ctx, chainService, policyService, env)
	if err != nil || chain == nil {
		return err
	}

	recordID := argocdsvc.BuildChangeRequestApprovalRecordID(item.ID)
	instance, err := instanceService.Create(ctx, recordID, chain)
	if err != nil {
		return err
	}
	if err := argocdService.AttachApproval(item.ID, instance.ID, chain.ID, chain.Name); err != nil {
		return err
	}
	item.ApprovalInstanceID = &instance.ID
	item.ApprovalChainID = &chain.ID
	item.ApprovalChainName = chain.Name
	item.ApprovalStatus = "pending"
	item.AutoMergeStatus = "pending"
	if err := argocdService.NotifyApprovalPending(ctx, item.ID); err != nil {
		item.ErrorMessage = strings.TrimSpace(strings.Trim(item.ErrorMessage+"; MR 审批提示回写失败: "+err.Error(), "; "))
		_ = s.newChangeRepo().Update(item)
	}
	return nil
}

func matchGitOpsApprovalChain(ctx context.Context, chainService *approvalsvc.ChainService, policyService *approvalsvc.EnvAuditPolicyService, env string) (*models.ApprovalChain, error) {
	if chainService == nil {
		return nil, nil
	}
	if policyService != nil && strings.TrimSpace(env) != "" {
		policy, err := policyService.GetByEnvName(env)
		if err == nil && policy != nil && policy.RequireChain && policy.DefaultChainID != nil {
			return chainService.GetWithNodes(ctx, *policy.DefaultChainID)
		}
	}
	return chainService.Match(ctx, 0, env)
}

func summarizeGitOpsPrecheckErrors(precheck *argocdsvc.ChangeRequestPrecheck) string {
	if precheck == nil || len(precheck.Checks) == 0 {
		return "GitOps 自动交接预检未通过"
	}
	messages := make([]string, 0, len(precheck.Checks))
	for _, check := range precheck.Checks {
		if check.Passed {
			continue
		}
		messages = append(messages, check.Message)
	}
	if len(messages) == 0 {
		return "GitOps 自动交接预检未通过"
	}
	return strings.Join(messages, "; ")
}

func buildAutoHandoffDescription(run *models.PipelineRun, pipeline *models.Pipeline, applicationName, envName, imageRepository, imageTag string) string {
	lines := []string{
		fmt.Sprintf("流水线 `%s` 运行成功，平台自动发起 GitOps 变更。", fallbackText(pipeline.Name, run.PipelineName)),
		fmt.Sprintf("运行编号: #%d", run.ID),
		fmt.Sprintf("应用: %s", fallbackText(applicationName, "-")),
		fmt.Sprintf("环境: %s", fallbackText(envName, "-")),
		fmt.Sprintf("镜像: %s:%s", imageRepository, imageTag),
		fmt.Sprintf("Git 分支: %s", fallbackText(run.GitBranch, "-")),
	}
	if strings.TrimSpace(run.GitCommit) != "" {
		lines = append(lines, fmt.Sprintf("Git Commit: %s", run.GitCommit))
	}
	return strings.Join(lines, "\n")
}

func (s *GitOpsHandoffService) upsertDeployRecord(ctx context.Context, run *models.PipelineRun, pipeline *models.Pipeline, input *argocdsvc.CreateChangeRequestInput, change *infrastructure.GitOpsChangeRequest, status, errMsg string) {
	if s == nil || s.db == nil || run == nil || pipeline == nil || input == nil {
		return
	}
	applicationName := strings.TrimSpace(input.ApplicationName)
	if applicationName == "" {
		return
	}

	var app models.Application
	if err := s.db.WithContext(ctx).Where("name = ?", applicationName).First(&app).Error; err != nil {
		return
	}

	now := time.Now()
	finishedAt := run.FinishedAt
	if finishedAt == nil {
		finishedAt = &now
	}

	version := firstNonEmpty(strings.TrimSpace(input.ImageTag), shortGitCommit(run.GitCommit), fmt.Sprintf("run-%d", run.ID))
	operator := firstNonEmpty(strings.TrimSpace(run.TriggerBy), "system")
	branch := firstNonEmpty(strings.TrimSpace(run.GitBranch), strings.TrimSpace(input.TargetBranch), strings.TrimSpace(pipeline.GitBranch), "main")
	marker := fmt.Sprintf("Pipeline Run #%d", run.ID)
	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = fmt.Sprintf("由流水线 %s 触发 GitOps 交付", pipeline.Name)
	}
	description = strings.TrimSpace(fmt.Sprintf("%s\n\n%s", marker, description))
	if change != nil && change.ID > 0 {
		description = strings.TrimSpace(fmt.Sprintf("%s\nGitOps Change Request #%d", description, change.ID))
	}
	if msg := strings.TrimSpace(errMsg); msg != "" {
		description = strings.TrimSpace(fmt.Sprintf("%s\n错误：%s", description, msg))
	}

	recordStatus := firstNonEmpty(strings.TrimSpace(status), strings.TrimSpace(run.Status), "success")
	needApproval := change != nil && (change.ApprovalInstanceID != nil || change.ApprovalChainID != nil)
	operatorID := firstUintPtrValue(pipeline.CreatedBy)
	var changeRequestID *uint
	if change != nil && change.ID > 0 {
		id := change.ID
		changeRequestID = &id
	}

	var record models.DeployRecord
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND deploy_method = ? AND description LIKE ?", app.ID, "gitops", "%"+marker+"%").
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = models.DeployRecord{
			ApplicationID: app.ID,
			AppName:       app.Name,
			EnvName:       strings.TrimSpace(input.Env),
			Version:       version,
			Branch:        branch,
			CommitID:      strings.TrimSpace(run.GitCommit),
			ImageTag:      version,
			DeployType:    "pipeline",
			DeployMethod:  "gitops",
			Status:        recordStatus,
			Description:   description,
			NeedApproval:  needApproval,
			Duration:      run.Duration,
			ErrorMsg:      strings.TrimSpace(errMsg),
			Operator:      operator,
			OperatorID:    operatorID,
			StartedAt:     run.StartedAt,
			FinishedAt:    finishedAt,
		}
		if change != nil && change.ApprovalChainID != nil {
			record.ApprovalChainID = change.ApprovalChainID
		}
		record.GitOpsChangeRequestID = changeRequestID
		_ = s.db.WithContext(ctx).Create(&record).Error
		return
	}
	if err != nil {
		return
	}

	updates := map[string]any{
		"app_name":      app.Name,
		"env_name":      strings.TrimSpace(input.Env),
		"version":       version,
		"branch":        branch,
		"commit_id":     strings.TrimSpace(run.GitCommit),
		"image_tag":     version,
		"deploy_type":   "pipeline",
		"deploy_method": "gitops",
		"status":        recordStatus,
		"description":   description,
		"need_approval": needApproval,
		"duration":      run.Duration,
		"error_msg":     strings.TrimSpace(errMsg),
		"operator":      operator,
		"operator_id":   operatorID,
		"started_at":    run.StartedAt,
		"finished_at":   finishedAt,
	}
	if change != nil && change.ApprovalChainID != nil {
		updates["approval_chain_id"] = *change.ApprovalChainID
	}
	if changeRequestID != nil {
		updates["gitops_change_request_id"] = *changeRequestID
	}
	_ = s.db.WithContext(ctx).Model(&record).Updates(updates).Error
}

func firstUintPtrValue(v *uint) uint {
	if v == nil {
		return 0
	}
	return *v
}

func parseEnvUint(env map[string]string, key string) (uint, error) {
	value := strings.TrimSpace(env[key])
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}

func parseEnvIntValue(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func shortGitCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *GitOpsHandoffService) updateRunGitOpsHandoff(runID uint, status, message string, changeRequestID *uint) {
	if s == nil || s.db == nil || runID == 0 {
		return
	}
	updates := map[string]interface{}{
		"git_ops_handoff_status":  strings.TrimSpace(status),
		"git_ops_handoff_message": strings.TrimSpace(message),
	}
	if changeRequestID != nil && *changeRequestID > 0 {
		updates["git_ops_change_request_id"] = *changeRequestID
	}
	_ = s.db.Model(&models.PipelineRun{}).Where("id = ?", runID).Updates(updates).Error
}

func valueFromApp(app *infrastructure.ArgoCDApplication, name bool) string {
	if app == nil {
		return ""
	}
	if name {
		return firstNonEmpty(app.ApplicationName, app.Name)
	}
	return strings.TrimSpace(app.Env)
}
