package envinstance

import (
	"fmt"

	"devops/internal/models/deploy"
	appRepo "devops/internal/modules/application/repository"
)

type Service struct {
	repo *appRepo.EnvInstanceRepository
}

func NewService(repo *appRepo.EnvInstanceRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(f appRepo.EnvInstanceFilter, page, pageSize int) ([]deploy.EnvInstance, int64, error) {
	return s.repo.List(f, page, pageSize)
}

func (s *Service) GetByID(id uint) (*deploy.EnvInstance, error) {
	return s.repo.GetByID(id)
}

func (s *Service) Create(inst *deploy.EnvInstance) error {
	// 检查是否已存在同应用同环境的实例
	existing, err := s.repo.GetByAppEnv(inst.ApplicationID, inst.Env)
	if err == nil && existing.ID > 0 {
		return fmt.Errorf("应用在 %s 环境已有实例", inst.Env)
	}
	return s.repo.Create(inst)
}

func (s *Service) Update(inst *deploy.EnvInstance) error {
	return s.repo.Update(inst)
}

func (s *Service) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *Service) ListByApp(appID uint) ([]deploy.EnvInstance, error) {
	return s.repo.ListByApp(appID)
}

// EnvMatrix 获取环境矩阵
func (s *Service) EnvMatrix(envs []string) ([]deploy.EnvInstance, error) {
	return s.repo.EnvMatrix(envs)
}

// UpdateDeployInfo 部署后更新实例信息
func (s *Service) UpdateDeployInfo(appID uint, env string, imageURL, imageTag, imageDigest, operator string) error {
	inst, err := s.repo.GetByAppEnv(appID, env)
	if err != nil {
		return fmt.Errorf("环境实例不存在: %w", err)
	}
	inst.ImageURL = imageURL
	inst.ImageTag = imageTag
	inst.ImageDigest = imageDigest
	inst.LastDeployBy = operator
	inst.Status = "running"
	return s.repo.Update(inst)
}
