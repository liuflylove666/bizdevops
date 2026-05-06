package infrastructure

import "time"

// JiraInstance Jira 实例配置
type JiraInstance struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	BaseURL   string    `gorm:"size:500;not null" json:"base_url"`
	Username  string    `gorm:"size:200" json:"username"`
	Token     string    `gorm:"size:500" json:"token"`
	AuthType  string    `gorm:"size:20;default:'token'" json:"auth_type"` // token / basic
	IsDefault bool      `gorm:"default:false" json:"is_default"`
	Status    string    `gorm:"size:20;default:'active'" json:"status"`
}

func (JiraInstance) TableName() string {
	return "jira_instances"
}

// JiraProjectMapping Jira 项目与 DevOps 项目映射
type JiraProjectMapping struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	JiraInstanceID  uint      `gorm:"not null;index" json:"jira_instance_id"`
	JiraProjectKey  string    `gorm:"size:50;not null" json:"jira_project_key"`
	JiraProjectName string    `gorm:"size:200" json:"jira_project_name"`
	DevopsProjectID *uint     `json:"devops_project_id"`
	DevopsAppID     *uint     `json:"devops_app_id"`
}

func (JiraProjectMapping) TableName() string {
	return "jira_project_mappings"
}
