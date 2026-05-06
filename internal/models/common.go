package models

import (
	"time"

	"devops/internal/types"
)

// JSONMap 与 types.JSONMap 同型，保留 models.JSONMap 引用以兼容现有代码
type JSONMap = types.JSONMap

// EncryptionKey 加密密钥
type EncryptionKey struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	KeyID        string     `gorm:"size:100;uniqueIndex" json:"key_id"`
	EncryptedKey []byte     `gorm:"type:blob" json:"-"`
	Algorithm    string     `gorm:"size:20;default:'AES-256-GCM'" json:"algorithm"`
	Status       string     `gorm:"size:20;default:'active'" json:"status"` // active, rotating, retired
	Version      int        `gorm:"default:1" json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	RotatedAt    *time.Time `json:"rotated_at,omitempty"`
}

func (EncryptionKey) TableName() string { return "encryption_keys" }
