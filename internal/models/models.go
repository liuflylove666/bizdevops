// Package models 定义数据库模型
//
// 本包包含 DevOps 平台的所有数据库模型定义，按功能领域拆分为多个子包：
//
// 子包结构:
//   - notification/  - 消息通知模型（Telegram）
//   - infrastructure/ - 基础设施模型（K8s、CronHPA）
//   - deploy/        - 部署流程模型（部署记录、审批、流水线）
//   - monitoring/    - 监控告警模型（告警、健康检查、日志、成本）
//   - system/        - 系统管理模型（用户、RBAC、权限、审计）
//   - application/   - 应用管理模型（应用、环境配置）
//
// 向后兼容:
//
//	本包提供类型别名，允许继续使用 models.TypeName 的方式访问类型。
//	新代码建议直接导入子包使用。
//
// 使用示例:
//
//	// 方式1: 使用类型别名（向后兼容）
//	import "devops/internal/models"
//	user := &models.User{Username: "admin"}
//
//	// 方式2: 直接使用子包（推荐）
//	import "devops/internal/models/system"
//	user := &system.User{Username: "admin"}
package models

import (
	dbmodel "devops/internal/domain/database/model"
	"devops/internal/domain/notification/model"
	"devops/internal/models/application"
	"devops/internal/models/deploy"
	"devops/internal/models/infrastructure"
	"devops/internal/models/monitoring"
	modelpipeline "devops/internal/models/pipeline"
	"devops/internal/models/system"
)

// ==================== 通知领域类型别名 ====================

// TelegramBot Telegram 机器人 (别名)
type TelegramBot = model.TelegramBot

// TelegramMessageLog Telegram 消息日志 (别名)
type TelegramMessageLog = model.TelegramMessageLog

// ==================== 数据库管理领域类型别名 ====================

// DBInstance 数据库实例 (别名)
type DBInstance = dbmodel.DBInstance

// DBQueryLog 数据库查询日志 (别名)
type DBQueryLog = dbmodel.DBQueryLog

// SQLChangeTicket SQL 变更工单 (别名)
type SQLChangeTicket = dbmodel.SQLChangeTicket

// SQLChangeStatement SQL 变更语句 (别名)
type SQLChangeStatement = dbmodel.SQLChangeStatement

// SQLChangeWorkflowDetail SQL 工单工作流明细 (别名)
type SQLChangeWorkflowDetail = dbmodel.SQLChangeWorkflowDetail

// AuditStep 审批步骤 (别名)
type AuditStep = dbmodel.AuditStep

// SQLAuditRuleSet SQL 审核规则集 (别名)
type SQLAuditRuleSet = dbmodel.SQLAuditRuleSet

// AuditRuleConfig SQL 审核规则配置 (别名)
type AuditRuleConfig = dbmodel.AuditRuleConfig

// SQLRollbackScript SQL 回滚脚本 (别名)
type SQLRollbackScript = dbmodel.SQLRollbackScript

// DBInstanceACL 实例 ACL 权限绑定 (别名)
type DBInstanceACL = dbmodel.DBInstanceACL

// ==================== 基础设施领域类型别名 ====================

// K8sCluster K8s集群 (别名)
type K8sCluster = infrastructure.K8sCluster

// CronHPA 定时HPA (别名)
type CronHPA = infrastructure.CronHPA

// NacosInstance Nacos实例 (别名)
type NacosInstance = infrastructure.NacosInstance

// JiraInstance Jira实例 (别名)
type JiraInstance = infrastructure.JiraInstance

// JiraProjectMapping Jira项目映射 (别名)
type JiraProjectMapping = infrastructure.JiraProjectMapping

// SonarQubeInstance SonarQube实例 (别名)
type SonarQubeInstance = infrastructure.SonarQubeInstance

// SonarQubeProjectBinding SonarQube项目绑定 (别名)
type SonarQubeProjectBinding = infrastructure.SonarQubeProjectBinding

// ArgoCDApplication Argo CD 应用快照 (别名)
type ArgoCDApplication = infrastructure.ArgoCDApplication

// ==================== 部署领域类型别名 ====================

// DeployRecord 部署记录 (别名)
type DeployRecord = deploy.DeployRecord

// DeployLock 部署锁 (别名)
type DeployLock = deploy.DeployLock

// DeployWindow 部署窗口 (别名)
type DeployWindow = deploy.DeployWindow

// Task 任务 (别名)
type Task = deploy.Task

// ApprovalRule 审批规则 (别名)
type ApprovalRule = deploy.ApprovalRule

// ApprovalRecord 审批记录 (别名)
type ApprovalRecord = deploy.ApprovalRecord

// ApprovalChain 审批链 (别名)
type ApprovalChain = deploy.ApprovalChain

// ApprovalNode 审批节点 (别名)
type ApprovalNode = deploy.ApprovalNode

// ApprovalInstance 审批实例 (别名)
type ApprovalInstance = deploy.ApprovalInstance

// ApprovalNodeInstance 审批节点实例 (别名)
type ApprovalNodeInstance = deploy.ApprovalNodeInstance

// ApprovalAction 审批动作 (别名)
type ApprovalAction = deploy.ApprovalAction

// EnvAuditPolicy 环境审核策略 (别名)
type EnvAuditPolicy = deploy.EnvAuditPolicy

// Pipeline 流水线 (别名)
type Pipeline = deploy.Pipeline

// PipelineRun 流水线运行记录 (别名)
type PipelineRun = deploy.PipelineRun

// StageRun 阶段运行记录 (别名)
type StageRun = deploy.StageRun

// StepRun 步骤运行记录 (别名)
type StepRun = deploy.StepRun

// PipelineTemplate 流水线模板 (别名)
type PipelineTemplate = modelpipeline.PipelineTemplate

// PipelineCredential 流水线凭证 (别名)
type PipelineCredential = deploy.PipelineCredential

// PipelineVariable 流水线变量 (别名)
type PipelineVariable = deploy.PipelineVariable

// GitRepository Git仓库 (别名)
type GitRepository = deploy.GitRepository

// BuildJob 构建任务 (别名)
type BuildJob = deploy.BuildJob

// Artifact 构建制品 (别名)
type Artifact = deploy.Artifact

// BuildCache 构建缓存 (别名)
type BuildCache = deploy.BuildCache

// BuildWorkspace 构建工作空间 (别名)
type BuildWorkspace = deploy.BuildWorkspace

// WebhookLog Webhook日志 (别名)
type WebhookLog = deploy.WebhookLog

// ArtifactRegistry 制品库 (别名)
type ArtifactRegistry = deploy.ArtifactRegistry

// EnvPromotionPolicy 环境晋级策略 (别名)
type EnvPromotionPolicy = deploy.EnvPromotionPolicy

// EnvPromotionRecord 镜像晋级记录 (别名)
type EnvPromotionRecord = deploy.EnvPromotionRecord

// EnvPromotionStep 晋级步骤 (别名)
type EnvPromotionStep = deploy.EnvPromotionStep

// NacosRelease Nacos配置发布单 (别名)
type NacosRelease = deploy.NacosRelease

// Release 统一发布主单 (别名)
type Release = deploy.Release

// ReleaseItem 发布主单关联子项 (别名)
type ReleaseItem = deploy.ReleaseItem

// ReleaseGateResult 发布 Gate 结果快照 (别名)
type ReleaseGateResult = deploy.ReleaseGateResult

// ChangeEvent 统一变更事件 (别名)
type ChangeEvent = deploy.ChangeEvent

// EnvInstance 环境实例 (别名)
type EnvInstance = deploy.EnvInstance

// ==================== 监控领域类型别名 ====================

// AlertConfig 告警配置 (别名)
type AlertConfig = monitoring.AlertConfig

// Incident 事故记录 (别名)
type Incident = monitoring.Incident

// AlertHistory 告警历史 (别名)
type AlertHistory = monitoring.AlertHistory

// AlertSilence 告警静默规则 (别名)
type AlertSilence = monitoring.AlertSilence

// AlertEscalation 告警升级规则 (别名)
type AlertEscalation = monitoring.AlertEscalation

// AlertEscalationLog 告警升级记录 (别名)
type AlertEscalationLog = monitoring.AlertEscalationLog

// OncallSchedule 值班排班表 (别名)
type OncallSchedule = monitoring.OncallSchedule

// OncallShift 值班班次 (别名)
type OncallShift = monitoring.OncallShift

// OncallOverride 值班临时替换 (别名)
type OncallOverride = monitoring.OncallOverride

// AlertAssignment 告警分配记录 (别名)
type AlertAssignment = monitoring.AlertAssignment

// HealthCheckConfig 健康检查配置 (别名)
type HealthCheckConfig = monitoring.HealthCheckConfig

// HealthCheckHistory 健康检查历史 (别名)
type HealthCheckHistory = monitoring.HealthCheckHistory

// LogAlertRule 日志告警规则 (别名)
type LogAlertRule = monitoring.LogAlertRule

// LogAlertHistory 日志告警历史 (别名)
type LogAlertHistory = monitoring.LogAlertHistory

// LogHighlightRule 日志染色规则 (别名)
type LogHighlightRule = monitoring.LogHighlightRule

// LogParseTemplate 日志解析模板 (别名)
type LogParseTemplate = monitoring.LogParseTemplate

// LogDataSource 日志数据源 (别名)
type LogDataSource = monitoring.LogDataSource

// LogBookmark 日志书签 (别名)
type LogBookmark = monitoring.LogBookmark

// LogSavedQuery 日志快捷查询 (别名)
type LogSavedQuery = monitoring.LogSavedQuery

// JSONObject JSON对象 (别名)
type JSONObject = monitoring.JSONObject

// ParseField 解析字段 (别名)
type ParseField = monitoring.ParseField

// ResourceCost 资源成本 (别名)
type ResourceCost = monitoring.ResourceCost

// CostSummary 成本汇总 (别名)
type CostSummary = monitoring.CostSummary

// CostSuggestion 成本优化建议 (别名)
type CostSuggestion = monitoring.CostSuggestion

// CostConfig 成本配置 (别名)
type CostConfig = monitoring.CostConfig

// CostBudget 成本预算 (别名)
type CostBudget = monitoring.CostBudget

// CostAlert 成本告警 (别名)
type CostAlert = monitoring.CostAlert

// ResourceActivity 资源活跃度 (别名)
type ResourceActivity = monitoring.ResourceActivity

// JSONArray JSON数组 (别名) - 使用 monitoring 包的定义以兼容日志服务
type JSONArray = monitoring.JSONArray

// ==================== 系统管理领域类型别名 ====================

// User 用户 (别名)
type User = system.User

// Role 角色 (别名)
type Role = system.Role

// Permission 权限 (别名)
type Permission = system.Permission

// RolePermission 角色权限关联 (别名)
type RolePermission = system.RolePermission

// UserRole 用户角色关联 (别名)
type UserRole = system.UserRole

// AuditLog 审计日志 (别名)
type AuditLog = system.AuditLog

// Deprecated: OANotifyConfig 为历史遗留命名；新部署请使用 Telegram 等统一通知通道。
type OANotifyConfig = system.OANotifyConfig

// SystemConfig 系统配置 (别名)
type SystemConfig = system.SystemConfig

// MessageTemplate 消息模板 (别名)
type MessageTemplate = system.MessageTemplate

// ImageRegistry 镜像仓库 (别名)
type ImageRegistry = system.ImageRegistry

// ImageScan 镜像扫描 (别名)
type ImageScan = system.ImageScan

// ComplianceRule 合规规则 (别名)
type ComplianceRule = system.ComplianceRule

// ConfigCheck 配置检查 (别名)
type ConfigCheck = system.ConfigCheck

// SecurityAuditLog 安全审计日志 (别名)
type SecurityAuditLog = system.SecurityAuditLog

// SecurityReport 安全报告 (别名)
type SecurityReport = system.SecurityReport

// ==================== 应用管理领域类型别名 ====================

// Organization 组织 (别名)
type Organization = application.Organization

// Project 项目 (别名)
type Project = application.Project

// EnvDefinition 环境定义 (别名)
type EnvDefinition = application.EnvDefinition

// Application 应用 (别名)
type Application = application.Application

// ApplicationRepoBinding 应用仓库绑定 (别名)
type ApplicationRepoBinding = application.ApplicationRepoBinding

// ApplicationEnv 应用环境 (别名)
type ApplicationEnv = application.ApplicationEnv

// ApplicationReadinessCheck 应用接入就绪度检查快照 (别名)
type ApplicationReadinessCheck = application.ApplicationReadinessCheck

// ==================== 权限常量和函数别名 ====================

// 角色常量
const (
	RoleSuperAdmin = system.RoleSuperAdmin
	RoleAdmin      = system.RoleAdmin
	RoleUser       = system.RoleUser
	RoleGuest      = system.RoleGuest
)

// 权限常量
const (
	PermUserView   = system.PermUserView
	PermUserCreate = system.PermUserCreate
	PermUserUpdate = system.PermUserUpdate
	PermUserDelete = system.PermUserDelete
	PermUserRole   = system.PermUserRole
	PermUserStatus = system.PermUserStatus

	PermAppView   = system.PermAppView
	PermAppCreate = system.PermAppCreate
	PermAppUpdate = system.PermAppUpdate
	PermAppDelete = system.PermAppDelete
	PermAppDeploy = system.PermAppDeploy

	PermApprovalView   = system.PermApprovalView
	PermApprovalCreate = system.PermApprovalCreate
	PermApprovalUpdate = system.PermApprovalUpdate
	PermApprovalDelete = system.PermApprovalDelete

	PermK8sView   = system.PermK8sView
	PermK8sCreate = system.PermK8sCreate
	PermK8sUpdate = system.PermK8sUpdate
	PermK8sDelete = system.PermK8sDelete
	PermK8sExec   = system.PermK8sExec

	PermSystemView   = system.PermSystemView
	PermSystemUpdate = system.PermSystemUpdate
)

// RolePermissions 角色权限映射 (别名)
var RolePermissions = system.RolePermissions

// GetRoleLevel 获取角色等级 (别名)
var GetRoleLevel = system.GetRoleLevel

// CanManageRole 检查是否可以管理目标角色 (别名)
var CanManageRole = system.CanManageRole

// HasPermission 检查角色是否有某个权限 (别名)
var HasPermission = system.HasPermission

// IsSuperAdmin 检查是否是超级管理员 (别名)
var IsSuperAdmin = system.IsSuperAdmin

// IsProtectedUser 检查用户是否受保护 (别名)
var IsProtectedUser = system.IsProtectedUser
