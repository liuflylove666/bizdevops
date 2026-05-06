package repository

import (
	"devops/internal/models/deploy"

	"gorm.io/gorm"
)

type ChangeEventRepository struct{ db *gorm.DB }

func NewChangeEventRepository(db *gorm.DB) *ChangeEventRepository {
	return &ChangeEventRepository{db: db}
}

type ChangeEventFilter struct {
	EventType     string
	ApplicationID *uint
	Env           string
	Status        string
	Operator      string
	StartTime     string
	EndTime       string
}

func (r *ChangeEventRepository) List(f ChangeEventFilter, page, pageSize int) ([]deploy.ChangeEvent, int64, error) {
	var list []deploy.ChangeEvent
	var total int64
	q := r.db.Model(&deploy.ChangeEvent{})
	if f.EventType != "" {
		q = q.Where("event_type = ?", f.EventType)
	}
	if f.ApplicationID != nil {
		q = q.Where("application_id = ?", *f.ApplicationID)
	}
	if f.Env != "" {
		q = q.Where("env = ?", f.Env)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Operator != "" {
		q = q.Where("operator LIKE ?", "%"+f.Operator+"%")
	}
	if f.StartTime != "" {
		q = q.Where("created_at >= ?", f.StartTime)
	}
	if f.EndTime != "" {
		q = q.Where("created_at <= ?", f.EndTime)
	}
	q.Count(&total)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *ChangeEventRepository) Create(e *deploy.ChangeEvent) error {
	return r.db.Create(e).Error
}

func (r *ChangeEventRepository) ListByApplication(appID uint, limit int) ([]deploy.ChangeEvent, error) {
	var list []deploy.ChangeEvent
	q := r.db.Where("application_id = ?", appID).Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return list, q.Find(&list).Error
}

func (r *ChangeEventRepository) CountByType(eventType string) (int64, error) {
	var count int64
	return count, r.db.Model(&deploy.ChangeEvent{}).Where("event_type = ?", eventType).Count(&count).Error
}

// Stats 统计各事件类型数量
func (r *ChangeEventRepository) Stats() ([]EventTypeStat, error) {
	var stats []EventTypeStat
	err := r.db.Model(&deploy.ChangeEvent{}).
		Select("event_type, count(*) as count").
		Group("event_type").
		Scan(&stats).Error
	return stats, err
}

type EventTypeStat struct {
	EventType string `json:"event_type"`
	Count     int64  `json:"count"`
}
