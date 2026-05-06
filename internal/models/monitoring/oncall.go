package monitoring

import "time"

// OncallSchedule 值班排班表
type OncallSchedule struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	Timezone    string    `gorm:"size:50;default:'Asia/Shanghai'" json:"timezone"`
	RotationType string   `gorm:"size:20;default:'weekly'" json:"rotation_type"` // daily / weekly / custom
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedBy   uint      `gorm:"default:0" json:"created_by"`
}

func (OncallSchedule) TableName() string {
	return "oncall_schedules"
}

// OncallShift 值班班次
type OncallShift struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	ScheduleID uint      `gorm:"not null;index" json:"schedule_id"`
	UserID     uint      `gorm:"not null;index" json:"user_id"`
	UserName   string    `gorm:"size:100" json:"user_name"`
	StartTime  time.Time `gorm:"not null;index" json:"start_time"`
	EndTime    time.Time `gorm:"not null" json:"end_time"`
	ShiftType  string    `gorm:"size:20;default:'primary'" json:"shift_type"` // primary / backup
}

func (OncallShift) TableName() string {
	return "oncall_shifts"
}

// OncallOverride 值班临时替换
type OncallOverride struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	ScheduleID      uint      `gorm:"not null;index" json:"schedule_id"`
	OriginalUserID  uint      `gorm:"not null" json:"original_user_id"`
	OriginalUserName string   `gorm:"size:100" json:"original_user_name"`
	OverrideUserID  uint      `gorm:"not null" json:"override_user_id"`
	OverrideUserName string   `gorm:"size:100" json:"override_user_name"`
	StartTime       time.Time `gorm:"not null" json:"start_time"`
	EndTime         time.Time `gorm:"not null" json:"end_time"`
	Reason          string    `gorm:"size:500" json:"reason"`
	CreatedBy       uint      `gorm:"default:0" json:"created_by"`
}

func (OncallOverride) TableName() string {
	return "oncall_overrides"
}

// AlertAssignment 告警分配记录
type AlertAssignment struct {
	ID             uint       `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	AlertHistoryID uint       `gorm:"not null;index" json:"alert_history_id"`
	AssigneeID     uint       `gorm:"not null;index" json:"assignee_id"`
	AssigneeName   string     `gorm:"size:100" json:"assignee_name"`
	ScheduleID     *uint      `json:"schedule_id"`
	Status         string     `gorm:"size:20;default:'pending'" json:"status"` // pending / claimed / resolved / escalated
	ClaimedAt      *time.Time `json:"claimed_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	Comment        string     `gorm:"type:text" json:"comment"`
}

func (AlertAssignment) TableName() string {
	return "alert_assignments"
}
