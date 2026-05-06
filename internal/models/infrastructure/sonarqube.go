package infrastructure

import "time"

type SonarQubeInstance struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	BaseURL   string    `gorm:"size:500;not null" json:"base_url"`
	Token     string    `gorm:"size:500" json:"token"`
	IsDefault bool      `gorm:"default:false" json:"is_default"`
	Status    string    `gorm:"size:20;default:'active'" json:"status"`
}

func (SonarQubeInstance) TableName() string {
	return "sonarqube_instances"
}

type SonarQubeProjectBinding struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	SonarQubeID       uint      `gorm:"not null;index" json:"sonarqube_id"`
	SonarProjectKey   string    `gorm:"size:200;not null" json:"sonar_project_key"`
	SonarProjectName  string    `gorm:"size:200" json:"sonar_project_name"`
	DevopsAppID       *uint     `json:"devops_app_id"`
	DevopsAppName     string    `gorm:"size:200" json:"devops_app_name"`
	QualityGateStatus string    `gorm:"size:20" json:"quality_gate_status"`
}

func (SonarQubeProjectBinding) TableName() string {
	return "sonarqube_project_bindings"
}
