package approval

import (
	"context"
	"devops/internal/models"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrApprovalNotFound = errors.New("审批记录不存在")
	ErrAlreadyApproved  = errors.New("该记录已被审批")
	ErrNotApprover      = errors.New("您不是该节点的审批人，无权进行此操作")
	ErrApprovalTimeout  = errors.New("审批已超时")
	ErrRecordNotPending = errors.New("记录状态不是待审批")
)

// DeployTrigger 部署触发器接口
type DeployTrigger interface {
	TriggerDeployAfterApproval(ctx context.Context, recordID uint) error
}

type ApprovalService struct {
	db          *gorm.DB
	ruleService *RuleService
}

func NewApprovalService(db *gorm.DB, ruleService *RuleService) *ApprovalService {
	return &ApprovalService{
		db:          db,
		ruleService: ruleService,
	}
}

// GetHistory 获取审批历史
func (s *ApprovalService) GetHistory(ctx context.Context, page, pageSize int, appID *uint, env string, status string) ([]models.DeployRecord, int64, error) {
	var records []models.DeployRecord
	var total int64

	query := s.db.Model(&models.DeployRecord{}).Where("need_approval = ?", true)

	if appID != nil && *appID > 0 {
		query = query.Where("application_id = ?", *appID)
	}
	if env != "" {
		query = query.Where("env_name = ?", env)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&records).Error
	return records, total, err
}

// GetApprovalRecords 获取某个交付记录的审批记录
func (s *ApprovalService) GetApprovalRecords(ctx context.Context, recordID uint) ([]models.ApprovalRecord, error) {
	var records []models.ApprovalRecord
	err := s.db.Where("record_id = ?", recordID).Order("created_at ASC").Find(&records).Error
	return records, err
}

// GetHistoryForExport 获取审批历史用于导出（不分页）
func (s *ApprovalService) GetHistoryForExport(ctx context.Context, env, status, startTime, endTime string) ([]map[string]interface{}, error) {
	query := s.db.Model(&models.DeployRecord{}).
		Select("deploy_records.*, applications.name as app_name").
		Joins("LEFT JOIN applications ON applications.id = deploy_records.application_id").
		Where("deploy_records.need_approval = ?", true)

	if env != "" {
		query = query.Where("deploy_records.env_name = ?", env)
	}
	if status != "" {
		query = query.Where("deploy_records.status = ?", status)
	}
	if startTime != "" {
		query = query.Where("deploy_records.created_at >= ?", startTime)
	}
	if endTime != "" {
		query = query.Where("deploy_records.created_at <= ?", endTime+" 23:59:59")
	}

	var results []map[string]interface{}
	err := query.Order("deploy_records.created_at DESC").Limit(10000).Find(&results).Error
	return results, err
}

// CheckApprovalRequired 检查是否需要审批
func (s *ApprovalService) CheckApprovalRequired(ctx context.Context, appID uint, env string) (bool, []string, error) {
	needApproval, approverIDs, err := s.ruleService.NeedApproval(appID, env)
	if err != nil {
		return false, nil, err
	}

	var approverNames []string
	if needApproval && len(approverIDs) > 0 {
		var users []models.User
		s.db.Where("id IN ?", approverIDs).Find(&users)
		for _, u := range users {
			approverNames = append(approverNames, u.Username)
		}
	}

	return needApproval, approverNames, nil
}
