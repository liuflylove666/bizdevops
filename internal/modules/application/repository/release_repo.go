package repository

import (
	"time"

	"devops/internal/models/deploy"

	"gorm.io/gorm"
)

type ReleaseRepository struct{ db *gorm.DB }

func NewReleaseRepository(db *gorm.DB) *ReleaseRepository {
	return &ReleaseRepository{db: db}
}

type ReleaseFilter struct {
	Env           string
	Status        string
	ApplicationID *uint
	ProjectID     *uint
	Title         string
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
}

func (r *ReleaseRepository) List(f ReleaseFilter, page, pageSize int) ([]deploy.Release, int64, error) {
	var list []deploy.Release
	var total int64
	q := r.db.Model(&deploy.Release{}).
		Select("releases.*, applications.project_id AS project_id, projects.display_name AS project_name").
		Joins("LEFT JOIN applications ON applications.id = releases.application_id").
		Joins("LEFT JOIN projects ON projects.id = applications.project_id")
	if f.Env != "" {
		q = q.Where("releases.env = ?", f.Env)
	}
	if f.Status != "" {
		q = q.Where("releases.status = ?", f.Status)
	}
	if f.ApplicationID != nil {
		q = q.Where("releases.application_id = ?", *f.ApplicationID)
	}
	if f.ProjectID != nil {
		q = q.Where("applications.project_id = ?", *f.ProjectID)
	}
	if f.Title != "" {
		q = q.Where("releases.title LIKE ?", "%"+f.Title+"%")
	}
	if f.CreatedFrom != nil {
		q = q.Where("releases.created_at >= ?", *f.CreatedFrom)
	}
	if f.CreatedTo != nil {
		q = q.Where("releases.created_at <= ?", *f.CreatedTo)
	}
	q.Count(&total)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	err := q.Order("releases.created_at DESC, releases.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *ReleaseRepository) GetByID(id uint) (*deploy.Release, error) {
	var rel deploy.Release
	return &rel, r.db.First(&rel, id).Error
}

func (r *ReleaseRepository) Create(rel *deploy.Release) error {
	return r.db.Create(rel).Error
}

func (r *ReleaseRepository) Update(rel *deploy.Release) error {
	return r.db.Save(rel).Error
}

func (r *ReleaseRepository) Delete(id uint) error {
	return r.db.Delete(&deploy.Release{}, id).Error
}

// ReleaseItem Repository

type ReleaseItemRepository struct{ db *gorm.DB }

func NewReleaseItemRepository(db *gorm.DB) *ReleaseItemRepository {
	return &ReleaseItemRepository{db: db}
}

func (r *ReleaseItemRepository) ListByRelease(releaseID uint) ([]deploy.ReleaseItem, error) {
	var list []deploy.ReleaseItem
	return list, r.db.Where("release_id = ?", releaseID).Order("sort_order ASC, id ASC").Find(&list).Error
}

func (r *ReleaseItemRepository) Create(item *deploy.ReleaseItem) error {
	return r.db.Create(item).Error
}

func (r *ReleaseItemRepository) Delete(id uint) error {
	return r.db.Delete(&deploy.ReleaseItem{}, id).Error
}

func (r *ReleaseItemRepository) DeleteByRelease(releaseID uint) error {
	return r.db.Where("release_id = ?", releaseID).Delete(&deploy.ReleaseItem{}).Error
}
