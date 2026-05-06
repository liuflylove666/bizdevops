package changelog

import (
	"devops/internal/models/deploy"
	appRepo "devops/internal/modules/application/repository"
)

type Service struct {
	repo *appRepo.ChangeEventRepository
}

func NewService(repo *appRepo.ChangeEventRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(f appRepo.ChangeEventFilter, page, pageSize int) ([]deploy.ChangeEvent, int64, error) {
	return s.repo.List(f, page, pageSize)
}

func (s *Service) ListByApplication(appID uint, limit int) ([]deploy.ChangeEvent, error) {
	return s.repo.ListByApplication(appID, limit)
}

func (s *Service) Stats() ([]appRepo.EventTypeStat, error) {
	return s.repo.Stats()
}

// RecordEvent 记录变更事件（供其他模块调用）
func (s *Service) RecordEvent(e *deploy.ChangeEvent) error {
	return s.repo.Create(e)
}
