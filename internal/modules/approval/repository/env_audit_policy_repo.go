package repository

import (
	"devops/internal/models"

	"gorm.io/gorm"
)

type EnvAuditPolicyRepository struct {
	db *gorm.DB
}

func NewEnvAuditPolicyRepository(db *gorm.DB) *EnvAuditPolicyRepository {
	return &EnvAuditPolicyRepository{db: db}
}

func (r *EnvAuditPolicyRepository) List() ([]models.EnvAuditPolicy, error) {
	var policies []models.EnvAuditPolicy
	err := r.db.Order("env_name ASC").Find(&policies).Error
	return policies, err
}

func (r *EnvAuditPolicyRepository) GetByID(id uint) (*models.EnvAuditPolicy, error) {
	var p models.EnvAuditPolicy
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *EnvAuditPolicyRepository) GetByEnvName(envName string) (*models.EnvAuditPolicy, error) {
	var p models.EnvAuditPolicy
	if err := r.db.Where("env_name = ? AND enabled = ?", envName, true).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *EnvAuditPolicyRepository) Create(p *models.EnvAuditPolicy) error {
	return r.db.Create(p).Error
}

func (r *EnvAuditPolicyRepository) Update(p *models.EnvAuditPolicy) error {
	return r.db.Save(p).Error
}

func (r *EnvAuditPolicyRepository) Delete(id uint) error {
	return r.db.Delete(&models.EnvAuditPolicy{}, id).Error
}
