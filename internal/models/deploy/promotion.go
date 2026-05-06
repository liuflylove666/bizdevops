package deploy

import "time"

// EnvPromotionPolicy 环境晋级策略
type EnvPromotionPolicy struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	ApplicationID uint      `gorm:"not null;uniqueIndex" json:"application_id"`
	EnvChain      string    `gorm:"size:500;not null;default:'[\"dev\",\"test\",\"uat\",\"gray\",\"prod\"]'" json:"env_chain"`
	NeedApproval  string    `gorm:"size:500;not null;default:'{}'" json:"need_approval"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (EnvPromotionPolicy) TableName() string { return "env_promotion_policies" }

// EnvPromotionRecord 镜像晋级记录
type EnvPromotionRecord struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	ApplicationID uint       `gorm:"not null;index" json:"application_id"`
	AppName       string     `gorm:"size:100" json:"app_name"`
	ImageURL      string     `gorm:"size:1000;not null" json:"image_url"`
	ImageTag      string     `gorm:"size:500;not null" json:"image_tag"`
	GitCommit     string     `gorm:"size:64" json:"git_commit"`
	GitBranch     string     `gorm:"size:200" json:"git_branch"`
	SourceRunID   *uint      `json:"source_run_id"`
	CurrentEnv    string     `gorm:"size:50;not null;default:'dev'" json:"current_env"`
	Status        string     `gorm:"size:20;not null;default:'active';index" json:"status"`
	CreatedBy     uint       `json:"created_by"`
	CreatedByName string     `gorm:"size:100" json:"created_by_name"`
	FinishedAt    *time.Time `json:"finished_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	Steps []EnvPromotionStep `gorm:"foreignKey:PromotionID" json:"steps,omitempty"`
}

func (EnvPromotionRecord) TableName() string { return "env_promotion_records" }

// EnvPromotionStep 晋级步骤
type EnvPromotionStep struct {
	ID             uint       `gorm:"primarykey" json:"id"`
	PromotionID    uint       `gorm:"not null;index" json:"promotion_id"`
	FromEnv        string     `gorm:"size:50;not null" json:"from_env"`
	ToEnv          string     `gorm:"size:50;not null" json:"to_env"`
	Status         string     `gorm:"size:20;not null;default:'pending'" json:"status"`
	DeployRecordID *uint      `json:"deploy_record_id"`
	ApproverID     *uint      `json:"approver_id"`
	ApproverName   string     `gorm:"size:100" json:"approver_name"`
	ApprovedAt     *time.Time `json:"approved_at"`
	RejectReason   string     `gorm:"size:500" json:"reject_reason"`
	OperatedBy     *uint      `json:"operated_by"`
	OperatedByName string     `gorm:"size:100" json:"operated_by_name"`
	OperatedAt     *time.Time `json:"operated_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (EnvPromotionStep) TableName() string { return "env_promotion_steps" }
