package sonarqube

import (
	"devops/internal/models/infrastructure"
	infraRepo "devops/internal/modules/infrastructure/repository"
	"devops/pkg/logger"
	"fmt"
	"strings"
)

type Service struct {
	instanceRepo *infraRepo.SonarQubeInstanceRepository
	bindingRepo  *infraRepo.SonarQubeBindingRepository
}

func NewService(
	instanceRepo *infraRepo.SonarQubeInstanceRepository,
	bindingRepo *infraRepo.SonarQubeBindingRepository,
) *Service {
	return &Service{instanceRepo: instanceRepo, bindingRepo: bindingRepo}
}

// --- Instance CRUD ---

func (s *Service) ListInstances() ([]infrastructure.SonarQubeInstance, error) {
	list, err := s.instanceRepo.List()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Token = maskToken(list[i].Token)
	}
	return list, nil
}

func (s *Service) GetInstance(id uint) (*infrastructure.SonarQubeInstance, error) {
	inst, err := s.instanceRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	inst.Token = maskToken(inst.Token)
	return inst, nil
}

func (s *Service) CreateInstance(inst *infrastructure.SonarQubeInstance) error {
	if inst.Token != "" {
		enc, err := encryptToken(inst.Token)
		if err != nil {
			return fmt.Errorf("token 加密失败: %w", err)
		}
		inst.Token = enc
	}
	return s.instanceRepo.Create(inst)
}

func (s *Service) UpdateInstance(inst *infrastructure.SonarQubeInstance) error {
	old, err := s.instanceRepo.GetByID(inst.ID)
	if err != nil {
		return err
	}
	if inst.Token == "" || inst.Token == maskToken(old.Token) {
		inst.Token = old.Token
	} else {
		enc, err := encryptToken(inst.Token)
		if err != nil {
			return fmt.Errorf("token 加密失败: %w", err)
		}
		inst.Token = enc
	}
	return s.instanceRepo.Update(inst)
}

func (s *Service) DeleteInstance(id uint) error {
	return s.instanceRepo.Delete(id)
}

func (s *Service) TestConnection(id uint) (map[string]interface{}, error) {
	client, err := s.getClient(id)
	if err != nil {
		return nil, err
	}
	return client.TestConnection()
}

// --- Binding CRUD ---

func (s *Service) ListBindings(sonarID uint) ([]infrastructure.SonarQubeProjectBinding, error) {
	return s.bindingRepo.ListByInstance(sonarID)
}

func (s *Service) CreateBinding(b *infrastructure.SonarQubeProjectBinding) error {
	return s.bindingRepo.Create(b)
}

func (s *Service) UpdateBinding(b *infrastructure.SonarQubeProjectBinding) error {
	return s.bindingRepo.Update(b)
}

func (s *Service) DeleteBinding(id uint) error {
	return s.bindingRepo.Delete(id)
}

// --- SonarQube API proxy ---

func (s *Service) ListProjects(instanceID uint, page, pageSize int) ([]SonarProject, int, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, 0, err
	}
	return client.ListProjects(page, pageSize)
}

func (s *Service) GetQualityGate(instanceID uint, projectKey string) (*QualityGate, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, err
	}
	return client.GetQualityGate(projectKey)
}

func (s *Service) GetMeasures(instanceID uint, projectKey string, metrics []string) ([]Measure, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		metrics = []string{
			"bugs", "vulnerabilities", "code_smells", "coverage",
			"duplicated_lines_density", "ncloc", "reliability_rating",
			"security_rating", "sqale_rating", "alert_status",
		}
	}
	return client.GetMeasures(projectKey, metrics)
}

func (s *Service) GetIssues(instanceID uint, projectKey string, page, pageSize int, severities string) ([]Issue, int, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, 0, err
	}
	return client.GetIssues(projectKey, page, pageSize, severities)
}

// --- helpers ---

func (s *Service) getClient(instanceID uint) (*Client, error) {
	inst, err := s.instanceRepo.GetByID(instanceID)
	if err != nil {
		return nil, fmt.Errorf("实例不存在")
	}
	token := s.resolveTokenWithMigration(inst)
	return NewClient(inst.BaseURL, token), nil
}

func (s *Service) resolveTokenWithMigration(inst *infrastructure.SonarQubeInstance) string {
	token, err := decryptToken(inst.Token)
	if err == nil {
		return token
	}

	// 兼容历史明文数据，并尝试在线迁移为密文。
	legacyPlainToken := inst.Token
	if legacyPlainToken == "" {
		return ""
	}
	enc, encErr := encryptToken(legacyPlainToken)
	if encErr == nil {
		inst.Token = enc
		if updateErr := s.instanceRepo.Update(inst); updateErr != nil {
			logger.L().WithField("instance_id", inst.ID).WithField("error", updateErr).Warn("SonarQube token migration writeback failed")
		}
	} else {
		logger.L().WithField("instance_id", inst.ID).WithField("error", encErr).Warn("SonarQube token migration encryption failed")
	}
	return legacyPlainToken
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + "****" + token[len(token)-4:]
}
