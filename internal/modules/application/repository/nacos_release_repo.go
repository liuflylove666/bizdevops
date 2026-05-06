package repository

import (
	"devops/internal/models/deploy"

	"gorm.io/gorm"
)

type NacosReleaseRepository struct{ db *gorm.DB }

func NewNacosReleaseRepository(db *gorm.DB) *NacosReleaseRepository {
	return &NacosReleaseRepository{db: db}
}

type NacosReleaseFilter struct {
	Env      string
	Status   string
	DataID   string
	ServiceID *uint
}

func (r *NacosReleaseRepository) List(f NacosReleaseFilter, page, pageSize int) ([]deploy.NacosRelease, int64, error) {
	var list []deploy.NacosRelease
	var total int64
	q := r.db.Model(&deploy.NacosRelease{})
	if f.Env != "" {
		q = q.Where("env = ?", f.Env)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.DataID != "" {
		q = q.Where("data_id LIKE ?", "%"+f.DataID+"%")
	}
	if f.ServiceID != nil {
		q = q.Where("service_id = ?", *f.ServiceID)
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

func (r *NacosReleaseRepository) GetByID(id uint) (*deploy.NacosRelease, error) {
	var nr deploy.NacosRelease
	return &nr, r.db.First(&nr, id).Error
}

func (r *NacosReleaseRepository) GetByApprovalInstanceID(approvalInstanceID uint) (*deploy.NacosRelease, error) {
	var nr deploy.NacosRelease
	return &nr, r.db.Where("approval_instance_id = ?", approvalInstanceID).First(&nr).Error
}

func (r *NacosReleaseRepository) Create(nr *deploy.NacosRelease) error {
	return r.db.Create(nr).Error
}

func (r *NacosReleaseRepository) Update(nr *deploy.NacosRelease) error {
	return r.db.Save(nr).Error
}

func (r *NacosReleaseRepository) Delete(id uint) error {
	return r.db.Delete(&deploy.NacosRelease{}, id).Error
}

func (r *NacosReleaseRepository) ListByRelease(releaseID uint) ([]deploy.NacosRelease, error) {
	var list []deploy.NacosRelease
	return list, r.db.Where("release_id = ?", releaseID).Order("id ASC").Find(&list).Error
}

func (r *NacosReleaseRepository) ListByService(serviceID uint, limit int) ([]deploy.NacosRelease, error) {
	var list []deploy.NacosRelease
	q := r.db.Where("service_id = ?", serviceID).Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return list, q.Find(&list).Error
}
