package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/models/infrastructure"
	modelpipeline "devops/internal/models/pipeline"
	"devops/pkg/dto"
	apperrors "devops/pkg/errors"
	"devops/pkg/logger"
)

// 流水线名称正则：只允许英文字母、数字、下划线、横线
var pipelineNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

var privilegedPipelineRoles = map[string]struct{}{
	"admin":         {},
	"administrator": {},
	"super_admin":   {},
}

// validatePipelineName 校验流水线名称
func validatePipelineName(name string) error {
	if name == "" {
		return fmt.Errorf("流水线名称不能为空")
	}
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("流水线名称长度必须在 2-64 个字符之间")
	}
	if !pipelineNameRegex.MatchString(name) {
		return fmt.Errorf("流水线名称只能包含英文字母、数字、下划线和横线，且必须以字母开头")
	}
	return nil
}

func isPrivilegedPipelineRole(role string) bool {
	_, ok := privilegedPipelineRoles[strings.ToLower(strings.TrimSpace(role))]
	return ok
}

// PipelineService 流水线服务
type PipelineService struct {
	db         *gorm.DB
	ciProvider *gitLabCIProvisioner
}

// NewPipelineService 创建流水线服务
func NewPipelineService(db *gorm.DB) *PipelineService {
	return &PipelineService{db: db, ciProvider: newGitLabCIProvisioner(db)}
}

// GetDB 获取数据库连接
func (s *PipelineService) GetDB() *gorm.DB {
	return s.db
}

// List 获取流水线列表
func (s *PipelineService) List(ctx context.Context, req *dto.PipelineListRequest) (*dto.PipelineListResponse, error) {
	var pipelines []models.Pipeline
	var total int64

	query := s.db.Model(&models.Pipeline{})

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.ProjectID > 0 {
		query = query.Where("project_id = ?", req.ProjectID)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.ApplicationID > 0 {
		query = query.Where("application_id = ?", req.ApplicationID)
	}
	if strings.TrimSpace(req.ApplicationName) != "" {
		query = query.Where("application_name = ?", strings.TrimSpace(req.ApplicationName))
	}
	if strings.TrimSpace(req.Env) != "" {
		query = query.Where("env = ?", strings.TrimSpace(req.Env))
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	query.Count(&total)
	query.Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&pipelines)

	items := make([]dto.PipelineItem, 0, len(pipelines))
	for _, p := range pipelines {
		applicationName, envName := pipelineDeliveryScope(p.ConfigJSON)
		if strings.TrimSpace(p.ApplicationName) != "" {
			applicationName = strings.TrimSpace(p.ApplicationName)
		}
		if strings.TrimSpace(p.Env) != "" {
			envName = strings.TrimSpace(p.Env)
		}
		managedCI := parseManagedConfig(p.ConfigJSON)
		item := dto.PipelineItem{
			ID:               p.ID,
			Name:             p.Name,
			Description:      p.Description,
			ProjectID:        p.ProjectID,
			ApplicationID:    p.ApplicationID,
			ApplicationName:  applicationName,
			Env:              envName,
			SourceTemplateID: p.SourceTemplateID,
			GitRepoID:        p.GitRepoID,
			GitBranch:        p.GitBranch,
			CIEngine:         "gitlab_runner",
			CIConfigPath:     firstNonEmptyString(managedCI.CIConfigPath, managedGitLabCIPath),
			DockerfilePath:   firstNonEmptyString(managedCI.DockerfilePath, inlineDockerfilePath),
			Status:           p.Status,
			LastRunAt:        p.LastRunAt,
			LastRunStatus:    p.LastRunStatus,
			CreatedBy:        p.CreatedBy,
			CreatedAt:        p.CreatedAt,
		}
		// 获取 Git 仓库 URL
		if p.GitRepoID != nil && *p.GitRepoID > 0 {
			var gitRepo models.GitRepository
			if s.db.First(&gitRepo, *p.GitRepoID).Error == nil {
				item.GitRepoURL = gitRepo.URL
			}
		}
		items = append(items, item)
	}

	return &dto.PipelineListResponse{
		Total: int(total),
		Items: items,
	}, nil
}

func pipelineDeliveryScope(configJSON string) (string, string) {
	var config struct {
		Variables []dto.Variable `json:"variables"`
		CI        any            `json:"ci,omitempty"`
	}
	if strings.TrimSpace(configJSON) == "" {
		return "", ""
	}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return "", ""
	}
	values := make(map[string]string, len(config.Variables))
	for _, variable := range config.Variables {
		values[strings.ToUpper(strings.TrimSpace(variable.Name))] = strings.TrimSpace(variable.Value)
	}
	return firstDeliveryValue(values, "APP_NAME", "APPLICATION_NAME"),
		firstDeliveryValue(values, "DEPLOY_ENV", "ENV", "ENV_NAME")
}

func firstDeliveryValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func deliveryScopeMatches(want, got string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	got = strings.TrimSpace(got)
	if got == "" {
		return false
	}
	return strings.EqualFold(want, got)
}

func paginatePipelines(items []models.Pipeline, page, pageSize int) []models.Pipeline {
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []models.Pipeline{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

// Get 获取流水线详情
func (s *PipelineService) Get(ctx context.Context, id uint) (*dto.PipelineDetailExtResponse, error) {
	var pipeline models.Pipeline
	if err := s.db.First(&pipeline, id).Error; err != nil {
		return nil, err
	}

	managedCI := parseManagedConfig(pipeline.ConfigJSON)
	applicationName, envName := pipelineDeliveryScope(pipeline.ConfigJSON)
	if strings.TrimSpace(pipeline.ApplicationName) != "" {
		applicationName = strings.TrimSpace(pipeline.ApplicationName)
	}
	if strings.TrimSpace(pipeline.Env) != "" {
		envName = strings.TrimSpace(pipeline.Env)
	}
	var config struct {
		Stages    []dto.Stage    `json:"stages"`
		Variables []dto.Variable `json:"variables"`
	}
	if pipeline.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(pipeline.ConfigJSON), &config)
	}
	defaultDockerfileReq := &dto.PipelineRequest{
		Name:      pipeline.Name,
		GitBranch: pipeline.GitBranch,
		Stages:    config.Stages,
		Variables: config.Variables,
	}
	result := &dto.PipelineDetailExtResponse{
		ID:                 pipeline.ID,
		Name:               pipeline.Name,
		Description:        pipeline.Description,
		ProjectID:          pipeline.ProjectID,
		ApplicationID:      pipeline.ApplicationID,
		ApplicationName:    applicationName,
		Env:                envName,
		SourceTemplateID:   pipeline.SourceTemplateID,
		GitRepoID:          pipeline.GitRepoID,
		GitBranch:          pipeline.GitBranch,
		CIEngine:           "gitlab_runner",
		CIConfigPath:       firstNonEmptyString(managedCI.CIConfigPath, managedGitLabCIPath),
		DockerfilePath:     firstNonEmptyString(managedCI.DockerfilePath, inlineDockerfilePath),
		GitLabCIYAML:       managedCI.GitLabCIYAML,
		GitLabCIYAMLCustom: managedCI.GitLabCIYAMLCustom,
		DockerfileContent:  firstNonEmptyString(managedCI.DockerfileContent, buildDockerfile(defaultDockerfileReq)),
		Status:             pipeline.Status,
		LastRunAt:          pipeline.LastRunAt,
		LastRunStatus:      pipeline.LastRunStatus,
		CreatedBy:          pipeline.CreatedBy,
		CreatedAt:          pipeline.CreatedAt,
		UpdatedAt:          pipeline.UpdatedAt,
	}

	// 获取 Git 仓库信息
	if pipeline.GitRepoID != nil && *pipeline.GitRepoID > 0 {
		var gitRepo models.GitRepository
		if s.db.First(&gitRepo, *pipeline.GitRepoID).Error == nil {
			result.GitRepoName = gitRepo.Name
			result.GitRepoURL = gitRepo.URL
		}
	}

	result.Stages = config.Stages
	result.Variables = config.Variables

	// 解析触发器配置
	if pipeline.TriggerConfigJSON != "" {
		var triggerConfig dto.TriggerConfig
		if err := json.Unmarshal([]byte(pipeline.TriggerConfigJSON), &triggerConfig); err == nil {
			result.TriggerConfig = triggerConfig
		}
	}

	return result, nil
}

func (s *PipelineService) PrepareForCreate(ctx context.Context, req *dto.PipelineRequest, role string) error {
	return s.prepareRequest(ctx, req, role, nil)
}

func (s *PipelineService) PrepareForUpdate(ctx context.Context, req *dto.PipelineRequest, role string) error {
	var existing models.Pipeline
	if err := s.db.First(&existing, req.ID).Error; err != nil {
		return err
	}
	return s.prepareRequest(ctx, req, role, &existing)
}

func (s *PipelineService) prepareRequest(ctx context.Context, req *dto.PipelineRequest, role string, existing *models.Pipeline) error {
	req.Name = strings.TrimSpace(req.Name)
	req.ApplicationName = strings.TrimSpace(req.ApplicationName)
	req.Env = strings.TrimSpace(req.Env)

	if req.SourceTemplateID == nil && existing != nil && existing.SourceTemplateID != nil {
		templateID := *existing.SourceTemplateID
		req.SourceTemplateID = &templateID
	}
	if req.ApplicationID == nil && existing != nil && existing.ApplicationID != nil {
		applicationID := *existing.ApplicationID
		req.ApplicationID = &applicationID
	}
	if req.ApplicationName == "" && existing != nil {
		req.ApplicationName = strings.TrimSpace(existing.ApplicationName)
	}
	if req.Env == "" && existing != nil {
		req.Env = strings.TrimSpace(existing.Env)
	}

	if err := s.prepareDeliveryContext(ctx, req); err != nil {
		return err
	}

	if isPrivilegedPipelineRole(role) {
		return nil
	}

	if req.SourceTemplateID == nil || *req.SourceTemplateID == 0 {
		return &ValidationError{Message: "当前角色仅支持基于模板创建或维护标准流水线，请先从模板市场选择模板"}
	}

	stages, err := s.loadStagesFromTemplate(ctx, *req.SourceTemplateID)
	if err != nil {
		return err
	}
	req.Stages = stages
	return nil
}

func (s *PipelineService) loadStagesFromTemplate(ctx context.Context, templateID uint) ([]dto.Stage, error) {
	var template modelpipeline.PipelineTemplate
	if err := s.db.WithContext(ctx).First(&template, templateID).Error; err != nil {
		return nil, &ValidationError{Message: "所选模板不存在，无法保存流水线"}
	}

	var config struct {
		Stages []dto.Stage `json:"stages"`
	}
	if err := json.Unmarshal([]byte(template.ConfigJSON), &config); err != nil {
		return nil, &ValidationError{Message: "所选模板配置无效，无法保存流水线"}
	}
	if len(config.Stages) == 0 {
		return nil, &ValidationError{Message: "所选模板缺少阶段定义，无法保存流水线"}
	}

	return config.Stages, nil
}

func (s *PipelineService) prepareDeliveryContext(ctx context.Context, req *dto.PipelineRequest) error {
	var app *models.Application
	if req.ApplicationID != nil && *req.ApplicationID > 0 {
		var found models.Application
		if err := s.db.WithContext(ctx).First(&found, *req.ApplicationID).Error; err != nil {
			return &ValidationError{Message: "关联应用不存在"}
		}
		app = &found
		req.ApplicationName = strings.TrimSpace(found.Name)
		if req.ProjectID == nil && found.ProjectID != nil {
			projectID := *found.ProjectID
			req.ProjectID = &projectID
		}
	}
	req.Variables = normalizeDeliveryVariables(req.Variables, req.ApplicationName, req.Env)
	req.Variables = s.normalizeGitOpsVariables(ctx, req.Variables, app, req.ApplicationName, req.Env)
	return nil
}

func (s *PipelineService) normalizeGitOpsVariables(ctx context.Context, variables []dto.Variable, app *models.Application, applicationName, envName string) []dto.Variable {
	applicationName = strings.TrimSpace(applicationName)
	envName = strings.TrimSpace(envName)
	if applicationName == "" || envName == "" {
		return variables
	}

	values := pipelineVariableMap(variables)
	var appEnv *models.ApplicationEnv
	if app != nil {
		if strings.TrimSpace(app.GitRepo) != "" {
			variables = upsertPipelineVariable(variables, "APP_GIT_REPO", strings.TrimSpace(app.GitRepo), false)
		}
		if strings.TrimSpace(app.Language) != "" {
			variables = upsertPipelineVariable(variables, "APP_LANGUAGE", strings.TrimSpace(app.Language), false)
		}
		var found models.ApplicationEnv
		if err := s.db.WithContext(ctx).Where("app_id = ? AND env_name = ?", app.ID, envName).First(&found).Error; err == nil {
			appEnv = &found
		}
	}
	var repo *infrastructure.GitOpsRepo
	if appEnv != nil && appEnv.GitOpsRepoID != nil && *appEnv.GitOpsRepoID > 0 {
		var found infrastructure.GitOpsRepo
		if err := s.db.WithContext(ctx).First(&found, *appEnv.GitOpsRepoID).Error; err == nil {
			repo = &found
		}
	}
	if repo == nil && strings.TrimSpace(values["GITOPS_REPO_ID"]) == "" {
		var found infrastructure.GitOpsRepo
		query := s.db.WithContext(ctx).Model(&infrastructure.GitOpsRepo{}).
			Where("application_name = ? AND (env = ? OR env = '')", applicationName, envName)
		if app != nil && app.ID > 0 {
			query = s.db.WithContext(ctx).Model(&infrastructure.GitOpsRepo{}).
				Where("(application_id = ? OR application_name = ?) AND (env = ? OR env = '')", app.ID, applicationName, envName)
		}
		if err := query.Order("sync_enabled DESC, id DESC").First(&found).Error; err == nil {
			repo = &found
		}
	}
	if repo != nil && strings.TrimSpace(values["GITOPS_REPO_ID"]) == "" {
		variables = upsertPipelineVariable(variables, "GITOPS_REPO_ID", fmt.Sprintf("%d", repo.ID), false)
		variables = upsertPipelineVariable(variables, "GITOPS_FILE_PATH", defaultGitOpsFilePath(repo.Path), false)
		variables = upsertPipelineVariable(variables, "GITOPS_TARGET_BRANCH", firstNonEmptyString(repo.Branch, "main"), false)
	}
	if appEnv != nil {
		variables = applyApplicationEnvVariables(variables, appEnv)
	}
	return variables
}

func applyApplicationEnvVariables(variables []dto.Variable, appEnv *models.ApplicationEnv) []dto.Variable {
	if appEnv == nil {
		return variables
	}
	if appEnv.GitOpsRepoID != nil && *appEnv.GitOpsRepoID > 0 {
		variables = upsertPipelineVariable(variables, "GITOPS_REPO_ID", fmt.Sprintf("%d", *appEnv.GitOpsRepoID), false)
	}
	if strings.TrimSpace(appEnv.GitOpsBranch) != "" {
		variables = upsertPipelineVariable(variables, "GITOPS_TARGET_BRANCH", strings.TrimSpace(appEnv.GitOpsBranch), false)
	}
	if strings.TrimSpace(appEnv.HelmValuesPath) != "" {
		variables = upsertPipelineVariable(variables, "GITOPS_FILE_PATH", strings.TrimSpace(appEnv.HelmValuesPath), false)
		variables = upsertPipelineVariable(variables, "HELM_VALUES_PATH", strings.TrimSpace(appEnv.HelmValuesPath), false)
	}
	if strings.TrimSpace(appEnv.HelmChartPath) != "" {
		variables = upsertPipelineVariable(variables, "HELM_CHART_PATH", strings.TrimSpace(appEnv.HelmChartPath), false)
	}
	if strings.TrimSpace(appEnv.HelmReleaseName) != "" {
		variables = upsertPipelineVariable(variables, "HELM_RELEASE_NAME", strings.TrimSpace(appEnv.HelmReleaseName), false)
	}
	if strings.TrimSpace(appEnv.GitOpsPath) != "" {
		variables = upsertPipelineVariable(variables, "GITOPS_PATH", strings.TrimSpace(appEnv.GitOpsPath), false)
	}
	if strings.TrimSpace(appEnv.K8sNamespace) != "" {
		variables = upsertPipelineVariable(variables, "K8S_NAMESPACE", strings.TrimSpace(appEnv.K8sNamespace), false)
	}
	if strings.TrimSpace(appEnv.K8sDeployment) != "" {
		variables = upsertPipelineVariable(variables, "K8S_DEPLOYMENT", strings.TrimSpace(appEnv.K8sDeployment), false)
	}
	if appEnv.Replicas > 0 {
		replicas := fmt.Sprintf("%d", appEnv.Replicas)
		variables = upsertPipelineVariable(variables, "HELM_REPLICAS", replicas, false)
		variables = upsertPipelineVariable(variables, "REPLICAS", replicas, false)
	}
	if strings.TrimSpace(appEnv.CPURequest) != "" {
		variables = upsertPipelineVariable(variables, "CPU_REQUEST", strings.TrimSpace(appEnv.CPURequest), false)
	}
	if strings.TrimSpace(appEnv.CPULimit) != "" {
		variables = upsertPipelineVariable(variables, "CPU_LIMIT", strings.TrimSpace(appEnv.CPULimit), false)
	}
	if strings.TrimSpace(appEnv.MemoryRequest) != "" {
		variables = upsertPipelineVariable(variables, "MEMORY_REQUEST", strings.TrimSpace(appEnv.MemoryRequest), false)
	}
	if strings.TrimSpace(appEnv.MemoryLimit) != "" {
		variables = upsertPipelineVariable(variables, "MEMORY_LIMIT", strings.TrimSpace(appEnv.MemoryLimit), false)
	}
	return variables
}

func pipelineVariableMap(variables []dto.Variable) map[string]string {
	values := make(map[string]string, len(variables))
	for _, variable := range variables {
		values[strings.ToUpper(strings.TrimSpace(variable.Name))] = strings.TrimSpace(variable.Value)
	}
	return values
}

func upsertPipelineVariable(variables []dto.Variable, name, value string, secret bool) []dto.Variable {
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if name == "" || value == "" {
		return variables
	}
	key := strings.ToUpper(name)
	for i := range variables {
		if strings.ToUpper(strings.TrimSpace(variables[i].Name)) == key {
			variables[i].Name = name
			variables[i].Value = value
			variables[i].IsSecret = secret
			return variables
		}
	}
	return append(variables, dto.Variable{Name: name, Value: value, IsSecret: secret})
}

func defaultGitOpsFilePath(repoPath string) string {
	base := strings.Trim(strings.TrimSpace(repoPath), "/")
	if base == "" || base == "." {
		return "deployment.yaml"
	}
	return base + "/deployment.yaml"
}

func normalizeDeliveryVariables(variables []dto.Variable, applicationName, envName string) []dto.Variable {
	values := make([]dto.Variable, 0, len(variables)+4)
	seen := make(map[string]int, len(variables)+4)
	for _, variable := range variables {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			continue
		}
		variable.Name = name
		key := strings.ToUpper(name)
		seen[key] = len(values)
		values = append(values, variable)
	}
	upsert := func(name, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToUpper(name)
		if idx, ok := seen[key]; ok {
			values[idx].Value = value
			values[idx].IsSecret = false
			return
		}
		seen[key] = len(values)
		values = append(values, dto.Variable{Name: name, Value: value})
	}
	upsert("APP_NAME", applicationName)
	upsert("APPLICATION_NAME", applicationName)
	upsert("DEPLOY_ENV", envName)
	if applicationName != "" && envName != "" {
		upsert("AUTO_GITOPS_HANDOFF", "true")
	}
	return values
}

// Create 创建流水线
func (s *PipelineService) Create(ctx context.Context, req *dto.PipelineRequest, userID uint) error {
	log := logger.L().WithField("name", req.Name)

	// 校验流水线名称
	if err := validatePipelineName(req.Name); err != nil {
		return err
	}
	req.DockerfileContent = firstNonEmptyString(req.DockerfileContent, buildDockerfile(req))
	if !req.GitLabCIYAMLCustom {
		req.GitLabCIYAML = ""
	}

	// 序列化配置
	config := struct {
		Stages    []dto.Stage    `json:"stages"`
		Variables []dto.Variable `json:"variables"`
		CI        map[string]any `json:"ci,omitempty"`
	}{
		Stages:    req.Stages,
		Variables: req.Variables,
		CI: map[string]any{
			"engine":                "gitlab_runner",
			"config_path":           managedGitLabCIPath,
			"dockerfile_path":       inlineDockerfilePath,
			"dockerfile_mode":       "inline",
			"gitlab_ci_yaml":        req.GitLabCIYAML,
			"gitlab_ci_yaml_custom": req.GitLabCIYAMLCustom,
			"dockerfile_content":    req.DockerfileContent,
		},
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	triggerConfigJSON, err := json.Marshal(req.TriggerConfig)
	if err != nil {
		return err
	}

	pipeline := &models.Pipeline{
		Name:              req.Name,
		Description:       req.Description,
		ProjectID:         req.ProjectID,
		ApplicationID:     req.ApplicationID,
		ApplicationName:   req.ApplicationName,
		Env:               req.Env,
		SourceTemplateID:  req.SourceTemplateID,
		GitRepoID:         req.GitRepoID,
		GitBranch:         req.GitBranch,
		ConfigJSON:        string(configJSON),
		TriggerConfigJSON: string(triggerConfigJSON),
		Status:            "active",
		CreatedBy:         &userID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(pipeline).Error; err != nil {
			return err
		}

		provisioner := newGitLabCIProvisioner(tx)
		result, err := provisioner.provision(ctx, pipeline, req, false)
		if err != nil {
			return err
		}
		log.WithField("repo", result.ProjectPath).WithField("branch", result.Branch).Info("已同步 GitLab Runner 流水线配置")
		return nil
	}); err != nil {
		log.WithField("error", err).Error("创建流水线失败")
		return err
	}

	log.Info("创建流水线成功")
	return nil
}

// Update 更新流水线
func (s *PipelineService) Update(ctx context.Context, req *dto.PipelineRequest) error {
	log := logger.L().WithField("id", req.ID)

	// 校验流水线名称
	if err := validatePipelineName(req.Name); err != nil {
		return err
	}
	req.DockerfileContent = firstNonEmptyString(req.DockerfileContent, buildDockerfile(req))
	if !req.GitLabCIYAMLCustom {
		req.GitLabCIYAML = ""
	}

	var pipeline models.Pipeline
	if err := s.db.First(&pipeline, req.ID).Error; err != nil {
		return err
	}

	// 序列化配置
	config := struct {
		Stages    []dto.Stage    `json:"stages"`
		Variables []dto.Variable `json:"variables"`
		CI        map[string]any `json:"ci,omitempty"`
	}{
		Stages:    req.Stages,
		Variables: req.Variables,
		CI: map[string]any{
			"engine":                "gitlab_runner",
			"config_path":           managedGitLabCIPath,
			"dockerfile_path":       inlineDockerfilePath,
			"dockerfile_mode":       "inline",
			"gitlab_ci_yaml":        req.GitLabCIYAML,
			"gitlab_ci_yaml_custom": req.GitLabCIYAMLCustom,
			"dockerfile_content":    req.DockerfileContent,
		},
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	triggerConfigJSON, err := json.Marshal(req.TriggerConfig)
	if err != nil {
		return err
	}

	pipeline.Name = req.Name
	pipeline.Description = req.Description
	pipeline.ProjectID = req.ProjectID
	pipeline.ApplicationID = req.ApplicationID
	pipeline.ApplicationName = req.ApplicationName
	pipeline.Env = req.Env
	pipeline.SourceTemplateID = req.SourceTemplateID
	pipeline.GitRepoID = req.GitRepoID
	pipeline.GitBranch = req.GitBranch
	pipeline.ConfigJSON = string(configJSON)
	pipeline.TriggerConfigJSON = string(triggerConfigJSON)
	pipeline.UpdatedAt = time.Now()

	// 设置默认值
	if pipeline.GitBranch == "" {
		pipeline.GitBranch = "main"
	}
	req.GitBranch = pipeline.GitBranch
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&pipeline).Error; err != nil {
			return err
		}

		provisioner := newGitLabCIProvisioner(tx)
		result, err := provisioner.provision(ctx, &pipeline, req, false)
		if err != nil {
			return err
		}
		log.WithField("repo", result.ProjectPath).WithField("branch", result.Branch).Info("已同步 GitLab Runner 流水线配置")
		return nil
	}); err != nil {
		log.WithField("error", err).Error("更新流水线失败")
		return err
	}

	log.Info("更新流水线成功")
	return nil
}

// Delete 删除流水线
func (s *PipelineService) Delete(ctx context.Context, id uint) error {
	// 删除相关的执行记录
	s.db.Where("pipeline_id = ?", id).Delete(&models.PipelineRun{})

	return s.db.Delete(&models.Pipeline{}, id).Error
}

// ToggleStatus 切换状态
func (s *PipelineService) ToggleStatus(ctx context.Context, id uint) error {
	var pipeline models.Pipeline
	if err := s.db.First(&pipeline, id).Error; err != nil {
		return err
	}

	if pipeline.Status == "active" {
		pipeline.Status = "disabled"
	} else {
		pipeline.Status = "active"
	}
	pipeline.UpdatedAt = time.Now()

	return s.db.Save(&pipeline).Error
}

// Validate 验证流水线配置
func (s *PipelineService) Validate(ctx context.Context, req *dto.PipelineRequest) error {
	if err := s.ciProvider.validate(ctx, req); err != nil {
		return err
	}

	// 检查阶段
	if len(req.Stages) == 0 {
		return &ValidationError{Message: "至少需要一个阶段"}
	}

	stageIDs := make(map[string]bool)
	for _, stage := range req.Stages {
		if stage.ID == "" {
			return &ValidationError{Message: "阶段ID不能为空"}
		}
		if stageIDs[stage.ID] {
			return &ValidationError{Message: "阶段ID重复: " + stage.ID}
		}
		stageIDs[stage.ID] = true

		// 检查依赖
		for _, dep := range stage.DependsOn {
			if !stageIDs[dep] {
				return &ValidationError{Message: "依赖的阶段不存在: " + dep}
			}
		}

		// 检查步骤
		if len(stage.Steps) == 0 {
			return &ValidationError{Message: "阶段至少需要一个步骤: " + stage.Name}
		}
	}

	return nil
}

// ValidationError 验证错误
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func (e *ValidationError) Is(target error) bool {
	_, ok := target.(*ValidationError)
	return ok
}

func (e *ValidationError) As(target interface{}) bool {
	if appErr, ok := target.(**apperrors.AppError); ok {
		*appErr = apperrors.New(apperrors.ErrCodeInvalidParams, e.Message)
		return true
	}
	return false
}
