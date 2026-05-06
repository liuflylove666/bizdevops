package deploy

import (
	"time"

	"devops/internal/types"
)

// Release 统一发布主单（v2.0 聚合根；见 ADR-0002）。
//
// v2.0 起 Release 是发布域的唯一聚合根，关联：
//   - 镜像部署（PipelineRun / DeployRecord，ReleaseItem.item_type=deployment/pipeline_run）
//   - 环境晋级（EnvPromotionRecord，item_type=promotion）
//   - 配置发布（NacosRelease，item_type=nacos_release）
//   - 数据库变更（SQLChangeTicket，item_type=database）
//
// 新增字段均可为空/带默认值，保证 AutoMigrate 对旧数据安全。
type Release struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Title           string     `gorm:"size:200;not null" json:"title"`
	ApplicationID   *uint      `gorm:"index" json:"application_id"`
	ApplicationName string     `gorm:"size:100" json:"application_name"`
	ProjectID       *uint      `gorm:"column:project_id;->;-:migration" json:"project_id,omitempty"`
	ProjectName     string     `gorm:"column:project_name;->;-:migration" json:"project_name,omitempty"`
	Env             string     `gorm:"size:30;not null;default:'dev';index" json:"env"`
	Version         string     `gorm:"size:100" json:"version"`
	Description     string     `gorm:"type:text" json:"description"`
	Status          string     `gorm:"size:20;not null;default:'draft';index" json:"status"`
	RiskLevel       string     `gorm:"size:20;default:'low'" json:"risk_level"`
	CreatedBy       uint       `json:"created_by"`
	CreatedByName   string     `gorm:"size:100" json:"created_by_name"`
	ApprovedBy      *uint      `json:"approved_by"`
	ApprovedByName  string     `gorm:"size:100" json:"approved_by_name"`
	ApprovedAt      *time.Time `json:"approved_at"`
	PublishedAt     *time.Time `json:"published_at"`
	PublishedBy     *uint      `json:"published_by"`
	PublishedByName string     `gorm:"size:100" json:"published_by_name"`
	RollbackAt      *time.Time `json:"rollback_at"`
	RejectReason    string     `gorm:"size:500" json:"reject_reason"`

	// ---------------------------------------------------------------
	// v2.0 扩展字段（ADR-0001 / ADR-0002 / ADR-0003 / ADR-0007）
	// ---------------------------------------------------------------
	//
	// RolloutStrategy 决定 CD 端如何渲染 Argo Rollouts CRD：
	//   direct       —— 直接替换（默认，等同 v1 行为）
	//   canary       —— 金丝雀
	//   blue_green   —— 蓝绿
	RolloutStrategy string `gorm:"size:20;default:'direct';index" json:"rollout_strategy"`

	// RolloutConfig 渐进式发布的策略参数（权重步长、分析模板、暂停时间等）。
	// 与 Argo Rollouts 字段对齐，留给策略渲染器消费。
	RolloutConfig types.JSONMap `gorm:"type:json" json:"rollout_config,omitempty"`

	// RiskScore 变更风险评分（0-100，越高越危险）。由风险评估器自动打分，可人工覆盖。
	RiskScore int `gorm:"default:0;index" json:"risk_score"`

	// RiskFactors 风险打分的明细（命中规则 + 权重），便于 UI 展示与审计。
	RiskFactors types.JSONMap `gorm:"type:json" json:"risk_factors,omitempty"`

	// ApprovalInstanceID 关联审批流实例；为兼容旧路径，ApprovedBy/ApprovedAt 仍保留。
	ApprovalInstanceID *uint `gorm:"index" json:"approval_instance_id,omitempty"`

	// GitOpsChangeRequestID 关联 GitOps PR 记录（见 internal/models/infrastructure.GitOpsChangeRequest）。
	// 发布主路径会填充该字段。
	GitOpsChangeRequestID *uint `gorm:"index" json:"gitops_change_request_id,omitempty"`

	// ArgoAppName 对应 Argo CD Application CR 的名称（同步后回填）。
	ArgoAppName string `gorm:"size:200;index" json:"argo_app_name,omitempty"`

	// ArgoSyncStatus 最新一次 Argo CD 同步状态（OutOfSync / Synced / Unknown）。
	ArgoSyncStatus string `gorm:"size:20" json:"argo_sync_status,omitempty"`

	// JiraIssueKeys 关联的 Jira Issue 键（逗号分隔），实现 端到端 Plan→Code→Release 追溯（ADR-0007）。
	JiraIssueKeys string `gorm:"size:500" json:"jira_issue_keys,omitempty"`

	// ---------------------------------------------------------------
	// 聚合子项（非数据库字段，查询时填充）
	// ---------------------------------------------------------------
	PipelineRuns   []PipelineRun  `gorm:"-" json:"pipeline_runs,omitempty"`
	NacosReleases  []NacosRelease `gorm:"-" json:"nacos_releases,omitempty"`
	BizVersionID   *uint          `gorm:"-" json:"biz_version_id,omitempty"`
	BizVersionName string         `gorm:"-" json:"biz_version_name,omitempty"`
	BizGoalID      *uint          `gorm:"-" json:"biz_goal_id,omitempty"`
	BizGoalName    string         `gorm:"-" json:"biz_goal_name,omitempty"`
}

// Release 状态机：
//
//	draft -> pending_approval -> approved -> publishing -> published -> rolled_back
//	draft -> pending_approval -> rejected
//
// v2.0 GitOps 路径新增中间态：
//
//	approved -> pr_opened -> pr_merged -> publishing -> published

func (Release) TableName() string { return "releases" }

// 发布策略枚举，便于引用与校验。
const (
	RolloutStrategyDirect    = "direct"
	RolloutStrategyCanary    = "canary"
	RolloutStrategyBlueGreen = "blue_green"
)

// IsValidRolloutStrategy 校验 RolloutStrategy 字段合法性。
func IsValidRolloutStrategy(s string) bool {
	switch s {
	case "", RolloutStrategyDirect, RolloutStrategyCanary, RolloutStrategyBlueGreen:
		return true
	default:
		return false
	}
}

// ReleaseItem 发布主单关联子项。
//
// v2.0 item_type 枚举扩展：
//
//	pipeline_run / nacos_release / sql_ticket     —— v1 已有
//	deployment / promotion / database  —— v2 新增
type ReleaseItem struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	ReleaseID  uint      `gorm:"not null;index" json:"release_id"`
	ItemType   string    `gorm:"size:30;not null;index" json:"item_type"`
	ItemID     uint      `gorm:"not null" json:"item_id"`
	ItemTitle  string    `gorm:"size:200" json:"item_title"`
	ItemStatus string    `gorm:"size:30" json:"item_status"`
	SortOrder  int       `gorm:"default:0" json:"sort_order"`

	// v2.0: 子项级补充元数据（例如 deployment 的镜像 tag / promotion 的源环境 等）。
	Payload types.JSONMap `gorm:"type:json" json:"payload,omitempty"`
}

func (ReleaseItem) TableName() string { return "release_items" }

// ReleaseGateResult records the latest gate evaluation snapshot for a release.
// Gate results are kept independent from the Release state machine so v2.1 can
// introduce explainable blocking without rewriting existing approval/publish flows.
type ReleaseGateResult struct {
	ID          uint          `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	ReleaseID   uint          `gorm:"not null;uniqueIndex:idx_release_gate_release_key,priority:1" json:"release_id"`
	GateKey     string        `gorm:"size:80;not null;uniqueIndex:idx_release_gate_release_key,priority:2" json:"gate_key"`
	GateName    string        `gorm:"size:120;not null" json:"gate_name"`
	Category    string        `gorm:"size:40;not null;index" json:"category"`
	Status      string        `gorm:"size:20;not null;index" json:"status"` // pass/warn/block/skip
	Severity    string        `gorm:"size:20;not null;default:'info'" json:"severity"`
	Policy      string        `gorm:"size:20;not null;default:'advisory'" json:"policy"` // required/advisory/manual
	Blocker     bool          `gorm:"not null;default:false;index" json:"blocker"`
	Message     string        `gorm:"size:500" json:"message"`
	Detail      types.JSONMap `gorm:"type:json" json:"detail,omitempty"`
	EvaluatedAt time.Time     `gorm:"not null;index" json:"evaluated_at"`
}

func (ReleaseGateResult) TableName() string { return "release_gate_results" }

// ReleaseItemType 枚举常量。
const (
	ReleaseItemTypePipelineRun   = "pipeline_run"
	ReleaseItemTypeNacosRelease  = "nacos_release"
	ReleaseItemTypeSQLTicket     = "sql_ticket"
	ReleaseItemTypeDeployment    = "deployment"
	ReleaseItemTypePromotion     = "promotion"
	ReleaseItemTypeDatabase      = "database"
)

// Release 状态枚举常量。v1 已有 + v2 新增均列出。
const (
	ReleaseStatusDraft           = "draft"
	ReleaseStatusPendingApproval = "pending_approval"
	ReleaseStatusApproved        = "approved"
	ReleaseStatusRejected        = "rejected"
	ReleaseStatusPROpened        = "pr_opened" // v2
	ReleaseStatusPRMerged        = "pr_merged" // v2
	ReleaseStatusPublishing      = "publishing"
	ReleaseStatusPublished       = "published"
	ReleaseStatusRolledBack      = "rolled_back"
	ReleaseStatusFailed          = "failed"
)
