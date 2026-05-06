package repository

import (
	"devops/internal/models/deploy"

	"gorm.io/gorm"
)

type EnvInstanceRepository struct{ db *gorm.DB }

func NewEnvInstanceRepository(db *gorm.DB) *EnvInstanceRepository {
	return &EnvInstanceRepository{db: db}
}

type EnvInstanceFilter struct {
	ApplicationID *uint
	Env           string
	ClusterID     *uint
	Status        string
}

func (r *EnvInstanceRepository) List(f EnvInstanceFilter, page, pageSize int) ([]deploy.EnvInstance, int64, error) {
	var list []deploy.EnvInstance
	var total int64
	q := r.db.Model(&deploy.EnvInstance{})
	if f.ApplicationID != nil {
		q = q.Where("application_id = ?", *f.ApplicationID)
	}
	if f.Env != "" {
		q = q.Where("env = ?", f.Env)
	}
	if f.ClusterID != nil {
		q = q.Where("cluster_id = ?", *f.ClusterID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	q.Count(&total)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	err := q.Order("application_name ASC, FIELD(env, 'dev', 'test', 'uat', 'gray', 'prod')").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *EnvInstanceRepository) GetByID(id uint) (*deploy.EnvInstance, error) {
	var inst deploy.EnvInstance
	return &inst, r.db.First(&inst, id).Error
}

func (r *EnvInstanceRepository) GetByAppEnv(appID uint, env string) (*deploy.EnvInstance, error) {
	var inst deploy.EnvInstance
	return &inst, r.db.Where("application_id = ? AND env = ?", appID, env).First(&inst).Error
}

func (r *EnvInstanceRepository) Create(inst *deploy.EnvInstance) error {
	return r.db.Create(inst).Error
}

func (r *EnvInstanceRepository) Update(inst *deploy.EnvInstance) error {
	return r.db.Save(inst).Error
}

func (r *EnvInstanceRepository) Delete(id uint) error {
	return r.db.Delete(&deploy.EnvInstance{}, id).Error
}

func (r *EnvInstanceRepository) ListByApp(appID uint) ([]deploy.EnvInstance, error) {
	var list []deploy.EnvInstance
	return list, r.db.Where("application_id = ?", appID).Order("FIELD(env, 'dev', 'test', 'uat', 'gray', 'prod')").Find(&list).Error
}

// EnvMatrix 获取环境矩阵视图（所有应用的所有环境实例）
func (r *EnvInstanceRepository) EnvMatrix(envs []string) ([]deploy.EnvInstance, error) {
	var list []deploy.EnvInstance
	q := r.db.Order("application_name ASC, FIELD(env, 'dev', 'test', 'uat', 'gray', 'prod')")
	if len(envs) > 0 {
		q = q.Where("env IN ?", envs)
	}
	return list, q.Find(&list).Error
}
