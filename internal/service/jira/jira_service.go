package jira

import (
	"devops/internal/models"
	"devops/internal/repository"
	"devops/pkg/logger"
	"fmt"
)

type Service struct {
	instRepo    *repository.JiraInstanceRepository
	mappingRepo *repository.JiraProjectMappingRepository
}

func NewService(instRepo *repository.JiraInstanceRepository, mappingRepo *repository.JiraProjectMappingRepository) *Service {
	return &Service{instRepo: instRepo, mappingRepo: mappingRepo}
}

// --- Instance CRUD ---

func (s *Service) ListInstances() ([]models.JiraInstance, error) {
	list, err := s.instRepo.List()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Token = "******"
	}
	return list, nil
}

func (s *Service) GetInstance(id uint) (*models.JiraInstance, error) {
	inst, err := s.instRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	inst.Token = "******"
	return inst, nil
}

func (s *Service) CreateInstance(inst *models.JiraInstance) error {
	if inst.Token != "" {
		enc, err := encryptToken(inst.Token)
		if err != nil {
			return fmt.Errorf("encrypt token: %w", err)
		}
		inst.Token = enc
	}
	return s.instRepo.Create(inst)
}

func (s *Service) UpdateInstance(inst *models.JiraInstance) error {
	if inst.Token == "" || inst.Token == "******" {
		old, err := s.instRepo.GetByID(inst.ID)
		if err == nil {
			inst.Token = old.Token
		}
	} else {
		enc, err := encryptToken(inst.Token)
		if err != nil {
			return fmt.Errorf("encrypt token: %w", err)
		}
		inst.Token = enc
	}
	return s.instRepo.Update(inst)
}

func (s *Service) DeleteInstance(id uint) error {
	return s.instRepo.Delete(id)
}

func (s *Service) TestConnection(id uint) error {
	inst, err := s.instRepo.GetByID(id)
	if err != nil {
		return err
	}
	client, err := s.getClient(inst)
	if err != nil {
		return err
	}
	return client.TestConnection()
}

// --- Mapping CRUD ---

func (s *Service) ListMappings(instanceID uint) ([]models.JiraProjectMapping, error) {
	return s.mappingRepo.ListByInstance(instanceID)
}

func (s *Service) CreateMapping(m *models.JiraProjectMapping) error {
	return s.mappingRepo.Create(m)
}

func (s *Service) UpdateMapping(m *models.JiraProjectMapping) error {
	return s.mappingRepo.Update(m)
}

func (s *Service) DeleteMapping(id uint) error {
	return s.mappingRepo.Delete(id)
}

// --- Jira API proxy ---

func (s *Service) getClient(inst *models.JiraInstance) (*JiraClient, error) {
	token := s.resolveTokenWithMigration(inst)
	return NewJiraClient(inst.BaseURL, inst.Username, token, inst.AuthType), nil
}

func (s *Service) resolveTokenWithMigration(inst *models.JiraInstance) string {
	token, err := decryptToken(inst.Token)
	if err == nil {
		return token
	}

	// 兼容历史明文数据，并尝试在线自愈回写为密文。
	legacyPlainToken := inst.Token
	if legacyPlainToken == "" {
		return ""
	}
	enc, encErr := encryptToken(legacyPlainToken)
	if encErr == nil {
		inst.Token = enc
		if updateErr := s.instRepo.Update(inst); updateErr != nil {
			logger.L().WithField("instance_id", inst.ID).WithField("error", updateErr).Warn("Jira token migration writeback failed")
		}
	} else {
		logger.L().WithField("instance_id", inst.ID).WithField("error", encErr).Warn("Jira token migration encryption failed")
	}
	return legacyPlainToken
}

func (s *Service) getClientByID(instanceID uint) (*JiraClient, error) {
	inst, err := s.instRepo.GetByID(instanceID)
	if err != nil {
		return nil, err
	}
	return s.getClient(inst)
}

func (s *Service) ListProjects(instanceID uint) ([]map[string]interface{}, error) {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return nil, err
	}
	return client.ListProjects()
}

func (s *Service) SearchIssues(instanceID uint, jql string, startAt, maxResults int) (map[string]interface{}, error) {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return nil, err
	}
	fields := []string{"summary", "status", "assignee", "priority", "issuetype", "created", "updated", "labels"}
	return client.SearchIssues(jql, startAt, maxResults, fields)
}

func (s *Service) GetIssue(instanceID uint, issueKey string) (map[string]interface{}, error) {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return nil, err
	}
	return client.GetIssue(issueKey)
}

func (s *Service) GetBoards(instanceID uint, projectKey string) (map[string]interface{}, error) {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return nil, err
	}
	return client.GetBoards(projectKey)
}

func (s *Service) GetSprints(instanceID uint, boardID int, state string) (map[string]interface{}, error) {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return nil, err
	}
	return client.GetSprints(boardID, state)
}

func (s *Service) GetSprintIssues(instanceID uint, sprintID int, startAt, maxResults int) (map[string]interface{}, error) {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return nil, err
	}
	return client.GetSprintIssues(sprintID, startAt, maxResults)
}

func (s *Service) AddComment(instanceID uint, issueKey, comment string) (map[string]interface{}, error) {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return nil, err
	}
	return client.AddIssueComment(issueKey, comment)
}

func (s *Service) TransitionIssue(instanceID uint, issueKey, transitionID string) error {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return err
	}
	return client.TransitionIssue(issueKey, transitionID)
}

func (s *Service) GetTransitions(instanceID uint, issueKey string) (map[string]interface{}, error) {
	client, err := s.getClientByID(instanceID)
	if err != nil {
		return nil, err
	}
	return client.GetTransitions(issueKey)
}
