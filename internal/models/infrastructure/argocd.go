package infrastructure

import "time"

// ArgoCDInstance Argo CD 实例
type ArgoCDInstance struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	ServerURL string    `gorm:"size:500;not null" json:"server_url"`
	AuthToken string    `gorm:"size:1000" json:"auth_token"`   // 加密存储
	Insecure  bool      `gorm:"default:false" json:"insecure"` // 跳过 TLS 证书验证
	IsDefault bool      `gorm:"default:false" json:"is_default"`
	Status    string    `gorm:"size:20;default:'active'" json:"status"`
	CreatedBy *uint     `json:"created_by"`
}

func (ArgoCDInstance) TableName() string { return "argocd_instances" }

// ArgoCDApplication Argo CD 应用（同步到本地的 Application 快照）
type ArgoCDApplication struct {
	ID               uint       `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ArgoCDInstanceID uint       `gorm:"not null;index" json:"argocd_instance_id"`
	Name             string     `gorm:"size:200;not null" json:"name"`
	Project          string     `gorm:"size:200;default:'default'" json:"project"`
	RepoURL          string     `gorm:"size:500" json:"repo_url"`
	RepoPath         string     `gorm:"size:500" json:"repo_path"`
	TargetRevision   string     `gorm:"size:200;default:'HEAD'" json:"target_revision"`
	DestServer       string     `gorm:"size:500" json:"dest_server"`
	DestNamespace    string     `gorm:"size:200" json:"dest_namespace"`
	SyncStatus       string     `gorm:"size:30;default:'Unknown';index" json:"sync_status"`   // Synced / OutOfSync / Unknown
	HealthStatus     string     `gorm:"size:30;default:'Unknown';index" json:"health_status"` // Healthy / Degraded / Progressing / Missing / Unknown
	SyncPolicy       string     `gorm:"size:20;default:'manual'" json:"sync_policy"`          // manual / auto
	LastSyncAt       *time.Time `json:"last_sync_at"`
	DriftDetected    bool       `gorm:"default:false" json:"drift_detected"`
	ApplicationID    *uint      `gorm:"index" json:"application_id"` // 关联 DevOps 应用
	ApplicationName  string     `gorm:"size:100" json:"application_name"`
	Env              string     `gorm:"size:30" json:"env"`
}

func (ArgoCDApplication) TableName() string { return "argocd_applications" }

// GitOpsRepo Git 部署仓库声明
type GitOpsRepo struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Name            string    `gorm:"size:100;not null" json:"name"`
	RepoURL         string    `gorm:"size:500;not null" json:"repo_url"`
	Branch          string    `gorm:"size:200;default:'main'" json:"branch"`
	Path            string    `gorm:"size:500;default:'/'" json:"path"`
	AuthType        string    `gorm:"size:20;default:'token'" json:"auth_type"` // token / ssh / none
	AuthCredential  string    `gorm:"size:1000" json:"auth_credential"`         // 加密存储
	ApplicationID   *uint     `gorm:"index" json:"application_id"`
	ApplicationName string    `gorm:"size:100" json:"application_name"`
	Env             string    `gorm:"size:30" json:"env"`
	SyncEnabled     bool      `gorm:"default:true" json:"sync_enabled"`
	LastCommitHash  string    `gorm:"size:64" json:"last_commit_hash"`
	LastCommitMsg   string    `gorm:"size:500" json:"last_commit_msg"`
	CreatedBy       *uint     `json:"created_by"`
}

func (GitOpsRepo) TableName() string { return "gitops_repos" }

// GitOpsChangeRequest GitOps 变更请求
type GitOpsChangeRequest struct {
	ID                  uint       `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	GitOpsRepoID        uint       `gorm:"not null;index" json:"gitops_repo_id"`
	ArgoCDApplicationID *uint      `gorm:"index" json:"argocd_application_id"`
	ApplicationID       *uint      `gorm:"index" json:"application_id"`
	ApplicationName     string     `gorm:"size:100" json:"application_name"`
	Env                 string     `gorm:"size:30;index" json:"env"`
	PipelineID          *uint      `gorm:"index" json:"pipeline_id"`
	PipelineRunID       *uint      `gorm:"index" json:"pipeline_run_id"`
	Title               string     `gorm:"size:200;not null" json:"title"`
	Description         string     `gorm:"type:text" json:"description"`
	FilePath            string     `gorm:"size:500;not null" json:"file_path"`
	ImageRepository     string     `gorm:"size:500;not null" json:"image_repository"`
	ImageTag            string     `gorm:"size:200;not null" json:"image_tag"`
	HelmChartPath       string     `gorm:"size:500" json:"helm_chart_path"`
	HelmValuesPath      string     `gorm:"size:500" json:"helm_values_path"`
	HelmReleaseName     string     `gorm:"size:200" json:"helm_release_name"`
	Replicas            int        `gorm:"default:0" json:"replicas"`
	CPURequest          string     `gorm:"size:50" json:"cpu_request"`
	CPULimit            string     `gorm:"size:50" json:"cpu_limit"`
	MemoryRequest       string     `gorm:"size:50" json:"memory_request"`
	MemoryLimit         string     `gorm:"size:50" json:"memory_limit"`
	SourceBranch        string     `gorm:"size:200" json:"source_branch"`
	TargetBranch        string     `gorm:"size:200" json:"target_branch"`
	Status              string     `gorm:"size:30;default:'draft';index" json:"status"` // draft/submitted/open/failed
	Provider            string     `gorm:"size:30" json:"provider"`
	MergeRequestIID     string     `gorm:"size:100" json:"merge_request_iid"`
	MergeRequestURL     string     `gorm:"size:1000" json:"merge_request_url"`
	LastCommitSHA       string     `gorm:"size:100" json:"last_commit_sha"`
	ApprovalInstanceID  *uint      `gorm:"index" json:"approval_instance_id"`
	ApprovalChainID     *uint      `gorm:"index" json:"approval_chain_id"`
	ApprovalChainName   string     `gorm:"size:100" json:"approval_chain_name"`
	ApprovalStatus      string     `gorm:"size:30;default:'none';index" json:"approval_status"` // none/pending/approved/rejected/cancelled/failed
	ApprovalFinishedAt  *time.Time `json:"approval_finished_at"`
	AutoMergeStatus     string     `gorm:"size:30;default:'pending';index" json:"auto_merge_status"` // manual/pending/success/failed/skipped
	AutoMergedAt        *time.Time `json:"auto_merged_at"`
	ErrorMessage        string     `gorm:"type:text" json:"error_message"`
	CreatedBy           *uint      `json:"created_by"`
}

func (GitOpsChangeRequest) TableName() string { return "gitops_change_requests" }
