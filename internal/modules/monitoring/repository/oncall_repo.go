package repository

import (
	"devops/internal/models"
	"time"

	"gorm.io/gorm"
)

// OncallScheduleRepository 排班表仓库
type OncallScheduleRepository struct{ db *gorm.DB }

func NewOncallScheduleRepository(db *gorm.DB) *OncallScheduleRepository {
	return &OncallScheduleRepository{db: db}
}

func (r *OncallScheduleRepository) List() ([]models.OncallSchedule, error) {
	var list []models.OncallSchedule
	return list, r.db.Order("name ASC").Find(&list).Error
}

func (r *OncallScheduleRepository) GetByID(id uint) (*models.OncallSchedule, error) {
	var s models.OncallSchedule
	return &s, r.db.First(&s, id).Error
}

func (r *OncallScheduleRepository) Create(s *models.OncallSchedule) error { return r.db.Create(s).Error }
func (r *OncallScheduleRepository) Update(s *models.OncallSchedule) error { return r.db.Save(s).Error }
func (r *OncallScheduleRepository) Delete(id uint) error {
	return r.db.Delete(&models.OncallSchedule{}, id).Error
}

// OncallShiftRepository 班次仓库
type OncallShiftRepository struct{ db *gorm.DB }

func NewOncallShiftRepository(db *gorm.DB) *OncallShiftRepository {
	return &OncallShiftRepository{db: db}
}

func (r *OncallShiftRepository) ListBySchedule(scheduleID uint, start, end time.Time) ([]models.OncallShift, error) {
	var list []models.OncallShift
	q := r.db.Where("schedule_id = ?", scheduleID)
	if !start.IsZero() {
		q = q.Where("end_time >= ?", start)
	}
	if !end.IsZero() {
		q = q.Where("start_time <= ?", end)
	}
	return list, q.Order("start_time ASC").Find(&list).Error
}

func (r *OncallShiftRepository) GetCurrentOnCall(scheduleID uint, now time.Time) ([]models.OncallShift, error) {
	var list []models.OncallShift
	return list, r.db.Where("schedule_id = ? AND start_time <= ? AND end_time > ?", scheduleID, now, now).
		Order("shift_type ASC").Find(&list).Error
}

func (r *OncallShiftRepository) Create(s *models.OncallShift) error  { return r.db.Create(s).Error }
func (r *OncallShiftRepository) Delete(id uint) error                { return r.db.Delete(&models.OncallShift{}, id).Error }
func (r *OncallShiftRepository) BatchCreate(shifts []models.OncallShift) error {
	if len(shifts) == 0 {
		return nil
	}
	return r.db.Create(&shifts).Error
}
func (r *OncallShiftRepository) DeleteByScheduleRange(scheduleID uint, start, end time.Time) error {
	return r.db.Where("schedule_id = ? AND start_time >= ? AND end_time <= ?", scheduleID, start, end).
		Delete(&models.OncallShift{}).Error
}

// OncallOverrideRepository 临时替换仓库
type OncallOverrideRepository struct{ db *gorm.DB }

func NewOncallOverrideRepository(db *gorm.DB) *OncallOverrideRepository {
	return &OncallOverrideRepository{db: db}
}

func (r *OncallOverrideRepository) ListBySchedule(scheduleID uint) ([]models.OncallOverride, error) {
	var list []models.OncallOverride
	return list, r.db.Where("schedule_id = ? AND end_time > ?", scheduleID, time.Now()).
		Order("start_time ASC").Find(&list).Error
}

func (r *OncallOverrideRepository) GetActiveOverride(scheduleID, originalUserID uint, now time.Time) (*models.OncallOverride, error) {
	var o models.OncallOverride
	err := r.db.Where("schedule_id = ? AND original_user_id = ? AND start_time <= ? AND end_time > ?",
		scheduleID, originalUserID, now, now).First(&o).Error
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OncallOverrideRepository) Create(o *models.OncallOverride) error { return r.db.Create(o).Error }
func (r *OncallOverrideRepository) Delete(id uint) error {
	return r.db.Delete(&models.OncallOverride{}, id).Error
}

// AlertAssignmentRepository 告警分配仓库
type AlertAssignmentRepository struct{ db *gorm.DB }

func NewAlertAssignmentRepository(db *gorm.DB) *AlertAssignmentRepository {
	return &AlertAssignmentRepository{db: db}
}

func (r *AlertAssignmentRepository) GetByAlertID(alertID uint) (*models.AlertAssignment, error) {
	var a models.AlertAssignment
	return &a, r.db.Where("alert_history_id = ?", alertID).First(&a).Error
}

func (r *AlertAssignmentRepository) ListByAssignee(userID uint, status string) ([]models.AlertAssignment, error) {
	var list []models.AlertAssignment
	q := r.db.Where("assignee_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	return list, q.Order("created_at DESC").Limit(50).Find(&list).Error
}

func (r *AlertAssignmentRepository) Create(a *models.AlertAssignment) error { return r.db.Create(a).Error }
func (r *AlertAssignmentRepository) Update(a *models.AlertAssignment) error { return r.db.Save(a).Error }
