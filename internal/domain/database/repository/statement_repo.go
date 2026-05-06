package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
)

type SQLChangeStatementRepository struct {
	db *gorm.DB
}

func NewSQLChangeStatementRepository(db *gorm.DB) *SQLChangeStatementRepository {
	return &SQLChangeStatementRepository{db: db}
}

func (r *SQLChangeStatementRepository) BulkCreate(ctx context.Context, list []model.SQLChangeStatement) error {
	if len(list) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(list, 100).Error
}

func (r *SQLChangeStatementRepository) ListByTicket(ctx context.Context, ticketID uint) ([]model.SQLChangeStatement, error) {
	var list []model.SQLChangeStatement
	err := r.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Order("seq ASC").Find(&list).Error
	return list, err
}

func (r *SQLChangeStatementRepository) UpdateFields(ctx context.Context, id uint, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.SQLChangeStatement{}).Where("id = ?", id).Updates(fields).Error
}

type StatementFilter struct {
	TicketID   uint
	WorkID     string
	InstanceID uint
	State      string
	Applicant  string
}

type StatementListItem struct {
	model.SQLChangeStatement
	TicketTitle    string `json:"ticket_title"`
	TicketWorkID   string `json:"ticket_work_id"`
	Applicant      string `json:"applicant"`
	InstanceID     uint   `json:"instance_id"`
	SchemaName     string `json:"schema_name"`
	TicketStatus   int    `json:"ticket_status"`
}

func (r *SQLChangeStatementRepository) List(ctx context.Context, f StatementFilter, page, pageSize int) ([]StatementListItem, int64, error) {
	var list []StatementListItem
	var total int64

	q := r.db.WithContext(ctx).Table("sql_change_statements s").
		Select("s.*, t.title AS ticket_title, t.work_id AS ticket_work_id, t.applicant, t.instance_id, t.schema_name, t.status AS ticket_status").
		Joins("JOIN sql_change_tickets t ON t.id = s.ticket_id")

	if f.TicketID > 0 {
		q = q.Where("s.ticket_id = ?", f.TicketID)
	}
	if f.WorkID != "" {
		q = q.Where("t.work_id LIKE ?", "%"+f.WorkID+"%")
	}
	if f.InstanceID > 0 {
		q = q.Where("t.instance_id = ?", f.InstanceID)
	}
	if f.State != "" {
		q = q.Where("s.state = ?", f.State)
	}
	if f.Applicant != "" {
		q = q.Where("t.applicant = ?", f.Applicant)
	}

	countQ := r.db.WithContext(ctx).Table("sql_change_statements s").
		Joins("JOIN sql_change_tickets t ON t.id = s.ticket_id")
	if f.TicketID > 0 {
		countQ = countQ.Where("s.ticket_id = ?", f.TicketID)
	}
	if f.WorkID != "" {
		countQ = countQ.Where("t.work_id LIKE ?", "%"+f.WorkID+"%")
	}
	if f.InstanceID > 0 {
		countQ = countQ.Where("t.instance_id = ?", f.InstanceID)
	}
	if f.State != "" {
		countQ = countQ.Where("s.state = ?", f.State)
	}
	if f.Applicant != "" {
		countQ = countQ.Where("t.applicant = ?", f.Applicant)
	}
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := q.Order("s.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
