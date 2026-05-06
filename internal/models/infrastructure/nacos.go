package infrastructure

import (
	"time"

	"gorm.io/gorm"
)

type NacosInstance struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	Name        string         `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Addr        string         `gorm:"size:500;not null" json:"addr"`
	Username    string         `gorm:"size:100" json:"username"`
	Password    string         `gorm:"size:500" json:"password,omitempty"`
	Env         string         `gorm:"size:30;not null;default:'dev'" json:"env"`
	Description string         `gorm:"type:text" json:"description"`
	Status      string         `gorm:"size:20;not null;default:'active'" json:"status"`
	IsDefault   bool           `gorm:"default:false" json:"is_default"`
	CreatedBy   *uint          `gorm:"index" json:"created_by"`
}

func (NacosInstance) TableName() string { return "nacos_instances" }
