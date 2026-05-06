// Package model 定义消息通知领域的数据模型
package model

import "time"

// ==================== Telegram 模型 ====================

// TelegramBot Telegram 机器人模型
type TelegramBot struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Name          string    `gorm:"size:100;not null" json:"name"`
	Token         string    `gorm:"size:200;not null" json:"token"`
	DefaultChatID string    `gorm:"column:default_chat_id;size:100" json:"default_chat_id"`
	APIBaseURL    string    `gorm:"column:api_base_url;size:200" json:"api_base_url"`
	Description   string    `gorm:"type:text" json:"description"`
	Status        string    `gorm:"size:20;default:'active';not null" json:"status"`
	IsDefault     bool      `gorm:"column:is_default;default:false" json:"is_default"`
	CreatedBy     *uint     `gorm:"column:created_by" json:"created_by"`
}

func (TelegramBot) TableName() string { return "telegram_bots" }

// TelegramMessageLog Telegram 消息发送记录
type TelegramMessageLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	BotID     uint      `gorm:"column:bot_id;index" json:"bot_id"`
	ChatID    string    `gorm:"size:100" json:"chat_id"`
	ParseMode string    `gorm:"size:20" json:"parse_mode"`
	Content   string    `gorm:"type:text" json:"content"`
	Source    string    `gorm:"size:50" json:"source"`
	Status    string    `gorm:"size:20;default:'success'" json:"status"`
	ErrorMsg  string    `gorm:"type:text" json:"error_msg"`
}

func (TelegramMessageLog) TableName() string { return "telegram_message_logs" }
