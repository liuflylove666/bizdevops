package deploy

import "time"

// NacosRelease Nacos 配置发布单
type NacosRelease struct {
	ID               uint       `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Title            string     `gorm:"size:200;not null" json:"title"`
	NacosInstanceID  uint       `gorm:"not null;index" json:"nacos_instance_id"`
	NacosInstanceName string    `gorm:"size:100" json:"nacos_instance_name"`
	Tenant           string     `gorm:"size:200" json:"tenant"`
	Group            string     `gorm:"size:200;not null;default:'DEFAULT_GROUP'" json:"group"`
	DataID           string     `gorm:"size:200;not null" json:"data_id"`
	Env              string     `gorm:"size:30;not null;default:'dev'" json:"env"`
	ConfigType       string     `gorm:"size:20;default:'yaml'" json:"config_type"`
	ContentBefore    string     `gorm:"type:longtext" json:"content_before"`
	ContentAfter     string     `gorm:"type:longtext" json:"content_after"`
	ContentHash      string     `gorm:"size:64" json:"content_hash"`
	ServiceID        *uint      `gorm:"index" json:"service_id"`
	ServiceName      string     `gorm:"size:100" json:"service_name"`
	ReleaseID        *uint      `gorm:"index" json:"release_id"`
	Status           string     `gorm:"size:20;not null;default:'draft';index" json:"status"`
	RiskLevel        string     `gorm:"size:20;default:'low'" json:"risk_level"`
	Description      string     `gorm:"type:text" json:"description"`
	CreatedBy        uint       `json:"created_by"`
	CreatedByName    string     `gorm:"size:100" json:"created_by_name"`
	ApprovedBy       *uint      `json:"approved_by"`
	ApprovedByName   string     `gorm:"size:100" json:"approved_by_name"`
	ApprovedAt       *time.Time `json:"approved_at"`
	ApprovalInstanceID *uint    `gorm:"index" json:"approval_instance_id,omitempty"`
	ApprovalChainID    *uint    `gorm:"index" json:"approval_chain_id,omitempty"`
	ApprovalChainName  string   `gorm:"size:100" json:"approval_chain_name"`
	PublishedAt      *time.Time `json:"published_at"`
	PublishedBy      *uint      `json:"published_by"`
	PublishedByName  string     `gorm:"size:100" json:"published_by_name"`
	RollbackFromID   *uint      `json:"rollback_from_id"`
	RejectReason     string     `gorm:"size:500" json:"reject_reason"`
}

// Status: draft -> pending_approval -> approved -> published -> rolled_back
//         draft -> pending_approval -> rejected

func (NacosRelease) TableName() string { return "nacos_releases" }
