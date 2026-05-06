package nacos

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"devops/internal/models"
	approvalrepo "devops/internal/modules/approval/repository"
	"devops/internal/models/deploy"
	appRepo "devops/internal/modules/application/repository"
	infraRepo "devops/internal/modules/infrastructure/repository"
	"devops/pkg/logger"
)

type ReleaseService struct {
	repo          *appRepo.NacosReleaseRepository
	instRepo      *infraRepo.NacosInstanceRepository
	chainService  approvalChainMatcher
	instanceMaker approvalInstanceCreator
	policyRepo    *approvalrepo.EnvAuditPolicyRepository
}

type approvalChainMatcher interface {
	GetWithNodes(ctx context.Context, id uint) (*models.ApprovalChain, error)
	Match(ctx context.Context, appID uint, env string) (*models.ApprovalChain, error)
}

type approvalInstanceCreator interface {
	Create(ctx context.Context, recordID uint, chain *models.ApprovalChain) (*models.ApprovalInstance, error)
}

const nacosReleaseApprovalRecordOffset uint = 2200000000

func NewReleaseService(repo *appRepo.NacosReleaseRepository, instRepo *infraRepo.NacosInstanceRepository) *ReleaseService {
	return &ReleaseService{repo: repo, instRepo: instRepo}
}

func (s *ReleaseService) SetApprovalFlow(chainService approvalChainMatcher, instanceMaker approvalInstanceCreator) {
	s.chainService = chainService
	s.instanceMaker = instanceMaker
}

func (s *ReleaseService) SetEnvAuditPolicyRepo(policyRepo *approvalrepo.EnvAuditPolicyRepository) {
	s.policyRepo = policyRepo
}

func BuildNacosReleaseApprovalRecordID(releaseID uint) uint {
	return nacosReleaseApprovalRecordOffset + releaseID
}

func ResolveNacosReleaseIDFromApprovalRecord(recordID uint) (uint, bool) {
	if recordID < nacosReleaseApprovalRecordOffset {
		return 0, false
	}
	return recordID - nacosReleaseApprovalRecordOffset, true
}

// List 分页查询发布单
func (s *ReleaseService) List(f appRepo.NacosReleaseFilter, page, pageSize int) ([]deploy.NacosRelease, int64, error) {
	return s.repo.List(f, page, pageSize)
}

// GetByID 查询发布单详情
func (s *ReleaseService) GetByID(id uint) (*deploy.NacosRelease, error) {
	return s.repo.GetByID(id)
}

// CreateDraft 创建草稿发布单，自动拉取 Nacos 当前内容作为变更前快照
func (s *ReleaseService) CreateDraft(ctx context.Context, nr *deploy.NacosRelease) error {
	nr.Status = "draft"

	// 尝试拉取 Nacos 当前配置作为 ContentBefore
	if nr.NacosInstanceID > 0 && nr.DataID != "" {
		client, err := s.getClient(ctx, nr.NacosInstanceID)
		if err == nil {
			content, err := client.GetConfig(nr.Tenant, nr.Group, nr.DataID)
			if err == nil {
				nr.ContentBefore = content
			}
		}
	}

	if nr.ContentAfter != "" {
		nr.ContentHash = hashContent(nr.ContentAfter)
	}

	return s.repo.Create(nr)
}

// Update 更新草稿发布单
func (s *ReleaseService) Update(nr *deploy.NacosRelease) error {
	existing, err := s.repo.GetByID(nr.ID)
	if err != nil {
		return fmt.Errorf("发布单不存在: %w", err)
	}
	if existing.Status != "draft" {
		return fmt.Errorf("只能编辑草稿状态的发布单")
	}
	if nr.ContentAfter != "" {
		nr.ContentHash = hashContent(nr.ContentAfter)
	}
	return s.repo.Update(nr)
}

// Delete 删除草稿发布单
func (s *ReleaseService) Delete(id uint) error {
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("发布单不存在: %w", err)
	}
	if existing.Status != "draft" {
		return fmt.Errorf("只能删除草稿状态的发布单")
	}
	return s.repo.Delete(id)
}

// SubmitForApproval 提交审批
func (s *ReleaseService) SubmitForApproval(ctx context.Context, id uint) (*deploy.NacosRelease, error) {
	nr, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布单不存在: %w", err)
	}
	if nr.Status != "draft" {
		return nil, fmt.Errorf("只有草稿状态才能提交审批")
	}
	if nr.ContentAfter == "" {
		return nil, fmt.Errorf("变更后内容不能为空")
	}
	nr.Status = "pending_approval"
	nr.RejectReason = ""
	if err := s.attachApprovalFlow(ctx, nr); err != nil {
		return nil, err
	}
	if err := s.repo.Update(nr); err != nil {
		return nil, err
	}
	return nr, nil
}

// Approve 审批通过
func (s *ReleaseService) Approve(id, approverID uint, approverName string) (*deploy.NacosRelease, error) {
	nr, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布单不存在: %w", err)
	}
	if nr.Status != "pending_approval" {
		return nil, fmt.Errorf("只有待审批状态才能审批")
	}
	nr.Status = "approved"
	nr.ApprovedBy = &approverID
	nr.ApprovedByName = approverName
	now := time.Now()
	nr.ApprovedAt = &now
	if err := s.repo.Update(nr); err != nil {
		return nil, err
	}
	return nr, nil
}

func (s *ReleaseService) GetByApprovalInstanceID(approvalInstanceID uint) (*deploy.NacosRelease, error) {
	return s.repo.GetByApprovalInstanceID(approvalInstanceID)
}

// Reject 驳回
func (s *ReleaseService) Reject(id, approverID uint, approverName, reason string) (*deploy.NacosRelease, error) {
	nr, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布单不存在: %w", err)
	}
	if nr.Status != "pending_approval" {
		return nil, fmt.Errorf("只有待审批状态才能驳回")
	}
	nr.Status = "rejected"
	nr.ApprovedBy = &approverID
	nr.ApprovedByName = approverName
	nr.RejectReason = reason
	now := time.Now()
	nr.ApprovedAt = &now
	if err := s.repo.Update(nr); err != nil {
		return nil, err
	}
	return nr, nil
}

// Publish 发布配置到 Nacos
func (s *ReleaseService) Publish(ctx context.Context, id, publisherID uint, publisherName string) (*deploy.NacosRelease, error) {
	nr, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布单不存在: %w", err)
	}
	if nr.Status != "approved" {
		return nil, fmt.Errorf("只有已审批状态才能发布")
	}

	client, err := s.getClient(ctx, nr.NacosInstanceID)
	if err != nil {
		return nil, fmt.Errorf("获取 Nacos 客户端失败: %w", err)
	}

	if err := client.PublishConfig(nr.Tenant, nr.Group, nr.DataID, nr.ContentAfter, nr.ConfigType); err != nil {
		return nil, fmt.Errorf("发布到 Nacos 失败: %w", err)
	}

	nr.Status = "published"
	nr.PublishedBy = &publisherID
	nr.PublishedByName = publisherName
	now := time.Now()
	nr.PublishedAt = &now
	if err := s.repo.Update(nr); err != nil {
		return nil, err
	}
	return nr, nil
}

// Rollback 回滚：基于已发布的发布单创建一个回滚发布单并立即发布
func (s *ReleaseService) Rollback(ctx context.Context, id, userID uint, userName string) (*deploy.NacosRelease, error) {
	original, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("发布单不存在: %w", err)
	}
	if original.Status != "published" {
		return nil, fmt.Errorf("只有已发布状态才能回滚")
	}
	if original.ContentBefore == "" {
		return nil, fmt.Errorf("无变更前内容，无法回滚")
	}

	// 创建回滚发布单
	rollback := &deploy.NacosRelease{
		Title:             fmt.Sprintf("回滚: %s", original.Title),
		NacosInstanceID:   original.NacosInstanceID,
		NacosInstanceName: original.NacosInstanceName,
		Tenant:            original.Tenant,
		Group:             original.Group,
		DataID:            original.DataID,
		Env:               original.Env,
		ConfigType:        original.ConfigType,
		ContentBefore:     original.ContentAfter,
		ContentAfter:      original.ContentBefore,
		ContentHash:       hashContent(original.ContentBefore),
		ServiceID:         original.ServiceID,
		ServiceName:       original.ServiceName,
		ReleaseID:         original.ReleaseID,
		Status:            "approved",
		RiskLevel:         "high",
		Description:       fmt.Sprintf("回滚发布单 #%d", original.ID),
		CreatedBy:         userID,
		CreatedByName:     userName,
		ApprovedBy:        &userID,
		ApprovedByName:    userName,
		RollbackFromID:    &original.ID,
	}
	now := time.Now()
	rollback.ApprovedAt = &now

	if err := s.repo.Create(rollback); err != nil {
		return nil, fmt.Errorf("创建回滚发布单失败: %w", err)
	}

	// 直接发布回滚内容
	client, err := s.getClient(ctx, original.NacosInstanceID)
	if err != nil {
		return nil, fmt.Errorf("获取 Nacos 客户端失败: %w", err)
	}
	if err := client.PublishConfig(original.Tenant, original.Group, original.DataID, original.ContentBefore, original.ConfigType); err != nil {
		return nil, fmt.Errorf("回滚发布到 Nacos 失败: %w", err)
	}

	rollback.Status = "published"
	rollback.PublishedBy = &userID
	rollback.PublishedByName = userName
	rollback.PublishedAt = &now
	if err := s.repo.Update(rollback); err != nil {
		return nil, err
	}

	// 标记原发布单为已回滚
	original.Status = "rolled_back"
	_ = s.repo.Update(original)

	return rollback, nil
}

// FetchCurrentContent 从 Nacos 拉取当前配置内容（用于 Diff 预览）
func (s *ReleaseService) FetchCurrentContent(ctx context.Context, instanceID uint, tenant, group, dataID string) (string, error) {
	client, err := s.getClient(ctx, instanceID)
	if err != nil {
		return "", err
	}
	return client.GetConfig(tenant, group, dataID)
}

// ListByService 查询服务关联的发布单
func (s *ReleaseService) ListByService(serviceID uint, limit int) ([]deploy.NacosRelease, error) {
	return s.repo.ListByService(serviceID, limit)
}

func (s *ReleaseService) getClient(ctx context.Context, instanceID uint) (*NacosClient, error) {
	inst, err := s.instRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("Nacos 实例不存在: %w", err)
	}
	pwd, decryptErr := decryptPassword(inst.Password)
	if decryptErr != nil {
		// 兼容历史明文，并尝试在线迁移为密文。
		pwd = inst.Password
		if pwd != "" {
			if enc, encErr := encryptPassword(pwd); encErr == nil {
				inst.Password = enc
				if updateErr := s.instRepo.Update(ctx, inst); updateErr != nil {
					logger.L().WithField("instance_id", inst.ID).WithField("error", updateErr).Warn("Nacos release credential migration writeback failed")
				}
			} else {
				logger.L().WithField("instance_id", inst.ID).WithField("error", encErr).Warn("Nacos release credential migration encryption failed")
			}
		}
	}
	return NewNacosClient(inst.Addr, inst.Username, pwd), nil
}

func (s *ReleaseService) attachApprovalFlow(ctx context.Context, nr *deploy.NacosRelease) error {
	if nr == nil || nr.ApprovalInstanceID != nil || s.chainService == nil || s.instanceMaker == nil {
		return nil
	}

	chain, err := s.matchApprovalChain(ctx, nr.Env)
	if err != nil {
		return err
	}
	if chain == nil {
		return nil
	}

	recordID := BuildNacosReleaseApprovalRecordID(nr.ID)
	instance, err := s.instanceMaker.Create(ctx, recordID, chain)
	if err != nil {
		return err
	}
	nr.ApprovalInstanceID = &instance.ID
	nr.ApprovalChainID = &chain.ID
	nr.ApprovalChainName = chain.Name
	return nil
}

func (s *ReleaseService) matchApprovalChain(ctx context.Context, env string) (*models.ApprovalChain, error) {
	if s.chainService == nil {
		return nil, nil
	}
	if s.policyRepo != nil && env != "" {
		policy, err := s.policyRepo.GetByEnvName(env)
		if err == nil && policy != nil && policy.RequireChain && policy.DefaultChainID != nil {
			return s.chainService.GetWithNodes(ctx, *policy.DefaultChainID)
		}
	}
	return s.chainService.Match(ctx, 0, env)
}

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}
