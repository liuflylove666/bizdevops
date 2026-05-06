package repository

import (
	"context"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"devops/internal/models"
)

var repositoryDB *gorm.DB
var repositoryRedis *redis.Client

// SetDB 设置仓储数据库连接
func SetDB(db *gorm.DB) {
	repositoryDB = db
}

// GetDB 获取数据库连接
func GetDB(ctx context.Context) *gorm.DB {
	return repositoryDB
}

// SetRedis 设置 Redis 连接
func SetRedis(rdb *redis.Client) {
	repositoryRedis = rdb
}

// GetRedis 获取 Redis 连接
func GetRedis() *redis.Client {
	return repositoryRedis
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.K8sCluster{},
		&models.Task{},
		&models.MessageTemplate{},
		&models.SystemConfig{},
	)
}

// GormDB 接口用于类型断言
type GormDB interface {
	WithContext(ctx context.Context) *gorm.DB
}
