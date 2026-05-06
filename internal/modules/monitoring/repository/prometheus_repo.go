package repository

import (
	"devops/internal/models/monitoring"

	"gorm.io/gorm"
)

type PrometheusInstanceRepository struct{ db *gorm.DB }

func NewPrometheusInstanceRepository(db *gorm.DB) *PrometheusInstanceRepository {
	return &PrometheusInstanceRepository{db: db}
}

func (r *PrometheusInstanceRepository) List() ([]monitoring.PrometheusInstance, error) {
	var list []monitoring.PrometheusInstance
	return list, r.db.Order("id ASC").Find(&list).Error
}

func (r *PrometheusInstanceRepository) GetByID(id uint) (*monitoring.PrometheusInstance, error) {
	var inst monitoring.PrometheusInstance
	return &inst, r.db.First(&inst, id).Error
}

func (r *PrometheusInstanceRepository) GetDefault() (*monitoring.PrometheusInstance, error) {
	var inst monitoring.PrometheusInstance
	err := r.db.Where("is_default = ? AND status = ?", true, "active").First(&inst).Error
	return &inst, err
}

func (r *PrometheusInstanceRepository) Create(inst *monitoring.PrometheusInstance) error {
	return r.db.Create(inst).Error
}

func (r *PrometheusInstanceRepository) Update(inst *monitoring.PrometheusInstance) error {
	return r.db.Save(inst).Error
}

func (r *PrometheusInstanceRepository) Delete(id uint) error {
	return r.db.Delete(&monitoring.PrometheusInstance{}, id).Error
}
