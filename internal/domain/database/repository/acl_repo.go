package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
)

type DBInstanceACLRepository struct {
	db *gorm.DB
}

func NewDBInstanceACLRepository(db *gorm.DB) *DBInstanceACLRepository {
	return &DBInstanceACLRepository{db: db}
}

func (r *DBInstanceACLRepository) Create(ctx context.Context, m *model.DBInstanceACL) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *DBInstanceACLRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.DBInstanceACL{}, id).Error
}

func (r *DBInstanceACLRepository) ListByInstance(ctx context.Context, instanceID uint) ([]model.DBInstanceACL, error) {
	var list []model.DBInstanceACL
	err := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *DBInstanceACLRepository) FindForUser(ctx context.Context, userID uint, roleIDs []uint, instanceID uint) ([]model.DBInstanceACL, error) {
	var acls []model.DBInstanceACL
	q := r.db.WithContext(ctx).Where("instance_id = ?", instanceID)
	conditions := r.db.Where("subject_type = ? AND subject_id = ?", model.ACLSubjectUser, userID)
	if len(roleIDs) > 0 {
		conditions = conditions.Or("subject_type = ? AND subject_id IN ?", model.ACLSubjectRole, roleIDs)
	}
	q = q.Where(conditions)
	if err := q.Find(&acls).Error; err != nil {
		return nil, err
	}
	return acls, nil
}

func (r *DBInstanceACLRepository) HasAccess(ctx context.Context, userID uint, roleIDs []uint, instanceID uint, minLevel string) (bool, error) {
	acls, err := r.FindForUser(ctx, userID, roleIDs, instanceID)
	if err != nil {
		return false, err
	}
	minRank := model.ACLLevelRank(minLevel)
	for _, a := range acls {
		if model.ACLLevelRank(a.AccessLevel) >= minRank {
			return true, nil
		}
	}
	return false, nil
}

func (r *DBInstanceACLRepository) HasSchemaAccess(ctx context.Context, userID uint, roleIDs []uint, instanceID uint, schema string, minLevel string) (bool, error) {
	acls, err := r.FindForUser(ctx, userID, roleIDs, instanceID)
	if err != nil {
		return false, err
	}
	minRank := model.ACLLevelRank(minLevel)
	for _, a := range acls {
		if model.ACLLevelRank(a.AccessLevel) >= minRank && a.HasSchema(schema) {
			return true, nil
		}
	}
	return false, nil
}

func (r *DBInstanceACLRepository) AccessibleInstanceIDs(ctx context.Context, userID uint, roleIDs []uint) ([]uint, error) {
	var ids []uint
	q := r.db.WithContext(ctx).Model(&model.DBInstanceACL{}).Select("DISTINCT instance_id")
	conditions := r.db.Where("subject_type = ? AND subject_id = ?", model.ACLSubjectUser, userID)
	if len(roleIDs) > 0 {
		conditions = conditions.Or("subject_type = ? AND subject_id IN ?", model.ACLSubjectRole, roleIDs)
	}
	q = q.Where(conditions)
	if err := q.Pluck("instance_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *DBInstanceACLRepository) AccessibleSchemas(ctx context.Context, userID uint, roleIDs []uint, instanceID uint) ([]string, bool, error) {
	acls, err := r.FindForUser(ctx, userID, roleIDs, instanceID)
	if err != nil {
		return nil, false, err
	}
	schemaSet := map[string]struct{}{}
	for _, a := range acls {
		if a.AllSchemas() {
			return nil, true, nil
		}
		for _, s := range a.SchemaList() {
			schemaSet[s] = struct{}{}
		}
	}
	out := make([]string, 0, len(schemaSet))
	for s := range schemaSet {
		out = append(out, s)
	}
	return out, false, nil
}

func (r *DBInstanceACLRepository) DeleteByInstance(ctx context.Context, instanceID uint) error {
	return r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Delete(&model.DBInstanceACL{}).Error
}
