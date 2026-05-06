// Package dto
//
// release.go: 发布主单（Release）相关的请求/响应 DTO（v2.0）。
// 配合 internal/models/deploy/release.go 使用。
package dto

import "time"

// ReleaseV2DTO 发布主单的列表/详情统一响应体（v2.0）。
//
// 注：用 V2 后缀以与未来可能引入的 ReleaseV1DTO 区分；
// 字段命名与 model.Release 保持一致以减少前后端心智负担。
type ReleaseV2DTO struct {
	ID              uint       `json:"id"`
	Title           string     `json:"title"`
	ApplicationID   *uint      `json:"application_id"`
	ApplicationName string     `json:"application_name"`
	Env             string     `json:"env"`
	Version         string     `json:"version"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`
	RiskLevel       string     `json:"risk_level"`
	RiskScore       int        `json:"risk_score"`
	RolloutStrategy string     `json:"rollout_strategy"`
	CreatedBy       uint       `json:"created_by"`
	CreatedByName   string     `json:"created_by_name"`
	CreatedAt       time.Time  `json:"created_at"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	ApprovedByName  string     `json:"approved_by_name,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	PublishedByName string     `json:"published_by_name,omitempty"`

	// v2.0 新增字段
	ApprovalInstanceID    *uint                  `json:"approval_instance_id,omitempty"`
	GitOpsChangeRequestID *uint                  `json:"gitops_change_request_id,omitempty"`
	ArgoAppName           string                 `json:"argo_app_name,omitempty"`
	ArgoSyncStatus        string                 `json:"argo_sync_status,omitempty"`
	JiraIssueKeys         []string               `json:"jira_issue_keys,omitempty"`
	RolloutConfig         map[string]interface{} `json:"rollout_config,omitempty"`
	RiskFactors           map[string]interface{} `json:"risk_factors,omitempty"`

	// 子项汇总（按类型计数，UI 用于"3 部署 / 1 配置 / 2 数据库" 这种概览）
	ItemSummary map[string]int `json:"item_summary,omitempty"`
}

// CreateReleaseRequest 创建发布主单请求体。
type CreateReleaseRequest struct {
	Title           string                 `json:"title" binding:"required,max=200"`
	ApplicationID   *uint                  `json:"application_id"`
	ApplicationName string                 `json:"application_name"`
	Env             string                 `json:"env" binding:"required"`
	Version         string                 `json:"version"`
	Description     string                 `json:"description"`
	RiskLevel       string                 `json:"risk_level"`
	RolloutStrategy string                 `json:"rollout_strategy"` // direct/canary/blue_green
	RolloutConfig   map[string]interface{} `json:"rollout_config,omitempty"`
	JiraIssueKeys   []string               `json:"jira_issue_keys,omitempty"`
}

// CreateReleaseFromPipelineRunRequest 从一次成功 CI 运行生成发布主单。
// 这是应用交付主链路中 CI -> Release 的标准入口。
type CreateReleaseFromPipelineRunRequest struct {
	PipelineRunID     uint                   `json:"pipeline_run_id" binding:"required"`
	ExistingReleaseID uint                   `json:"existing_release_id,omitempty"`
	Title             string                 `json:"title"`
	Env               string                 `json:"env"`
	Version           string                 `json:"version"`
	Description       string                 `json:"description"`
	RiskLevel         string                 `json:"risk_level"`
	RolloutStrategy   string                 `json:"rollout_strategy"`
	RolloutConfig     map[string]interface{} `json:"rollout_config,omitempty"`
}

// UpdateReleaseRequest 编辑发布主单请求体（仅 draft 状态可编辑）。
type UpdateReleaseRequest struct {
	Title           string                 `json:"title" binding:"max=200"`
	Description     string                 `json:"description"`
	RiskLevel       string                 `json:"risk_level"`
	RolloutStrategy string                 `json:"rollout_strategy"`
	RolloutConfig   map[string]interface{} `json:"rollout_config,omitempty"`
	JiraIssueKeys   []string               `json:"jira_issue_keys,omitempty"`
}

// ReleaseListResponse 列表响应。
type ReleaseListResponse struct {
	List       []ReleaseV2DTO `json:"list"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

// ReleaseOverviewDTO 聚合发布主链路状态，供 Release 详情首屏使用。
type ReleaseOverviewDTO struct {
	ReleaseID     uint                      `json:"release_id"`
	Status        string                    `json:"status"`
	CurrentStage  string                    `json:"current_stage"`
	Blocked       bool                      `json:"blocked"`
	BlockedReason string                    `json:"blocked_reason,omitempty"`
	NextAction    string                    `json:"next_action"`
	Approval      ReleaseOverviewApproval   `json:"approval"`
	GitOps        ReleaseOverviewGitOps     `json:"gitops"`
	ArgoCD        ReleaseOverviewArgoCD     `json:"argocd"`
	Stages        []ReleaseOverviewStageDTO `json:"stages"`
}

type ReleaseOverviewStageDTO struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"` // wait/process/finish/error
	Message string `json:"message,omitempty"`
}

type ReleaseOverviewApproval struct {
	InstanceID       *uint      `json:"instance_id,omitempty"`
	Status           string     `json:"status"`
	ChainName        string     `json:"chain_name,omitempty"`
	CurrentNodeOrder int        `json:"current_node_order,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

type ReleaseOverviewGitOps struct {
	ChangeRequestID *uint      `json:"change_request_id,omitempty"`
	Status          string     `json:"status"`
	MRURL           string     `json:"mr_url,omitempty"`
	ApprovalStatus  string     `json:"approval_status,omitempty"`
	AutoMergeStatus string     `json:"auto_merge_status,omitempty"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
}

type ReleaseOverviewArgoCD struct {
	ApplicationID *uint      `json:"application_id,omitempty"`
	AppName       string     `json:"app_name,omitempty"`
	SyncStatus    string     `json:"sync_status,omitempty"`
	HealthStatus  string     `json:"health_status,omitempty"`
	DriftDetected bool       `json:"drift_detected"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
}

// GitOpsPRRequest 触发 GitOps PR 生成的请求体（v2.0）。
//
// 调用前置：Release 已 approved。
type GitOpsPRRequest struct {
	ReleaseID    uint   `json:"release_id" binding:"required"`
	TargetBranch string `json:"target_branch"` // 默认 main
	CommitMsg    string `json:"commit_message,omitempty"`
	DryRun       bool   `json:"dry_run,omitempty"`
}

// GitOpsPRResponse GitOps PR 生成结果。
type GitOpsPRResponse struct {
	ChangeRequestID uint     `json:"change_request_id"`
	PRURL           string   `json:"pr_url,omitempty"`
	BranchName      string   `json:"branch_name"`
	FilesChanged    []string `json:"files_changed"`
	DryRun          bool     `json:"dry_run"`
	Message         string   `json:"message,omitempty"`
}
