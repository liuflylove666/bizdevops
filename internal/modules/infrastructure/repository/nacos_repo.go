package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/models/infrastructure"
)

type NacosInstanceRepository struct {
	db *gorm.DB
}

func NewNacosInstanceRepository(db *gorm.DB) *NacosInstanceRepository {
	return &NacosInstanceRepository{db: db}
}

func (r *NacosInstanceRepository) GetByID(ctx context.Context, id uint) (*infrastructure.NacosInstance, error) {
	var inst infrastructure.NacosInstance
	if err := r.db.WithContext(ctx).First(&inst, id).Error; err != nil {
		return nil, err
	}
	return &inst, nil
}

func (r *NacosInstanceRepository) List(ctx context.Context, env string) ([]infrastructure.NacosInstance, error) {
	var list []infrastructure.NacosInstance
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if env != "" {
		q = q.Where("env = ?", env)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *NacosInstanceRepository) Create(ctx context.Context, inst *infrastructure.NacosInstance) error {
	return r.db.WithContext(ctx).Create(inst).Error
}

func (r *NacosInstanceRepository) Update(ctx context.Context, inst *infrastructure.NacosInstance) error {
	return r.db.WithContext(ctx).Save(inst).Error
}

func (r *NacosInstanceRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&infrastructure.NacosInstance{}, id).Error
}
