package database

import (
	"context"
	"encoding/json"
	"fmt"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
)

type RuleService struct {
	repo *dbrepo.SQLAuditRuleRepository
}

func NewRuleService(repo *dbrepo.SQLAuditRuleRepository) *RuleService {
	return &RuleService{repo: repo}
}

type RuleInput struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Config      model.AuditRuleConfig `json:"config"`
	IsDefault   bool                  `json:"is_default"`
}

func (s *RuleService) List(ctx context.Context) ([]model.SQLAuditRuleSet, error) {
	return s.repo.List(ctx)
}

func (s *RuleService) Get(ctx context.Context, id uint) (*model.SQLAuditRuleSet, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RuleService) Create(ctx context.Context, in *RuleInput) (*model.SQLAuditRuleSet, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("规则集名称必填")
	}
	cfgJSON, err := json.Marshal(in.Config)
	if err != nil {
		return nil, err
	}
	m := &model.SQLAuditRuleSet{
		Name:        in.Name,
		Description: in.Description,
		Config:      cfgJSON,
		IsDefault:   in.IsDefault,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	if in.IsDefault {
		_ = s.repo.SetDefault(ctx, m.ID)
	}
	return m, nil
}

func (s *RuleService) Update(ctx context.Context, id uint, in *RuleInput) (*model.SQLAuditRuleSet, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		m.Name = in.Name
	}
	m.Description = in.Description
	cfgJSON, err := json.Marshal(in.Config)
	if err != nil {
		return nil, err
	}
	m.Config = cfgJSON
	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	if in.IsDefault {
		_ = s.repo.SetDefault(ctx, m.ID)
	}
	return m, nil
}

func (s *RuleService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *RuleService) SetDefault(ctx context.Context, id uint) error {
	return s.repo.SetDefault(ctx, id)
}

// DefaultConfig 返回前端使用的默认配置模板
func (s *RuleService) DefaultConfig() model.AuditRuleConfig {
	return model.DefaultAuditRuleConfig()
}
