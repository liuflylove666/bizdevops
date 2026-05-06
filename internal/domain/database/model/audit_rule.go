package model

import (
	"time"

	"gorm.io/datatypes"
)

// SQLAuditRuleSet 审核规则集。config 字段存储 AuditRuleConfig JSON。
type SQLAuditRuleSet struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Name        string         `gorm:"size:100;not null;uniqueIndex:idx_sar_name" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Config      datatypes.JSON `gorm:"not null" json:"config"`
	IsDefault   bool           `gorm:"column:is_default;not null;default:0" json:"is_default"`
}

func (SQLAuditRuleSet) TableName() string { return "sql_audit_rules" }

// AuditRuleConfig 可开关的规则配置。字段全是指针/带默认，方便未存的走默认值。
type AuditRuleConfig struct {
	// DML
	RequireWhere         *bool `json:"require_where"`          // UPDATE/DELETE 必须 WHERE
	SuggestDMLLimit      *bool `json:"suggest_dml_limit"`      // UPDATE/DELETE 建议 LIMIT
	TautologyWhere       *bool `json:"tautology_where"`        // 1=1 恒真
	// Select
	SelectStar           *bool `json:"select_star"`            // SELECT *
	SelectLimit          *bool `json:"select_limit"`           // SELECT 必须 LIMIT
	// DDL
	NoDrop               *bool `json:"no_drop"`                // 禁 DROP TABLE/DATABASE
	NoTruncate           *bool `json:"no_truncate"`            // 禁 TRUNCATE
	RenameTable          *bool `json:"rename_table"`           // RENAME TABLE 告警
	AlterDrop            *bool `json:"alter_drop"`             // ALTER DROP 子句告警
	CreateEngine         *bool `json:"create_engine"`          // CREATE TABLE 需 ENGINE=
	CreateCharset        *bool `json:"create_charset"`         // CREATE TABLE 需 charset
	CreatePrimaryKey     *bool `json:"create_primary_key"`     // CREATE TABLE 需主键
	// 安全
	NoLockTables         *bool `json:"no_lock_tables"`
	NoSetGlobal          *bool `json:"no_set_global"`
	NoGrant              *bool `json:"no_grant"`
	// Insert
	InsertColumns        *bool `json:"insert_columns"`
	// 阈值
	MaxStatementPerTicket int  `json:"max_statement_per_ticket"` // 单工单最大语句数, 0=不限
	MaxStatementBytes    int   `json:"max_statement_bytes"`      // 单条 SQL 最大字节, 0=100KB 默认
}

// DefaultAuditRuleConfig 默认全开
func DefaultAuditRuleConfig() AuditRuleConfig {
	t := true
	return AuditRuleConfig{
		RequireWhere: &t, SuggestDMLLimit: &t, TautologyWhere: &t,
		SelectStar: &t, SelectLimit: &t,
		NoDrop: &t, NoTruncate: &t, RenameTable: &t, AlterDrop: &t,
		CreateEngine: &t, CreateCharset: &t, CreatePrimaryKey: &t,
		NoLockTables: &t, NoSetGlobal: &t, NoGrant: &t,
		InsertColumns:        &t,
		MaxStatementPerTicket: 0,
		MaxStatementBytes:    0,
	}
}
