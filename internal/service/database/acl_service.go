package database

import (
	"context"
	"fmt"
	"strings"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
	"devops/internal/models/system"
)

type ACLService struct {
	repo  *dbrepo.DBInstanceACLRepository
	instR *dbrepo.DBInstanceRepository
}

func NewACLService(repo *dbrepo.DBInstanceACLRepository, instR *dbrepo.DBInstanceRepository) *ACLService {
	return &ACLService{repo: repo, instR: instR}
}

type ACLBindInput struct {
	SubjectType string `json:"subject_type" binding:"required"`
	SubjectID   uint   `json:"subject_id" binding:"required"`
	AccessLevel string `json:"access_level" binding:"required"`
	SchemaNames string `json:"schema_names"`
}

func (s *ACLService) Bind(ctx context.Context, instanceID uint, in *ACLBindInput, createdBy uint) (*model.DBInstanceACL, error) {
	if in.SubjectType != model.ACLSubjectUser && in.SubjectType != model.ACLSubjectRole {
		return nil, fmt.Errorf("subject_type 只能是 user 或 role")
	}
	if model.ACLLevelRank(in.AccessLevel) == 0 {
		return nil, fmt.Errorf("access_level 只能是 read/write/owner")
	}
	schemas := normalizeSchemaNames(in.SchemaNames)
	m := &model.DBInstanceACL{
		InstanceID:  instanceID,
		SubjectType: in.SubjectType,
		SubjectID:   in.SubjectID,
		AccessLevel: in.AccessLevel,
		SchemaNames: schemas,
		CreatedBy:   &createdBy,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *ACLService) Unbind(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *ACLService) ListByInstance(ctx context.Context, instanceID uint) ([]model.DBInstanceACL, error) {
	return s.repo.ListByInstance(ctx, instanceID)
}

func (s *ACLService) CanAccess(ctx context.Context, userID uint, role string, roleIDs []uint, instanceID uint, minLevel string) bool {
	if role == system.RoleSuperAdmin || role == system.RoleAdmin {
		return true
	}
	inst, err := s.instR.GetByID(ctx, instanceID)
	if err != nil {
		return false
	}
	if inst.CreatedBy != nil && *inst.CreatedBy == userID {
		return true
	}
	ok, _ := s.repo.HasAccess(ctx, userID, roleIDs, instanceID, minLevel)
	return ok
}

func (s *ACLService) CanAccessSchema(ctx context.Context, userID uint, role string, roleIDs []uint, instanceID uint, schema string, minLevel string) bool {
	if role == system.RoleSuperAdmin || role == system.RoleAdmin {
		return true
	}
	inst, err := s.instR.GetByID(ctx, instanceID)
	if err != nil {
		return false
	}
	if inst.CreatedBy != nil && *inst.CreatedBy == userID {
		return true
	}
	ok, _ := s.repo.HasSchemaAccess(ctx, userID, roleIDs, instanceID, schema, minLevel)
	return ok
}

func (s *ACLService) AccessibleInstanceIDs(ctx context.Context, userID uint, role string, roleIDs []uint) ([]uint, bool) {
	if role == system.RoleSuperAdmin || role == system.RoleAdmin {
		return nil, true
	}
	ids, err := s.repo.AccessibleInstanceIDs(ctx, userID, roleIDs)
	if err != nil {
		return nil, false
	}
	createdIDs, _ := s.instR.ListIDsByCreator(ctx, userID)
	merged := mergeUniqueUints(ids, createdIDs)
	return merged, false
}

func (s *ACLService) AccessibleSchemas(ctx context.Context, userID uint, role string, roleIDs []uint, instanceID uint) ([]string, bool) {
	if role == system.RoleSuperAdmin || role == system.RoleAdmin {
		return nil, true
	}
	inst, err := s.instR.GetByID(ctx, instanceID)
	if err != nil {
		return nil, false
	}
	if inst.CreatedBy != nil && *inst.CreatedBy == userID {
		return nil, true
	}
	schemas, isAll, err := s.repo.AccessibleSchemas(ctx, userID, roleIDs, instanceID)
	if err != nil {
		return nil, false
	}
	return schemas, isAll
}

func normalizeSchemaNames(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, ",")
}

func mergeUniqueUints(a, b []uint) []uint {
	m := make(map[uint]struct{}, len(a)+len(b))
	for _, v := range a {
		m[v] = struct{}{}
	}
	for _, v := range b {
		m[v] = struct{}{}
	}
	result := make([]uint, 0, len(m))
	for v := range m {
		result = append(result, v)
	}
	return result
}
