package repository

import (
	"devops/internal/models"
	"strings"

	"gorm.io/gorm"
)

type JiraInstanceRepository struct {
	db *gorm.DB
}

func NewJiraInstanceRepository(db *gorm.DB) *JiraInstanceRepository {
	return &JiraInstanceRepository{db: db}
}

func (r *JiraInstanceRepository) List() ([]models.JiraInstance, error) {
	var list []models.JiraInstance
	err := r.db.Order("is_default DESC, name ASC").Find(&list).Error
	return list, err
}

func (r *JiraInstanceRepository) GetByID(id uint) (*models.JiraInstance, error) {
	var inst models.JiraInstance
	if err := r.db.First(&inst, id).Error; err != nil {
		return nil, err
	}
	return &inst, nil
}

func (r *JiraInstanceRepository) Create(inst *models.JiraInstance) error {
	return r.db.Create(inst).Error
}

func (r *JiraInstanceRepository) Update(inst *models.JiraInstance) error {
	return r.db.Save(inst).Error
}

func (r *JiraInstanceRepository) Delete(id uint) error {
	return r.db.Delete(&models.JiraInstance{}, id).Error
}

// JiraProjectMappingRepository

type JiraProjectMappingRepository struct {
	db *gorm.DB
}

func NewJiraProjectMappingRepository(db *gorm.DB) *JiraProjectMappingRepository {
	return &JiraProjectMappingRepository{db: db}
}

func (r *JiraProjectMappingRepository) ListByInstance(instanceID uint) ([]models.JiraProjectMapping, error) {
	var list []models.JiraProjectMapping
	err := r.db.Where("jira_instance_id = ?", instanceID).Find(&list).Error
	return list, err
}

func (r *JiraProjectMappingRepository) GetByID(id uint) (*models.JiraProjectMapping, error) {
	var m models.JiraProjectMapping
	if err := r.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *JiraProjectMappingRepository) GetByInstanceAndProjectKey(instanceID uint, projectKey string) (*models.JiraProjectMapping, error) {
	var m models.JiraProjectMapping
	if err := r.db.
		Where("jira_instance_id = ? AND jira_project_key = ?", instanceID, strings.TrimSpace(projectKey)).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *JiraProjectMappingRepository) FindByProjectKey(projectKey string) (*models.JiraProjectMapping, error) {
	var m models.JiraProjectMapping
	if err := r.db.
		Where("jira_project_key = ?", strings.TrimSpace(projectKey)).
		Order("updated_at DESC, id DESC").
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *JiraProjectMappingRepository) Create(m *models.JiraProjectMapping) error {
	return r.db.Create(m).Error
}

func (r *JiraProjectMappingRepository) Update(m *models.JiraProjectMapping) error {
	return r.db.Save(m).Error
}

func (r *JiraProjectMappingRepository) Delete(id uint) error {
	return r.db.Delete(&models.JiraProjectMapping{}, id).Error
}
