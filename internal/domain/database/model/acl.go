package model

import (
	"strings"
	"time"
)

const (
	ACLSubjectUser = "user"
	ACLSubjectRole = "role"

	ACLLevelRead  = "read"
	ACLLevelWrite = "write"
	ACLLevelOwner = "owner"
)

type DBInstanceACL struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	InstanceID  uint      `gorm:"column:instance_id;not null;uniqueIndex:idx_ia_subject" json:"instance_id"`
	SubjectType string    `gorm:"column:subject_type;size:10;not null;uniqueIndex:idx_ia_subject" json:"subject_type"`
	SubjectID   uint      `gorm:"column:subject_id;not null;uniqueIndex:idx_ia_subject" json:"subject_id"`
	AccessLevel string    `gorm:"column:access_level;size:20;not null;default:'read'" json:"access_level"`
	SchemaNames string    `gorm:"column:schema_names;size:1000;not null;default:''" json:"schema_names"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   *uint     `gorm:"column:created_by" json:"created_by"`

	SubjectName string `gorm:"-" json:"subject_name,omitempty"`
}

func (DBInstanceACL) TableName() string { return "db_instance_acl" }

func (a *DBInstanceACL) SchemaList() []string {
	if a.SchemaNames == "" {
		return nil
	}
	parts := strings.Split(a.SchemaNames, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (a *DBInstanceACL) AllSchemas() bool {
	return a.SchemaNames == ""
}

func (a *DBInstanceACL) HasSchema(schema string) bool {
	if a.AllSchemas() {
		return true
	}
	for _, s := range a.SchemaList() {
		if s == schema {
			return true
		}
	}
	return false
}

func ACLLevelRank(level string) int {
	switch level {
	case ACLLevelOwner:
		return 3
	case ACLLevelWrite:
		return 2
	case ACLLevelRead:
		return 1
	default:
		return 0
	}
}
