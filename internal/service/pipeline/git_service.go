package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
	"devops/pkg/dto"
	"devops/pkg/logger"
)

// GitService Git 仓库服务
type GitService struct {
	db            *gorm.DB
	credentialSvc *CredentialService
}

// NewGitService 创建 Git 仓库服务
func NewGitService(db *gorm.DB) *GitService {
	return &GitService{
		db:            db,
		credentialSvc: NewCredentialService(db),
	}
}

// List 获取 Git 仓库列表
func (s *GitService) List(ctx context.Context, req *dto.GitRepoListRequest) (*dto.GitRepoListResponse, error) {
	var repos []models.GitRepository
	var total int64

	query := s.db.Model(&models.GitRepository{})

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Provider != "" {
		query = query.Where("provider = ?", req.Provider)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("created_at DESC").Find(&repos).Error; err != nil {
		return nil, err
	}

	items := make([]dto.GitRepoItem, len(repos))
	for i, repo := range repos {
		items[i] = s.toGitRepoItem(&repo)
	}

	return &dto.GitRepoListResponse{
		Total: int(total),
		Items: items,
	}, nil
}

// Get 获取 Git 仓库详情
func (s *GitService) Get(ctx context.Context, id uint) (*dto.GitRepoItem, error) {
	var repo models.GitRepository
	if err := s.db.First(&repo, id).Error; err != nil {
		return nil, err
	}

	item := s.toGitRepoItem(&repo)
	return &item, nil
}

// Create 创建 Git 仓库
func (s *GitService) Create(ctx context.Context, req *dto.GitRepoRequest) (*dto.GitRepoItem, error) {
	// 检测 Provider
	provider := req.Provider
	if provider == "" {
		provider = s.detectProvider(req.URL)
	}

	// 生成 Webhook Secret
	webhookSecret := s.generateWebhookSecret()

	var repo models.GitRepository
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo = models.GitRepository{
			Name:          req.Name,
			URL:           req.URL,
			Provider:      provider,
			DefaultBranch: req.DefaultBranch,
			CredentialID:  req.CredentialID,
			WebhookSecret: webhookSecret,
			Description:   req.Description,
		}

		if repo.DefaultBranch == "" {
			repo.DefaultBranch = "main"
		}

		if err := tx.Create(&repo).Error; err != nil {
			return err
		}

		repo.WebhookURL = s.generateWebhookURL(repo.Provider, repo.ID)
		if err := tx.Save(&repo).Error; err != nil {
			return err
		}

		return s.syncProviderWebhook(ctx, tx, &repo)
	}); err != nil {
		return nil, err
	}

	item := s.toGitRepoItem(&repo)
	return &item, nil
}

// Update 更新 Git 仓库
func (s *GitService) Update(ctx context.Context, id uint, req *dto.GitRepoRequest) (*dto.GitRepoItem, error) {
	var repo models.GitRepository

	provider := req.Provider
	if provider == "" {
		provider = s.detectProvider(req.URL)
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&repo, id).Error; err != nil {
			return err
		}

		repo.Name = req.Name
		repo.URL = req.URL
		repo.Provider = provider
		repo.DefaultBranch = req.DefaultBranch
		repo.CredentialID = req.CredentialID
		repo.Description = req.Description
		repo.WebhookURL = s.generateWebhookURL(repo.Provider, repo.ID)

		if err := tx.Save(&repo).Error; err != nil {
			return err
		}

		return s.syncProviderWebhook(ctx, tx, &repo)
	}); err != nil {
		return nil, err
	}

	item := s.toGitRepoItem(&repo)
	return &item, nil
}

// Delete 删除 Git 仓库
func (s *GitService) Delete(ctx context.Context, id uint) error {
	// 检查是否有流水线引用
	var count int64
	s.db.Model(&models.Pipeline{}).Where("git_repo_id = ?", id).Count(&count)
	if count > 0 {
		return fmt.Errorf("该仓库被 %d 个流水线引用，无法删除", count)
	}

	return s.db.Delete(&models.GitRepository{}, id).Error
}

// TestConnection 测试仓库连接
func (s *GitService) TestConnection(ctx context.Context, req *dto.GitTestConnectionRequest) (*dto.GitTestConnectionResponse, error) {
	log := logger.L().WithField("url", req.URL)
	log.Info("测试 Git 仓库连接")

	cred, err := s.loadGitCredential(ctx, req.CredentialID)
	if err != nil {
		return &dto.GitTestConnectionResponse{
			Success: false,
			Message: "获取凭证失败: " + err.Error(),
		}, nil
	}

	provider := s.detectProvider(req.URL)
	if err := s.validateRemoteRepository(ctx, provider, req.URL, cred); err != nil {
		return &dto.GitTestConnectionResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &dto.GitTestConnectionResponse{
		Success: true,
		Message: "连接成功，仓库可访问",
	}, nil
}

// GetBranches 获取分支列表
func (s *GitService) GetBranches(ctx context.Context, id uint) ([]dto.GitBranchItem, error) {
	var repo models.GitRepository
	if err := s.db.First(&repo, id).Error; err != nil {
		return nil, err
	}
	cred, err := s.loadGitCredential(ctx, repo.CredentialID)
	if err != nil {
		return nil, err
	}
	return s.fetchBranches(ctx, &repo, cred)
}

// GetTags 获取 Tag 列表
func (s *GitService) GetTags(ctx context.Context, id uint) ([]dto.GitTagItem, error) {
	var repo models.GitRepository
	if err := s.db.First(&repo, id).Error; err != nil {
		return nil, err
	}
	cred, err := s.loadGitCredential(ctx, repo.CredentialID)
	if err != nil {
		return nil, err
	}
	return s.fetchTags(ctx, &repo, cred)
}

// RegenerateWebhookSecret 重新生成 Webhook Secret
func (s *GitService) RegenerateWebhookSecret(ctx context.Context, id uint) (string, error) {
	var repo models.GitRepository
	if err := s.db.First(&repo, id).Error; err != nil {
		return "", err
	}

	repo.WebhookSecret = s.generateWebhookSecret()
	if err := s.db.Save(&repo).Error; err != nil {
		return "", err
	}

	return repo.WebhookSecret, nil
}

// GetByID 根据 ID 获取仓库（内部使用）
func (s *GitService) GetByID(ctx context.Context, id uint) (*models.GitRepository, error) {
	var repo models.GitRepository
	if err := s.db.First(&repo, id).Error; err != nil {
		return nil, err
	}
	return &repo, nil
}

// toGitRepoItem 转换为 DTO
func (s *GitService) toGitRepoItem(repo *models.GitRepository) dto.GitRepoItem {
	item := dto.GitRepoItem{
		ID:            repo.ID,
		Name:          repo.Name,
		URL:           repo.URL,
		Provider:      repo.Provider,
		DefaultBranch: repo.DefaultBranch,
		CredentialID:  repo.CredentialID,
		WebhookURL:    repo.WebhookURL,
		Description:   repo.Description,
		CreatedAt:     repo.CreatedAt,
	}

	// 获取凭证名称
	if repo.CredentialID != nil {
		var cred models.PipelineCredential
		if err := s.db.First(&cred, *repo.CredentialID).Error; err == nil {
			item.CredentialName = cred.Name
		}
	}

	return item
}

// detectProvider 检测 Git 提供商
func (s *GitService) detectProvider(repoURL string) string {
	lowerURL := strings.ToLower(repoURL)
	if strings.Contains(lowerURL, "github.com") {
		return "github"
	}
	if strings.Contains(lowerURL, "gitlab.com") || strings.Contains(lowerURL, "gitlab") {
		return "gitlab"
	}
	if strings.Contains(lowerURL, "gitee.com") {
		return "gitee"
	}
	return "custom"
}

// generateWebhookSecret 生成 Webhook Secret
func (s *GitService) generateWebhookSecret() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateWebhookURL 生成 Webhook URL
func (s *GitService) generateWebhookURL(provider string, repoID uint) string {
	switch provider {
	case "github":
		return fmt.Sprintf("/app/api/v1/webhook/github/%d", repoID)
	case "gitee":
		return fmt.Sprintf("/app/api/v1/webhook/gitee/%d", repoID)
	case "gitlab":
		fallthrough
	default:
		return fmt.Sprintf("/app/api/v1/webhook/gitlab/%d", repoID)
	}
}

func (s *GitService) syncProviderWebhook(ctx context.Context, db *gorm.DB, repo *models.GitRepository) error {
	switch repo.Provider {
	case "gitlab":
		return s.syncGitLabWebhook(ctx, db, repo)
	default:
		return nil
	}
}

type gitCredential struct {
	Username string
	Password string
	Token    string
}

func (s *GitService) loadGitCredential(ctx context.Context, credentialID *uint) (*gitCredential, error) {
	if credentialID == nil {
		return &gitCredential{}, nil
	}
	cred, err := s.credentialSvc.GetDecryptedData(ctx, *credentialID)
	if err != nil {
		return nil, err
	}
	return &gitCredential{
		Username: strings.TrimSpace(cred.Username),
		Password: strings.TrimSpace(cred.Password),
		Token:    strings.TrimSpace(cred.Token),
	}, nil
}

func (s *GitService) validateRemoteRepository(ctx context.Context, provider, repoURL string, cred *gitCredential) error {
	switch provider {
	case "gitlab":
		_, err := s.fetchGitLabProject(ctx, repoURL, cred)
		return err
	case "github":
		_, err := s.fetchGitHubRepository(ctx, repoURL, cred)
		return err
	case "gitee":
		_, err := s.fetchGiteeRepository(ctx, repoURL, cred)
		return err
	default:
		return s.probeRepositoryURL(ctx, repoURL, cred)
	}
}

func (s *GitService) fetchBranches(ctx context.Context, repo *models.GitRepository, cred *gitCredential) ([]dto.GitBranchItem, error) {
	switch repo.Provider {
	case "gitlab":
		return s.fetchGitLabBranches(ctx, repo, cred)
	case "github":
		return s.fetchGitHubBranches(ctx, repo, cred)
	case "gitee":
		return s.fetchGiteeBranches(ctx, repo, cred)
	default:
		return []dto.GitBranchItem{{Name: repo.DefaultBranch, IsDefault: true}}, nil
	}
}

func (s *GitService) fetchTags(ctx context.Context, repo *models.GitRepository, cred *gitCredential) ([]dto.GitTagItem, error) {
	switch repo.Provider {
	case "gitlab":
		return s.fetchGitLabTags(ctx, repo, cred)
	case "github":
		return s.fetchGitHubTags(ctx, repo, cred)
	case "gitee":
		return s.fetchGiteeTags(ctx, repo, cred)
	default:
		return []dto.GitTagItem{}, nil
	}
}

func (s *GitService) fetchGitLabProject(ctx context.Context, repoURL string, cred *gitCredential) (map[string]any, error) {
	projectPath, baseURL, err := parseGitProjectURL(repoURL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v4/projects/" + url.PathEscape(projectPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "gitlab", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *GitService) fetchGitLabBranches(ctx context.Context, repo *models.GitRepository, cred *gitCredential) ([]dto.GitBranchItem, error) {
	projectPath, baseURL, err := parseGitProjectURL(repo.URL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v4/projects/" + url.PathEscape(projectPath) + "/repository/branches?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "gitlab", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Name      string `json:"name"`
		Default   bool   `json:"default"`
		Commit    struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]dto.GitBranchItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, dto.GitBranchItem{Name: item.Name, IsDefault: item.Default, CommitSHA: item.Commit.ID})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsDefault != items[j].IsDefault {
			return items[i].IsDefault
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (s *GitService) fetchGitLabTags(ctx context.Context, repo *models.GitRepository, cred *gitCredential) ([]dto.GitTagItem, error) {
	projectPath, baseURL, err := parseGitProjectURL(repo.URL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v4/projects/" + url.PathEscape(projectPath) + "/repository/tags?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "gitlab", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Name   string `json:"name"`
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]dto.GitTagItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, dto.GitTagItem{Name: item.Name, CommitSHA: item.Commit.ID})
	}
	return items, nil
}

func (s *GitService) fetchGitHubRepository(ctx context.Context, repoURL string, cred *gitCredential) (map[string]any, error) {
	owner, repoName, baseURL, err := parseGitOwnerRepo(repoURL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/repos/" + owner + "/" + repoName
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "github", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *GitService) fetchGitHubBranches(ctx context.Context, repo *models.GitRepository, cred *gitCredential) ([]dto.GitBranchItem, error) {
	repoPayload, err := s.fetchGitHubRepository(ctx, repo.URL, cred)
	if err != nil {
		return nil, err
	}
	defaultBranch, _ := repoPayload["default_branch"].(string)
	owner, repoName, baseURL, err := parseGitOwnerRepo(repo.URL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/repos/" + owner + "/" + repoName + "/branches?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "github", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]dto.GitBranchItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, dto.GitBranchItem{Name: item.Name, IsDefault: item.Name == defaultBranch, CommitSHA: item.Commit.SHA})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsDefault != items[j].IsDefault {
			return items[i].IsDefault
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (s *GitService) fetchGitHubTags(ctx context.Context, repo *models.GitRepository, cred *gitCredential) ([]dto.GitTagItem, error) {
	owner, repoName, baseURL, err := parseGitOwnerRepo(repo.URL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/repos/" + owner + "/" + repoName + "/tags?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "github", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]dto.GitTagItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, dto.GitTagItem{Name: item.Name, CommitSHA: item.Commit.SHA})
	}
	return items, nil
}

func (s *GitService) fetchGiteeRepository(ctx context.Context, repoURL string, cred *gitCredential) (map[string]any, error) {
	owner, repoName, baseURL, err := parseGitOwnerRepo(repoURL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v5/repos/" + owner + "/" + repoName
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "gitee", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *GitService) fetchGiteeBranches(ctx context.Context, repo *models.GitRepository, cred *gitCredential) ([]dto.GitBranchItem, error) {
	repoPayload, err := s.fetchGiteeRepository(ctx, repo.URL, cred)
	if err != nil {
		return nil, err
	}
	defaultBranch, _ := repoPayload["default_branch"].(string)
	owner, repoName, baseURL, err := parseGitOwnerRepo(repo.URL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v5/repos/" + owner + "/" + repoName + "/branches"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "gitee", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]dto.GitBranchItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, dto.GitBranchItem{Name: item.Name, IsDefault: item.Name == defaultBranch, CommitSHA: item.Commit.SHA})
	}
	return items, nil
}

func (s *GitService) fetchGiteeTags(ctx context.Context, repo *models.GitRepository, cred *gitCredential) ([]dto.GitTagItem, error) {
	owner, repoName, baseURL, err := parseGitOwnerRepo(repo.URL)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v5/repos/" + owner + "/" + repoName + "/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyGitAuthHeaders(req, "gitee", cred)
	body, err := doGitAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]dto.GitTagItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, dto.GitTagItem{Name: item.Name, CommitSHA: item.Commit.SHA})
	}
	return items, nil
}

func (s *GitService) probeRepositoryURL(ctx context.Context, repoURL string, cred *gitCredential) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, repoURL, nil)
	if err != nil {
		return err
	}
	applyGitAuthHeaders(req, "custom", cred)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接仓库失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	if resp.StatusCode == http.StatusMethodNotAllowed {
		getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, repoURL, nil)
		applyGitAuthHeaders(getReq, "custom", cred)
		getResp, getErr := client.Do(getReq)
		if getErr == nil {
			defer getResp.Body.Close()
			if getResp.StatusCode >= 200 && getResp.StatusCode < 400 {
				return nil
			}
			return fmt.Errorf("仓库返回状态码 %d", getResp.StatusCode)
		}
	}
	return fmt.Errorf("仓库返回状态码 %d", resp.StatusCode)
}

func applyGitAuthHeaders(req *http.Request, provider string, cred *gitCredential) {
	if req == nil || cred == nil {
		return
	}
	token := cred.Token
	if token == "" {
		token = cred.Password
	}
	switch provider {
	case "gitlab":
		if token != "" {
			req.Header.Set("PRIVATE-TOKEN", token)
		}
	case "github":
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
	case "gitee":
		if token != "" {
			q := req.URL.Query()
			q.Set("access_token", token)
			req.URL.RawQuery = q.Encode()
		}
	default:
		if cred.Username != "" && token != "" {
			req.SetBasicAuth(cred.Username, token)
		}
	}
}

func doGitAPIRequest(req *http.Request) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求远端仓库失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("远端仓库返回异常: %s", message)
	}
	return body, nil
}

func parseGitProjectURL(repoURL string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil {
		return "", "", err
	}
	projectPath := strings.Trim(strings.TrimSuffix(parsed.Path, ".git"), "/")
	if projectPath == "" {
		return "", "", fmt.Errorf("无法解析仓库路径")
	}
	baseURL := parsed.Scheme + "://" + parsed.Host
	return projectPath, baseURL, nil
}

func parseGitOwnerRepo(repoURL string) (string, string, string, error) {
	projectPath, baseURL, err := parseGitProjectURL(repoURL)
	if err != nil {
		return "", "", "", err
	}
	parts := strings.Split(projectPath, "/")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("无法解析 owner/repo")
	}
	owner := parts[0]
	repoName := parts[len(parts)-1]
	return owner, repoName, baseURL, nil
}

func (s *GitService) syncGitLabWebhook(ctx context.Context, db *gorm.DB, repo *models.GitRepository) error {
	if repo == nil || repo.CredentialID == nil {
		return nil
	}

	credentialSvc := NewCredentialService(db)
	cred, err := credentialSvc.GetDecryptedData(ctx, *repo.CredentialID)
	if err != nil {
		return fmt.Errorf("获取 GitLab 凭证失败: %w", err)
	}

	token := strings.TrimSpace(cred.Token)
	if token == "" {
		token = strings.TrimSpace(cred.Password)
	}
	if token == "" {
		return fmt.Errorf("GitLab 凭证缺少可用 Token")
	}

	baseURL, projectPath, err := s.parseGitLabRepositoryURL(repo.URL)
	if err != nil {
		return fmt.Errorf("解析 GitLab 仓库地址失败: %w", err)
	}

	callbackURL, err := s.resolveWebhookCallbackURL(repo.WebhookURL)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	if s.isLocalWebhookTarget(callbackURL) {
		if err := s.enableGitLabLocalWebhookRequests(ctx, client, baseURL, token); err != nil {
			logger.L().WithField("repo_id", repo.ID).WithField("error", err).
				Warn("启用 GitLab 本地 Webhook 请求失败，继续尝试创建项目 Hook")
		}
	}

	if err := s.upsertGitLabProjectHook(ctx, client, baseURL, projectPath, callbackURL, repo.WebhookSecret, token); err != nil {
		return fmt.Errorf("同步 GitLab Webhook 失败: %w", err)
	}
	return nil
}

func (s *GitService) parseGitLabRepositoryURL(repoURL string) (string, string, error) {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("当前仅支持 HTTP/HTTPS GitLab 仓库地址")
	}

	projectPath := strings.Trim(parsed.Path, "/")
	projectPath = strings.TrimSuffix(projectPath, ".git")
	if projectPath == "" {
		return "", "", fmt.Errorf("仓库路径为空")
	}

	host := parsed.Host
	if strings.TrimSpace(os.Getenv("MYSQL_HOST")) == "mysql" && s.isLocalHostName(parsed.Hostname()) {
		host = "gitlab"
	}

	return fmt.Sprintf("%s://%s", parsed.Scheme, host), projectPath, nil
}

func (s *GitService) resolveWebhookCallbackURL(webhookPath string) (string, error) {
	if webhookPath == "" {
		return "", fmt.Errorf("Webhook 地址为空")
	}
	if strings.HasPrefix(webhookPath, "http://") || strings.HasPrefix(webhookPath, "https://") {
		return webhookPath, nil
	}
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("WEBHOOK_BASE_URL")), "/")
	if baseURL == "" && strings.TrimSpace(os.Getenv("MYSQL_HOST")) == "mysql" {
		baseURL = "http://devops"
	}
	if baseURL == "" {
		port := strings.TrimSpace(os.Getenv("PORT"))
		switch port {
		case "", "80":
			baseURL = "http://localhost"
		case "443":
			baseURL = "https://localhost"
		default:
			baseURL = fmt.Sprintf("http://localhost:%s", port)
		}
	}
	return baseURL + webhookPath, nil
}

func (s *GitService) isLocalWebhookTarget(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return s.isLocalHostName(parsed.Hostname())
}

func (s *GitService) isLocalHostName(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "devops", "gitlab", "host.docker.internal":
		return true
	default:
		return strings.HasSuffix(strings.ToLower(host), ".local")
	}
}

func (s *GitService) enableGitLabLocalWebhookRequests(ctx context.Context, client *http.Client, baseURL, token string) error {
	form := url.Values{}
	form.Set("allow_local_requests_from_web_hooks_and_services", "true")
	_, _, err := s.gitLabRequest(ctx, client, http.MethodPut, baseURL, "/api/v4/application/settings", token, form)
	return err
}

func (s *GitService) upsertGitLabProjectHook(ctx context.Context, client *http.Client, baseURL, projectPath, callbackURL, secret, token string) error {
	endpoint := fmt.Sprintf("/api/v4/projects/%s/hooks", url.PathEscape(projectPath))
	body, _, err := s.gitLabRequest(ctx, client, http.MethodGet, baseURL, endpoint, token, nil)
	if err != nil {
		return err
	}

	var hooks []gitLabProjectHook
	if err := json.Unmarshal(body, &hooks); err != nil {
		return fmt.Errorf("解析 GitLab Hook 列表失败: %w", err)
	}

	form := url.Values{}
	form.Set("url", callbackURL)
	form.Set("push_events", "true")
	form.Set("enable_ssl_verification", "false")
	form.Set("token", secret)

	for _, hook := range hooks {
		if hook.URL != callbackURL {
			continue
		}
		_, _, err = s.gitLabRequest(ctx, client, http.MethodPut, baseURL, fmt.Sprintf("%s/%d", endpoint, hook.ID), token, form)
		return err
	}

	_, _, err = s.gitLabRequest(ctx, client, http.MethodPost, baseURL, endpoint, token, form)
	return err
}

func (s *GitService) gitLabRequest(ctx context.Context, client *http.Client, method, baseURL, endpoint, token string, form url.Values) ([]byte, int, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+endpoint, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

type gitLabProjectHook struct {
	ID                    int    `json:"id"`
	URL                   string `json:"url"`
	PushEvents            bool   `json:"push_events"`
	EnableSSLVerification bool   `json:"enable_ssl_verification"`
}
