package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
)

type SQLAuditRuleRepository struct {
	db *gorm.DB
}

func NewSQLAuditRuleRepository(db *gorm.DB) *SQLAuditRuleRepository {
	return &SQLAuditRuleRepository{db: db}
}

func (r *SQLAuditRuleRepository) Create(ctx context.Context, m *model.SQLAuditRuleSet) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *SQLAuditRuleRepository) Update(ctx context.Context, m *model.SQLAuditRuleSet) error {
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *SQLAuditRuleRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.SQLAuditRuleSet{}, id).Error
}

func (r *SQLAuditRuleRepository) GetByID(ctx context.Context, id uint) (*model.SQLAuditRuleSet, error) {
	var m model.SQLAuditRuleSet
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *SQLAuditRuleRepository) GetDefault(ctx context.Context) (*model.SQLAuditRuleSet, error) {
	var m model.SQLAuditRuleSet
	if err := r.db.WithContext(ctx).Where("is_default = ?", true).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *SQLAuditRuleRepository) List(ctx context.Context) ([]model.SQLAuditRuleSet, error) {
	var list []model.SQLAuditRuleSet
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

// SetDefault 事务内把指定 id 设为唯一默认
func (r *SQLAuditRuleRepository) SetDefault(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SQLAuditRuleSet{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.SQLAuditRuleSet{}).Where("id = ?", id).Update("is_default", true).Error
	})
}
