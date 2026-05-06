-- patch_200_release_aggregate.sql
--
-- v2.0 Sprint 2：把 Release 升级为发布域聚合根（ADR-0002）。
-- 为 releases / release_items 表新增字段，全部带默认值，对旧数据安全。
--
-- 同时由 GORM AutoMigrate 自动建立索引；本脚本提供手工 ALTER 路径，
-- 用于生产环境无 AutoMigrate 权限的场景。
--
-- 执行前置：先备份。
--   mysqldump -h $HOST -u $USER -p$PWD $DB releases release_items > releases_v1_backup.sql

-- ---------------------------------------------------------------------
-- releases：扩 9 个字段
-- ---------------------------------------------------------------------
ALTER TABLE releases
  ADD COLUMN rollout_strategy        VARCHAR(20)  NOT NULL DEFAULT 'direct',
  ADD COLUMN rollout_config          JSON         NULL,
  ADD COLUMN risk_score              INT          NOT NULL DEFAULT 0,
  ADD COLUMN risk_factors            JSON         NULL,
  ADD COLUMN approval_instance_id    BIGINT UNSIGNED NULL,
  ADD COLUMN gitops_change_request_id BIGINT UNSIGNED NULL,
  ADD COLUMN argo_app_name           VARCHAR(200) NULL,
  ADD COLUMN argo_sync_status        VARCHAR(20)  NULL,
  ADD COLUMN jira_issue_keys         VARCHAR(500) NULL;

-- 索引：rollout_strategy 用于按策略筛选；risk_score 用于风险榜单；
--      approval_instance_id / gitops_change_request_id 用于详情页关联。
ALTER TABLE releases
  ADD INDEX idx_releases_rollout_strategy (rollout_strategy),
  ADD INDEX idx_releases_risk_score (risk_score),
  ADD INDEX idx_releases_approval_instance_id (approval_instance_id),
  ADD INDEX idx_releases_gitops_change_request_id (gitops_change_request_id),
  ADD INDEX idx_releases_argo_app_name (argo_app_name);

-- ---------------------------------------------------------------------
-- release_items：扩 1 个字段（payload）
-- ---------------------------------------------------------------------
ALTER TABLE release_items
  ADD COLUMN payload JSON NULL;

-- ---------------------------------------------------------------------
-- 旧数据兜底：填默认 rollout_strategy
-- ---------------------------------------------------------------------
UPDATE releases SET rollout_strategy = 'direct' WHERE rollout_strategy IS NULL OR rollout_strategy = '';

-- ---------------------------------------------------------------------
-- 兼容性说明
-- ---------------------------------------------------------------------
-- 1. v1 调用方读取 Release 时不需要这些新字段，零兼容成本
-- 2. v2 GitOps PR 路径现已固定启用（不再依赖 feature flag）
-- 3. risk_score=0 表示尚未触发评分（首次提交审批时会自动计算）
