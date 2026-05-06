-- 在已存在旧版 init_tables 建库结果上，为 applications / approval_instances 补齐服务目录与 SQL 工单审批所需列与索引。
-- 全新部署若已使用更新后的 init_tables.sql 则无需执行。
-- 需要 MySQL 8.0.29+（ADD COLUMN IF NOT EXISTS）。索引若已存在，对应 CREATE INDEX 报错可忽略。

ALTER TABLE `applications` ADD COLUMN IF NOT EXISTS `organization_id` bigint unsigned DEFAULT NULL COMMENT '所属组织' AFTER `description`;
ALTER TABLE `applications` ADD COLUMN IF NOT EXISTS `project_id` bigint unsigned DEFAULT NULL COMMENT '所属项目' AFTER `organization_id`;

ALTER TABLE `approval_instances` ADD COLUMN IF NOT EXISTS `target_type` varchar(20) NOT NULL DEFAULT 'deploy' COMMENT '业务对象: deploy/sql_change' AFTER `record_id`;
ALTER TABLE `approval_instances` ADD COLUMN IF NOT EXISTS `target_id` bigint unsigned DEFAULT NULL COMMENT '业务对象ID' AFTER `target_type`;

CREATE INDEX `idx_app_org` ON `applications` (`organization_id`);
CREATE INDEX `idx_app_proj` ON `applications` (`project_id`);
CREATE INDEX `idx_ai_target` ON `approval_instances` (`target_type`, `target_id`);
