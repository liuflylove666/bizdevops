package repository

import (
	"devops/internal/models/infrastructure"

	"gorm.io/gorm"
)

// ArgoCDInstanceRepository Argo CD 实例仓��
type ArgoCDInstanceRepository struct{ db *gorm.DB }

func NewArgoCDInstanceRepository(db *gorm.DB) *ArgoCDInstanceRepository {
	return &ArgoCDInstanceRepository{db: db}
}

func (r *ArgoCDInstanceRepository) List() ([]infrastructure.ArgoCDInstance, error) {
	var list []infrastructure.ArgoCDInstance
	return list, r.db.Order("id ASC").Find(&list).Error
}

func (r *ArgoCDInstanceRepository) GetByID(id uint) (*infrastructure.ArgoCDInstance, error) {
	var inst infrastructure.ArgoCDInstance
	return &inst, r.db.First(&inst, id).Error
}

func (r *ArgoCDInstanceRepository) Create(inst *infrastructure.ArgoCDInstance) error {
	return r.db.Create(inst).Error
}

func (r *ArgoCDInstanceRepository) Update(inst *infrastructure.ArgoCDInstance) error {
	return r.db.Save(inst).Error
}

func (r *ArgoCDInstanceRepository) Delete(id uint) error {
	return r.db.Delete(&infrastructure.ArgoCDInstance{}, id).Error
}

// ArgoCDApplicationRepository Argo CD 应用仓库
type ArgoCDApplicationRepository struct{ db *gorm.DB }

func NewArgoCDApplicationRepository(db *gorm.DB) *ArgoCDApplicationRepository {
	return &ArgoCDApplicationRepository{db: db}
}

type ArgoCDAppFilter struct {
	InstanceID   *uint
	ProjectID    *uint
	SyncStatus   string
	HealthStatus string
	Env          string
	DriftOnly    bool
}

type ArgoCDAppSummary struct {
	Total      int64 `json:"total"`
	Synced     int64 `json:"synced"`
	OutOfSync  int64 `json:"out_of_sync"`
	Healthy    int64 `json:"healthy"`
	Degraded   int64 `json:"degraded"`
	Drifted    int64 `json:"drifted"`
	AutoSync   int64 `json:"auto_sync"`
}

func (r *ArgoCDApplicationRepository) List(f ArgoCDAppFilter, page, pageSize int) ([]infrastructure.ArgoCDApplication, int64, error) {
	var list []infrastructure.ArgoCDApplication
	var total int64
	q := r.db.Model(&infrastructure.ArgoCDApplication{})
	if f.InstanceID != nil {
		q = q.Where("argocd_instance_id = ?", *f.InstanceID)
	}
	if f.ProjectID != nil {
		q = q.Joins("JOIN applications ON applications.id = argocd_applications.application_id").
			Where("applications.project_id = ?", *f.ProjectID)
	}
	if f.SyncStatus != "" {
		q = q.Where("sync_status = ?", f.SyncStatus)
	}
	if f.HealthStatus != "" {
		q = q.Where("health_status = ?", f.HealthStatus)
	}
	if f.Env != "" {
		q = q.Where("env = ?", f.Env)
	}
	if f.DriftOnly {
		q = q.Where("drift_detected = ?", true)
	}
	q.Count(&total)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	err := q.Order("name ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *ArgoCDApplicationRepository) GetByID(id uint) (*infrastructure.ArgoCDApplication, error) {
	var app infrastructure.ArgoCDApplication
	return &app, r.db.First(&app, id).Error
}

func (r *ArgoCDApplicationRepository) Create(app *infrastructure.ArgoCDApplication) error {
	return r.db.Create(app).Error
}

func (r *ArgoCDApplicationRepository) Update(app *infrastructure.ArgoCDApplication) error {
	return r.db.Save(app).Error
}

func (r *ArgoCDApplicationRepository) Delete(id uint) error {
	return r.db.Delete(&infrastructure.ArgoCDApplication{}, id).Error
}

func (r *ArgoCDApplicationRepository) ListByInstance(instanceID uint) ([]infrastructure.ArgoCDApplication, error) {
	var list []infrastructure.ArgoCDApplication
	return list, r.db.Where("argocd_instance_id = ?", instanceID).Order("name ASC").Find(&list).Error
}

func (r *ArgoCDApplicationRepository) Summary(projectID *uint) (ArgoCDAppSummary, error) {
	summary := ArgoCDAppSummary{}
	base := r.db.Model(&infrastructure.ArgoCDApplication{})
	if projectID != nil {
		base = base.Joins("JOIN applications ON applications.id = argocd_applications.application_id").
			Where("applications.project_id = ?", *projectID)
	}

	if err := base.Count(&summary.Total).Error; err != nil {
		return summary, err
	}
	if err := base.Session(&gorm.Session{}).Where("sync_status = ?", "Synced").Count(&summary.Synced).Error; err != nil {
		return summary, err
	}
	if err := base.Session(&gorm.Session{}).Where("sync_status = ?", "OutOfSync").Count(&summary.OutOfSync).Error; err != nil {
		return summary, err
	}
	if err := base.Session(&gorm.Session{}).Where("health_status = ?", "Healthy").Count(&summary.Healthy).Error; err != nil {
		return summary, err
	}
	if err := base.Session(&gorm.Session{}).Where("health_status = ?", "Degraded").Count(&summary.Degraded).Error; err != nil {
		return summary, err
	}
	if err := base.Session(&gorm.Session{}).Where("drift_detected = ?", true).Count(&summary.Drifted).Error; err != nil {
		return summary, err
	}
	if err := base.Session(&gorm.Session{}).Where("sync_policy = ?", "auto").Count(&summary.AutoSync).Error; err != nil {
		return summary, err
	}
	return summary, nil
}

// GitOpsRepoRepository Git 部署仓库声明
type GitOpsRepoRepository struct{ db *gorm.DB }

func NewGitOpsRepoRepository(db *gorm.DB) *GitOpsRepoRepository {
	return &GitOpsRepoRepository{db: db}
}

func (r *GitOpsRepoRepository) List(projectID *uint, page, pageSize int) ([]infrastructure.GitOpsRepo, int64, error) {
	var list []infrastructure.GitOpsRepo
	var total int64
	q := r.db.Model(&infrastructure.GitOpsRepo{})
	if projectID != nil {
		q = q.Joins("JOIN applications ON applications.id = gitops_repos.application_id").
			Where("applications.project_id = ?", *projectID)
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

func (r *GitOpsRepoRepository) GetByID(id uint) (*infrastructure.GitOpsRepo, error) {
	var repo infrastructure.GitOpsRepo
	return &repo, r.db.First(&repo, id).Error
}

func (r *GitOpsRepoRepository) Create(repo *infrastructure.GitOpsRepo) error {
	return r.db.Create(repo).Error
}

func (r *GitOpsRepoRepository) Update(repo *infrastructure.GitOpsRepo) error {
	return r.db.Save(repo).Error
}

func (r *GitOpsRepoRepository) Delete(id uint) error {
	return r.db.Delete(&infrastructure.GitOpsRepo{}, id).Error
}

func (r *GitOpsRepoRepository) CountSyncEnabled() (int64, error) {
	var count int64
	err := r.db.Model(&infrastructure.GitOpsRepo{}).Where("sync_enabled = ?", true).Count(&count).Error
	return count, err
}

// GitOpsChangeRequestRepository GitOps 变更请求
type GitOpsChangeRequestRepository struct{ db *gorm.DB }

func NewGitOpsChangeRequestRepository(db *gorm.DB) *GitOpsChangeRequestRepository {
	return &GitOpsChangeRequestRepository{db: db}
}

func (r *GitOpsChangeRequestRepository) List(projectID *uint, page, pageSize int) ([]infrastructure.GitOpsChangeRequest, int64, error) {
	var list []infrastructure.GitOpsChangeRequest
	var total int64
	q := r.db.Model(&infrastructure.GitOpsChangeRequest{})
	if projectID != nil {
		q = q.Joins("JOIN applications ON applications.id = gitops_change_requests.application_id").
			Where("applications.project_id = ?", *projectID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *GitOpsChangeRequestRepository) GetByID(id uint) (*infrastructure.GitOpsChangeRequest, error) {
	var item infrastructure.GitOpsChangeRequest
	return &item, r.db.First(&item, id).Error
}

func (r *GitOpsChangeRequestRepository) GetByApprovalInstanceID(approvalInstanceID uint) (*infrastructure.GitOpsChangeRequest, error) {
	var item infrastructure.GitOpsChangeRequest
	return &item, r.db.Where("approval_instance_id = ?", approvalInstanceID).First(&item).Error
}

func (r *GitOpsChangeRequestRepository) Create(item *infrastructure.GitOpsChangeRequest) error {
	return r.db.Create(item).Error
}

func (r *GitOpsChangeRequestRepository) Update(item *infrastructure.GitOpsChangeRequest) error {
	return r.db.Save(item).Error
}

func (r *GitOpsChangeRequestRepository) CountByStatus(projectID *uint, status string) (int64, error) {
	var count int64
	q := r.db.Model(&infrastructure.GitOpsChangeRequest{}).Where("status = ?", status)
	if projectID != nil {
		q = q.Joins("JOIN applications ON applications.id = gitops_change_requests.application_id").
			Where("applications.project_id = ?", *projectID)
	}
	err := q.Count(&count).Error
	return count, err
}
