package biz

import "time"

// BizGoal 业务目标
type BizGoal struct {
	ID          uint       `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Name        string     `gorm:"size:120;not null;uniqueIndex" json:"name"`
	Code        string     `gorm:"size:64;uniqueIndex" json:"code"`
	Owner       string     `gorm:"size:100" json:"owner"`
	Status      string     `gorm:"size:20;default:'planning';index" json:"status"`
	Priority    string     `gorm:"size:20;default:'medium'" json:"priority"`
	Description string     `gorm:"type:text" json:"description"`
	ValueMetric string     `gorm:"size:200" json:"value_metric"`
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
}

func (BizGoal) TableName() string { return "biz_goals" }

// BizRequirement 需求项
type BizRequirement struct {
	ID          uint       `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ExternalKey string     `gorm:"size:80;index" json:"external_key,omitempty"` // 外部系统主键（如 Jira issue key）
	JiraEpicKey string     `gorm:"size:80;index" json:"jira_epic_key,omitempty"`
	JiraLabels  string     `gorm:"type:text" json:"jira_labels,omitempty"`     // 逗号分隔
	JiraComponents string  `gorm:"type:text" json:"jira_components,omitempty"` // 逗号分隔
	GoalID      *uint      `gorm:"index" json:"goal_id"`
	VersionID   *uint      `gorm:"index" json:"version_id"`
	ApplicationID *uint    `gorm:"index" json:"application_id"`
	PipelineID  *uint      `gorm:"index" json:"pipeline_id"`
	Title       string     `gorm:"size:200;not null;index" json:"title"`
	Source      string     `gorm:"size:50;default:'manual'" json:"source"`
	Owner       string     `gorm:"size:100" json:"owner"`
	Priority    string     `gorm:"size:20;default:'medium';index" json:"priority"`
	Status      string     `gorm:"size:20;default:'backlog';index" json:"status"`
	Description string     `gorm:"type:text" json:"description"`
	ValueScore  int        `gorm:"default:0" json:"value_score"`
	DueDate     *time.Time `json:"due_date"`

	GoalName    string `gorm:"-" json:"goal_name,omitempty"`
	VersionName string `gorm:"-" json:"version_name,omitempty"`
	ApplicationName string `gorm:"-" json:"application_name,omitempty"`
	PipelineName string `gorm:"-" json:"pipeline_name,omitempty"`
}

func (BizRequirement) TableName() string { return "biz_requirements" }

// BizVersion 版本计划
type BizVersion struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Name         string     `gorm:"size:120;not null;uniqueIndex" json:"name"`
	Code         string     `gorm:"size:64;uniqueIndex" json:"code"`
	GoalID       *uint      `gorm:"index" json:"goal_id"`
	ApplicationID *uint     `gorm:"index" json:"application_id"`
	PipelineID   *uint      `gorm:"index" json:"pipeline_id"`
	ReleaseID    *uint      `gorm:"index" json:"release_id"`
	Owner        string     `gorm:"size:100" json:"owner"`
	Status       string     `gorm:"size:20;default:'planning';index" json:"status"`
	Description  string     `gorm:"type:text" json:"description"`
	StartDate    *time.Time `json:"start_date"`
	ReleaseDate  *time.Time `json:"release_date"`
	WindowStart  *time.Time `json:"window_start"`
	WindowEnd    *time.Time `json:"window_end"`

	GoalName string `gorm:"-" json:"goal_name,omitempty"`
	ApplicationName string `gorm:"-" json:"application_name,omitempty"`
	PipelineName string `gorm:"-" json:"pipeline_name,omitempty"`
	ReleaseTitle string `gorm:"-" json:"release_title,omitempty"`
}

func (BizVersion) TableName() string { return "biz_versions" }
