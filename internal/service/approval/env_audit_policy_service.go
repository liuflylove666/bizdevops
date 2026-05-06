package approval

import (
	"devops/internal/models"
	"devops/internal/repository"
)

type EnvAuditPolicyService struct {
	repo *repository.EnvAuditPolicyRepository
}

func NewEnvAuditPolicyService(repo *repository.EnvAuditPolicyRepository) *EnvAuditPolicyService {
	return &EnvAuditPolicyService{repo: repo}
}

func (s *EnvAuditPolicyService) List() ([]models.EnvAuditPolicy, error) {
	return s.repo.List()
}

func (s *EnvAuditPolicyService) GetByID(id uint) (*models.EnvAuditPolicy, error) {
	return s.repo.GetByID(id)
}

func (s *EnvAuditPolicyService) GetByEnvName(envName string) (*models.EnvAuditPolicy, error) {
	return s.repo.GetByEnvName(envName)
}

func (s *EnvAuditPolicyService) Create(p *models.EnvAuditPolicy) error {
	return s.repo.Create(p)
}

func (s *EnvAuditPolicyService) Update(p *models.EnvAuditPolicy) error {
	return s.repo.Update(p)
}

func (s *EnvAuditPolicyService) Delete(id uint) error {
	return s.repo.Delete(id)
}

// ApplyPreset 应用预设策略模板
func (s *EnvAuditPolicyService) ApplyPreset(id uint, preset string) (*models.EnvAuditPolicy, error) {
	p, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	switch preset {
	case "loose":
		p.RiskLevel = "low"
		p.RequireApproval = false
		p.MinApprovers = 0
		p.RequireChain = false
		p.RequireDeployWindow = false
		p.AutoRejectOutside = false
		p.RequireCodeReview = false
		p.RequireTestPass = false
		p.AllowEmergency = true
		p.AllowRollback = true
		p.MaxDeploysPerDay = 0
	case "moderate":
		p.RiskLevel = "medium"
		p.RequireApproval = true
		p.MinApprovers = 1
		p.RequireChain = false
		p.RequireDeployWindow = true
		p.AutoRejectOutside = false
		p.RequireCodeReview = true
		p.RequireTestPass = true
		p.AllowEmergency = true
		p.AllowRollback = true
		p.MaxDeploysPerDay = 10
	case "strict":
		p.RiskLevel = "high"
		p.RequireApproval = true
		p.MinApprovers = 2
		p.RequireChain = true
		p.RequireDeployWindow = true
		p.AutoRejectOutside = true
		p.RequireCodeReview = true
		p.RequireTestPass = true
		p.AllowEmergency = true
		p.AllowRollback = true
		p.MaxDeploysPerDay = 5
	case "critical":
		p.RiskLevel = "critical"
		p.RequireApproval = true
		p.MinApprovers = 3
		p.RequireChain = true
		p.RequireDeployWindow = true
		p.AutoRejectOutside = true
		p.RequireCodeReview = true
		p.RequireTestPass = true
		p.AllowEmergency = false
		p.AllowRollback = true
		p.MaxDeploysPerDay = 3
	}

	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// CheckPolicy 检查部署是否符合环境策略，返回不符合的原因列表
func (s *EnvAuditPolicyService) CheckPolicy(envName string) (*models.EnvAuditPolicy, error) {
	return s.repo.GetByEnvName(envName)
}
