package database

import (
	"devops/internal/config"
	dbmodel "devops/internal/domain/database/model"
	notimodel "devops/internal/domain/notification/model"
	"devops/internal/models/application"
	modelbiz "devops/internal/models/biz"
	"devops/internal/models/deploy"
	"devops/internal/models/infrastructure"
	"devops/internal/models/monitoring"
	modelpipeline "devops/internal/models/pipeline"
	systemmodel "devops/internal/models/system"
	"devops/internal/service/feature"
	"devops/pkg/logger"
	"devops/pkg/utils"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitMySQL 初始化 MySQL 连接（带重试）
func InitMySQL(cfg *config.Config) (*gorm.DB, error) {
	var db *gorm.DB

	err := utils.RetryWithBackoffSimple("MySQL", func() error {
		var connErr error
		db, connErr = gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
		if connErr != nil {
			return connErr
		}
		// 测试连接
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Ping()
	})

	if err != nil {
		return nil, err
	}

	// 配置连接池
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		sqlDB.SetMaxIdleConns(cfg.MySQLMaxIdleConns)
		sqlDB.SetMaxOpenConns(cfg.MySQLMaxOpenConns)
		sqlDB.SetConnMaxLifetime(cfg.MySQLConnMaxLifetime)
	}

	if cfg.Debug {
		db = db.Debug()
	}

	logger.L().Info("[MySQL] Connected successfully to %s:%d/%s", cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDatabase)
	return db, nil
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	// 对增量功能做兜底补表，避免旧数据卷跳过 init SQL 后访问新页面直接报 500。
	if err := db.AutoMigrate(
		// 流水线核心表增量字段兜底
		&deploy.Pipeline{},
		&deploy.PipelineRun{},
		&deploy.StageRun{},
		&deploy.StepRun{},
		&deploy.GitRepository{},
		&deploy.WebhookLog{},

		// 通知与数据库工单增量表
		&notimodel.TelegramBot{},
		&notimodel.TelegramMessageLog{},
		&dbmodel.DBInstance{},
		&dbmodel.DBQueryLog{},
		&dbmodel.SQLChangeTicket{},
		&dbmodel.SQLChangeStatement{},
		&dbmodel.SQLRollbackScript{},
		&dbmodel.SQLAuditRuleSet{},
		&dbmodel.SQLChangeWorkflowDetail{},
		&dbmodel.DBInstanceACL{},

		// 应用与发布增量表
		&application.Organization{},
		&application.Project{},
		&application.EnvDefinition{},
		&application.ApplicationEnv{},
		&application.ApplicationRepoBinding{},
		&application.ApplicationReadinessCheck{},
		&modelbiz.BizGoal{},
		&modelbiz.BizRequirement{},
		&modelbiz.BizVersion{},
		&deploy.EnvAuditPolicy{},
		&deploy.EnvPromotionPolicy{},
		&deploy.EnvPromotionRecord{},
		&deploy.EnvPromotionStep{},
		&deploy.NacosRelease{},
		&deploy.Release{},
		&deploy.ReleaseItem{},
		&deploy.ReleaseGateResult{},
		&deploy.ChangeEvent{},
		&deploy.EnvInstance{},
		&modelpipeline.PipelineTemplate{},
		&modelpipeline.PipelineTemplateRating{},
		&modelpipeline.PipelineTemplateFavorite{},
		&modelpipeline.PipelineStageTemplate{},
		&modelpipeline.PipelineStepTemplate{},

		// 基础设施与可观测性增量表
		&infrastructure.JiraInstance{},
		&infrastructure.JiraProjectMapping{},
		&infrastructure.SonarQubeInstance{},
		&infrastructure.SonarQubeProjectBinding{},
		&infrastructure.ArgoCDInstance{},
		&infrastructure.ArgoCDApplication{},
		&infrastructure.GitOpsRepo{},
		&infrastructure.GitOpsChangeRequest{},
		&monitoring.OncallSchedule{},
		&monitoring.OncallShift{},
		&monitoring.OncallOverride{},
		&monitoring.AlertAssignment{},
		&monitoring.PrometheusInstance{},
		&monitoring.Incident{}, // v2.1: 生产事故

		// 安全模块增量表与字段兜底
		&systemmodel.ImageRegistry{},
		&systemmodel.ImageScan{},
		&systemmodel.ComplianceRule{},
		&systemmodel.ConfigCheck{},
		&systemmodel.SecurityAuditLog{},

		// 平台治理：Feature Flag（v2.0 重构使用）
		&feature.FeatureFlag{},
	); err != nil {
		logger.L().Error("[MySQL] AutoMigrate failed: %v", err)
		return err
	}
	return nil
}
