package deploy

import "time"

// ChangeEvent 统一变更事件
type ChangeEvent struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	EventType       string    `gorm:"size:30;not null;index" json:"event_type"` // deploy, nacos_release, sql_ticket, pipeline_run, promotion, release
	EventID         uint      `gorm:"not null" json:"event_id"`
	Title           string    `gorm:"size:300;not null" json:"title"`
	Description     string    `gorm:"type:text" json:"description"`
	ApplicationID   *uint     `gorm:"index" json:"application_id"`
	ApplicationName string    `gorm:"size:100" json:"application_name"`
	Env             string    `gorm:"size:30;index" json:"env"`
	Status          string    `gorm:"size:30" json:"status"`
	RiskLevel       string    `gorm:"size:20" json:"risk_level"`
	Operator        string    `gorm:"size:100" json:"operator"`
	OperatorID      uint      `json:"operator_id"`
	Metadata        string    `gorm:"type:text" json:"metadata"` // JSON 扩展字段
}

func (ChangeEvent) TableName() string { return "change_events" }
