package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
)

type DBQueryLogRepository struct {
	db *gorm.DB
}

func NewDBQueryLogRepository(db *gorm.DB) *DBQueryLogRepository {
	return &DBQueryLogRepository{db: db}
}

type DBQueryLogFilter struct {
	InstanceID uint
	Username   string
	Status     string
}

func (r *DBQueryLogRepository) Create(ctx context.Context, m *model.DBQueryLog) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *DBQueryLogRepository) List(ctx context.Context, f DBQueryLogFilter, page, pageSize int) ([]model.DBQueryLog, int64, error) {
	var list []model.DBQueryLog
	var total int64

	q := r.db.WithContext(ctx).Model(&model.DBQueryLog{})
	if f.InstanceID > 0 {
		q = q.Where("instance_id = ?", f.InstanceID)
	}
	if f.Username != "" {
		q = q.Where("username = ?", f.Username)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
