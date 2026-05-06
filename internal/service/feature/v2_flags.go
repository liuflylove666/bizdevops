// Package feature - v2.0 重构相关的 Feature Flag 常量与种子注入。
//
// 单一真相源：docs/roadmap/v2.0-feature-flags.md
// 任何新增 v2.0 Flag 都必须同步更新本文件与上述文档。
//
// 使用方式：release 域 v2 Flag 已在 E7-06 中完成清理。
package feature

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// ============================================================================
// v2.0 Feature Flag 常量
// ============================================================================
//
// 命名规范：<domain>.<feature>_<variant>
// 域前缀：release

// V2FlagSpec 描述一个 v2.0 Flag 的种子定义。
type V2FlagSpec struct {
	Name              string
	DisplayName       string
	Description       string
	DefaultEnabled    bool // 种子默认值
	RolloutPercentage int  // 0-100，0 表示完全关闭，100 表示全量
}

// V2Flags 是 v2.0 重构的全部 Flag 种子清单。
//
// 维护规则：
//   - 每条 Flag 必须有清晰的 DisplayName 与 Description（用于管理后台展示）
//   - Default 默认应为 false（除遗留兼容类）
//   - 调整 Default 不会影响数据库中已有记录（Seed 是 idempotent insert-if-not-exists）
var V2Flags = []V2FlagSpec{
}

// SeedV2Flags 把 V2 Flag 清单种子化到数据库。
//
// 行为：
//   - 已存在的 Flag（按 name 唯一）保持不动，不会覆盖人工调整过的状态
//   - 不存在的 Flag 用 spec 默认值插入
//   - 整个过程在事务中执行，要么全部成功，要么全部回滚
//
// 调用时机：在 main.go 中 AutoMigrate 之后、IoC 容器初始化之前。
func SeedV2Flags(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("feature.SeedV2Flags: db 为 nil")
	}

	// 确保表存在（兜底，正常情况下 AutoMigrate 已建好）
	if err := db.AutoMigrate(&FeatureFlag{}); err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, spec := range V2Flags {
			var existing FeatureFlag
			err := tx.Where("name = ?", spec.Name).First(&existing).Error
			if err == nil {
				// 已存在，跳过（保留人工调整的状态）
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			rollout := spec.RolloutPercentage
			if rollout == 0 && spec.DefaultEnabled {
				rollout = 100
			}
			flag := FeatureFlag{
				Name:              spec.Name,
				DisplayName:       spec.DisplayName,
				Description:       spec.Description,
				IsEnabled:         spec.DefaultEnabled,
				RolloutPercentage: rollout,
			}
			if err := tx.Create(&flag).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
