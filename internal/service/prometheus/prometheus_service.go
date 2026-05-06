package prometheus

import (
	"fmt"
	"strings"

	"devops/internal/models/monitoring"
	monitoringRepo "devops/internal/modules/monitoring/repository"
	"devops/pkg/logger"
)

type Service struct {
	repo *monitoringRepo.PrometheusInstanceRepository
}

func NewService(repo *monitoringRepo.PrometheusInstanceRepository) *Service {
	return &Service{repo: repo}
}

// --- Instance CRUD ---

func (s *Service) ListInstances() ([]monitoring.PrometheusInstance, error) {
	list, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Password = maskPassword(list[i].Password)
	}
	return list, nil
}

func (s *Service) GetInstance(id uint) (*monitoring.PrometheusInstance, error) {
	inst, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	inst.Password = ""
	return inst, nil
}

func (s *Service) CreateInstance(inst *monitoring.PrometheusInstance) error {
	if inst.Password != "" {
		enc, err := encryptPassword(inst.Password)
		if err != nil {
			return fmt.Errorf("加密凭证失败: %w", err)
		}
		inst.Password = enc
	}
	return s.repo.Create(inst)
}

func (s *Service) UpdateInstance(inst *monitoring.PrometheusInstance) error {
	if inst.Password == "" {
		old, err := s.repo.GetByID(inst.ID)
		if err == nil {
			inst.Password = old.Password
		}
	} else {
		enc, err := encryptPassword(inst.Password)
		if err != nil {
			return fmt.Errorf("加密凭证失败: %w", err)
		}
		inst.Password = enc
	}
	return s.repo.Update(inst)
}

func (s *Service) DeleteInstance(id uint) error {
	return s.repo.Delete(id)
}

func (s *Service) TestConnection(id uint) error {
	client, err := s.getClient(id)
	if err != nil {
		return err
	}
	return client.TestConnection()
}

// --- Proxy Query ---

func (s *Service) Query(instanceID uint, query, ts string) (interface{}, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, err
	}
	resp, err := client.Query(query, ts)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (s *Service) QueryRange(instanceID uint, query, start, end, step string) (interface{}, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, err
	}
	resp, err := client.QueryRange(query, start, end, step)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (s *Service) Labels(instanceID uint) (interface{}, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, err
	}
	resp, err := client.Labels()
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (s *Service) LabelValues(instanceID uint, name string) (interface{}, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, err
	}
	resp, err := client.LabelValues(name)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (s *Service) Series(instanceID uint, matchers []string, start, end string) (interface{}, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, err
	}
	resp, err := client.Series(matchers, start, end)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (s *Service) Targets(instanceID uint) (interface{}, error) {
	client, err := s.getClient(instanceID)
	if err != nil {
		return nil, err
	}
	resp, err := client.Targets()
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// --- Helpers ---

func (s *Service) getClient(instanceID uint) (*PromClient, error) {
	var inst *monitoring.PrometheusInstance
	var err error

	if instanceID == 0 {
		inst, err = s.repo.GetDefault()
	} else {
		inst, err = s.repo.GetByID(instanceID)
	}
	if err != nil {
		return nil, fmt.Errorf("实例不存在: %w", err)
	}
	pwd := s.resolvePasswordWithMigration(inst)
	return NewPromClient(inst.URL, inst.AuthType, inst.Username, pwd), nil
}

func (s *Service) resolvePasswordWithMigration(inst *monitoring.PrometheusInstance) string {
	password, err := decryptPassword(inst.Password)
	if err == nil {
		return password
	}

	// 兼容历史明文，并尝试在线迁移为密文。
	legacyPlainPassword := inst.Password
	if legacyPlainPassword == "" {
		return ""
	}
	enc, encErr := encryptPassword(legacyPlainPassword)
	if encErr == nil {
		inst.Password = enc
		if updateErr := s.repo.Update(inst); updateErr != nil {
			logger.L().WithField("instance_id", inst.ID).WithField("error", updateErr).Warn("Prometheus credential migration writeback failed")
		}
	} else {
		logger.L().WithField("instance_id", inst.ID).WithField("error", encErr).Warn("Prometheus credential migration encryption failed")
	}
	return legacyPlainPassword
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) > 8 {
		return token[:4] + strings.Repeat("*", 8) + token[len(token)-4:]
	}
	return "****"
}
