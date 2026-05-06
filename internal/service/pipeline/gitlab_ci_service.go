package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"devops/internal/models"
	"devops/pkg/dto"
)

const (
	managedGitLabCIPath     = ".gitlab-ci.yml"
	inlineDockerfilePath    = ".jeridevops.Dockerfile"
	gitLabProvisioningLabel = "JeriDevOps managed GitLab Runner pipeline"
)

type gitLabCIProvisioner struct {
	db     *gorm.DB
	gitSvc *GitService
	client *http.Client
}

type gitLabCIManagedConfig struct {
	Enabled            bool
	CIConfigPath       string
	DockerfilePath     string
	GitLabCIYAML       string
	GitLabCIYAMLCustom bool
	DockerfileContent  string
}

type gitLabCIProvisioningResult struct {
	Repo           models.GitRepository
	Branch         string
	ProjectPath    string
	BaseURL        string
	GitLabPipeline gitLabPipelineInfo
}

type gitLabPipelineInfo struct {
	ID     int
	IID    int
	Status string
	WebURL string
	SHA    string
}

type gitLabJobInfo struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	Stage      string     `json:"stage"`
	Status     string     `json:"status"`
	WebURL     string     `json:"web_url"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type ciVariable struct {
	Value    string
	IsSecret bool
}

func newGitLabCIProvisioner(db *gorm.DB) *gitLabCIProvisioner {
	return &gitLabCIProvisioner{
		db:     db,
		gitSvc: NewGitService(db),
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func defaultGitLabCIManagedConfig() gitLabCIManagedConfig {
	return gitLabCIManagedConfig{
		Enabled:        true,
		CIConfigPath:   managedGitLabCIPath,
		DockerfilePath: inlineDockerfilePath,
	}
}

func (p *gitLabCIProvisioner) validate(ctx context.Context, req *dto.PipelineRequest) error {
	_, _, _, _, err := p.prepare(ctx, req, true)
	return err
}

func (p *gitLabCIProvisioner) provision(ctx context.Context, pipeline *models.Pipeline, req *dto.PipelineRequest, trigger bool) (*gitLabCIProvisioningResult, error) {
	repo, branch, cred, token, err := p.prepare(ctx, req, true)
	if err != nil {
		return nil, err
	}

	baseURL, projectPath, err := p.gitSvc.parseGitLabRepositoryURL(repo.URL)
	if err != nil {
		return nil, &ValidationError{Message: "解析 GitLab 仓库地址失败: " + err.Error()}
	}

	files := []gitLabManagedFile{
		{
			Path:    managedGitLabCIPath,
			Content: buildGitLabCIYAML(pipeline, req),
			Message: fmt.Sprintf("chore(ci): manage GitLab CI for %s", pipeline.Name),
		},
	}

	for _, file := range files {
		if err := p.upsertGitLabFile(ctx, baseURL, projectPath, file.Path, branch, token, file.Content, file.Message); err != nil {
			return nil, err
		}
	}

	result := &gitLabCIProvisioningResult{
		Repo:        repo,
		Branch:      branch,
		ProjectPath: projectPath,
		BaseURL:     baseURL,
	}

	if trigger {
		info, err := p.createGitLabPipeline(ctx, baseURL, projectPath, branch, token, nil)
		if err != nil {
			return nil, err
		}
		result.GitLabPipeline = info
	}

	_ = cred
	return result, nil
}

func (p *gitLabCIProvisioner) prepare(ctx context.Context, req *dto.PipelineRequest, validateRepo bool) (models.GitRepository, string, *gitCredential, string, error) {
	if req.GitRepoID == nil || *req.GitRepoID == 0 {
		return models.GitRepository{}, "", nil, "", &ValidationError{Message: "请选择 GitLab 仓库"}
	}

	var repo models.GitRepository
	query := p.db.WithContext(ctx)
	if validateRepo {
		query = query.Where("provider = ?", "gitlab")
	}
	if err := query.First(&repo, *req.GitRepoID).Error; err != nil {
		if validateRepo {
			return models.GitRepository{}, "", nil, "", &ValidationError{Message: "GitLab 仓库不存在"}
		}
		return models.GitRepository{ID: *req.GitRepoID}, firstNonEmptyString(req.GitBranch, "main"), nil, "", nil
	}
	if validateRepo && strings.ToLower(strings.TrimSpace(repo.Provider)) != "gitlab" {
		return models.GitRepository{}, "", nil, "", &ValidationError{Message: "创建流水线仅支持 GitLab 仓库，请先在 Git 仓库管理中配置 GitLab 项目"}
	}
	if validateRepo && (repo.CredentialID == nil || *repo.CredentialID == 0) {
		return models.GitRepository{}, "", nil, "", &ValidationError{Message: "GitLab 仓库必须绑定 Token 凭证，用于自动写入 .gitlab-ci.yml"}
	}
	if repo.CredentialID == nil || *repo.CredentialID == 0 {
		branch := firstNonEmptyString(req.GitBranch, repo.DefaultBranch, "main")
		req.GitBranch = branch
		return repo, branch, nil, "", nil
	}

	cred, err := p.gitSvc.loadGitCredential(ctx, repo.CredentialID)
	if err != nil {
		return models.GitRepository{}, "", nil, "", fmt.Errorf("获取 GitLab 凭证失败: %w", err)
	}
	token := firstNonEmptyString(cred.Token, cred.Password)
	if token == "" {
		return models.GitRepository{}, "", nil, "", &ValidationError{Message: "GitLab 凭证缺少 Token 或 Password"}
	}

	branch := firstNonEmptyString(req.GitBranch, repo.DefaultBranch, "main")
	req.GitBranch = branch
	return repo, branch, cred, token, nil
}

type gitLabManagedFile struct {
	Path    string
	Content string
	Message string
}

type gitLabFileResponse struct {
	FilePath     string `json:"file_path"`
	Content      string `json:"content"`
	Encoding     string `json:"encoding"`
	LastCommitID string `json:"last_commit_id"`
}

func (p *gitLabCIProvisioner) gitLabFileExists(ctx context.Context, baseURL, projectPath, filePath, ref, token string) (bool, error) {
	_, exists, err := p.gitLabFileContent(ctx, baseURL, projectPath, filePath, ref, token)
	return exists, err
}

func (p *gitLabCIProvisioner) gitLabFileContent(ctx context.Context, baseURL, projectPath, filePath, ref, token string) (string, bool, error) {
	endpoint := fmt.Sprintf("/api/v4/projects/%s/repository/files/%s?ref=%s",
		url.PathEscape(projectPath), url.PathEscape(filePath), url.QueryEscape(ref))
	body, status, err := p.gitSvc.gitLabRequest(ctx, p.client, http.MethodGet, baseURL, endpoint, token, nil)
	if err != nil {
		if status == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}

	var payload gitLabFileResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", true, fmt.Errorf("解析 GitLab 文件 %s 失败: %w", filePath, err)
	}
	if payload.Encoding != "base64" {
		return payload.Content, true, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.Content)
	if err != nil {
		return "", true, fmt.Errorf("解码 GitLab 文件 %s 失败: %w", filePath, err)
	}
	return string(decoded), true, nil
}

func (p *gitLabCIProvisioner) upsertGitLabFile(ctx context.Context, baseURL, projectPath, filePath, branch, token, content, commitMessage string) error {
	endpoint := fmt.Sprintf("/api/v4/projects/%s/repository/files/%s",
		url.PathEscape(projectPath), url.PathEscape(filePath))

	form := url.Values{}
	form.Set("branch", branch)
	form.Set("content", content)
	form.Set("commit_message", commitMessage)

	current, exists, err := p.gitLabFileContent(ctx, baseURL, projectPath, filePath, branch, token)
	if err != nil {
		return fmt.Errorf("读取 GitLab 文件 %s 失败: %w", filePath, err)
	}
	if exists {
		if current == content {
			return nil
		}
		_, _, err = p.gitSvc.gitLabRequest(ctx, p.client, http.MethodPut, baseURL, endpoint, token, form)
		if err != nil {
			return fmt.Errorf("更新 GitLab 文件 %s 失败: %w", filePath, err)
		}
		return nil
	}

	_, _, err = p.gitSvc.gitLabRequest(ctx, p.client, http.MethodPost, baseURL, endpoint, token, form)
	if err != nil {
		return fmt.Errorf("创建 GitLab 文件 %s 失败: %w", filePath, err)
	}
	return nil
}

func (p *gitLabCIProvisioner) createGitLabPipeline(ctx context.Context, baseURL, projectPath, branch, token string, variables []dto.Variable) (gitLabPipelineInfo, error) {
	endpoint := fmt.Sprintf("/api/v4/projects/%s/pipeline", url.PathEscape(projectPath))
	form := url.Values{}
	form.Set("ref", branch)
	idx := 0
	for _, variable := range variables {
		name := normalizeGitLabVariableName(variable.Name)
		if name == "" || variable.IsSecret {
			continue
		}
		form.Set(fmt.Sprintf("variables[%d][key]", idx), name)
		form.Set(fmt.Sprintf("variables[%d][value]", idx), variable.Value)
		idx++
	}

	body, _, err := p.gitSvc.gitLabRequest(ctx, p.client, http.MethodPost, baseURL, endpoint, token, form)
	if err != nil {
		return gitLabPipelineInfo{}, fmt.Errorf("触发 GitLab Pipeline 失败: %w", err)
	}

	var payload struct {
		ID     int    `json:"id"`
		IID    int    `json:"iid"`
		Status string `json:"status"`
		WebURL string `json:"web_url"`
		SHA    string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return gitLabPipelineInfo{}, fmt.Errorf("解析 GitLab Pipeline 响应失败: %w", err)
	}
	return gitLabPipelineInfo{
		ID:     payload.ID,
		IID:    payload.IID,
		Status: payload.Status,
		WebURL: payload.WebURL,
		SHA:    payload.SHA,
	}, nil
}

func (p *gitLabCIProvisioner) cancelGitLabPipeline(ctx context.Context, repo *models.GitRepository, branch string, pipelineID int) error {
	req := &dto.PipelineRequest{GitRepoID: &repo.ID, GitBranch: branch}
	_, _, _, token, err := p.prepare(ctx, req, true)
	if err != nil {
		return err
	}
	baseURL, projectPath, err := p.gitSvc.parseGitLabRepositoryURL(repo.URL)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("/api/v4/projects/%s/pipelines/%d/cancel", url.PathEscape(projectPath), pipelineID)
	_, _, err = p.gitSvc.gitLabRequest(ctx, p.client, http.MethodPost, baseURL, endpoint, token, nil)
	if err != nil {
		return fmt.Errorf("取消 GitLab Pipeline 失败: %w", err)
	}
	return nil
}

func (p *gitLabCIProvisioner) getGitLabPipeline(ctx context.Context, baseURL, projectPath, token string, pipelineID int) (gitLabPipelineInfo, error) {
	endpoint := fmt.Sprintf("/api/v4/projects/%s/pipelines/%d", url.PathEscape(projectPath), pipelineID)
	body, _, err := p.gitSvc.gitLabRequest(ctx, p.client, http.MethodGet, baseURL, endpoint, token, nil)
	if err != nil {
		return gitLabPipelineInfo{}, fmt.Errorf("获取 GitLab Pipeline 状态失败: %w", err)
	}

	var payload struct {
		ID     int    `json:"id"`
		IID    int    `json:"iid"`
		Status string `json:"status"`
		WebURL string `json:"web_url"`
		SHA    string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return gitLabPipelineInfo{}, fmt.Errorf("解析 GitLab Pipeline 状态失败: %w", err)
	}
	return gitLabPipelineInfo{
		ID:     payload.ID,
		IID:    payload.IID,
		Status: payload.Status,
		WebURL: payload.WebURL,
		SHA:    payload.SHA,
	}, nil
}

func (p *gitLabCIProvisioner) getLatestPipeline(ctx context.Context, repo *models.GitRepository, branch string) (gitLabPipelineInfo, error) {
	req := &dto.PipelineRequest{GitRepoID: &repo.ID, GitBranch: branch}
	_, _, _, token, err := p.prepare(ctx, req, true)
	if err != nil {
		return gitLabPipelineInfo{}, err
	}
	baseURL, projectPath, err := p.gitSvc.parseGitLabRepositoryURL(repo.URL)
	if err != nil {
		return gitLabPipelineInfo{}, err
	}

	endpoint := fmt.Sprintf("/api/v4/projects/%s/pipelines?ref=%s&per_page=1",
		url.PathEscape(projectPath), url.QueryEscape(branch))
	body, _, err := p.gitSvc.gitLabRequest(ctx, p.client, http.MethodGet, baseURL, endpoint, token, nil)
	if err != nil {
		return gitLabPipelineInfo{}, fmt.Errorf("获取 GitLab Pipeline 列表失败: %w", err)
	}

	var payload []struct {
		ID     int    `json:"id"`
		IID    int    `json:"iid"`
		Status string `json:"status"`
		WebURL string `json:"web_url"`
		SHA    string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return gitLabPipelineInfo{}, fmt.Errorf("解析 GitLab Pipeline 列表失败: %w", err)
	}
	if len(payload) == 0 {
		return gitLabPipelineInfo{}, fmt.Errorf("GitLab 分支 %s 暂无 Pipeline", branch)
	}
	return gitLabPipelineInfo{
		ID:     payload[0].ID,
		IID:    payload[0].IID,
		Status: payload[0].Status,
		WebURL: payload[0].WebURL,
		SHA:    payload[0].SHA,
	}, nil
}

func (p *gitLabCIProvisioner) gitLabPipelineStatus(ctx context.Context, repo *models.GitRepository, branch string, pipelineID int) (gitLabPipelineInfo, error) {
	req := &dto.PipelineRequest{GitRepoID: &repo.ID, GitBranch: branch}
	_, _, _, token, err := p.prepare(ctx, req, true)
	if err != nil {
		return gitLabPipelineInfo{}, err
	}
	baseURL, projectPath, err := p.gitSvc.parseGitLabRepositoryURL(repo.URL)
	if err != nil {
		return gitLabPipelineInfo{}, err
	}
	return p.getGitLabPipeline(ctx, baseURL, projectPath, token, pipelineID)
}

func (p *gitLabCIProvisioner) gitLabPipelineJobs(ctx context.Context, repo *models.GitRepository, branch string, pipelineID int) ([]gitLabJobInfo, error) {
	req := &dto.PipelineRequest{GitRepoID: &repo.ID, GitBranch: branch}
	_, _, _, token, err := p.prepare(ctx, req, true)
	if err != nil {
		return nil, err
	}
	baseURL, projectPath, err := p.gitSvc.parseGitLabRepositoryURL(repo.URL)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("/api/v4/projects/%s/pipelines/%d/jobs?per_page=100", url.PathEscape(projectPath), pipelineID)
	body, _, err := p.gitSvc.gitLabRequest(ctx, p.client, http.MethodGet, baseURL, endpoint, token, nil)
	if err != nil {
		return nil, fmt.Errorf("获取 GitLab Job 列表失败: %w", err)
	}
	var jobs []gitLabJobInfo
	if err := json.Unmarshal(body, &jobs); err != nil {
		return nil, fmt.Errorf("解析 GitLab Job 列表失败: %w", err)
	}
	return jobs, nil
}

func (p *gitLabCIProvisioner) gitLabJobTrace(ctx context.Context, repo *models.GitRepository, branch string, jobID int) (string, error) {
	req := &dto.PipelineRequest{GitRepoID: &repo.ID, GitBranch: branch}
	_, _, _, token, err := p.prepare(ctx, req, true)
	if err != nil {
		return "", err
	}
	baseURL, projectPath, err := p.gitSvc.parseGitLabRepositoryURL(repo.URL)
	if err != nil {
		return "", err
	}
	endpoint := fmt.Sprintf("/api/v4/projects/%s/jobs/%d/trace", url.PathEscape(projectPath), jobID)
	body, _, err := p.gitSvc.gitLabRequest(ctx, p.client, http.MethodGet, baseURL, endpoint, token, nil)
	if err != nil {
		return "", fmt.Errorf("获取 GitLab Job Trace 失败: %w", err)
	}
	return string(body), nil
}

func buildGitLabCIYAML(_ *models.Pipeline, req *dto.PipelineRequest) string {
	if custom := strings.TrimSpace(req.GitLabCIYAML); req.GitLabCIYAMLCustom && custom != "" {
		if !strings.HasSuffix(custom, "\n") {
			custom += "\n"
		}
		return custom
	}

	vars := collectCIVariables(req)
	branch := firstNonEmptyString(req.GitBranch, "main")
	hasDockerBuild := pipelineHasDockerBuild(req.Stages)
	stages := buildGitLabStages(req.Stages, hasDockerBuild)
	defaultImageRepository := defaultDockerImageRepository(req, vars)
	variables := orderedMap{
		{"DOCKER_TLS_CERTDIR", ""},
		{"DOCKER_DRIVER", "overlay2"},
		{"IMAGE_TAG", firstNonEmptyString(vars.value("IMAGE_TAG"), "$CI_COMMIT_SHORT_SHA")},
		{"IMAGE_NAME", firstNonEmptyString(vars.value("IMAGE_NAME"), vars.value("GITOPS_IMAGE_REPOSITORY"), defaultImageRepository)},
		{"GITOPS_IMAGE_REPOSITORY", firstNonEmptyString(vars.value("GITOPS_IMAGE_REPOSITORY"), vars.value("IMAGE_NAME"), defaultImageRepository)},
		{"DOCKER_IMAGE", firstNonEmptyString(vars.value("DOCKER_IMAGE"), "$IMAGE_NAME:$IMAGE_TAG")},
	}
	for _, key := range sortedVariableKeys(vars) {
		if key == "DOCKER_TLS_CERTDIR" || key == "DOCKER_DRIVER" || key == "IMAGE_TAG" || key == "IMAGE_NAME" || key == "GITOPS_IMAGE_REPOSITORY" || key == "DOCKER_IMAGE" {
			continue
		}
		variables = append(variables, orderedPair{Key: key, Value: vars.value(key)})
	}

	doc := orderedMap{
		{"# " + gitLabProvisioningLabel, nil},
		{"stages", stages},
		{"variables", variables},
	}

	jobNames := make(map[string]int)
	for _, stage := range req.Stages {
		hasDockerJob := false
		for _, step := range stage.Steps {
			if isRedundantDockerfileCompileStep(step, hasDockerBuild) {
				continue
			}
			if step.Type == "docker_push" && hasDockerJob {
				continue
			}
			job, ok := buildGitLabJob(stage, step, vars, branch, req)
			if ok {
				if step.Type == "docker_build" || step.Type == "docker_push" {
					hasDockerJob = true
				}
				doc = append(doc, orderedPair{uniqueGitLabJobName(gitLabJobName(stage, step), jobNames), job})
			}
		}
	}

	if !doc.hasJob() {
		doc = append(doc, orderedPair{"build_image", buildDockerBuildJob("package", vars, branch, req)})
	}

	return marshalOrderedYAML(doc)
}

func defaultDockerImageRepository(req *dto.PipelineRequest, vars ciVariables) string {
	registry := firstNonEmptyString(vars.value("DOCKER_REGISTRY"), "localhost:5001")
	namespace := firstNonEmptyString(vars.value("IMAGE_NAMESPACE"), "jeridevops")
	name := sanitizeDockerImageSegment(firstNonEmptyString(req.ApplicationName, vars.value("APP_NAME"), vars.value("APPLICATION_NAME"), req.Name, "app"))
	if namespace = strings.Trim(strings.TrimSpace(namespace), "/"); namespace == "" {
		return strings.TrimRight(registry, "/") + "/" + name
	}
	return strings.TrimRight(registry, "/") + "/" + namespace + "/" + name
}

func sanitizeDockerImageSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '-' || r == '/' || r == ' ' {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "app"
	}
	return out
}

func buildDockerfile(req *dto.PipelineRequest) string {
	vars := collectCIVariables(req)
	language := inferPipelineLanguage(req, vars)

	switch language {
	case "go", "golang":
		return strings.TrimSpace(`FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG BUILD_COMMAND=""
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; elif [ -d ./cmd/server ]; then CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/server; else CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .; fi

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/app /app/app
EXPOSE 8080
ENTRYPOINT ["/app/app"]
`) + "\n"
	case "rust", "cargo":
		return strings.TrimSpace(`FROM rust:1.87-alpine AS builder
WORKDIR /src
RUN apk add --no-cache musl-dev pkgconfig openssl-dev build-base
COPY Cargo.toml Cargo.lock* ./
COPY src ./src
COPY . .
ARG BUILD_COMMAND=""
ARG APP_PORT="8080"
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; else cargo build --release; fi
RUN bin_path="$(find target/release -maxdepth 1 -type f -perm -111 | head -n 1)" && cp "$bin_path" /tmp/app

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata libgcc libstdc++
COPY --from=builder /tmp/app /app/app
EXPOSE ${APP_PORT}
ENTRYPOINT ["/app/app"]
`) + "\n"
	case "java", "maven", "spring":
		return strings.TrimSpace(`FROM maven:3.9-eclipse-temurin-17 AS builder
WORKDIR /src
COPY pom.xml ./
RUN mvn -B -DskipTests dependency:go-offline
COPY . .
ARG BUILD_COMMAND="mvn -B clean package -DskipTests"
RUN sh -c "$BUILD_COMMAND"

FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
COPY --from=builder /src/target/*.jar /app/app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
`) + "\n"
	case "node", "nodejs", "npm", "vue", "react":
		return strings.TrimSpace(`FROM node:20-alpine AS builder
WORKDIR /src
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY . .
ARG BUILD_COMMAND="npm run build --if-present"
RUN sh -c "$BUILD_COMMAND"
RUN npm prune --omit=dev

FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /src /app
EXPOSE 3000
CMD ["npm", "start"]
`) + "\n"
	case "python", "python3", "django", "flask":
		return strings.TrimSpace(`FROM python:3.12-alpine
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
COPY requirements*.txt ./
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi
COPY . .
EXPOSE 8000
CMD ["python", "app.py"]
`) + "\n"
	default:
		return strings.TrimSpace(`FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache bash ca-certificates
COPY . .
ARG BUILD_COMMAND=""
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; else echo "No BUILD_COMMAND configured; packaging repository content."; fi
CMD ["sh", "-c", "echo 'Image built by JeriDevOps GitLab Runner'; sleep infinity"]
`) + "\n"
	}
}

func buildGitLabStages(stages []dto.Stage, hasDockerBuild bool) []string {
	result := make([]string, 0, len(stages)+1)
	seen := make(map[string]struct{})
	for _, stage := range stages {
		if !stageHasGitLabJob(stage, hasDockerBuild) {
			continue
		}
		name := gitLabStageName(stage)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	if len(result) == 0 {
		result = append(result, "package")
	}
	return result
}

func stageHasGitLabJob(stage dto.Stage, hasDockerBuild bool) bool {
	for _, step := range stage.Steps {
		if gitLabStepProducesJob(step) && !isRedundantDockerfileCompileStep(step, hasDockerBuild) {
			return true
		}
	}
	return false
}

func gitLabStepProducesJob(step dto.Step) bool {
	switch step.Type {
	case "git":
		return false
	case "docker_build", "docker_push":
		return true
	case "container", "shell", "scan", "notify":
		return len(extractStepCommands(step)) > 0
	default:
		return false
	}
}

func pipelineHasDockerBuild(stages []dto.Stage) bool {
	for _, stage := range stages {
		for _, step := range stage.Steps {
			if step.Type == "docker_build" || step.Type == "docker_push" {
				return true
			}
		}
	}
	return false
}

func isRedundantDockerfileCompileStep(step dto.Step, hasDockerBuild bool) bool {
	if !hasDockerBuild || step.Type != "container" {
		return false
	}
	commands := extractStepCommands(step)
	if len(commands) == 0 {
		return false
	}
	hasCompile := false
	for _, command := range commands {
		lower := strings.ToLower(command)
		if strings.Contains(lower, "go test") {
			return false
		}
		if strings.Contains(lower, "go build") {
			hasCompile = true
		}
	}
	return hasCompile
}

func rewriteNpmCIIfNoLockfile(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "npm ci" {
		// Many repos (especially smoke/demo) have no package-lock.json; plain `npm ci` always fails.
		return `if [ -f package-lock.json ] || [ -f npm-shrinkwrap.json ]; then npm ci; else npm install; fi`
	}
	return cmd
}

func buildGitLabJob(stage dto.Stage, step dto.Step, vars ciVariables, branch string, req *dto.PipelineRequest) (orderedMap, bool) {
	stageName := gitLabStageName(stage)
	switch step.Type {
	case "git":
		return nil, false
	case "docker_build", "docker_push":
		return buildDockerBuildJob(stageName, vars, branch, req, step), true
	case "container", "shell", "scan", "notify":
		commands := extractStepCommands(step)
		for i := range commands {
			commands[i] = rewriteNpmCIIfNoLockfile(commands[i])
		}
		if len(commands) == 0 {
			return nil, false
		}
		image := firstNonEmptyString(extractStepString(step, "image"), inferStepImage(commands), "alpine:3.20")
		job := orderedMap{
			{"stage", stageName},
			{"image", image},
			{"script", commands},
			{"rules", []orderedMap{
				{
					{"if", fmt.Sprintf(`$CI_COMMIT_BRANCH == "%s"`, branch)},
					{"when", "on_success"},
				},
			}},
		}
		return job, true
	default:
		return nil, false
	}
}

func buildDockerBuildJob(stageName string, vars ciVariables, branch string, req *dto.PipelineRequest, step ...dto.Step) orderedMap {
	dockerfile := inlineDockerfilePath
	contextDir := "."
	image := "$DOCKER_IMAGE"
	if len(step) > 0 {
		contextDir = firstNonEmptyString(extractStepString(step[0], "context"), contextDir)
		if configuredImage := extractStepString(step[0], "image"); configuredImage != "" {
			image = configuredImage
		}
	}
	beforeScript := []string{
		`docker info`,
		`if [ -n "$CI_REGISTRY" ]; then docker login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" "$CI_REGISTRY"; fi`,
	}
	if registry := vars.value("DOCKER_REGISTRY"); registry != "" {
		beforeScript = append(beforeScript, fmt.Sprintf(`if [ -n "$DOCKER_REGISTRY_USERNAME" ] && [ -n "$DOCKER_REGISTRY_PASSWORD" ]; then docker login -u "$DOCKER_REGISTRY_USERNAME" -p "$DOCKER_REGISTRY_PASSWORD" %q; fi`, registry))
	}

	script := []string{
		renderDockerfileScript(req),
		fmt.Sprintf("docker build --pull -f %s -t %s %s", shellQuote(dockerfile), shellQuote(image), shellQuote(contextDir)),
		fmt.Sprintf("docker push %s", shellQuote(image)),
	}
	return orderedMap{
		{"stage", firstNonEmptyString(stageName, "package")},
		{"image", "docker:26"},
		{"services", []string{"docker:26-dind"}},
		{"before_script", beforeScript},
		{"script", script},
		{"rules", []orderedMap{
			{
				{"if", fmt.Sprintf(`$CI_COMMIT_BRANCH == "%s"`, branch)},
				{"when", "on_success"},
			},
		}},
	}
}

func renderDockerfileScript(req *dto.PipelineRequest) string {
	content := strings.TrimRight(req.DockerfileContent, "\r\n")
	if strings.TrimSpace(content) == "" {
		content = strings.TrimRight(buildDockerfile(req), "\r\n")
	}
	return fmt.Sprintf("cat > %s <<'JERIDEVOPS_DOCKERFILE'\n%s\nJERIDEVOPS_DOCKERFILE", inlineDockerfilePath, content)
}

func pipelineProvisioningVariables(info gitLabPipelineInfo) map[string]string {
	result := map[string]string{
		"CI_ENGINE":          "gitlab_runner",
		"GITLAB_PIPELINE_ID": strconv.Itoa(info.ID),
	}
	if info.IID > 0 {
		result["GITLAB_PIPELINE_IID"] = strconv.Itoa(info.IID)
	}
	if strings.TrimSpace(info.WebURL) != "" {
		result["GITLAB_PIPELINE_URL"] = strings.TrimSpace(info.WebURL)
	}
	return result
}

func mergePipelineParameters(base map[string]string, overlays ...map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range base {
		result[key] = value
	}
	for _, overlay := range overlays {
		for key, value := range overlay {
			result[key] = value
		}
	}
	return result
}

func isGitLabRunnerPipeline(pipeline *models.Pipeline) bool {
	if pipeline == nil {
		return false
	}
	if managed := parseManagedConfig(pipeline.ConfigJSON); managed.Enabled {
		return true
	}
	return pipeline.GitRepoID != nil && *pipeline.GitRepoID > 0
}

func parseManagedConfig(configJSON string) gitLabCIManagedConfig {
	var payload struct {
		CI map[string]interface{} `json:"ci"`
	}
	if strings.TrimSpace(configJSON) == "" || json.Unmarshal([]byte(configJSON), &payload) != nil || payload.CI == nil {
		return gitLabCIManagedConfig{}
	}

	engine := strings.ToLower(strings.TrimSpace(fmt.Sprint(payload.CI["engine"])))
	if engine != "gitlab_runner" {
		return gitLabCIManagedConfig{}
	}
	return gitLabCIManagedConfig{
		Enabled:            true,
		CIConfigPath:       firstNonEmptyString(stringFromCIMap(payload.CI, "config_path"), managedGitLabCIPath),
		DockerfilePath:     firstNonEmptyString(stringFromCIMap(payload.CI, "dockerfile_path"), inlineDockerfilePath),
		GitLabCIYAML:       stringFromCIMap(payload.CI, "gitlab_ci_yaml"),
		GitLabCIYAMLCustom: boolFromCIMap(payload.CI, "gitlab_ci_yaml_custom"),
		DockerfileContent:  stringFromCIMap(payload.CI, "dockerfile_content"),
	}
}

func withManagedConfig(configJSON string, managed gitLabCIManagedConfig) string {
	var payload map[string]interface{}
	if strings.TrimSpace(configJSON) != "" {
		_ = json.Unmarshal([]byte(configJSON), &payload)
	}
	if payload == nil {
		payload = make(map[string]interface{})
	}
	payload["ci"] = map[string]interface{}{
		"engine":                "gitlab_runner",
		"config_path":           firstNonEmptyString(managed.CIConfigPath, managedGitLabCIPath),
		"dockerfile_path":       firstNonEmptyString(managed.DockerfilePath, inlineDockerfilePath),
		"dockerfile_mode":       "inline",
		"gitlab_ci_yaml":        managed.GitLabCIYAML,
		"gitlab_ci_yaml_custom": managed.GitLabCIYAMLCustom,
		"dockerfile_content":    managed.DockerfileContent,
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return configJSON
	}
	return string(out)
}

func stringFromCIMap(values map[string]interface{}, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func boolFromCIMap(values map[string]interface{}, key string) bool {
	value, ok := values[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "true")
	}
}

type ciVariables map[string]ciVariable

func (v ciVariables) value(name string) string {
	item, ok := v[normalizeGitLabVariableName(name)]
	if !ok || item.IsSecret {
		return ""
	}
	return strings.TrimSpace(item.Value)
}

func collectCIVariables(req *dto.PipelineRequest) ciVariables {
	result := make(ciVariables)
	for _, variable := range req.Variables {
		name := normalizeGitLabVariableName(variable.Name)
		if name == "" || !gitLabCIVariableNameExportable(name) {
			continue
		}
		val := strings.TrimSpace(variable.Value)
		if strings.EqualFold(val, "[object Object]") {
			continue
		}
		result[name] = ciVariable{Value: val, IsSecret: variable.IsSecret}
	}
	return result
}

// gitLabCIVariableNameExportable rejects names like "0" / "1" produced when a JS
// bug used Object.entries() on an array; bash cannot `export` such identifiers.
func gitLabCIVariableNameExportable(name string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	if first >= '0' && first <= '9' {
		return false
	}
	return true
}

func sortedVariableKeys(vars ciVariables) []string {
	keys := make([]string, 0, len(vars))
	for key, variable := range vars {
		if key == "" || variable.IsSecret {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func inferPipelineLanguage(req *dto.PipelineRequest, vars ciVariables) string {
	for _, name := range []string{"APP_LANGUAGE", "LANGUAGE", "RUNTIME", "FRAMEWORK"} {
		if value := strings.ToLower(vars.value(name)); value != "" {
			return value
		}
	}
	for _, stage := range req.Stages {
		for _, step := range stage.Steps {
			image := strings.ToLower(extractStepString(step, "image"))
			switch {
			case strings.Contains(image, "golang"):
				return "go"
			case strings.Contains(image, "maven"), strings.Contains(image, "jdk"), strings.Contains(image, "java"):
				return "java"
			case strings.Contains(image, "node"), strings.Contains(image, "npm"):
				return "node"
			case strings.Contains(image, "python"):
				return "python"
			}
			for _, cmd := range extractStepCommands(step) {
				lower := strings.ToLower(cmd)
				switch {
				case strings.Contains(lower, "go build") || strings.Contains(lower, "go test"):
					return "go"
				case strings.Contains(lower, "mvn ") || strings.Contains(lower, "gradle"):
					return "java"
				case strings.Contains(lower, "npm ") || strings.Contains(lower, "pnpm ") || strings.Contains(lower, "yarn "):
					return "node"
				case strings.Contains(lower, "pip ") || strings.Contains(lower, "pytest") || strings.Contains(lower, "python "):
					return "python"
				}
			}
		}
	}
	return "universal"
}

func inferStepImage(commands []string) string {
	for _, cmd := range commands {
		lower := strings.ToLower(cmd)
		switch {
		case strings.Contains(lower, "go build") || strings.Contains(lower, "go test"):
			return "golang:1.25-alpine"
		case strings.Contains(lower, "mvn ") || strings.Contains(lower, "gradle"):
			return "maven:3.9-eclipse-temurin-17"
		case strings.Contains(lower, "npm ") || strings.Contains(lower, "pnpm ") || strings.Contains(lower, "yarn "):
			return "node:20-alpine"
		case strings.Contains(lower, "pip ") || strings.Contains(lower, "pytest") || strings.Contains(lower, "python "):
			return "python:3.12-alpine"
		}
	}
	return ""
}

func extractStepCommands(step dto.Step) []string {
	var commands []string
	if step.Config == nil {
		return commands
	}
	for _, key := range []string{"commands", "script", "command"} {
		value, ok := step.Config[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []string:
			for _, cmd := range typed {
				if strings.TrimSpace(cmd) != "" {
					commands = append(commands, strings.TrimSpace(cmd))
				}
			}
		case []interface{}:
			for _, item := range typed {
				if cmd := strings.TrimSpace(fmt.Sprint(item)); cmd != "" {
					commands = append(commands, cmd)
				}
			}
		case string:
			if strings.TrimSpace(typed) != "" {
				commands = append(commands, strings.TrimSpace(typed))
			}
		}
		if len(commands) > 0 {
			return commands
		}
	}
	return commands
}

func extractStepString(step dto.Step, key string) string {
	if step.Config == nil {
		return ""
	}
	value, ok := step.Config[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func shellQuote(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "''"
	}
	if strings.HasPrefix(value, "$") && !strings.ContainsAny(value, " \t\n'\"") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func sanitizeGitLabStageName(id, name string) string {
	return semanticGitLabIdentifier(id, name, "stage")
}

func sanitizeGitLabJobName(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := sanitizeGitLabIdentifier(part); value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return "job"
	}
	return strings.Join(values, "_")
}

func gitLabStageName(stage dto.Stage) string {
	return semanticGitLabIdentifier(stage.ID, stage.Name, "stage")
}

func gitLabJobName(stage dto.Stage, step dto.Step) string {
	switch step.Type {
	case "docker_build", "docker_push":
		return "build_and_push_image"
	}
	if value := sanitizeGitLabIdentifier(step.ID); value != "" && value != "item" && !looksGeneratedClientID(step.ID) {
		return value
	}
	if value := sanitizeGitLabIdentifier(step.Name); value != "" && value != "item" && !looksGenericClientName(step.Name) {
		return value
	}
	stepName := semanticGitLabIdentifier(step.ID, step.Name, "job")
	if stepName != "job" {
		return stepName
	}
	return sanitizeGitLabJobName(gitLabStageName(stage), stepName)
}

func uniqueGitLabJobName(base string, seen map[string]int) string {
	base = sanitizeGitLabIdentifier(base)
	if base == "" || base == "item" {
		base = "job"
	}
	count := seen[base]
	if count == 0 {
		seen[base] = 1
		return base
	}
	seen[base] = count + 1
	return fmt.Sprintf("%s_%d", base, count+1)
}

func semanticGitLabIdentifier(id, name, fallback string) string {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if !looksGeneratedClientID(id) {
		if value := sanitizeGitLabIdentifier(id); value != "" && value != "item" {
			return value
		}
	}
	if alias := gitLabNameAlias(name); alias != "" {
		return alias
	}
	if value := sanitizeGitLabIdentifier(name); value != "" && value != "item" {
		return value
	}
	if alias := gitLabNameAlias(id); alias != "" {
		return alias
	}
	if value := sanitizeGitLabIdentifier(id); value != "" && value != "item" {
		return value
	}
	if value := sanitizeGitLabIdentifier(fallback); value != "" && value != "item" {
		return value
	}
	return "item"
}

func looksGeneratedClientID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 8 {
		return false
	}
	hasDigit := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			return false
		}
	}
	return hasDigit
}

func looksGenericClientName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	lower := strings.ToLower(value)
	if lower == "job" || lower == "step" || lower == "stage" {
		return true
	}
	return strings.HasPrefix(value, "步骤 ") || strings.HasPrefix(value, "阶段 ")
}

func gitLabNameAlias(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(value, "代码检出") || strings.Contains(value, "源码检出") || strings.Contains(value, "拉取代码") || strings.Contains(lower, "git clone") || strings.Contains(lower, "checkout"):
		return "checkout"
	case strings.Contains(value, "单元测试") || strings.Contains(value, "测试") || strings.Contains(lower, "go test") || strings.Contains(lower, "maven test") || strings.Contains(lower, "npm test") || strings.Contains(lower, "pytest"):
		return "test"
	case strings.Contains(value, "镜像构建") || strings.Contains(value, "生成镜像") || strings.Contains(value, "构建镜像") || strings.Contains(value, "推送镜像") || strings.Contains(lower, "docker build") || strings.Contains(lower, "docker push") || strings.Contains(lower, "image"):
		return "image"
	case strings.Contains(value, "编译构建") || strings.Contains(value, "编译") || strings.Contains(lower, "compile"):
		return "build"
	case strings.Contains(value, "GitOps") || strings.Contains(value, "gitops") || strings.Contains(value, "交接") || strings.Contains(lower, "handoff"):
		return "gitops"
	case strings.Contains(value, "发布") || strings.Contains(value, "部署") || strings.Contains(lower, "deploy") || strings.Contains(lower, "release"):
		return "deploy"
	}
	return ""
}

func sanitizeGitLabIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "item"
	}
	return result
}

func normalizeGitLabVariableName(name string) string {
	name = strings.ToUpper(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type orderedPair struct {
	Key   string
	Value interface{}
}

type orderedMap []orderedPair

func (m orderedMap) hasJob() bool {
	for _, pair := range m {
		if strings.HasPrefix(pair.Key, "#") {
			continue
		}
		switch pair.Key {
		case "stages", "variables":
			continue
		default:
			return true
		}
	}
	return false
}

func marshalOrderedYAML(m orderedMap) string {
	node := orderedMapYAMLNode(m, 0)
	out, err := yaml.Marshal(node)
	if err != nil {
		return "# " + gitLabProvisioningLabel + "\n"
	}
	return string(out)
}

func orderedMapYAMLNode(m orderedMap, level int) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, pair := range m {
		if strings.HasPrefix(pair.Key, "#") && pair.Value == nil {
			node.HeadComment = strings.TrimPrefix(pair.Key, "# ")
			continue
		}
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: pair.Key}
		valueNode := yamlNode(pair.Value, level+1)
		node.Content = append(node.Content, keyNode, valueNode)
	}
	return node
}

func yamlNode(value interface{}, level int) *yaml.Node {
	switch typed := value.(type) {
	case orderedMap:
		return orderedMapYAMLNode(typed, level)
	case []orderedMap:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range typed {
			node.Content = append(node.Content, orderedMapYAMLNode(item, level+1))
		}
		return node
	case []string:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range typed {
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: item})
		}
		return node
	case string:
		style := yaml.Style(0)
		if strings.Contains(typed, "$") || strings.Contains(typed, ":") || strings.Contains(typed, "[") || strings.Contains(typed, "]") {
			style = yaml.DoubleQuotedStyle
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Value: typed, Style: style}
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(typed)}
	case bool:
		if typed {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
	default:
		bytes, _ := json.Marshal(typed)
		return &yaml.Node{Kind: yaml.ScalarNode, Value: string(bytes)}
	}
}

func encodeGitLabExternalRef(info gitLabPipelineInfo) string {
	if info.ID <= 0 {
		return ""
	}
	payload := map[string]interface{}{
		"provider":     "gitlab",
		"pipeline_id":  info.ID,
		"pipeline_iid": info.IID,
		"web_url":      info.WebURL,
	}
	body, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(body)
}

func decodeGitLabExternalRef(value string) (gitLabPipelineInfo, bool) {
	if strings.TrimSpace(value) == "" {
		return gitLabPipelineInfo{}, false
	}
	body, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		if id, parseErr := strconv.Atoi(strings.TrimSpace(value)); parseErr == nil && id > 0 {
			return gitLabPipelineInfo{ID: id}, true
		}
		return gitLabPipelineInfo{}, false
	}
	var payload struct {
		Provider    string `json:"provider"`
		PipelineID  int    `json:"pipeline_id"`
		PipelineIID int    `json:"pipeline_iid"`
		WebURL      string `json:"web_url"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return gitLabPipelineInfo{}, false
	}
	if payload.Provider != "gitlab" || payload.PipelineID <= 0 {
		return gitLabPipelineInfo{}, false
	}
	return gitLabPipelineInfo{ID: payload.PipelineID, IID: payload.PipelineIID, WebURL: payload.WebURL}, true
}

func gitLabStatusToRunStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success":
		return "success"
	case "failed", "skipped":
		return "failed"
	case "canceled", "cancelled":
		return "cancelled"
	case "running", "pending", "created", "preparing", "waiting_for_resource",
		"manual", "blocked", "scheduled":
		// manual/blocked: pipeline waiting on manual jobs or protected resources — still in-flight from DevOps' perspective
		return "running"
	default:
		return "pending"
	}
}

func gitLabStatusIsTerminal(status string) bool {
	switch gitLabStatusToRunStatus(status) {
	case "success", "failed", "cancelled":
		return true
	default:
		return false
	}
}
