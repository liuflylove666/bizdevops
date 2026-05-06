package repository

import (
	"devops/internal/models/infrastructure"
	"strings"

	"gorm.io/gorm"
)

type SonarQubeInstanceRepository struct{ db *gorm.DB }

func NewSonarQubeInstanceRepository(db *gorm.DB) *SonarQubeInstanceRepository {
	return &SonarQubeInstanceRepository{db: db}
}

func (r *SonarQubeInstanceRepository) List() ([]infrastructure.SonarQubeInstance, error) {
	var list []infrastructure.SonarQubeInstance
	return list, r.db.Order("name ASC").Find(&list).Error
}

func (r *SonarQubeInstanceRepository) GetByID(id uint) (*infrastructure.SonarQubeInstance, error) {
	var s infrastructure.SonarQubeInstance
	return &s, r.db.First(&s, id).Error
}

func (r *SonarQubeInstanceRepository) Create(s *infrastructure.SonarQubeInstance) error {
	return r.db.Create(s).Error
}

func (r *SonarQubeInstanceRepository) Update(s *infrastructure.SonarQubeInstance) error {
	return r.db.Save(s).Error
}

func (r *SonarQubeInstanceRepository) Delete(id uint) error {
	return r.db.Delete(&infrastructure.SonarQubeInstance{}, id).Error
}

type SonarQubeBindingRepository struct{ db *gorm.DB }

func NewSonarQubeBindingRepository(db *gorm.DB) *SonarQubeBindingRepository {
	return &SonarQubeBindingRepository{db: db}
}

func (r *SonarQubeBindingRepository) ListByInstance(sonarID uint) ([]infrastructure.SonarQubeProjectBinding, error) {
	var list []infrastructure.SonarQubeProjectBinding
	return list, r.db.Where("sonar_qube_id = ?", sonarID).Find(&list).Error
}

func (r *SonarQubeBindingRepository) GetByID(id uint) (*infrastructure.SonarQubeProjectBinding, error) {
	var b infrastructure.SonarQubeProjectBinding
	return &b, r.db.First(&b, id).Error
}

func (r *SonarQubeBindingRepository) GetByAppName(appName string) (*infrastructure.SonarQubeProjectBinding, error) {
	var b infrastructure.SonarQubeProjectBinding
	return &b, r.db.Where("LOWER(devops_app_name) = ?", strings.ToLower(strings.TrimSpace(appName))).First(&b).Error
}

func (r *SonarQubeBindingRepository) Create(b *infrastructure.SonarQubeProjectBinding) error {
	return r.db.Create(b).Error
}

func (r *SonarQubeBindingRepository) Update(b *infrastructure.SonarQubeProjectBinding) error {
	return r.db.Save(b).Error
}

func (r *SonarQubeBindingRepository) Delete(id uint) error {
	return r.db.Delete(&infrastructure.SonarQubeProjectBinding{}, id).Error
}
