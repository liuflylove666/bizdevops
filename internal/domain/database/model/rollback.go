package model

import "time"

// SQLRollbackScript 工单执行前预生成的反向 SQL
type SQLRollbackScript struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	TicketID    uint      `gorm:"column:ticket_id;not null;index" json:"ticket_id"`
	WorkID      string    `gorm:"column:work_id;size:64;not null;index" json:"work_id"`
	StatementID uint      `gorm:"column:statement_id" json:"statement_id"`
	RollbackSQL string    `gorm:"column:rollback_sql;type:longtext;not null" json:"rollback_sql"`
}

func (SQLRollbackScript) TableName() string { return "sql_rollback_scripts" }
