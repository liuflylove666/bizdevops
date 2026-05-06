// Package repository
//
// incident_repo.go: 生产事故仓库（v2.1）。
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"devops/internal/models/monitoring"
)

// IncidentRepository 事故仓库。
type IncidentRepository struct{ db *gorm.DB }

// NewIncidentRepository 构造仓库。
func NewIncidentRepository(db *gorm.DB) *IncidentRepository {
	return &IncidentRepository{db: db}
}

// IncidentFilter 列表筛选。
type IncidentFilter struct {
	Env       string
	Status    string
	Severity  string
	AppID     *uint
	ProjectID *uint
	ReleaseID *uint
	Keyword   string
	From      *time.Time
	To        *time.Time
}

// List 分页列表。
func (r *IncidentRepository) List(f IncidentFilter, page, pageSize int) ([]monitoring.Incident, int64, error) {
	var list []monitoring.Incident
	var total int64
	q := r.db.Model(&monitoring.Incident{})
	if f.Env != "" {
		q = q.Where("env = ?", f.Env)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Severity != "" {
		q = q.Where("severity = ?", f.Severity)
	}
	if f.AppID != nil {
		q = q.Where("application_id = ?", *f.AppID)
	}
	if f.ProjectID != nil {
		q = q.Joins("JOIN applications ON applications.id = incidents.application_id").
			Where("applications.project_id = ?", *f.ProjectID)
	}
	if f.ReleaseID != nil {
		q = q.Where("release_id = ?", *f.ReleaseID)
	}
	if f.Keyword != "" {
		q = q.Where("title LIKE ? OR description LIKE ?", "%"+f.Keyword+"%", "%"+f.Keyword+"%")
	}
	if f.From != nil {
		q = q.Where("detected_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("detected_at <= ?", *f.To)
	}
	q.Count(&total)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	err := q.Order("detected_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// GetByID 主键查询。
func (r *IncidentRepository) GetByID(id uint) (*monitoring.Incident, error) {
	var inc monitoring.Incident
	return &inc, r.db.First(&inc, id).Error
}

// Create 创建。
func (r *IncidentRepository) Create(inc *monitoring.Incident) error {
	return r.db.Create(inc).Error
}

// Update 更新（全字段保存）。
func (r *IncidentRepository) Update(inc *monitoring.Incident) error {
	return r.db.Save(inc).Error
}

// Delete 软删除占位 — 当前直删；如改为软删除可加 DeletedAt 字段。
func (r *IncidentRepository) Delete(id uint) error {
	return r.db.Delete(&monitoring.Incident{}, id).Error
}

// ListResolvedInWindow 拉取在指定窗口内 detected 且已 resolved 的事故，
// 用于 DORA MTTR 计算。
//
// 选择 detected_at（而非 resolved_at）作为窗口判断字段，
// 与 Release 创建时间口径一致，便于环比对齐。
func (r *IncidentRepository) ListResolvedInWindow(ctx context.Context, env string, from, to time.Time) ([]monitoring.Incident, error) {
	var list []monitoring.Incident
	q := r.db.WithContext(ctx).
		Where("status = ?", monitoring.IncidentStatusResolved).
		Where("resolved_at IS NOT NULL").
		Where("detected_at >= ? AND detected_at <= ?", from, to)
	if env != "" {
		q = q.Where("env = ?", env)
	}
	return list, q.Order("detected_at ASC").Find(&list).Error
}

// GetByFingerprint 按告警指纹 + env 查找最近一次 open/mitigated 事故。
//
// 用于 Alert→Incident 联动：同指纹 firing 需要"合并而非新建"，否则会造成
// 同一告警在不同时间抖动产生多条 incident，污染 MTTR 统计。
// 仅返回尚未 resolved 的；已解决的再次 firing 应视为新事故。
func (r *IncidentRepository) GetByFingerprint(fingerprint, env string) (*monitoring.Incident, error) {
	if fingerprint == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var inc monitoring.Incident
	q := r.db.Where("alert_fingerprint = ?", fingerprint).
		Where("status <> ?", monitoring.IncidentStatusResolved).
		Order("detected_at DESC")
	if env != "" {
		q = q.Where("env = ?", env)
	}
	return &inc, q.First(&inc).Error
}

// CountOpenByApp 用于面板：每个应用当前未关闭事故数。
func (r *IncidentRepository) CountOpenByApp(appID uint) (int64, error) {
	var n int64
	err := r.db.Model(&monitoring.Incident{}).
		Where("application_id = ?", appID).
		Where("status <> ?", monitoring.IncidentStatusResolved).
		Count(&n).Error
	return n, err
}
