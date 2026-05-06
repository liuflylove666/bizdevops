package repository

import (
	modelbiz "devops/internal/models/biz"
	"slices"
	"strings"

	"gorm.io/gorm"
)

type GoalFilter struct {
	Status  string
	Keyword string
}

type RequirementFilter struct {
	Status        string
	Priority      string
	Source        string
	ExternalKey   string
	JiraEpicKey   string
	JiraLabel     string
	JiraComponent string
	GoalID        *uint
	VersionID     *uint
	Keyword       string
}

type VersionFilter struct {
	Status  string
	GoalID  *uint
	Keyword string
}

type BizGoalRepository struct{ db *gorm.DB }
type BizRequirementRepository struct{ db *gorm.DB }
type BizVersionRepository struct{ db *gorm.DB }

func NewBizGoalRepository(db *gorm.DB) *BizGoalRepository { return &BizGoalRepository{db: db} }
func NewBizRequirementRepository(db *gorm.DB) *BizRequirementRepository { return &BizRequirementRepository{db: db} }
func NewBizVersionRepository(db *gorm.DB) *BizVersionRepository { return &BizVersionRepository{db: db} }

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}

func keywordLike(keyword string) string {
	return "%" + strings.TrimSpace(keyword) + "%"
}

func (r *BizGoalRepository) List(filter GoalFilter, page, pageSize int) ([]modelbiz.BizGoal, int64, error) {
	var list []modelbiz.BizGoal
	var total int64
	page, pageSize = normalizePage(page, pageSize)
	q := r.db.Model(&modelbiz.BizGoal{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		like := keywordLike(filter.Keyword)
		q = q.Where("name LIKE ? OR code LIKE ? OR owner LIKE ?", like, like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *BizGoalRepository) GetByID(id uint) (*modelbiz.BizGoal, error) {
	var item modelbiz.BizGoal
	return &item, r.db.First(&item, id).Error
}

func (r *BizGoalRepository) FindByIDs(ids []uint) ([]modelbiz.BizGoal, error) {
	var list []modelbiz.BizGoal
	if len(ids) == 0 {
		return list, nil
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	return list, r.db.Where("id IN ?", ids).Find(&list).Error
}

func (r *BizGoalRepository) Create(item *modelbiz.BizGoal) error { return r.db.Create(item).Error }
func (r *BizGoalRepository) Update(item *modelbiz.BizGoal) error { return r.db.Save(item).Error }
func (r *BizGoalRepository) Delete(id uint) error { return r.db.Delete(&modelbiz.BizGoal{}, id).Error }

func (r *BizRequirementRepository) List(filter RequirementFilter, page, pageSize int) ([]modelbiz.BizRequirement, int64, error) {
	var list []modelbiz.BizRequirement
	var total int64
	page, pageSize = normalizePage(page, pageSize)
	q := r.db.Model(&modelbiz.BizRequirement{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Priority != "" {
		q = q.Where("priority = ?", filter.Priority)
	}
	if filter.Source != "" {
		q = q.Where("source = ?", filter.Source)
	}
	if strings.TrimSpace(filter.ExternalKey) != "" {
		q = q.Where("external_key = ?", strings.TrimSpace(filter.ExternalKey))
	}
	if strings.TrimSpace(filter.JiraEpicKey) != "" {
		q = q.Where("jira_epic_key = ?", strings.TrimSpace(filter.JiraEpicKey))
	}
	if strings.TrimSpace(filter.JiraLabel) != "" {
		q = q.Where("jira_labels LIKE ?", keywordLike(filter.JiraLabel))
	}
	if strings.TrimSpace(filter.JiraComponent) != "" {
		q = q.Where("jira_components LIKE ?", keywordLike(filter.JiraComponent))
	}
	if filter.GoalID != nil {
		q = q.Where("goal_id = ?", *filter.GoalID)
	}
	if filter.VersionID != nil {
		q = q.Where("version_id = ?", *filter.VersionID)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		like := keywordLike(filter.Keyword)
		q = q.Where("title LIKE ? OR owner LIKE ?", like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *BizRequirementRepository) GetByID(id uint) (*modelbiz.BizRequirement, error) {
	var item modelbiz.BizRequirement
	return &item, r.db.First(&item, id).Error
}

func (r *BizRequirementRepository) GetByExternalKey(externalKey string) (*modelbiz.BizRequirement, error) {
	var item modelbiz.BizRequirement
	if err := r.db.Where("external_key = ?", strings.TrimSpace(externalKey)).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *BizRequirementRepository) ListByGoalID(goalID uint) ([]modelbiz.BizRequirement, error) {
	var list []modelbiz.BizRequirement
	return list, r.db.Where("goal_id = ?", goalID).Order("id DESC").Find(&list).Error
}

func (r *BizRequirementRepository) ListByVersionID(versionID uint) ([]modelbiz.BizRequirement, error) {
	var list []modelbiz.BizRequirement
	return list, r.db.Where("version_id = ?", versionID).Order("id DESC").Find(&list).Error
}

func (r *BizRequirementRepository) Create(item *modelbiz.BizRequirement) error {
	return r.db.Create(item).Error
}

func (r *BizRequirementRepository) Update(item *modelbiz.BizRequirement) error {
	return r.db.Save(item).Error
}

func (r *BizRequirementRepository) Delete(id uint) error {
	return r.db.Delete(&modelbiz.BizRequirement{}, id).Error
}

func (r *BizVersionRepository) List(filter VersionFilter, page, pageSize int) ([]modelbiz.BizVersion, int64, error) {
	var list []modelbiz.BizVersion
	var total int64
	page, pageSize = normalizePage(page, pageSize)
	q := r.db.Model(&modelbiz.BizVersion{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.GoalID != nil {
		q = q.Where("goal_id = ?", *filter.GoalID)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		like := keywordLike(filter.Keyword)
		q = q.Where("name LIKE ? OR code LIKE ? OR owner LIKE ?", like, like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *BizVersionRepository) GetByID(id uint) (*modelbiz.BizVersion, error) {
	var item modelbiz.BizVersion
	return &item, r.db.First(&item, id).Error
}

func (r *BizVersionRepository) FindByIDs(ids []uint) ([]modelbiz.BizVersion, error) {
	var list []modelbiz.BizVersion
	if len(ids) == 0 {
		return list, nil
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	return list, r.db.Where("id IN ?", ids).Find(&list).Error
}

func (r *BizVersionRepository) ListByGoalID(goalID uint) ([]modelbiz.BizVersion, error) {
	var list []modelbiz.BizVersion
	return list, r.db.Where("goal_id = ?", goalID).Order("id DESC").Find(&list).Error
}

func (r *BizVersionRepository) Create(item *modelbiz.BizVersion) error { return r.db.Create(item).Error }
func (r *BizVersionRepository) Update(item *modelbiz.BizVersion) error { return r.db.Save(item).Error }
func (r *BizVersionRepository) Delete(id uint) error { return r.db.Delete(&modelbiz.BizVersion{}, id).Error }
