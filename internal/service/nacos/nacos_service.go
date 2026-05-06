package nacos

import (
	"context"
	"fmt"

	"devops/internal/models/infrastructure"
	infraRepo "devops/internal/modules/infrastructure/repository"
	"devops/pkg/logger"
)

type Service struct {
	repo *infraRepo.NacosInstanceRepository
}

func NewService(repo *infraRepo.NacosInstanceRepository) *Service {
	return &Service{repo: repo}
}

// --- Instance CRUD ---

func (s *Service) CreateInstance(ctx context.Context, inst *infrastructure.NacosInstance) error {
	if inst.Password != "" {
		enc, err := encryptPassword(inst.Password)
		if err != nil {
			return fmt.Errorf("加密密码失败: %w", err)
		}
		inst.Password = enc
	}
	return s.repo.Create(ctx, inst)
}

func (s *Service) UpdateInstance(ctx context.Context, inst *infrastructure.NacosInstance) error {
	if inst.Password == "" {
		old, err := s.repo.GetByID(ctx, inst.ID)
		if err == nil {
			inst.Password = old.Password
		}
	} else {
		enc, err := encryptPassword(inst.Password)
		if err != nil {
			return fmt.Errorf("加密密码失败: %w", err)
		}
		inst.Password = enc
	}
	return s.repo.Update(ctx, inst)
}

func (s *Service) DeleteInstance(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) GetInstance(ctx context.Context, id uint) (*infrastructure.NacosInstance, error) {
	inst, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	inst.Password = ""
	return inst, nil
}

func (s *Service) ListInstances(ctx context.Context, env string) ([]infrastructure.NacosInstance, error) {
	list, err := s.repo.List(ctx, env)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Password = ""
	}
	return list, nil
}

func (s *Service) TestConnection(ctx context.Context, id uint) error {
	inst, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	pwd := s.resolvePasswordWithMigration(ctx, inst)
	client := NewNacosClient(inst.Addr, inst.Username, pwd)
	return client.TestConnection()
}

// --- Config Proxy ---

func (s *Service) getClient(ctx context.Context, instanceID uint) (*NacosClient, error) {
	inst, err := s.repo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("实例不存在: %w", err)
	}
	pwd := s.resolvePasswordWithMigration(ctx, inst)
	return NewNacosClient(inst.Addr, inst.Username, pwd), nil
}

func (s *Service) resolvePasswordWithMigration(ctx context.Context, inst *infrastructure.NacosInstance) string {
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
		if updateErr := s.repo.Update(ctx, inst); updateErr != nil {
			logger.L().WithField("instance_id", inst.ID).WithField("error", updateErr).Warn("Nacos credential migration writeback failed")
		}
	} else {
		logger.L().WithField("instance_id", inst.ID).WithField("error", encErr).Warn("Nacos credential migration encryption failed")
	}
	return legacyPlainPassword
}

func (s *Service) ListNamespaces(ctx context.Context, instanceID uint) ([]NacosNamespace, error) {
	client, err := s.getClient(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return client.ListNamespaces()
}

func (s *Service) ListConfigs(ctx context.Context, instanceID uint, tenant, group, dataID string, page, pageSize int) (*ConfigListResult, error) {
	client, err := s.getClient(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return client.ListConfigs(tenant, group, dataID, page, pageSize)
}

func (s *Service) GetConfig(ctx context.Context, instanceID uint, tenant, group, dataID string) (string, error) {
	client, err := s.getClient(ctx, instanceID)
	if err != nil {
		return "", err
	}
	return client.GetConfig(tenant, group, dataID)
}

func (s *Service) PublishConfig(ctx context.Context, instanceID uint, tenant, group, dataID, content, configType string) error {
	client, err := s.getClient(ctx, instanceID)
	if err != nil {
		return err
	}
	return client.PublishConfig(tenant, group, dataID, content, configType)
}

func (s *Service) DeleteConfig(ctx context.Context, instanceID uint, tenant, group, dataID string) error {
	client, err := s.getClient(ctx, instanceID)
	if err != nil {
		return err
	}
	return client.DeleteConfig(tenant, group, dataID)
}

func (s *Service) ListConfigHistory(ctx context.Context, instanceID uint, tenant, group, dataID string, page, pageSize int) (*HistoryListResult, error) {
	client, err := s.getClient(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return client.ListConfigHistory(tenant, group, dataID, page, pageSize)
}

func (s *Service) GetConfigHistoryDetail(ctx context.Context, instanceID uint, tenant, group, dataID string, nid int64) (*ConfigHistoryItem, error) {
	client, err := s.getClient(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return client.GetConfigHistoryDetail(tenant, group, dataID, nid)
}

// --- Cross-env Comparison ---

type ConfigCompareItem struct {
	DataID        string `json:"data_id"`
	Group         string `json:"group"`
	SourceContent string `json:"source_content"`
	TargetContent string `json:"target_content"`
	Same          bool   `json:"same"`
}

func (s *Service) CompareConfigs(ctx context.Context, sourceInstanceID, targetInstanceID uint, sourceTenant, targetTenant, group string) ([]ConfigCompareItem, error) {
	srcClient, err := s.getClient(ctx, sourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("源实例: %w", err)
	}
	tgtClient, err := s.getClient(ctx, targetInstanceID)
	if err != nil {
		return nil, fmt.Errorf("目标实例: %w", err)
	}

	srcResult, err := srcClient.ListConfigs(sourceTenant, group, "", 1, 500)
	if err != nil {
		return nil, fmt.Errorf("查询源配置: %w", err)
	}

	tgtResult, err := tgtClient.ListConfigs(targetTenant, group, "", 1, 500)
	if err != nil {
		return nil, fmt.Errorf("查询目标配置: %w", err)
	}

	tgtMap := make(map[string]NacosConfigItem)
	for _, item := range tgtResult.PageItems {
		tgtMap[item.Group+"/"+item.DataID] = item
	}

	var items []ConfigCompareItem
	for _, src := range srcResult.PageItems {
		key := src.Group + "/" + src.DataID
		ci := ConfigCompareItem{
			DataID:        src.DataID,
			Group:         src.Group,
			SourceContent: src.Content,
		}
		if tgt, ok := tgtMap[key]; ok {
			ci.TargetContent = tgt.Content
			ci.Same = src.MD5 == tgt.MD5
			delete(tgtMap, key)
		}
		items = append(items, ci)
	}
	for _, tgt := range tgtMap {
		items = append(items, ConfigCompareItem{
			DataID:        tgt.DataID,
			Group:         tgt.Group,
			TargetContent: tgt.Content,
		})
	}
	return items, nil
}

func (s *Service) SyncConfig(ctx context.Context, sourceInstanceID, targetInstanceID uint, sourceTenant, targetTenant, group, dataID string) error {
	srcClient, err := s.getClient(ctx, sourceInstanceID)
	if err != nil {
		return err
	}
	content, err := srcClient.GetConfig(sourceTenant, group, dataID)
	if err != nil {
		return fmt.Errorf("读取源配置: %w", err)
	}
	tgtClient, err := s.getClient(ctx, targetInstanceID)
	if err != nil {
		return err
	}
	return tgtClient.PublishConfig(targetTenant, group, dataID, content, "")
}
