package application

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/models/infrastructure"
	"devops/pkg/dto"
)

type OnboardingService struct {
	db *gorm.DB
}

func NewOnboardingService(db *gorm.DB) *OnboardingService {
	return &OnboardingService{db: db}
}

func (s *OnboardingService) Save(ctx context.Context, req *dto.ApplicationOnboardingRequest, userID uint) (*dto.ApplicationOnboardingResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("接入参数不能为空")
	}

	var out *dto.ApplicationOnboardingResponse
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		app, created, err := s.saveApplication(ctx, tx, req.ApplicationID, req.App, userID)
		if err != nil {
			return err
		}

		out = &dto.ApplicationOnboardingResponse{
			ApplicationID:   app.ID,
			ApplicationName: app.Name,
			Created:         created,
			UpdatedSections: []string{"application"},
		}

		var gitRepoID *uint
		var gitBranch string
		if req.Repo != nil {
			id, bindingID, branch, err := s.saveRepo(ctx, tx, app, req.Repo, userID)
			if err != nil {
				return err
			}
			gitRepoID = id
			gitBranch = branch
			out.GitRepoID = id
			out.RepoBindingID = bindingID
			out.UpdatedSections = append(out.UpdatedSections, "repo")
		}

		var envName string
		if req.Env != nil {
			env, err := s.saveEnv(ctx, tx, app, req.Env)
			if err != nil {
				return err
			}
			envName = env.EnvName
			out.EnvID = &env.ID
			out.UpdatedSections = append(out.UpdatedSections, "env")
			if gitBranch == "" {
				gitBranch = env.Branch
			}
		}

		if req.Pipeline != nil {
			id, err := s.savePipeline(ctx, tx, app, req.Pipeline, gitRepoID, gitBranch, envName, userID)
			if err != nil {
				return err
			}
			if id != nil {
				out.PipelineID = id
				out.UpdatedSections = append(out.UpdatedSections, "pipeline")
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	readiness, err := NewReadinessService(s.db).Refresh(ctx, out.ApplicationID)
	if err != nil {
		return nil, err
	}
	out.Readiness = readiness
	return out, nil
}

func (s *OnboardingService) saveApplication(ctx context.Context, tx *gorm.DB, appID *uint, input dto.ApplicationOnboardingAppInput, userID uint) (*models.Application, bool, error) {
	name := normalizeSlug(input.Name)
	if appID != nil && *appID > 0 {
		var app models.Application
		if err := tx.WithContext(ctx).First(&app, *appID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, false, fmt.Errorf("应用不存在")
			}
			return nil, false, err
		}
		updates := applicationUpdates(input)
		if name != "" {
			updates["name"] = name
		}
		if len(updates) > 0 {
			if err := tx.WithContext(ctx).Model(&app).Updates(updates).Error; err != nil {
				return nil, false, err
			}
		}
		if err := tx.WithContext(ctx).First(&app, app.ID).Error; err != nil {
			return nil, false, err
		}
		return &app, false, nil
	}

	if name == "" {
		return nil, false, fmt.Errorf("应用名称不能为空")
	}
	app := &models.Application{
		Name:           name,
		DisplayName:    strings.TrimSpace(input.DisplayName),
		Description:    strings.TrimSpace(input.Description),
		OrganizationID: input.OrganizationID,
		ProjectID:      input.ProjectID,
		GitRepo:        strings.TrimSpace(input.GitRepo),
		Language:       strings.TrimSpace(input.Language),
		Framework:      strings.TrimSpace(input.Framework),
		Team:           strings.TrimSpace(input.Team),
		Owner:          strings.TrimSpace(input.Owner),
		Status:         firstOnboardingValue(input.Status, "active"),
	}
	if userID > 0 {
		app.CreatedBy = &userID
	}
	if err := tx.WithContext(ctx).Create(app).Error; err != nil {
		return nil, false, err
	}
	return app, true, nil
}

func applicationUpdates(input dto.ApplicationOnboardingAppInput) map[string]any {
	updates := map[string]any{}
	if strings.TrimSpace(input.DisplayName) != "" {
		updates["display_name"] = strings.TrimSpace(input.DisplayName)
	}
	if strings.TrimSpace(input.Description) != "" {
		updates["description"] = strings.TrimSpace(input.Description)
	}
	if input.OrganizationID != nil {
		updates["organization_id"] = input.OrganizationID
	}
	if input.ProjectID != nil {
		updates["project_id"] = input.ProjectID
	}
	if strings.TrimSpace(input.GitRepo) != "" {
		updates["git_repo"] = strings.TrimSpace(input.GitRepo)
	}
	if strings.TrimSpace(input.Language) != "" {
		updates["language"] = strings.TrimSpace(input.Language)
	}
	if strings.TrimSpace(input.Framework) != "" {
		updates["framework"] = strings.TrimSpace(input.Framework)
	}
	if strings.TrimSpace(input.Team) != "" {
		updates["team"] = strings.TrimSpace(input.Team)
	}
	if strings.TrimSpace(input.Owner) != "" {
		updates["owner"] = strings.TrimSpace(input.Owner)
	}
	if strings.TrimSpace(input.Status) != "" {
		updates["status"] = strings.TrimSpace(input.Status)
	}
	return updates
}

func (s *OnboardingService) saveRepo(ctx context.Context, tx *gorm.DB, app *models.Application, input *dto.ApplicationOnboardingRepoInput, userID uint) (*uint, *uint, string, error) {
	if input == nil {
		return nil, nil, "", nil
	}
	role := firstOnboardingValue(input.Role, "primary")
	isDefault := false
	if input.IsDefault != nil {
		isDefault = *input.IsDefault
	}

	gitRepoID := uint(0)
	branch := strings.TrimSpace(input.DefaultBranch)
	if input.GitRepoID != nil && *input.GitRepoID > 0 {
		var repo models.GitRepository
		if err := tx.WithContext(ctx).First(&repo, *input.GitRepoID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil, "", fmt.Errorf("Git 仓库不存在")
			}
			return nil, nil, "", err
		}
		gitRepoID = repo.ID
		branch = firstOnboardingValue(branch, repo.DefaultBranch, "main")
		if app.GitRepo == "" && repo.URL != "" {
			if err := tx.WithContext(ctx).Model(app).Update("git_repo", repo.URL).Error; err != nil {
				return nil, nil, "", err
			}
		}
	} else if strings.TrimSpace(input.URL) != "" {
		repo := models.GitRepository{
			Name:          firstOnboardingValue(input.Name, app.Name),
			URL:           strings.TrimSpace(input.URL),
			Provider:      firstOnboardingValue(input.Provider, "custom"),
			DefaultBranch: firstOnboardingValue(input.DefaultBranch, "main"),
			Description:   "created by application onboarding",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := tx.WithContext(ctx).
			Where("url = ?", repo.URL).
			Assign(map[string]any{
				"name":           repo.Name,
				"provider":       repo.Provider,
				"default_branch": repo.DefaultBranch,
				"updated_at":     time.Now(),
			}).
			FirstOrCreate(&repo).Error; err != nil {
			return nil, nil, "", err
		}
		gitRepoID = repo.ID
		branch = firstOnboardingValue(branch, repo.DefaultBranch, "main")
		if err := tx.WithContext(ctx).Model(app).Update("git_repo", repo.URL).Error; err != nil {
			return nil, nil, "", err
		}
	} else {
		return nil, nil, "", fmt.Errorf("请选择或填写 Git 仓库")
	}

	var count int64
	if err := tx.WithContext(ctx).Model(&models.ApplicationRepoBinding{}).Where("application_id = ?", app.ID).Count(&count).Error; err != nil {
		return nil, nil, "", err
	}
	if count == 0 || input.IsDefault == nil {
		isDefault = true
	}
	if isDefault {
		if err := tx.WithContext(ctx).Model(&models.ApplicationRepoBinding{}).Where("application_id = ?", app.ID).Update("is_default", false).Error; err != nil {
			return nil, nil, "", err
		}
	}

	binding := models.ApplicationRepoBinding{
		ApplicationID: app.ID,
		GitRepoID:     gitRepoID,
		Role:          role,
		IsDefault:     isDefault,
	}
	if userID > 0 {
		binding.CreatedBy = &userID
	}
	if err := tx.WithContext(ctx).
		Where("application_id = ? AND git_repo_id = ?", app.ID, gitRepoID).
		Assign(map[string]any{"role": role, "is_default": isDefault, "created_by": binding.CreatedBy}).
		FirstOrCreate(&binding).Error; err != nil {
		return nil, nil, "", err
	}
	return &gitRepoID, &binding.ID, branch, nil
}

func (s *OnboardingService) saveEnv(ctx context.Context, tx *gorm.DB, app *models.Application, input *dto.ApplicationOnboardingEnvInput) (*models.ApplicationEnv, error) {
	env := &models.ApplicationEnv{
		ApplicationID:       app.ID,
		EnvName:             firstOnboardingValue(input.EnvName, "test"),
		Branch:              strings.TrimSpace(input.Branch),
		GitOpsRepoID:        input.GitOpsRepoID,
		ArgoCDApplicationID: input.ArgoCDApplicationID,
		GitOpsBranch:        strings.TrimSpace(input.GitOpsBranch),
		GitOpsPath:          cleanOnboardingPath(input.GitOpsPath),
		HelmChartPath:       cleanOnboardingPath(input.HelmChartPath),
		HelmValuesPath:      cleanOnboardingPath(input.HelmValuesPath),
		HelmReleaseName:     strings.TrimSpace(input.HelmReleaseName),
		K8sClusterID:        input.K8sClusterID,
		K8sNamespace:        strings.TrimSpace(input.K8sNamespace),
		K8sDeployment:       strings.TrimSpace(input.K8sDeployment),
		Replicas:            input.Replicas,
		CPURequest:          strings.TrimSpace(input.CPURequest),
		CPULimit:            strings.TrimSpace(input.CPULimit),
		MemoryRequest:       strings.TrimSpace(input.MemoryRequest),
		MemoryLimit:         strings.TrimSpace(input.MemoryLimit),
		Config:              strings.TrimSpace(input.Config),
	}
	if env.Replicas <= 0 {
		env.Replicas = 1
	}
	if env.GitOpsRepoID != nil && *env.GitOpsRepoID == 0 {
		env.GitOpsRepoID = nil
	}
	if env.ArgoCDApplicationID != nil && *env.ArgoCDApplicationID == 0 {
		env.ArgoCDApplicationID = nil
	}
	if env.GitOpsRepoID != nil {
		var repo infrastructure.GitOpsRepo
		if err := tx.WithContext(ctx).First(&repo, *env.GitOpsRepoID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("GitOps 部署仓库不存在")
			}
			return nil, err
		}
		env.GitOpsBranch = firstOnboardingValue(env.GitOpsBranch, repo.Branch)
		env.GitOpsPath = firstOnboardingValue(env.GitOpsPath, cleanOnboardingPath(repo.Path))
	}
	if env.ArgoCDApplicationID != nil {
		var argoApp infrastructure.ArgoCDApplication
		if err := tx.WithContext(ctx).First(&argoApp, *env.ArgoCDApplicationID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("ArgoCD 应用不存在")
			}
			return nil, err
		}
		env.K8sNamespace = firstOnboardingValue(env.K8sNamespace, argoApp.DestNamespace)
	}
	env.Branch = firstOnboardingValue(env.Branch, "main")
	env.GitOpsBranch = firstOnboardingValue(env.GitOpsBranch, env.Branch, "main")
	env.HelmChartPath = firstOnboardingValue(env.HelmChartPath, env.GitOpsPath)
	env.HelmValuesPath = firstOnboardingValue(env.HelmValuesPath, defaultOnboardingValuesPath(env.GitOpsPath, env.EnvName))
	env.HelmReleaseName = firstOnboardingValue(env.HelmReleaseName, env.K8sDeployment, app.Name+"-"+env.EnvName)

	var existing models.ApplicationEnv
	err := tx.WithContext(ctx).Where("app_id = ? AND env_name = ?", app.ID, env.EnvName).First(&existing).Error
	if err == nil {
		if err := tx.WithContext(ctx).Model(&existing).Updates(map[string]any{
			"branch":                env.Branch,
			"gitops_repo_id":        env.GitOpsRepoID,
			"argocd_application_id": env.ArgoCDApplicationID,
			"gitops_branch":         env.GitOpsBranch,
			"gitops_path":           env.GitOpsPath,
			"helm_chart_path":       env.HelmChartPath,
			"helm_values_path":      env.HelmValuesPath,
			"helm_release_name":     env.HelmReleaseName,
			"k8s_cluster_id":        env.K8sClusterID,
			"k8s_namespace":         env.K8sNamespace,
			"k8s_deployment":        env.K8sDeployment,
			"replicas":              env.Replicas,
			"cpu_request":           env.CPURequest,
			"cpu_limit":             env.CPULimit,
			"memory_request":        env.MemoryRequest,
			"memory_limit":          env.MemoryLimit,
			"config":                env.Config,
		}).Error; err != nil {
			return nil, err
		}
		if err := tx.WithContext(ctx).First(&existing, existing.ID).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err := tx.WithContext(ctx).Create(env).Error; err != nil {
		return nil, err
	}
	return env, nil
}

func (s *OnboardingService) savePipeline(ctx context.Context, tx *gorm.DB, app *models.Application, input *dto.ApplicationOnboardingPipelineInput, fallbackRepoID *uint, fallbackBranch, fallbackEnv string, userID uint) (*uint, error) {
	if input == nil {
		return nil, nil
	}
	envName := firstOnboardingValue(input.Env, fallbackEnv, "test")
	if input.PipelineID != nil && *input.PipelineID > 0 {
		var pipeline models.Pipeline
		if err := tx.WithContext(ctx).First(&pipeline, *input.PipelineID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("流水线不存在")
			}
			return nil, err
		}
		updates := map[string]any{
			"application_id":   app.ID,
			"application_name": app.Name,
			"env":              envName,
			"updated_at":       time.Now(),
		}
		if input.GitRepoID != nil && *input.GitRepoID > 0 {
			updates["git_repo_id"] = input.GitRepoID
		} else if fallbackRepoID != nil && *fallbackRepoID > 0 {
			updates["git_repo_id"] = fallbackRepoID
		}
		if strings.TrimSpace(input.GitBranch) != "" {
			updates["git_branch"] = strings.TrimSpace(input.GitBranch)
		}
		if err := tx.WithContext(ctx).Model(&pipeline).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &pipeline.ID, nil
	}
	if !input.Create {
		return nil, nil
	}

	repoID := input.GitRepoID
	if repoID == nil || *repoID == 0 {
		repoID = fallbackRepoID
	}
	branch := firstOnboardingValue(input.GitBranch, fallbackBranch, "main")
	name := safePipelineName(firstOnboardingValue(input.Name, app.Name+"-"+envName+"-ci"))

	var existing models.Pipeline
	result := tx.WithContext(ctx).Where("application_id = ? AND env = ?", app.ID, envName).Order("id ASC").Limit(1).Find(&existing)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		return &existing.ID, nil
	}

	configJSON, triggerJSON, err := onboardingPipelineConfig(app, envName, input, branch)
	if err != nil {
		return nil, err
	}
	pipeline := models.Pipeline{
		Name:              name,
		Description:       firstOnboardingValue(input.Description, "created by application onboarding"),
		ProjectID:         app.ProjectID,
		ApplicationID:     &app.ID,
		ApplicationName:   app.Name,
		Env:               envName,
		SourceTemplateID:  input.SourceTemplateID,
		GitRepoID:         repoID,
		GitBranch:         branch,
		ConfigJSON:        configJSON,
		TriggerConfigJSON: triggerJSON,
		Status:            "active",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if userID > 0 {
		pipeline.CreatedBy = &userID
	}
	if err := tx.WithContext(ctx).Create(&pipeline).Error; err != nil {
		return nil, err
	}
	return &pipeline.ID, nil
}

func onboardingPipelineConfig(app *models.Application, envName string, input *dto.ApplicationOnboardingPipelineInput, branch string) (string, string, error) {
	stages := []dto.Stage{
		{
			ID:   "build",
			Name: "Build",
			Steps: []dto.Step{
				{
					ID:   "test",
					Name: "Test",
					Type: "container",
					Config: map[string]interface{}{
						"image":    defaultBuildImage(app.Language),
						"commands": defaultTestCommands(app.Language),
					},
				},
				{
					ID:   "docker-build",
					Name: "Docker Build",
					Type: "docker_build",
					Config: map[string]interface{}{
						"context": ".",
					},
				},
				{
					ID:   "docker-push",
					Name: "Docker Push",
					Type: "docker_push",
				},
			},
		},
	}
	config := struct {
		Stages    []dto.Stage    `json:"stages"`
		Variables []dto.Variable `json:"variables"`
		CI        map[string]any `json:"ci,omitempty"`
	}{
		Stages: stages,
		Variables: []dto.Variable{
			{Name: "APP_NAME", Value: app.Name},
			{Name: "APPLICATION_NAME", Value: app.Name},
			{Name: "DEPLOY_ENV", Value: envName},
			{Name: "GIT_BRANCH", Value: branch},
			{Name: "AUTO_GITOPS_HANDOFF", Value: "true"},
		},
		CI: map[string]any{
			"engine":          "gitlab_runner",
			"config_path":     ".gitlab-ci.yml",
			"dockerfile_path": ".jeridevops.Dockerfile",
			"dockerfile_mode": "inline",
		},
	}
	if input.SourceTemplateID != nil && *input.SourceTemplateID > 0 {
		config.Variables = append(config.Variables, dto.Variable{Name: "SOURCE_TEMPLATE_ID", Value: fmt.Sprintf("%d", *input.SourceTemplateID)})
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", "", err
	}
	triggerJSON, err := json.Marshal(dto.TriggerConfig{Manual: true})
	if err != nil {
		return "", "", err
	}
	return string(configJSON), string(triggerJSON), nil
}

func defaultBuildImage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return "golang:1.25-alpine"
	case "java":
		return "maven:3.9-eclipse-temurin-21"
	case "python":
		return "python:3.12-alpine"
	case "node", "nodejs", "javascript", "typescript":
		return "node:20-alpine"
	default:
		return "alpine:3.20"
	}
}

func defaultTestCommands(language string) []interface{} {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return []interface{}{"go test ./..."}
	case "java":
		return []interface{}{"mvn test"}
	case "python":
		return []interface{}{"python -m pytest"}
	case "node", "nodejs", "javascript", "typescript":
		return []interface{}{"npm test -- --runInBand", "npm run build"}
	default:
		return []interface{}{"echo onboarding pipeline ready"}
	}
}

var invalidPipelineNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func safePipelineName(value string) string {
	value = invalidPipelineNameChars.ReplaceAllString(strings.TrimSpace(value), "-")
	value = strings.Trim(value, "-_")
	if value == "" {
		return "app-onboarding-ci"
	}
	first := value[0]
	if (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') {
		return value
	}
	return "app-" + value
}

func normalizeSlug(value string) string {
	return strings.TrimSpace(value)
}

func cleanOnboardingPath(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" || value == "." {
		return ""
	}
	return path.Clean(value)
}

func defaultOnboardingValuesPath(basePath, envName string) string {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		envName = "values"
	}
	basePath = cleanOnboardingPath(basePath)
	if basePath == "" {
		return path.Join("values", envName+".yaml")
	}
	return path.Join(basePath, "values", envName+".yaml")
}

func firstOnboardingValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
