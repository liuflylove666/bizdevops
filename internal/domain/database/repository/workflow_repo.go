package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
)

type SQLChangeWorkflowRepository struct {
	db *gorm.DB
}

func NewSQLChangeWorkflowRepository(db *gorm.DB) *SQLChangeWorkflowRepository {
	return &SQLChangeWorkflowRepository{db: db}
}

func (r *SQLChangeWorkflowRepository) Create(ctx context.Context, m *model.SQLChangeWorkflowDetail) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *SQLChangeWorkflowRepository) ListByTicket(ctx context.Context, ticketID uint) ([]model.SQLChangeWorkflowDetail, error) {
	var list []model.SQLChangeWorkflowDetail
	err := r.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Order("id ASC").Find(&list).Error
	return list, err
}
