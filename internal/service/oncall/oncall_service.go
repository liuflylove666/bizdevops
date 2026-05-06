package oncall

import (
	"devops/internal/models"
	"devops/internal/repository"
	"fmt"
	"time"
)

type Service struct {
	scheduleRepo   *repository.OncallScheduleRepository
	shiftRepo      *repository.OncallShiftRepository
	overrideRepo   *repository.OncallOverrideRepository
	assignmentRepo *repository.AlertAssignmentRepository
}

func NewService(
	scheduleRepo *repository.OncallScheduleRepository,
	shiftRepo *repository.OncallShiftRepository,
	overrideRepo *repository.OncallOverrideRepository,
	assignmentRepo *repository.AlertAssignmentRepository,
) *Service {
	return &Service{
		scheduleRepo:   scheduleRepo,
		shiftRepo:      shiftRepo,
		overrideRepo:   overrideRepo,
		assignmentRepo: assignmentRepo,
	}
}

// --- Schedule CRUD ---

func (s *Service) ListSchedules() ([]models.OncallSchedule, error) {
	return s.scheduleRepo.List()
}

func (s *Service) GetSchedule(id uint) (*models.OncallSchedule, error) {
	return s.scheduleRepo.GetByID(id)
}

func (s *Service) CreateSchedule(sch *models.OncallSchedule) error {
	return s.scheduleRepo.Create(sch)
}

func (s *Service) UpdateSchedule(sch *models.OncallSchedule) error {
	return s.scheduleRepo.Update(sch)
}

func (s *Service) DeleteSchedule(id uint) error {
	return s.scheduleRepo.Delete(id)
}

// --- Shift CRUD ---

func (s *Service) ListShifts(scheduleID uint, start, end time.Time) ([]models.OncallShift, error) {
	return s.shiftRepo.ListBySchedule(scheduleID, start, end)
}

func (s *Service) CreateShift(shift *models.OncallShift) error {
	return s.shiftRepo.Create(shift)
}

func (s *Service) DeleteShift(id uint) error {
	return s.shiftRepo.Delete(id)
}

func (s *Service) BatchCreateShifts(shifts []models.OncallShift) error {
	return s.shiftRepo.BatchCreate(shifts)
}

// GenerateWeeklyShifts 自动生成周轮排班
func (s *Service) GenerateWeeklyShifts(scheduleID uint, userIDs []uint, userNames []string, startDate time.Time, weeks int) error {
	if len(userIDs) == 0 || len(userIDs) != len(userNames) {
		return fmt.Errorf("用户列表不能为空")
	}

	var shifts []models.OncallShift
	for w := 0; w < weeks; w++ {
		idx := w % len(userIDs)
		weekStart := startDate.AddDate(0, 0, w*7)
		weekEnd := weekStart.AddDate(0, 0, 7)
		shifts = append(shifts, models.OncallShift{
			ScheduleID: scheduleID,
			UserID:     userIDs[idx],
			UserName:   userNames[idx],
			StartTime:  weekStart,
			EndTime:    weekEnd,
			ShiftType:  "primary",
		})
	}
	return s.shiftRepo.BatchCreate(shifts)
}

// GenerateDailyShifts 自动生成日轮排班
func (s *Service) GenerateDailyShifts(scheduleID uint, userIDs []uint, userNames []string, startDate time.Time, days int) error {
	if len(userIDs) == 0 || len(userIDs) != len(userNames) {
		return fmt.Errorf("用户列表不能为空")
	}

	var shifts []models.OncallShift
	for d := 0; d < days; d++ {
		idx := d % len(userIDs)
		dayStart := startDate.AddDate(0, 0, d)
		dayEnd := dayStart.AddDate(0, 0, 1)
		shifts = append(shifts, models.OncallShift{
			ScheduleID: scheduleID,
			UserID:     userIDs[idx],
			UserName:   userNames[idx],
			StartTime:  dayStart,
			EndTime:    dayEnd,
			ShiftType:  "primary",
		})
	}
	return s.shiftRepo.BatchCreate(shifts)
}

// --- Override ---

func (s *Service) ListOverrides(scheduleID uint) ([]models.OncallOverride, error) {
	return s.overrideRepo.ListBySchedule(scheduleID)
}

func (s *Service) CreateOverride(o *models.OncallOverride) error {
	return s.overrideRepo.Create(o)
}

func (s *Service) DeleteOverride(id uint) error {
	return s.overrideRepo.Delete(id)
}

// --- Current OnCall ---

// GetCurrentOnCall 获取当前值班人（考虑临时替换）
func (s *Service) GetCurrentOnCall(scheduleID uint) ([]models.OncallShift, error) {
	now := time.Now()
	shifts, err := s.shiftRepo.GetCurrentOnCall(scheduleID, now)
	if err != nil {
		return nil, err
	}

	// 检查临时替换
	for i, shift := range shifts {
		override, err := s.overrideRepo.GetActiveOverride(scheduleID, shift.UserID, now)
		if err == nil && override != nil {
			shifts[i].UserID = override.OverrideUserID
			shifts[i].UserName = override.OverrideUserName
		}
	}
	return shifts, nil
}

// --- Alert Assignment ---

func (s *Service) AssignAlert(alertID, scheduleID uint) (*models.AlertAssignment, error) {
	shifts, err := s.GetCurrentOnCall(scheduleID)
	if err != nil || len(shifts) == 0 {
		return nil, fmt.Errorf("当前无值班人")
	}

	primary := shifts[0]
	assignment := &models.AlertAssignment{
		AlertHistoryID: alertID,
		AssigneeID:     primary.UserID,
		AssigneeName:   primary.UserName,
		ScheduleID:     &scheduleID,
		Status:         "pending",
	}
	if err := s.assignmentRepo.Create(assignment); err != nil {
		return nil, err
	}
	return assignment, nil
}

func (s *Service) ClaimAlert(alertID, userID uint, userName string) error {
	a, err := s.assignmentRepo.GetByAlertID(alertID)
	if err != nil {
		// 无分配记录则创建
		a = &models.AlertAssignment{
			AlertHistoryID: alertID,
			AssigneeID:     userID,
			AssigneeName:   userName,
			Status:         "claimed",
		}
		now := time.Now()
		a.ClaimedAt = &now
		return s.assignmentRepo.Create(a)
	}
	now := time.Now()
	a.AssigneeID = userID
	a.AssigneeName = userName
	a.Status = "claimed"
	a.ClaimedAt = &now
	return s.assignmentRepo.Update(a)
}

func (s *Service) ResolveAlert(alertID, userID uint, comment string) error {
	a, err := s.assignmentRepo.GetByAlertID(alertID)
	if err != nil {
		return fmt.Errorf("告警分配不存在")
	}
	now := time.Now()
	a.Status = "resolved"
	a.ResolvedAt = &now
	a.Comment = comment
	return s.assignmentRepo.Update(a)
}

func (s *Service) GetAlertAssignment(alertID uint) (*models.AlertAssignment, error) {
	return s.assignmentRepo.GetByAlertID(alertID)
}

func (s *Service) ListMyAssignments(userID uint, status string) ([]models.AlertAssignment, error) {
	return s.assignmentRepo.ListByAssignee(userID, status)
}
