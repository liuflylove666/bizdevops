// Package repository 提供向后兼容的repository导入
// 这是一个临时的兼容层，用于在重构期间保持旧的import路径工作
package repository

import (
	// Auth模块
	authRepo "devops/internal/modules/auth/repository"
	// Application模块
	appRepo "devops/internal/modules/application/repository"
	// Approval模块
	approvalRepo "devops/internal/modules/approval/repository"
	// Infrastructure模块
	infraRepo "devops/internal/modules/infrastructure/repository"
	// Database模块
	databaseRepo "devops/internal/domain/database/repository"
	// Notification模块
	notificationRepo "devops/internal/domain/notification/repository"
	// Monitoring模块
	monitoringRepo "devops/internal/modules/monitoring/repository"
	// System模块
	systemRepo "devops/internal/modules/system/repository"
)

// Auth模块类型别名
type (
	RoleRepository           = authRepo.RoleRepository
	PermissionRepository     = authRepo.PermissionRepository
	RolePermissionRepository = authRepo.RolePermissionRepository
	UserRoleRepository       = authRepo.UserRoleRepository
	UserRepository           = authRepo.UserRepository
)

// Auth模块函数别名
var (
	NewRoleRepository           = authRepo.NewRoleRepository
	NewPermissionRepository     = authRepo.NewPermissionRepository
	NewRolePermissionRepository = authRepo.NewRolePermissionRepository
	NewUserRoleRepository       = authRepo.NewUserRoleRepository
	NewUserRepository           = authRepo.NewUserRepository
)

// Application模块类型别名
type (
	ApplicationRepository    = appRepo.ApplicationRepository
	ApplicationFilter        = appRepo.ApplicationFilter
	ApplicationRepoBindingRepository = appRepo.ApplicationRepoBindingRepository
	ApplicationEnvRepository = appRepo.ApplicationEnvRepository
	DeployRecordRepository        = appRepo.DeployRecordRepository
	DeployRecordFilter            = appRepo.DeployRecordFilter
	DeployStatsFilter             = appRepo.DeployStatsFilter
	DeployStats                   = appRepo.DeployStats
	DeployLockRepository          = appRepo.DeployLockRepository
	ApprovalRecordRepository      = appRepo.ApprovalRecordRepository
	PromotionPolicyRepository     = appRepo.PromotionPolicyRepository
	PromotionRecordRepository     = appRepo.PromotionRecordRepository
	PromotionRecordFilter         = appRepo.PromotionRecordFilter
	PromotionStepRepository       = appRepo.PromotionStepRepository
	NacosReleaseRepository        = appRepo.NacosReleaseRepository
	NacosReleaseFilter            = appRepo.NacosReleaseFilter
	ReleaseRepository             = appRepo.ReleaseRepository
	ReleaseFilter                 = appRepo.ReleaseFilter
	ReleaseItemRepository         = appRepo.ReleaseItemRepository
	ChangeEventRepository         = appRepo.ChangeEventRepository
	ChangeEventFilter             = appRepo.ChangeEventFilter
	EnvInstanceRepository         = appRepo.EnvInstanceRepository
	EnvInstanceFilter             = appRepo.EnvInstanceFilter
	OrganizationRepository        = appRepo.OrganizationRepository
	ProjectRepository             = appRepo.ProjectRepository
	EnvDefinitionRepository       = appRepo.EnvDefinitionRepository
)

// Application模块函数别名
var (
	NewApplicationRepository        = appRepo.NewApplicationRepository
	NewApplicationRepoBindingRepository = appRepo.NewApplicationRepoBindingRepository
	NewApplicationEnvRepository     = appRepo.NewApplicationEnvRepository
	NewDeployRecordRepository       = appRepo.NewDeployRecordRepository
	NewDeployLockRepository         = appRepo.NewDeployLockRepository
	NewApprovalRecordRepository     = appRepo.NewApprovalRecordRepository
	NewPromotionPolicyRepository    = appRepo.NewPromotionPolicyRepository
	NewPromotionRecordRepository    = appRepo.NewPromotionRecordRepository
	NewPromotionStepRepository      = appRepo.NewPromotionStepRepository
	NewNacosReleaseRepository       = appRepo.NewNacosReleaseRepository
	NewReleaseRepository            = appRepo.NewReleaseRepository
	NewReleaseItemRepository        = appRepo.NewReleaseItemRepository
	NewChangeEventRepository        = appRepo.NewChangeEventRepository
	NewEnvInstanceRepository        = appRepo.NewEnvInstanceRepository
	NewOrganizationRepository       = appRepo.NewOrganizationRepository
	NewProjectRepository            = appRepo.NewProjectRepository
	NewEnvDefinitionRepository      = appRepo.NewEnvDefinitionRepository
)

// Approval模块类型别名
type (
	ApprovalChainRepository        = approvalRepo.ApprovalChainRepository
	ApprovalInstanceRepository     = approvalRepo.ApprovalInstanceRepository
	ApprovalNodeRepository         = approvalRepo.ApprovalNodeRepository
	ApprovalNodeInstanceRepository = approvalRepo.ApprovalNodeInstanceRepository
	ApprovalActionRepository       = approvalRepo.ApprovalActionRepository
	ApprovalRuleRepository         = approvalRepo.ApprovalRuleRepository
	DeployWindowRepository         = approvalRepo.DeployWindowRepository
	EnvAuditPolicyRepository       = approvalRepo.EnvAuditPolicyRepository
	ChainFilter                    = approvalRepo.ChainFilter
	InstanceFilter                 = approvalRepo.InstanceFilter
)

// Approval模块函数别名
var (
	NewApprovalChainRepository        = approvalRepo.NewApprovalChainRepository
	NewApprovalInstanceRepository     = approvalRepo.NewApprovalInstanceRepository
	NewApprovalNodeRepository         = approvalRepo.NewApprovalNodeRepository
	NewApprovalNodeInstanceRepository = approvalRepo.NewApprovalNodeInstanceRepository
	NewApprovalActionRepository       = approvalRepo.NewApprovalActionRepository
	NewApprovalRuleRepository         = approvalRepo.NewApprovalRuleRepository
	NewDeployWindowRepository         = approvalRepo.NewDeployWindowRepository
	NewEnvAuditPolicyRepository       = approvalRepo.NewEnvAuditPolicyRepository
)

// Infrastructure模块类型别名
type (
	K8sClusterRepository       = infraRepo.K8sClusterRepository
	NacosInstanceRepository        = infraRepo.NacosInstanceRepository
	JiraInstanceRepository         = infraRepo.JiraInstanceRepository
	JiraProjectMappingRepository     = infraRepo.JiraProjectMappingRepository
	SonarQubeInstanceRepository      = infraRepo.SonarQubeInstanceRepository
	SonarQubeBindingRepository       = infraRepo.SonarQubeBindingRepository
	ArgoCDInstanceRepository         = infraRepo.ArgoCDInstanceRepository
	ArgoCDApplicationRepository      = infraRepo.ArgoCDApplicationRepository
	ArgoCDAppFilter                  = infraRepo.ArgoCDAppFilter
	GitOpsRepoRepository             = infraRepo.GitOpsRepoRepository
)

// Infrastructure模块函数别名
var (
	NewK8sClusterRepository       = infraRepo.NewK8sClusterRepository
	NewNacosInstanceRepository        = infraRepo.NewNacosInstanceRepository
	NewJiraInstanceRepository         = infraRepo.NewJiraInstanceRepository
	NewJiraProjectMappingRepository     = infraRepo.NewJiraProjectMappingRepository
	NewSonarQubeInstanceRepository      = infraRepo.NewSonarQubeInstanceRepository
	NewSonarQubeBindingRepository       = infraRepo.NewSonarQubeBindingRepository
	NewArgoCDInstanceRepository         = infraRepo.NewArgoCDInstanceRepository
	NewArgoCDApplicationRepository      = infraRepo.NewArgoCDApplicationRepository
	NewGitOpsRepoRepository             = infraRepo.NewGitOpsRepoRepository
)

// Notification模块类型别名
type (
	TelegramBotRepository        = notificationRepo.TelegramBotRepository
	TelegramMessageLogRepository = notificationRepo.TelegramMessageLogRepository
)

// Notification模块函数别名
var (
	NewTelegramBotRepository        = notificationRepo.NewTelegramBotRepository
	NewTelegramMessageLogRepository = notificationRepo.NewTelegramMessageLogRepository
)

// Database模块类型别名
type (
	DBInstanceRepository         = databaseRepo.DBInstanceRepository
	DBInstanceFilter             = databaseRepo.DBInstanceFilter
	DBQueryLogRepository         = databaseRepo.DBQueryLogRepository
	DBQueryLogFilter             = databaseRepo.DBQueryLogFilter
	SQLChangeTicketRepository    = databaseRepo.SQLChangeTicketRepository
	SQLChangeStatementRepository = databaseRepo.SQLChangeStatementRepository
	SQLChangeWorkflowRepository  = databaseRepo.SQLChangeWorkflowRepository
	SQLAuditRuleRepository       = databaseRepo.SQLAuditRuleRepository
	SQLRollbackRepository        = databaseRepo.SQLRollbackRepository
	TicketFilter                 = databaseRepo.TicketFilter
	DBInstanceACLRepository      = databaseRepo.DBInstanceACLRepository
	StatementFilter              = databaseRepo.StatementFilter
	StatementListItem            = databaseRepo.StatementListItem
)

// Database模块函数别名
var (
	NewDBInstanceRepository         = databaseRepo.NewDBInstanceRepository
	NewDBQueryLogRepository         = databaseRepo.NewDBQueryLogRepository
	NewSQLChangeTicketRepository    = databaseRepo.NewSQLChangeTicketRepository
	NewSQLChangeStatementRepository = databaseRepo.NewSQLChangeStatementRepository
	NewSQLChangeWorkflowRepository  = databaseRepo.NewSQLChangeWorkflowRepository
	NewSQLAuditRuleRepository       = databaseRepo.NewSQLAuditRuleRepository
	NewSQLRollbackRepository        = databaseRepo.NewSQLRollbackRepository
	NewDBInstanceACLRepository      = databaseRepo.NewDBInstanceACLRepository
)

// Monitoring模块类型别名
type (
	AlertConfigRepository        = monitoringRepo.AlertConfigRepository
	AlertHistoryRepository       = monitoringRepo.AlertHistoryRepository
	AlertSilenceRepository       = monitoringRepo.AlertSilenceRepository
	AlertEscalationRepository    = monitoringRepo.AlertEscalationRepository
	AlertEscalationLogRepository = monitoringRepo.AlertEscalationLogRepository
	HealthCheckConfigRepository  = monitoringRepo.HealthCheckConfigRepository
	HealthCheckHistoryRepository = monitoringRepo.HealthCheckHistoryRepository
	CertInfo                     = monitoringRepo.CertInfo
	ListFilters                  = monitoringRepo.ListFilters
	PrometheusInstanceRepository  = monitoringRepo.PrometheusInstanceRepository
	OncallScheduleRepository     = monitoringRepo.OncallScheduleRepository
	OncallShiftRepository        = monitoringRepo.OncallShiftRepository
	OncallOverrideRepository     = monitoringRepo.OncallOverrideRepository
	AlertAssignmentRepository    = monitoringRepo.AlertAssignmentRepository
)

// Monitoring模块函数别名
var (
	NewAlertConfigRepository        = monitoringRepo.NewAlertConfigRepository
	NewAlertHistoryRepository       = monitoringRepo.NewAlertHistoryRepository
	NewAlertSilenceRepository       = monitoringRepo.NewAlertSilenceRepository
	NewAlertEscalationRepository    = monitoringRepo.NewAlertEscalationRepository
	NewAlertEscalationLogRepository = monitoringRepo.NewAlertEscalationLogRepository
	NewHealthCheckConfigRepository  = monitoringRepo.NewHealthCheckConfigRepository
	NewHealthCheckHistoryRepository = monitoringRepo.NewHealthCheckHistoryRepository
	NewPrometheusInstanceRepository = monitoringRepo.NewPrometheusInstanceRepository
	NewOncallScheduleRepository     = monitoringRepo.NewOncallScheduleRepository
	NewOncallShiftRepository        = monitoringRepo.NewOncallShiftRepository
	NewOncallOverrideRepository     = monitoringRepo.NewOncallOverrideRepository
	NewAlertAssignmentRepository    = monitoringRepo.NewAlertAssignmentRepository
)

// System模块类型别名
type (
	AuditLogRepository = systemRepo.AuditLogRepository
	AuditLogFilter     = systemRepo.AuditLogFilter
	// Deprecated: OANotifyConfigRepository 为历史遗留命名，保留以便平滑迁移。
	OANotifyConfigRepository  = systemRepo.OANotifyConfigRepository
	MessageTemplateRepository = systemRepo.MessageTemplateRepository
)

// System模块函数别名
var (
	NewAuditLogRepository = systemRepo.NewAuditLogRepository
	// Deprecated: 见 OANotifyConfigRepository 上方说明。
	NewOANotifyConfigRepository  = systemRepo.NewOANotifyConfigRepository
	NewMessageTemplateRepository = systemRepo.NewMessageTemplateRepository
)
