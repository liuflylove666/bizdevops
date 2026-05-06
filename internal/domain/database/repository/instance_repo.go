package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
)

type DBInstanceRepository struct {
	db *gorm.DB
}

func NewDBInstanceRepository(db *gorm.DB) *DBInstanceRepository {
	return &DBInstanceRepository{db: db}
}

type DBInstanceFilter struct {
	Env    string
	DBType string
	Status string
	Name   string
}

func (r *DBInstanceRepository) Create(ctx context.Context, m *model.DBInstance) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *DBInstanceRepository) Update(ctx context.Context, m *model.DBInstance) error {
	return r.db.WithContext(ctx).Model(m).Where("id = ?", m.ID).Updates(map[string]interface{}{
		"name":         m.Name,
		"db_type":      m.DBType,
		"env":          m.Env,
		"host":         m.Host,
		"port":         m.Port,
		"username":     m.Username,
		"password":     m.Password,
		"default_db":   m.DefaultDB,
		"exclude_dbs":  m.ExcludeDBs,
		"params":       m.Params,
		"mode":         m.Mode,
		"status":       m.Status,
		"description":  m.Description,
	}).Error
}

func (r *DBInstanceRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.DBInstance{}, id).Error
}

func (r *DBInstanceRepository) GetByID(ctx context.Context, id uint) (*model.DBInstance, error) {
	var m model.DBInstance
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *DBInstanceRepository) List(ctx context.Context, f DBInstanceFilter, page, pageSize int) ([]model.DBInstance, int64, error) {
	var list []model.DBInstance
	var total int64

	q := r.db.WithContext(ctx).Model(&model.DBInstance{})
	if f.Env != "" {
		q = q.Where("env = ?", f.Env)
	}
	if f.DBType != "" {
		q = q.Where("db_type = ?", f.DBType)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Name != "" {
		q = q.Where("name LIKE ?", "%"+f.Name+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *DBInstanceRepository) ListIDsByCreator(ctx context.Context, userID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.WithContext(ctx).Model(&model.DBInstance{}).
		Where("created_by = ?", userID).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *DBInstanceRepository) ListByIDs(ctx context.Context, f DBInstanceFilter, ids []uint, page, pageSize int) ([]model.DBInstance, int64, error) {
	var list []model.DBInstance
	var total int64
	q := r.db.WithContext(ctx).Model(&model.DBInstance{}).Where("id IN ?", ids)
	if f.Env != "" {
		q = q.Where("env = ?", f.Env)
	}
	if f.DBType != "" {
		q = q.Where("db_type = ?", f.DBType)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Name != "" {
		q = q.Where("name LIKE ?", "%"+f.Name+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *DBInstanceRepository) ListAllByIDs(ctx context.Context, ids []uint) ([]model.DBInstance, error) {
	var list []model.DBInstance
	if err := r.db.WithContext(ctx).Where("status = ? AND id IN ?", "active", ids).Order("env, name").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *DBInstanceRepository) ListAll(ctx context.Context) ([]model.DBInstance, error) {
	var list []model.DBInstance
	if err := r.db.WithContext(ctx).Where("status = ?", "active").Order("env, name").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
