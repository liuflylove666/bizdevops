package database

import (
	"context"
	"errors"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
)

// InstanceService 对外暴露的实例 CRUD 服务（处理加密/解密 + 连接池失效）
type InstanceService struct {
	repo *dbrepo.DBInstanceRepository
	conn *Connector
}

func NewInstanceService(repo *dbrepo.DBInstanceRepository, conn *Connector) *InstanceService {
	return &InstanceService{repo: repo, conn: conn}
}

// Create 创建实例；password 为明文
func (s *InstanceService) Create(ctx context.Context, m *model.DBInstance, plainPassword string) error {
	if plainPassword == "" {
		return errors.New("密码不能为空")
	}
	enc, err := encryptPassword(plainPassword)
	if err != nil {
		return err
	}
	m.Password = enc
	return s.repo.Create(ctx, m)
}

// Update 更新实例；若 plainPassword 为空则保留原密码
func (s *InstanceService) Update(ctx context.Context, m *model.DBInstance, plainPassword string) error {
	if plainPassword == "" {
		old, err := s.repo.GetByID(ctx, m.ID)
		if err != nil {
			return err
		}
		m.Password = old.Password
	} else {
		enc, err := encryptPassword(plainPassword)
		if err != nil {
			return err
		}
		m.Password = enc
	}
	if err := s.repo.Update(ctx, m); err != nil {
		return err
	}
	s.conn.Invalidate(m.ID)
	return nil
}

func (s *InstanceService) Delete(ctx context.Context, id uint) error {
	s.conn.Invalidate(id)
	return s.repo.Delete(ctx, id)
}

func (s *InstanceService) Get(ctx context.Context, id uint) (*model.DBInstance, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *InstanceService) List(ctx context.Context, f dbrepo.DBInstanceFilter, page, pageSize int) ([]model.DBInstance, int64, error) {
	return s.repo.List(ctx, f, page, pageSize)
}

func (s *InstanceService) ListFiltered(ctx context.Context, f dbrepo.DBInstanceFilter, ids []uint, page, pageSize int) ([]model.DBInstance, int64, error) {
	return s.repo.ListByIDs(ctx, f, ids, page, pageSize)
}

func (s *InstanceService) ListAll(ctx context.Context) ([]model.DBInstance, error) {
	return s.repo.ListAll(ctx)
}

func (s *InstanceService) ListAllFiltered(ctx context.Context, ids []uint) ([]model.DBInstance, error) {
	return s.repo.ListAllByIDs(ctx, ids)
}

// Test 连通性测试：若未提供 plainPassword，则用库中已加密的
func (s *InstanceService) Test(ctx context.Context, m *model.DBInstance, plainPassword string) error {
	if plainPassword == "" {
		if m.ID > 0 {
			old, err := s.repo.GetByID(ctx, m.ID)
			if err != nil {
				return err
			}
			p, err := decryptPassword(old.Password)
			if err != nil {
				return err
			}
			plainPassword = p
			// 填充未提供的字段
			if m.Host == "" {
				m.Host = old.Host
			}
			if m.Port == 0 {
				m.Port = old.Port
			}
			if m.Username == "" {
				m.Username = old.Username
			}
			if m.DBType == "" {
				m.DBType = old.DBType
			}
			if m.DefaultDB == "" {
				m.DefaultDB = old.DefaultDB
			}
			if m.Params == "" {
				m.Params = old.Params
			}
		} else {
			return errors.New("密码不能为空")
		}
	}
	return TestConnection(m, plainPassword)
}
