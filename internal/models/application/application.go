// Package application 定义应用管理相关的数据模型
// 本文件包含应用管理相关的模型定义
package application

import (
	"time"
)

// ==================== 应用管理模型 ====================

// Organization 组织
type Organization struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	DisplayName string    `gorm:"size:200" json:"display_name"`
	Description string    `gorm:"type:text" json:"description"`
	Owner       string    `gorm:"size:100" json:"owner"`
	Status      string    `gorm:"size:20;default:'active'" json:"status"`
}

func (Organization) TableName() string { return "organizations" }

// Project 项目
type Project struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	OrganizationID uint      `gorm:"not null;index" json:"organization_id"`
	Name           string    `gorm:"size:100;not null" json:"name"`
	DisplayName    string    `gorm:"size:200" json:"display_name"`
	Description    string    `gorm:"type:text" json:"description"`
	Owner          string    `gorm:"size:100" json:"owner"`
	Status         string    `gorm:"size:20;default:'active'" json:"status"`
	OrgName        string    `gorm:"-" json:"org_name,omitempty"`
}

func (Project) TableName() string { return "projects" }

// EnvDefinition 自定义环境定义
type EnvDefinition struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Name        string    `gorm:"size:50;not null;uniqueIndex" json:"name"`
	DisplayName string    `gorm:"size:100" json:"display_name"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	Color       string    `gorm:"size:20;default:'blue'" json:"color"`
}

func (EnvDefinition) TableName() string { return "env_definitions" }

// Application 应用模型
// 存储应用的基本信息和关联配置
type Application struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Name              string    `gorm:"size:100;not null;uniqueIndex" json:"name"` // 应用名称，唯一
	DisplayName       string    `gorm:"size:200" json:"display_name"`              // 显示名称
	Description       string    `gorm:"type:text" json:"description"`              // 描述
	OrganizationID    *uint     `gorm:"index" json:"organization_id"`              // 所属组织
	ProjectID         *uint     `gorm:"index" json:"project_id"`                   // 所属项目
	GitRepo           string    `gorm:"size:500" json:"git_repo"`                  // Git 仓库地址
	Language          string    `gorm:"size:50" json:"language"`                   // 开发语言
	Framework         string    `gorm:"size:50" json:"framework"`                  // 框架
	Team              string    `gorm:"size:100;index" json:"team"`                // 所属团队
	Owner             string    `gorm:"size:100" json:"owner"`                     // 负责人
	OrgName           string    `gorm:"-" json:"org_name,omitempty"`
	ProjectName       string    `gorm:"-" json:"project_name,omitempty"`
	Status            string    `gorm:"size:20;default:'active'" json:"status"` // 状态
	NotifyPlatform    string    `gorm:"size:50" json:"notify_platform"`         // 通知平台
	NotifyAppID       *uint     `gorm:"index" json:"notify_app_id"`             // 通知应用ID
	NotifyReceiveID   string    `gorm:"size:200" json:"notify_receive_id"`      // 通知接收者ID
	NotifyReceiveType string    `gorm:"size:50" json:"notify_receive_type"`     // 接收者类型
	CreatedBy         *uint     `gorm:"index" json:"created_by"`
}

// TableName 指定表名
func (Application) TableName() string {
	return "applications"
}

// ApplicationRepoBinding 应用与标准 Git 仓库的绑定关系。
// 一个应用可绑定多个仓库，但同一应用只允许一个默认主仓库。
type ApplicationRepoBinding struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ApplicationID uint      `gorm:"not null;index:idx_app_repo_binding_app;uniqueIndex:uk_app_repo_binding" json:"application_id"`
	GitRepoID     uint      `gorm:"not null;index:idx_app_repo_binding_repo;uniqueIndex:uk_app_repo_binding" json:"git_repo_id"`
	Role          string    `gorm:"size:30;not null;default:'primary'" json:"role"`
	IsDefault     bool      `gorm:"not null;default:false;index:idx_app_repo_binding_default" json:"is_default"`
	CreatedBy     *uint     `gorm:"index" json:"created_by"`

	RepoName      string `gorm:"-" json:"repo_name,omitempty"`
	RepoURL       string `gorm:"-" json:"repo_url,omitempty"`
	RepoProvider  string `gorm:"-" json:"repo_provider,omitempty"`
	DefaultBranch string `gorm:"-" json:"default_branch,omitempty"`
}

func (ApplicationRepoBinding) TableName() string { return "application_repo_bindings" }

// ApplicationEnv 应用环境配置
// 存储应用在不同环境下的配置
type ApplicationEnv struct {
	ID                  uint      `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	ApplicationID       uint      `gorm:"column:app_id;not null;index" json:"application_id"`              // 应用ID
	EnvName             string    `gorm:"size:50;not null" json:"env_name"`                                // 环境名称
	Branch              string    `gorm:"size:100" json:"branch"`                                          // 业务代码分支
	GitOpsRepoID        *uint     `gorm:"column:gitops_repo_id;index" json:"gitops_repo_id"`               // GitOps 部署仓库ID
	ArgoCDApplicationID *uint     `gorm:"column:argocd_application_id;index" json:"argocd_application_id"` // ArgoCD 应用ID
	GitOpsBranch        string    `gorm:"column:gitops_branch;size:200" json:"gitops_branch"`              // GitOps 目标分支
	GitOpsPath          string    `gorm:"column:gitops_path;size:500" json:"gitops_path"`                  // GitOps 部署目录
	HelmChartPath       string    `gorm:"size:500" json:"helm_chart_path"`                                 // Helm Chart 路径
	HelmValuesPath      string    `gorm:"size:500" json:"helm_values_path"`                                // Helm values 文件路径
	HelmReleaseName     string    `gorm:"size:200" json:"helm_release_name"`                               // Helm Release 名称
	K8sClusterID        *uint     `gorm:"index" json:"k8s_cluster_id"`                                     // K8s 集群ID
	K8sNamespace        string    `gorm:"size:100" json:"k8s_namespace"`                                   // K8s 命名空间
	K8sDeployment       string    `gorm:"size:200" json:"k8s_deployment"`                                  // K8s Deployment
	Replicas            int       `gorm:"default:1" json:"replicas"`                                       // 副本数
	CPURequest          string    `gorm:"size:50" json:"cpu_request"`                                      // CPU request
	CPULimit            string    `gorm:"size:50" json:"cpu_limit"`                                        // CPU limit
	MemoryRequest       string    `gorm:"size:50" json:"memory_request"`                                   // Memory request
	MemoryLimit         string    `gorm:"size:50" json:"memory_limit"`                                     // Memory limit
	Config              string    `gorm:"type:text" json:"config"`                                         // 环境特定配置
}

// TableName 指定表名
func (ApplicationEnv) TableName() string {
	return "application_envs"
}

// ApplicationReadinessCheck records the latest delivery readiness snapshot for an application.
type ApplicationReadinessCheck struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ApplicationID uint      `gorm:"not null;uniqueIndex:idx_app_readiness_app_key;index" json:"application_id"`
	CheckKey      string    `gorm:"size:80;not null;uniqueIndex:idx_app_readiness_app_key" json:"check_key"`
	Title         string    `gorm:"size:120;not null" json:"title"`
	Description   string    `gorm:"size:500" json:"description"`
	Status        string    `gorm:"size:20;not null;index" json:"status"`
	Severity      string    `gorm:"size:20;not null;default:'info';index" json:"severity"`
	Path          string    `gorm:"size:300" json:"path"`
	Score         int       `gorm:"default:0" json:"score"`
	Level         string    `gorm:"size:30;not null;default:'not_ready'" json:"level"`
	CheckedAt     time.Time `gorm:"not null;index" json:"checked_at"`
}

func (ApplicationReadinessCheck) TableName() string {
	return "application_readiness_checks"
}
