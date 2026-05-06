package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/models/application"
)

// --- Organization ---

type OrganizationRepository struct{ db *gorm.DB }

func NewOrganizationRepository(db *gorm.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) List(ctx context.Context) ([]application.Organization, error) {
	var list []application.Organization
	return list, r.db.WithContext(ctx).Order("name").Find(&list).Error
}

func (r *OrganizationRepository) GetByID(ctx context.Context, id uint) (*application.Organization, error) {
	var org application.Organization
	return &org, r.db.WithContext(ctx).First(&org, id).Error
}

func (r *OrganizationRepository) Create(ctx context.Context, org *application.Organization) error {
	return r.db.WithContext(ctx).Create(org).Error
}

func (r *OrganizationRepository) Update(ctx context.Context, org *application.Organization) error {
	return r.db.WithContext(ctx).Save(org).Error
}

func (r *OrganizationRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&application.Organization{}, id).Error
}

// --- Project ---

type ProjectRepository struct{ db *gorm.DB }

func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) List(ctx context.Context, orgID uint) ([]application.Project, error) {
	var list []application.Project
	q := r.db.WithContext(ctx).Order("name")
	if orgID > 0 {
		q = q.Where("organization_id = ?", orgID)
	}
	return list, q.Find(&list).Error
}

func (r *ProjectRepository) GetByID(ctx context.Context, id uint) (*application.Project, error) {
	var proj application.Project
	return &proj, r.db.WithContext(ctx).First(&proj, id).Error
}

func (r *ProjectRepository) Create(ctx context.Context, proj *application.Project) error {
	return r.db.WithContext(ctx).Create(proj).Error
}

func (r *ProjectRepository) Update(ctx context.Context, proj *application.Project) error {
	return r.db.WithContext(ctx).Save(proj).Error
}

func (r *ProjectRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&application.Project{}, id).Error
}

// --- EnvDefinition ---

type EnvDefinitionRepository struct{ db *gorm.DB }

func NewEnvDefinitionRepository(db *gorm.DB) *EnvDefinitionRepository {
	return &EnvDefinitionRepository{db: db}
}

func (r *EnvDefinitionRepository) List(ctx context.Context) ([]application.EnvDefinition, error) {
	var list []application.EnvDefinition
	return list, r.db.WithContext(ctx).Order("sort_order, name").Find(&list).Error
}

func (r *EnvDefinitionRepository) Create(ctx context.Context, env *application.EnvDefinition) error {
	return r.db.WithContext(ctx).Create(env).Error
}

func (r *EnvDefinitionRepository) Update(ctx context.Context, env *application.EnvDefinition) error {
	return r.db.WithContext(ctx).Save(env).Error
}

func (r *EnvDefinitionRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&application.EnvDefinition{}, id).Error
}
