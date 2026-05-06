package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/models/deploy"
)

type PromotionPolicyRepository struct {
	db *gorm.DB
}

func NewPromotionPolicyRepository(db *gorm.DB) *PromotionPolicyRepository {
	return &PromotionPolicyRepository{db: db}
}

func (r *PromotionPolicyRepository) GetByAppID(ctx context.Context, appID uint) (*deploy.EnvPromotionPolicy, error) {
	var p deploy.EnvPromotionPolicy
	if err := r.db.WithContext(ctx).Where("application_id = ?", appID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PromotionPolicyRepository) Upsert(ctx context.Context, p *deploy.EnvPromotionPolicy) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *PromotionPolicyRepository) Delete(ctx context.Context, appID uint) error {
	return r.db.WithContext(ctx).Where("application_id = ?", appID).Delete(&deploy.EnvPromotionPolicy{}).Error
}

type PromotionRecordRepository struct {
	db *gorm.DB
}

func NewPromotionRecordRepository(db *gorm.DB) *PromotionRecordRepository {
	return &PromotionRecordRepository{db: db}
}

func (r *PromotionRecordRepository) Create(ctx context.Context, rec *deploy.EnvPromotionRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *PromotionRecordRepository) GetByID(ctx context.Context, id uint) (*deploy.EnvPromotionRecord, error) {
	var rec deploy.EnvPromotionRecord
	if err := r.db.WithContext(ctx).Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).First(&rec, id).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *PromotionRecordRepository) Update(ctx context.Context, rec *deploy.EnvPromotionRecord) error {
	return r.db.WithContext(ctx).Save(rec).Error
}

type PromotionRecordFilter struct {
	ApplicationID uint
	Status        string
	ImageTag      string
}

func (r *PromotionRecordRepository) List(ctx context.Context, f PromotionRecordFilter, page, pageSize int) ([]deploy.EnvPromotionRecord, int64, error) {
	q := r.db.WithContext(ctx).Model(&deploy.EnvPromotionRecord{})
	if f.ApplicationID > 0 {
		q = q.Where("application_id = ?", f.ApplicationID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.ImageTag != "" {
		q = q.Where("image_tag LIKE ?", "%"+f.ImageTag+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []deploy.EnvPromotionRecord
	if err := q.Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

type PromotionStepRepository struct {
	db *gorm.DB
}

func NewPromotionStepRepository(db *gorm.DB) *PromotionStepRepository {
	return &PromotionStepRepository{db: db}
}

func (r *PromotionStepRepository) Create(ctx context.Context, step *deploy.EnvPromotionStep) error {
	return r.db.WithContext(ctx).Create(step).Error
}

func (r *PromotionStepRepository) Update(ctx context.Context, step *deploy.EnvPromotionStep) error {
	return r.db.WithContext(ctx).Save(step).Error
}

func (r *PromotionStepRepository) GetByID(ctx context.Context, id uint) (*deploy.EnvPromotionStep, error) {
	var step deploy.EnvPromotionStep
	if err := r.db.WithContext(ctx).First(&step, id).Error; err != nil {
		return nil, err
	}
	return &step, nil
}
