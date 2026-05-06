package database

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
)

// Connector 按实例 ID 懒加载 *gorm.DB
type Connector struct {
	mu       sync.RWMutex
	pool     map[uint]*entry
	repo     *dbrepo.DBInstanceRepository
	idleTTL  time.Duration
}

type entry struct {
	db       *gorm.DB
	lastUsed time.Time
}

func NewConnector(repo *dbrepo.DBInstanceRepository) *Connector {
	return &Connector{
		pool:    make(map[uint]*entry),
		repo:    repo,
		idleTTL: 30 * time.Minute,
	}
}

// Get 获取指定实例的 gorm.DB
func (c *Connector) Get(ctx context.Context, instanceID uint) (*gorm.DB, *model.DBInstance, error) {
	inst, err := c.repo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, nil, fmt.Errorf("实例不存在: %w", err)
	}
	if inst.Status != "active" {
		return nil, nil, fmt.Errorf("实例已停用")
	}

	c.mu.RLock()
	if e, ok := c.pool[instanceID]; ok {
		e.lastUsed = time.Now()
		c.mu.RUnlock()
		return e.db, inst, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.pool[instanceID]; ok {
		e.lastUsed = time.Now()
		return e.db, inst, nil
	}

	db, err := openDB(inst)
	if err != nil {
		return nil, nil, err
	}
	c.pool[instanceID] = &entry{db: db, lastUsed: time.Now()}
	return db, inst, nil
}

// ResolvePlainPassword 返回实例的明文密码（用于 gh-ost 等需要明文凭据的外部工具）
func (c *Connector) ResolvePlainPassword(ctx context.Context, instanceID uint) (*model.DBInstance, string, error) {
	inst, err := c.repo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, "", err
	}
	plain, err := decryptPassword(inst.Password)
	if err != nil {
		return nil, "", err
	}
	return inst, plain, nil
}

// Invalidate 从连接池移除
func (c *Connector) Invalidate(instanceID uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.pool[instanceID]; ok {
		if sqlDB, err := e.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		delete(c.pool, instanceID)
	}
}

// TestConnection 用明文配置直接尝试连接（用于 CRUD 时的连通性检测）
func TestConnection(inst *model.DBInstance, plainPassword string) error {
	clone := *inst
	clone.Password = plainPassword
	db, err := openDBWithPlain(&clone)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return sqlDB.PingContext(ctx)
}

func openDB(inst *model.DBInstance) (*gorm.DB, error) {
	plain, err := decryptPassword(inst.Password)
	if err != nil {
		return nil, fmt.Errorf("密码解密失败: %w", err)
	}
	clone := *inst
	clone.Password = plain
	return openDBWithPlain(&clone)
}

func openDBWithPlain(inst *model.DBInstance) (*gorm.DB, error) {
	if inst.DBType != "" && inst.DBType != "mysql" {
		return nil, fmt.Errorf("暂不支持的数据库类型: %s", inst.DBType)
	}
	dialector := mysql.Open(buildMySQLDSN(inst))
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func buildMySQLDSN(inst *model.DBInstance) string {
	params := inst.Params
	if params == "" {
		params = "charset=utf8mb4&parseTime=True&loc=Local&timeout=5s&readTimeout=10s"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		inst.Username,
		url.QueryEscape(inst.Password),
		inst.Host,
		inst.Port,
		inst.DefaultDB,
		params,
	)
}

