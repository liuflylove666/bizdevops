// Package model 定义数据库管理领域的数据模型
package model

import "time"

// DBInstance 数据库实例
type DBInstance struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `gorm:"size:100;not null;uniqueIndex:idx_db_instance_name" json:"name"`
	DBType      string    `gorm:"column:db_type;size:20;not null;default:'mysql'" json:"db_type"`
	Env         string    `gorm:"size:20;not null;default:'dev'" json:"env"`
	Host        string    `gorm:"size:200;not null" json:"host"`
	Port        int       `gorm:"not null;default:3306" json:"port"`
	Username    string    `gorm:"size:100;not null" json:"username"`
	Password    string    `gorm:"size:500;not null" json:"-"`
	DefaultDB   string    `gorm:"column:default_db;size:100" json:"default_db"`
	ExcludeDBs  string    `gorm:"column:exclude_dbs;size:500" json:"exclude_dbs"`
	Params      string    `gorm:"size:500" json:"params"`
	Mode        int       `gorm:"not null;default:0" json:"mode"`
	Status      string    `gorm:"size:20;not null;default:'active'" json:"status"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedBy   *uint     `gorm:"column:created_by" json:"created_by"`
}

func (DBInstance) TableName() string { return "db_instances" }
