// Package repository
//
// 历史名为 oa_repo.go，当前仅保留 OANotifyConfigRepository 作为历史
// 遗留默认通知配置表的访问入口。OAData / OAAddress 已在 v2.0 清理中移除。
//
// Deprecated: OANotifyConfigRepository 为历史遗留仓储；新通知以 Telegram
// 领域仓储为准，后续版本会移除对本接口的调用。
package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/models"
)

// OANotifyConfigRepository 历史遗留默认通知配置仓储。
//
// Deprecated: 保留历史命名以兼容旧调用方，详见包注释。
type OANotifyConfigRepository struct {
	db *gorm.DB
}

// NewOANotifyConfigRepository 构造仓储实例。
//
// Deprecated: 保留历史命名以兼容旧调用方，详见包注释。
func NewOANotifyConfigRepository(db *gorm.DB) *OANotifyConfigRepository {
	return &OANotifyConfigRepository{db: db}
}

func (r *OANotifyConfigRepository) Create(ctx context.Context, config *models.OANotifyConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *OANotifyConfigRepository) GetByID(ctx context.Context, id uint) (*models.OANotifyConfig, error) {
	var config models.OANotifyConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *OANotifyConfigRepository) List(ctx context.Context, page, pageSize int) ([]models.OANotifyConfig, int64, error) {
	var list []models.OANotifyConfig
	var total int64

	query := r.db.WithContext(ctx).Model(&models.OANotifyConfig{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (r *OANotifyConfigRepository) ListActive(ctx context.Context) ([]models.OANotifyConfig, error) {
	var list []models.OANotifyConfig
	err := r.db.WithContext(ctx).Where("status = ?", "active").Find(&list).Error
	return list, err
}

func (r *OANotifyConfigRepository) Update(ctx context.Context, config *models.OANotifyConfig) error {
	return r.db.WithContext(ctx).Model(config).Where("id = ?", config.ID).Updates(map[string]interface{}{
		"name":            config.Name,
		"app_id":          config.AppID,
		"receive_id":      config.ReceiveID,
		"receive_id_type": config.ReceiveIDType,
		"description":     config.Description,
		"status":          config.Status,
		"is_default":      config.IsDefault,
	}).Error
}

func (r *OANotifyConfigRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.OANotifyConfig{}, id).Error
}

func (r *OANotifyConfigRepository) GetDefault(ctx context.Context) (*models.OANotifyConfig, error) {
	var config models.OANotifyConfig
	err := r.db.WithContext(ctx).Where("is_default = ? AND status = ?", true, "active").First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *OANotifyConfigRepository) SetDefault(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Model(&models.OANotifyConfig{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&models.OANotifyConfig{}).Where("id = ?", id).Update("is_default", true).Error
}
