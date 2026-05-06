package model

import "time"

// DBQueryLog 数据库查询控制台日志
type DBQueryLog struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	InstanceID uint      `gorm:"column:instance_id;index;not null" json:"instance_id"`
	Username   string    `gorm:"size:100;not null;index" json:"username"`
	SchemaName string    `gorm:"column:schema_name;size:100" json:"schema_name"`
	SQLText    string    `gorm:"column:sql_text;type:text;not null" json:"sql_text"`
	AffectRows int       `gorm:"column:affect_rows;not null;default:0" json:"affect_rows"`
	ExecMs     int       `gorm:"column:exec_ms;not null;default:0" json:"exec_ms"`
	Status     string    `gorm:"size:20;not null;default:'success';index" json:"status"`
	ErrorMsg   string    `gorm:"column:error_msg;type:text" json:"error_msg"`
}

func (DBQueryLog) TableName() string { return "db_query_logs" }
