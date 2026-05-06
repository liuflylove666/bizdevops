package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
)

type SQLChangeTicketRepository struct {
	db *gorm.DB
}

func NewSQLChangeTicketRepository(db *gorm.DB) *SQLChangeTicketRepository {
	return &SQLChangeTicketRepository{db: db}
}

type TicketFilter struct {
	Applicant  string
	Assignee   string // 处理人
	InstanceID uint
	Status     *int
	ChangeType *int
	Keyword    string // 标题/work_id 模糊
}

func (r *SQLChangeTicketRepository) Create(ctx context.Context, m *model.SQLChangeTicket) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *SQLChangeTicketRepository) GetByID(ctx context.Context, id uint) (*model.SQLChangeTicket, error) {
	var m model.SQLChangeTicket
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *SQLChangeTicketRepository) GetByWorkID(ctx context.Context, workID string) (*model.SQLChangeTicket, error) {
	var m model.SQLChangeTicket
	if err := r.db.WithContext(ctx).Where("work_id = ?", workID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *SQLChangeTicketRepository) GetByApprovalInstanceID(ctx context.Context, approvalInstanceID uint) (*model.SQLChangeTicket, error) {
	var m model.SQLChangeTicket
	if err := r.db.WithContext(ctx).Where("approval_instance_id = ?", approvalInstanceID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *SQLChangeTicketRepository) Update(ctx context.Context, m *model.SQLChangeTicket) error {
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *SQLChangeTicketRepository) UpdateFields(ctx context.Context, id uint, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.SQLChangeTicket{}).Where("id = ?", id).Updates(fields).Error
}

func (r *SQLChangeTicketRepository) List(ctx context.Context, f TicketFilter, page, pageSize int) ([]model.SQLChangeTicket, int64, error) {
	var list []model.SQLChangeTicket
	var total int64

	q := r.db.WithContext(ctx).Model(&model.SQLChangeTicket{})
	if f.Applicant != "" {
		q = q.Where("applicant = ?", f.Applicant)
	}
	if f.Assignee != "" {
		q = q.Where("assigned LIKE ?", "%"+f.Assignee+"%")
	}
	if f.InstanceID > 0 {
		q = q.Where("instance_id = ?", f.InstanceID)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.ChangeType != nil {
		q = q.Where("change_type = ?", *f.ChangeType)
	}
	if f.Keyword != "" {
		q = q.Where("title LIKE ? OR work_id LIKE ?", "%"+f.Keyword+"%", "%"+f.Keyword+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
