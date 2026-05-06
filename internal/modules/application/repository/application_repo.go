package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
)

type ApplicationRepository struct {
	db *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) *ApplicationRepository {
	return &ApplicationRepository{db: db}
}

func (r *ApplicationRepository) Create(ctx context.Context, app *models.Application) error {
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *ApplicationRepository) Update(ctx context.Context, app *models.Application) error {
	return r.db.WithContext(ctx).Save(app).Error
}

func (r *ApplicationRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Application{}, id).Error
}

func (r *ApplicationRepository) GetByID(ctx context.Context, id uint) (*models.Application, error) {
	var app models.Application
	if err := r.db.WithContext(ctx).First(&app, id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *ApplicationRepository) GetByName(ctx context.Context, name string) (*models.Application, error) {
	var app models.Application
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *ApplicationRepository) List(ctx context.Context, filter ApplicationFilter, page, pageSize int) ([]models.Application, int64, error) {
	var apps []models.Application
	var total int64

	query := r.db.WithContext(ctx).
		Model(&models.Application{}).
		Select("applications.*, organizations.display_name AS org_name, projects.display_name AS project_name").
		Joins("LEFT JOIN organizations ON organizations.id = applications.organization_id").
		Joins("LEFT JOIN projects ON projects.id = applications.project_id")

	if filter.Name != "" {
		query = query.Where("applications.name LIKE ? OR applications.display_name LIKE ?", "%"+filter.Name+"%", "%"+filter.Name+"%")
	}
	if filter.Team != "" {
		query = query.Where("applications.team = ?", filter.Team)
	}
	if filter.Status != "" {
		query = query.Where("applications.status = ?", filter.Status)
	}
	if filter.Language != "" {
		query = query.Where("applications.language = ?", filter.Language)
	}
	if filter.OrganizationID > 0 {
		query = query.Where("applications.organization_id = ?", filter.OrganizationID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("applications.project_id = ?", filter.ProjectID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("applications.created_at DESC").Offset(offset).Limit(pageSize).Find(&apps).Error; err != nil {
		return nil, 0, err
	}
	for i := range apps {
		if apps[i].OrgName == "" && apps[i].OrganizationID != nil && *apps[i].OrganizationID > 0 {
			var orgName string
			if err := r.db.WithContext(ctx).Table("organizations").Select("name").Where("id = ?", *apps[i].OrganizationID).Scan(&orgName).Error; err == nil {
				apps[i].OrgName = orgName
			}
		}
		if apps[i].ProjectName == "" && apps[i].ProjectID != nil && *apps[i].ProjectID > 0 {
			var projectName string
			if err := r.db.WithContext(ctx).Table("projects").Select("name").Where("id = ?", *apps[i].ProjectID).Scan(&projectName).Error; err == nil {
				apps[i].ProjectName = projectName
			}
		}
	}

	return apps, total, nil
}

func (r *ApplicationRepository) GetAllTeams(ctx context.Context) ([]string, error) {
	var teams []string
	if err := r.db.WithContext(ctx).Model(&models.Application{}).Distinct("team").Where("team != ''").Pluck("team", &teams).Error; err != nil {
		return nil, err
	}
	return teams, nil
}

type ApplicationFilter struct {
	Name           string
	Team           string
	Status         string
	Language       string
	OrganizationID uint
	ProjectID      uint
}

// ApplicationRepoBindingRepository 管理应用与标准 Git 仓库绑定关系。
type ApplicationRepoBindingRepository struct {
	db *gorm.DB
}

func NewApplicationRepoBindingRepository(db *gorm.DB) *ApplicationRepoBindingRepository {
	return &ApplicationRepoBindingRepository{db: db}
}

func (r *ApplicationRepoBindingRepository) List(ctx context.Context, appID uint) ([]models.ApplicationRepoBinding, error) {
	var list []models.ApplicationRepoBinding
	err := r.db.WithContext(ctx).
		Model(&models.ApplicationRepoBinding{}).
		Select("application_repo_bindings.*, git_repositories.name AS repo_name, git_repositories.url AS repo_url, git_repositories.provider AS repo_provider, git_repositories.default_branch AS default_branch").
		Joins("LEFT JOIN git_repositories ON git_repositories.id = application_repo_bindings.git_repo_id").
		Where("application_repo_bindings.application_id = ?", appID).
		Order("application_repo_bindings.is_default DESC, application_repo_bindings.id ASC").
		Scan(&list).Error
	return list, err
}

func (r *ApplicationRepoBindingRepository) GetDefault(ctx context.Context, appID uint) (*models.ApplicationRepoBinding, error) {
	list, err := r.List(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].IsDefault {
			return &list[i], nil
		}
	}
	if len(list) > 0 {
		return &list[0], nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *ApplicationRepoBindingRepository) Bind(ctx context.Context, binding *models.ApplicationRepoBinding) error {
	if binding == nil {
		return nil
	}
	if strings.TrimSpace(binding.Role) == "" {
		binding.Role = "primary"
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.ApplicationRepoBinding{}).Where("application_id = ?", binding.ApplicationID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			binding.IsDefault = true
		}
		if binding.IsDefault {
			if err := tx.Model(&models.ApplicationRepoBinding{}).
				Where("application_id = ?", binding.ApplicationID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Where("application_id = ? AND git_repo_id = ?", binding.ApplicationID, binding.GitRepoID).
			Assign(map[string]any{
				"role":       binding.Role,
				"is_default": binding.IsDefault,
				"created_by": binding.CreatedBy,
			}).
			FirstOrCreate(binding).Error
	})
}

func (r *ApplicationRepoBindingRepository) Delete(ctx context.Context, appID uint, bindingID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding models.ApplicationRepoBinding
		if err := tx.Where("application_id = ? AND id = ?", appID, bindingID).First(&binding).Error; err != nil {
			return err
		}
		if err := tx.Delete(&binding).Error; err != nil {
			return err
		}
		if !binding.IsDefault {
			return nil
		}
		var next models.ApplicationRepoBinding
		if err := tx.Where("application_id = ?", appID).Order("id ASC").First(&next).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		return tx.Model(&next).Update("is_default", true).Error
	})
}

func (r *ApplicationRepoBindingRepository) SetDefault(ctx context.Context, appID uint, bindingID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding models.ApplicationRepoBinding
		if err := tx.Where("application_id = ? AND id = ?", appID, bindingID).First(&binding).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.ApplicationRepoBinding{}).
			Where("application_id = ?", appID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&binding).Update("is_default", true).Error
	})
}

// ApplicationEnvRepository 应用环境仓库
type ApplicationEnvRepository struct {
	db *gorm.DB
}

func NewApplicationEnvRepository(db *gorm.DB) *ApplicationEnvRepository {
	return &ApplicationEnvRepository{db: db}
}

func (r *ApplicationEnvRepository) Create(ctx context.Context, env *models.ApplicationEnv) error {
	return r.db.WithContext(ctx).Create(env).Error
}

func (r *ApplicationEnvRepository) Update(ctx context.Context, env *models.ApplicationEnv) error {
	return r.db.WithContext(ctx).Save(env).Error
}

func (r *ApplicationEnvRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ApplicationEnv{}, id).Error
}

func (r *ApplicationEnvRepository) GetByID(ctx context.Context, id uint) (*models.ApplicationEnv, error) {
	var env models.ApplicationEnv
	if err := r.db.WithContext(ctx).First(&env, id).Error; err != nil {
		return nil, err
	}
	return &env, nil
}

func (r *ApplicationEnvRepository) GetByAppID(ctx context.Context, appID uint) ([]models.ApplicationEnv, error) {
	var envs []models.ApplicationEnv
	if err := r.db.WithContext(ctx).Where("app_id = ?", appID).Order("FIELD(env_name, 'dev', 'test', 'staging', 'uat', 'gray', 'prod'), env_name").Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

// DeployRecordRepository 交付记录仓库（底层表名沿用 deploy_records）
type DeployRecordRepository struct {
	db *gorm.DB
}

func NewDeployRecordRepository(db *gorm.DB) *DeployRecordRepository {
	return &DeployRecordRepository{db: db}
}

func (r *DeployRecordRepository) Create(ctx context.Context, record *models.DeployRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *DeployRecordRepository) Update(ctx context.Context, record *models.DeployRecord) error {
	return r.db.WithContext(ctx).Save(record).Error
}

func (r *DeployRecordRepository) GetByID(ctx context.Context, id uint) (*models.DeployRecord, error) {
	var record models.DeployRecord
	if err := r.db.WithContext(ctx).First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *DeployRecordRepository) List(ctx context.Context, filter DeployRecordFilter, page, pageSize int) ([]models.DeployRecord, int64, error) {
	var records []models.DeployRecord
	var total int64

	query := r.db.WithContext(ctx).Model(&models.DeployRecord{})

	if filter.ApplicationID > 0 {
		query = query.Where("application_id = ?", filter.ApplicationID)
	}
	if filter.AppName != "" {
		query = query.Where("app_name LIKE ?", "%"+filter.AppName+"%")
	}
	if filter.EnvName != "" {
		query = query.Where("env_name = ?", filter.EnvName)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

type DeployRecordFilter struct {
	ApplicationID uint
	AppName       string
	EnvName       string
	Status        string
	NeedApproval  *bool
	DeployType    string
}

// UpdateStatus 更新记录状态
func (r *DeployRecordRepository) UpdateStatus(ctx context.Context, id uint, status string, updates map[string]interface{}) error {
	updates["status"] = status
	return r.db.WithContext(ctx).Model(&models.DeployRecord{}).Where("id = ?", id).Updates(updates).Error
}

// GetLatestSuccess 获取最近一次成功的交付记录
func (r *DeployRecordRepository) GetLatestSuccess(ctx context.Context, appID uint, envName string) (*models.DeployRecord, error) {
	var record models.DeployRecord
	err := r.db.WithContext(ctx).
		Where("application_id = ? AND env_name = ? AND status = ?", appID, envName, "success").
		Order("created_at DESC").
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetPendingApproval 获取待审批的记录
func (r *DeployRecordRepository) GetPendingApproval(ctx context.Context, appID uint, envName string) ([]models.DeployRecord, error) {
	var records []models.DeployRecord
	err := r.db.WithContext(ctx).
		Where("application_id = ? AND env_name = ? AND status = ? AND need_approval = ?", appID, envName, "pending", true).
		Find(&records).Error
	return records, err
}

// GetStats 获取统计数据
func (r *DeployRecordRepository) GetStats(ctx context.Context, filter DeployStatsFilter) (*DeployStats, error) {
	var stats DeployStats

	query := r.db.WithContext(ctx).Model(&models.DeployRecord{})

	if filter.ApplicationID > 0 {
		query = query.Where("application_id = ?", filter.ApplicationID)
	}
	if filter.EnvName != "" {
		query = query.Where("env_name = ?", filter.EnvName)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where("created_at <= ?", filter.EndTime)
	}

	// 总数
	if err := query.Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	// 成功数
	var successCount int64
	r.db.WithContext(ctx).Model(&models.DeployRecord{}).
		Where("status = ?", "success").
		Count(&successCount)
	stats.Success = successCount

	// 失败数
	var failedCount int64
	r.db.WithContext(ctx).Model(&models.DeployRecord{}).
		Where("status = ?", "failed").
		Count(&failedCount)
	stats.Failed = failedCount

	// 平均耗时
	var avgDuration float64
	r.db.WithContext(ctx).Model(&models.DeployRecord{}).
		Where("status = ? AND duration > 0", "success").
		Select("COALESCE(AVG(duration), 0)").Scan(&avgDuration)
	stats.AvgDuration = int(avgDuration)

	// 成功率
	if stats.Total > 0 {
		stats.SuccessRate = float64(stats.Success) / float64(stats.Total) * 100
	}

	return &stats, nil
}

type DeployStatsFilter struct {
	ApplicationID uint
	EnvName       string
	StartTime     time.Time
	EndTime       time.Time
}

type DeployStats struct {
	Total       int64   `json:"total"`
	Success     int64   `json:"success"`
	Failed      int64   `json:"failed"`
	SuccessRate float64 `json:"success_rate"`
	AvgDuration int     `json:"avg_duration"`
}
