// Package system 定义系统管理相关的数据模型。
//
// 历史名为 oa.go —— 早期把 OA 同步、OA 地址、OA 通知混在一起。
// v2.0 清理后仅保留 OANotifyConfig（历史默认通知接收人配置）以及通用的
// SystemConfig / MessageTemplate。OAData / OAAddress 模型与对应表已移除。
package system

import (
	"time"

	"gorm.io/gorm"
)

// OANotifyConfig 历史默认通知接收人配置（表名 oa_notify_configs 保持不变）。
//
// Deprecated: 新部署请使用 Telegram 机器人与 chat_id；本模型仅兼容存量数据。
type OANotifyConfig struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Name          string    `gorm:"size:100;not null" json:"name"`                   // 配置名称
	AppID         uint      `gorm:"column:app_id" json:"app_id"`                     // 历史：关联应用 ID
	ReceiveID     string    `gorm:"size:100;not null" json:"receive_id"`             // 接收者ID
	ReceiveIDType string    `gorm:"size:50;not null" json:"receive_id_type"`         // ID类型: chat_id/open_id/user_id
	Description   string    `gorm:"type:text" json:"description"`                    // 描述
	Status        string    `gorm:"size:20;default:'active';not null" json:"status"` // 状态
	IsDefault     bool      `gorm:"default:false" json:"is_default"`                 // 是否默认
}

// TableName 指定表名。
func (OANotifyConfig) TableName() string {
	return "oa_notify_configs"
}

// SystemConfig 系统配置模型，存储平台级的通用配置键值对。
type SystemConfig struct {
	gorm.Model
	Key         string `gorm:"size:100;not null;uniqueIndex" json:"key"`
	Value       string `gorm:"type:text" json:"value"`
	Description string `gorm:"type:text" json:"description"`
}

// TableName 指定表名。
func (SystemConfig) TableName() string {
	return "system_configs"
}

// MessageTemplate 消息模板模型。
type MessageTemplate struct {
	gorm.Model
	Name        string `gorm:"size:100;not null" json:"name"`
	Type        string `gorm:"size:50;not null" json:"type"`
	Content     string `gorm:"type:text;not null" json:"content"`
	Description string `gorm:"type:text" json:"description"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	CreatedBy   *uint  `gorm:"index" json:"created_by"`
}

// TableName 指定表名。
func (MessageTemplate) TableName() string {
	return "message_templates"
}
