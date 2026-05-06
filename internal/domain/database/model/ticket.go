package model

import (
	"time"

	"gorm.io/datatypes"
)

// 工单状态常量
const (
	TicketStatusPending   = 0 // 审批中
	TicketStatusRejected  = 1 // 已驳回
	TicketStatusReady     = 2 // 待执行
	TicketStatusRunning   = 3 // 执行中
	TicketStatusSucceeded = 4 // 执行成功
	TicketStatusFailed    = 5 // 执行失败
	TicketStatusCancelled = 6 // 已撤回
)

// SQLChangeTicket SQL 变更工单
type SQLChangeTicket struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	WorkID             string         `gorm:"column:work_id;size:64;not null;uniqueIndex:idx_sct_work_id" json:"work_id"`
	Title              string         `gorm:"size:200;not null" json:"title"`
	Description        string         `gorm:"type:text" json:"description"`
	Applicant          string         `gorm:"size:100;not null;index" json:"applicant"`
	RealName           string         `gorm:"column:real_name;size:100" json:"real_name"`
	InstanceID         uint           `gorm:"column:instance_id;not null;index" json:"instance_id"`
	SchemaName         string         `gorm:"column:schema_name;size:100;not null" json:"schema_name"`
	ChangeType         int            `gorm:"column:change_type;not null;default:1" json:"change_type"` // 0 DDL 1 DML
	NeedBackup         bool           `gorm:"column:need_backup;not null;default:false" json:"need_backup"`
	Status             int            `gorm:"not null;default:0;index" json:"status"`
	ExecuteTime        *time.Time     `gorm:"column:execute_time" json:"execute_time"`
	DelayMode          string         `gorm:"column:delay_mode;size:20;default:'none'" json:"delay_mode"`
	ApprovalInstanceID *uint          `gorm:"column:approval_instance_id" json:"approval_instance_id"`
	AuditReport        datatypes.JSON `gorm:"column:audit_report" json:"audit_report"`
	AuditConfig        datatypes.JSON `gorm:"column:audit_config" json:"audit_config"`
	CurrentStep        int            `gorm:"column:current_step;not null;default:0" json:"current_step"`
	Assigned           string         `gorm:"size:500" json:"assigned"`
}

func (SQLChangeTicket) TableName() string { return "sql_change_tickets" }

// AuditStep 审批步骤（存储在 audit_config JSON 中）
type AuditStep struct {
	StepName  string   `json:"step_name"`
	Approvers []string `json:"approvers"`
}

// SQLChangeStatement 工单拆分后的语句
type SQLChangeStatement struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	TicketID   uint       `gorm:"column:ticket_id;not null;index" json:"ticket_id"`
	WorkID     string     `gorm:"column:work_id;size:64;not null;index" json:"work_id"`
	Seq        int        `gorm:"not null;default:0" json:"seq"`
	SQLText    string     `gorm:"column:sql_text;type:longtext;not null" json:"sql_text"`
	AffectRows int        `gorm:"column:affect_rows;not null;default:0" json:"affect_rows"`
	ExecMs     int        `gorm:"column:exec_ms;not null;default:0" json:"exec_ms"`
	State      string     `gorm:"size:20;not null;default:'pending'" json:"state"`
	ErrorMsg   string     `gorm:"column:error_msg;type:text" json:"error_msg"`
	ExecutedAt *time.Time `gorm:"column:executed_at" json:"executed_at"`
}

func (SQLChangeStatement) TableName() string { return "sql_change_statements" }

// SQLChangeWorkflowDetail 工单工作流动作
type SQLChangeWorkflowDetail struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	TicketID  uint      `gorm:"column:ticket_id;not null;index" json:"ticket_id"`
	WorkID    string    `gorm:"column:work_id;size:64;not null;index" json:"work_id"`
	Username  string    `gorm:"size:100;not null" json:"username"`
	Action    string    `gorm:"size:50;not null" json:"action"`
	Step      int       `gorm:"not null;default:0" json:"step"`
	Comment   string    `gorm:"type:text" json:"comment"`
}

func (SQLChangeWorkflowDetail) TableName() string { return "sql_change_workflow_details" }
