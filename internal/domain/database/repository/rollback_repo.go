package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
)

type SQLRollbackRepository struct {
	db *gorm.DB
}

func NewSQLRollbackRepository(db *gorm.DB) *SQLRollbackRepository {
	return &SQLRollbackRepository{db: db}
}

func (r *SQLRollbackRepository) Create(ctx context.Context, m *model.SQLRollbackScript) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *SQLRollbackRepository) ListByTicket(ctx context.Context, ticketID uint) ([]model.SQLRollbackScript, error) {
	var list []model.SQLRollbackScript
	err := r.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Order("id ASC").Find(&list).Error
	return list, err
}
