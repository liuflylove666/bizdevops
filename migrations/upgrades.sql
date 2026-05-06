-- 项目名称：devops
-- 文件名称：upgrades.sql
-- 作　　者：Jerion
-- 联系邮箱：416685476@qq.com
-- 功能描述：存量数据库升级补丁（仅对已有数据库执行，全新部署请使用 init_tables.sql）
-- 执行顺序：按章节顺序依次执行

-- ============================================
-- 1. 飞书相关补丁
-- ============================================

-- feishu_apps 补充 webhook 列（如列已存在可忽略报错）
ALTER TABLE `feishu_apps`
ADD COLUMN `webhook` varchar(500) DEFAULT '' COMMENT 'Webhook URL' AFTER `app_secret`;

-- 飞书消息发送记录表
CREATE TABLE IF NOT EXISTS `feishu_message_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `msg_type` varchar(50) NOT NULL COMMENT '消息类型: text/post/interactive',
  `receive_id` varchar(100) NOT NULL COMMENT '接收者ID',
  `receive_id_type` varchar(50) NOT NULL COMMENT 'ID类型: chat_id/open_id/user_id',
  `content` text COMMENT '消息内容',
  `title` varchar(200) DEFAULT '' COMMENT '卡片标题',
  `source` varchar(50) DEFAULT '' COMMENT '来源: manual/oa_sync',
  `status` varchar(20) DEFAULT 'success' COMMENT '状态: success/failed',
  `error_msg` text COMMENT '错误信息',
  `app_id` bigint unsigned DEFAULT NULL COMMENT '使用的飞书应用ID',
  PRIMARY KEY (`id`),
  KEY `idx_fml_msg_type` (`msg_type`),
  KEY `idx_fml_source` (`source`),
  KEY `idx_fml_status` (`status`),
  KEY `idx_fml_app_id` (`app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='飞书消息发送记录';

-- 飞书用户令牌表
CREATE TABLE IF NOT EXISTS `feishu_user_tokens` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `app_id` varchar(100) NOT NULL COMMENT '飞书 App ID',
  `access_token` text COMMENT '访问令牌',
  `refresh_token` text COMMENT '刷新令牌',
  `expires_at` datetime(3) DEFAULT NULL COMMENT '过期时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_fut_app_id` (`app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='飞书用户OAuth令牌';


-- ============================================
-- 2. 流水线模板补丁（fix_pipeline_templates_columns.sql）
-- ============================================

-- 检查并添加 language 列
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipeline_templates'
  AND COLUMN_NAME = 'language';
SET @sql = IF(@col_exists = 0,
    'ALTER TABLE pipeline_templates ADD COLUMN language VARCHAR(50) COMMENT \'编程语言: java, go, nodejs, python\' AFTER category',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 检查并添加 framework 列
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipeline_templates'
  AND COLUMN_NAME = 'framework';
SET @sql = IF(@col_exists = 0,
    'ALTER TABLE pipeline_templates ADD COLUMN framework VARCHAR(50) COMMENT \'框架: spring, gin, express, django\' AFTER language',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 检查并添加 idx_language 索引
SET @index_exists = 0;
SELECT COUNT(*) INTO @index_exists
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipeline_templates'
  AND INDEX_NAME = 'idx_language';
SET @sql = IF(@index_exists = 0,
    'ALTER TABLE pipeline_templates ADD INDEX idx_language (language)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================
-- 3. 负载均衡 hash_key 字段修复（fix_loadbalance_hash_key.sql）
-- ============================================

ALTER TABLE `traffic_loadbalance_config`
MODIFY COLUMN `hash_key` VARCHAR(20) DEFAULT NULL COMMENT '哈希键类型(header/cookie/source_ip/query_param)';

-- ============================================
-- 4. 制品仓库监控补丁（artifact_registry_monitoring.sql）
-- ============================================

-- 为 artifact_repositories 添加监控字段（如列已存在可忽略报错）
ALTER TABLE `artifact_repositories`
ADD COLUMN `connection_status` varchar(20) DEFAULT 'unknown' COMMENT '连接状态: connected/disconnected/checking/unknown' AFTER `enabled`,
ADD COLUMN `last_check_at` datetime(3) DEFAULT NULL COMMENT '最后检查时间' AFTER `connection_status`,
ADD COLUMN `last_error` text COMMENT '最后错误信息' AFTER `last_check_at`,
ADD COLUMN `enable_monitoring` tinyint(1) DEFAULT 1 COMMENT '是否启用监控' AFTER `last_error`,
ADD COLUMN `check_interval` int DEFAULT 300 COMMENT '检查间隔(秒)' AFTER `enable_monitoring`;

CREATE INDEX IF NOT EXISTS `idx_connection_status` ON `artifact_repositories`(`connection_status`);
CREATE INDEX IF NOT EXISTS `idx_enable_monitoring` ON `artifact_repositories`(`enable_monitoring`);

-- 创建统计视图
CREATE OR REPLACE VIEW `v_registry_connection_stats` AS
SELECT
  ar.id, ar.name, ar.type, ar.connection_status,
  ar.last_check_at, ar.enable_monitoring,
  COUNT(CASE WHEN arch.status = 'ok' THEN 1 END) AS success_count,
  COUNT(CASE WHEN arch.status = 'error' THEN 1 END) AS failed_count,
  ROUND(COUNT(CASE WHEN arch.status = 'ok' THEN 1 END) * 100.0 / NULLIF(COUNT(*), 0), 2) AS success_rate,
  AVG(arch.latency_ms) AS avg_response_time
FROM `artifact_repositories` ar
LEFT JOIN `artifact_registry_connection_history` arch
  ON ar.id = arch.registry_id
  AND arch.check_time >= DATE_SUB(NOW(), INTERVAL 7 DAY)
GROUP BY ar.id, ar.name, ar.type, ar.connection_status, ar.last_check_at, ar.enable_monitoring;

-- 触发器：连接状态变化时自动记录历史
DELIMITER $$

CREATE TRIGGER IF NOT EXISTS `after_registry_status_update`
AFTER UPDATE ON `artifact_repositories`
FOR EACH ROW
BEGIN
  IF OLD.connection_status != NEW.connection_status THEN
    INSERT INTO `artifact_registry_connection_history` (
      `registry_id`, `status`, `message`, `check_time`
    ) VALUES (
      NEW.id,
      CASE WHEN NEW.connection_status = 'connected' THEN 'ok' ELSE 'error' END,
      NEW.last_error,
      NEW.last_check_at
    );
  END IF;
END$$

-- 定期清理连接历史（保留最近 30 天）
CREATE EVENT IF NOT EXISTS `cleanup_registry_connection_history`
ON SCHEDULE EVERY 1 DAY
STARTS CURRENT_TIMESTAMP
DO
BEGIN
  DELETE FROM `artifact_registry_connection_history`
  WHERE `check_time` < DATE_SUB(NOW(), INTERVAL 30 DAY);
END$$

DELIMITER ;

-- ============================================
-- 5. 告警日志静默字段补丁（add_log_alert_silence_fields.sql）
-- ============================================

-- 为 log_alert_history 添加静默字段（如列已存在可忽略报错）
ALTER TABLE `log_alert_history`
ADD COLUMN `silenced` tinyint(1) DEFAULT 0 COMMENT '是否被静默';

ALTER TABLE `log_alert_history`
ADD COLUMN `silence_id` int unsigned DEFAULT NULL COMMENT '静默规则ID';

CREATE INDEX IF NOT EXISTS `idx_log_alert_history_silenced` ON `log_alert_history`(`silenced`);
CREATE INDEX IF NOT EXISTS `idx_log_alert_history_silence_id` ON `log_alert_history`(`silence_id`);

-- ============================================
-- 6. system_configs 补充 deleted_at 字段
-- ============================================

ALTER TABLE `system_configs`
ADD COLUMN `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间' AFTER `updated_at`;

CREATE INDEX IF NOT EXISTS `idx_sc_deleted_at` ON `system_configs`(`deleted_at`);

-- ============================================
-- 7. message_templates 补充 deleted_at 字段
-- ============================================

ALTER TABLE `message_templates`
ADD COLUMN `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间' AFTER `updated_at`;

CREATE INDEX IF NOT EXISTS `idx_mt_deleted_at` ON `message_templates`(`deleted_at`);

-- ============================================
-- 8. message_templates 列名与 Go 模型对齐
-- ============================================

-- 将 msg_type 重命名为 type
ALTER TABLE `message_templates`
  CHANGE COLUMN `msg_type` `type` varchar(50) NOT NULL DEFAULT 'text' COMMENT '模板类型: text/markdown/card';

-- 添加 is_active 字段
ALTER TABLE `message_templates`
  ADD COLUMN `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否激活' AFTER `description`;

-- 删除不再使用的字段（platform / title / variables）
ALTER TABLE `message_templates`
  DROP COLUMN IF EXISTS `platform`,
  DROP COLUMN IF EXISTS `title`,
  DROP COLUMN IF EXISTS `variables`;

-- 修正 created_by 允许 NULL（模型为 *uint）
ALTER TABLE `message_templates`
  MODIFY COLUMN `created_by` bigint unsigned DEFAULT NULL;

-- ============================================
-- 9. 数据库表结构与 Go Model 一致性修复补丁
-- ============================================
-- 注意：以下补丁已合并到 init_tables.sql（2026-04-14）
-- 全新部署无需执行此节，仅对存量数据库执行

-- 9.1 修复 k8s_clusters 表字段
ALTER TABLE `k8s_clusters`
  DROP COLUMN IF EXISTS `api_server`,
  DROP COLUMN IF EXISTS `token`,
  DROP COLUMN IF EXISTS `ca_cert`;

ALTER TABLE `k8s_clusters`
  ADD COLUMN IF NOT EXISTS `namespace` varchar(100) DEFAULT 'default' NOT NULL COMMENT '默认命名空间' AFTER `kubeconfig`,
  ADD COLUMN IF NOT EXISTS `registry` varchar(500) DEFAULT '' COMMENT '镜像仓库地址' AFTER `namespace`,
  ADD COLUMN IF NOT EXISTS `repository` varchar(200) DEFAULT '' COMMENT '镜像仓库名称' AFTER `registry`,
  ADD COLUMN IF NOT EXISTS `insecure_skip_tls` tinyint(1) DEFAULT 0 COMMENT '跳过 TLS 证书验证' AFTER `is_default`,
  ADD COLUMN IF NOT EXISTS `check_timeout` int DEFAULT 180 NOT NULL COMMENT '健康检查超时时间(秒)' AFTER `insecure_skip_tls`,
  ADD COLUMN IF NOT EXISTS `updated_by` bigint unsigned DEFAULT NULL COMMENT '更新者ID' AFTER `created_by`;

CREATE INDEX IF NOT EXISTS `idx_k8s_updated_by` ON `k8s_clusters`(`updated_by`);

-- 9.2 重建 feishu_requests 表
DROP TABLE IF EXISTS `feishu_requests`;
CREATE TABLE `feishu_requests` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `request_id` varchar(100) NOT NULL COMMENT '请求ID',
  `original_request` text COMMENT '原始请求内容',
  `disabled_actions` text COMMENT '禁用的操作',
  `action_counts` text COMMENT '操作计数',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_fr_request_id` (`request_id`),
  KEY `idx_fr_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='飞书请求记录';

-- 9.3 修复 application_envs 表
ALTER TABLE `application_envs`
  CHANGE COLUMN `env` `env_name` varchar(50) NOT NULL COMMENT '环境名称';

ALTER TABLE `application_envs`
  DROP COLUMN IF EXISTS `jenkins_instance_id`;

ALTER TABLE `application_envs`
  ADD COLUMN IF NOT EXISTS `branch` varchar(100) DEFAULT '' COMMENT 'Git 分支' AFTER `env_name`;

ALTER TABLE `application_envs`
  ADD COLUMN IF NOT EXISTS `k8s_cluster_id` bigint unsigned DEFAULT NULL COMMENT 'K8s集群ID' AFTER `branch`;

ALTER TABLE `application_envs`
  ADD COLUMN IF NOT EXISTS `gitops_repo_id` bigint unsigned DEFAULT NULL COMMENT 'GitOps部署仓库ID' AFTER `branch`,
  ADD COLUMN IF NOT EXISTS `argocd_application_id` bigint unsigned DEFAULT NULL COMMENT 'ArgoCD应用ID' AFTER `gitops_repo_id`,
  ADD COLUMN IF NOT EXISTS `gitops_branch` varchar(200) DEFAULT '' COMMENT 'GitOps目标分支' AFTER `argocd_application_id`,
  ADD COLUMN IF NOT EXISTS `gitops_path` varchar(500) DEFAULT '' COMMENT 'GitOps部署目录' AFTER `gitops_branch`,
  ADD COLUMN IF NOT EXISTS `helm_chart_path` varchar(500) DEFAULT '' COMMENT 'Helm Chart路径' AFTER `gitops_path`,
  ADD COLUMN IF NOT EXISTS `helm_values_path` varchar(500) DEFAULT '' COMMENT 'Helm values文件路径' AFTER `helm_chart_path`,
  ADD COLUMN IF NOT EXISTS `helm_release_name` varchar(200) DEFAULT '' COMMENT 'Helm Release名称' AFTER `helm_values_path`,
  ADD COLUMN IF NOT EXISTS `cpu_request` varchar(50) DEFAULT '' COMMENT 'CPU request' AFTER `replicas`,
  ADD COLUMN IF NOT EXISTS `cpu_limit` varchar(50) DEFAULT '' COMMENT 'CPU limit' AFTER `cpu_request`,
  ADD COLUMN IF NOT EXISTS `memory_request` varchar(50) DEFAULT '' COMMENT 'Memory request' AFTER `cpu_limit`,
  ADD COLUMN IF NOT EXISTS `memory_limit` varchar(50) DEFAULT '' COMMENT 'Memory limit' AFTER `memory_request`;

CREATE INDEX IF NOT EXISTS `idx_app_env_k8s_cluster` ON `application_envs`(`k8s_cluster_id`);
CREATE INDEX IF NOT EXISTS `idx_app_env_gitops_repo` ON `application_envs`(`gitops_repo_id`);
CREATE INDEX IF NOT EXISTS `idx_app_env_argocd_app` ON `application_envs`(`argocd_application_id`);

ALTER TABLE `applications`
  DROP INDEX IF EXISTS `idx_k8s_cluster`,
  DROP COLUMN IF EXISTS `k8s_cluster_id`,
  DROP COLUMN IF EXISTS `k8s_namespace`,
  DROP COLUMN IF EXISTS `k8s_deployment`;

-- 9.4 修复 artifact_repositories 表
ALTER TABLE `artifact_repositories`
  DROP COLUMN IF EXISTS `check_status`,
  DROP COLUMN IF EXISTS `check_message`,
  DROP COLUMN IF EXISTS `check_latency_ms`,
  DROP COLUMN IF EXISTS `total_images`,
  DROP COLUMN IF EXISTS `total_size_bytes`;

ALTER TABLE `artifact_repositories`
  ADD COLUMN IF NOT EXISTS `connection_status` varchar(20) DEFAULT 'unknown' COMMENT '连接状态' AFTER `enabled`,
  ADD COLUMN IF NOT EXISTS `last_error` text COMMENT '最后错误信息' AFTER `last_check_at`,
  ADD COLUMN IF NOT EXISTS `enable_monitoring` tinyint(1) DEFAULT 1 COMMENT '是否启用监控' AFTER `last_error`,
  ADD COLUMN IF NOT EXISTS `check_interval` int DEFAULT 300 COMMENT '检查间隔(秒)' AFTER `enable_monitoring`;

CREATE INDEX IF NOT EXISTS `idx_connection_status` ON `artifact_repositories`(`connection_status`);
CREATE INDEX IF NOT EXISTS `idx_enable_monitoring` ON `artifact_repositories`(`enable_monitoring`);

-- 9.5 修复 artifacts 表字段名
ALTER TABLE `artifacts`
  CHANGE COLUMN `download_cnt` `download_count` bigint DEFAULT 0 COMMENT '下载次数',
  CHANGE COLUMN `latest_version` `latest_ver` varchar(100) DEFAULT NULL COMMENT '最新版本';

-- 9.6 修复 artifact_versions 表字段名
ALTER TABLE `artifact_versions`
  CHANGE COLUMN `download_cnt` `download_count` bigint DEFAULT 0 COMMENT '下载次数';

-- 9.7 修复 alert_histories 表
ALTER TABLE `alert_histories`
  DROP COLUMN IF EXISTS `config_name`,
  DROP COLUMN IF EXISTS `target`,
  DROP COLUMN IF EXISTS `details`,
  DROP COLUMN IF EXISTS `notified`,
  DROP COLUMN IF EXISTS `notified_at`;

ALTER TABLE `alert_histories`
  ADD COLUMN IF NOT EXISTS `title` varchar(200) DEFAULT '' COMMENT '标题' AFTER `type`,
  ADD COLUMN IF NOT EXISTS `content` text COMMENT '内容' AFTER `title`,
  ADD COLUMN IF NOT EXISTS `level` varchar(20) DEFAULT 'warning' COMMENT '级别' AFTER `content`,
  ADD COLUMN IF NOT EXISTS `ack_status` varchar(20) DEFAULT 'pending' COMMENT '确认状态' AFTER `status`,
  ADD COLUMN IF NOT EXISTS `ack_by` bigint unsigned DEFAULT NULL COMMENT '确认人ID' AFTER `ack_status`,
  ADD COLUMN IF NOT EXISTS `ack_at` datetime(3) DEFAULT NULL COMMENT '确认时间' AFTER `ack_by`,
  ADD COLUMN IF NOT EXISTS `resolved_by` bigint unsigned DEFAULT NULL COMMENT '解决人ID' AFTER `ack_at`,
  ADD COLUMN IF NOT EXISTS `resolved_at` datetime(3) DEFAULT NULL COMMENT '解决时间' AFTER `resolved_by`,
  ADD COLUMN IF NOT EXISTS `resolve_comment` text COMMENT '解决备注' AFTER `resolved_at`,
  ADD COLUMN IF NOT EXISTS `silenced` tinyint(1) DEFAULT 0 COMMENT '是否被静默' AFTER `resolve_comment`,
  ADD COLUMN IF NOT EXISTS `silence_id` bigint unsigned DEFAULT NULL COMMENT '静默规则ID' AFTER `silenced`,
  ADD COLUMN IF NOT EXISTS `escalated` tinyint(1) DEFAULT 0 COMMENT '是否已升级' AFTER `silence_id`,
  ADD COLUMN IF NOT EXISTS `escalation_id` bigint unsigned DEFAULT NULL COMMENT '升级规则ID' AFTER `escalated`,
  ADD COLUMN IF NOT EXISTS `error_msg` text COMMENT '错误信息' AFTER `escalation_id`,
  ADD COLUMN IF NOT EXISTS `source_id` varchar(100) DEFAULT '' COMMENT '来源ID' AFTER `error_msg`,
  ADD COLUMN IF NOT EXISTS `source_url` varchar(500) DEFAULT '' COMMENT '来源URL' AFTER `source_id`;

-- 9.8 修复其他表
ALTER TABLE `dingtalk_bots`
  DROP COLUMN IF EXISTS `project`,
  DROP COLUMN IF EXISTS `message_template_id`;

CREATE INDEX IF NOT EXISTS `idx_wwb_created_by` ON `wechat_work_bots`(`created_by`);

ALTER TABLE `feishu_apps`
  MODIFY COLUMN `project` varchar(100) NOT NULL COMMENT '所属项目',
  MODIFY COLUMN `description` text COMMENT '描述',
  MODIFY COLUMN `status` varchar(20) NOT NULL COMMENT '状态: active/inactive';

ALTER TABLE `feishu_bots`
  MODIFY COLUMN `secret` varchar(100) DEFAULT '' COMMENT '签名密钥';

-- 9.9 创建缺失的流水线相关表
CREATE TABLE IF NOT EXISTS `pipeline_runs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `pipeline_id` bigint unsigned NOT NULL COMMENT '流水线ID',
  `run_number` int NOT NULL COMMENT '运行编号',
  `status` varchar(50) DEFAULT 'pending' COMMENT '状态: pending/running/success/failed/cancelled',
  `trigger_type` varchar(50) DEFAULT 'manual' COMMENT '触发类型: manual/webhook/schedule',
  `trigger_user` varchar(100) DEFAULT NULL COMMENT '触发用户',
  `start_time` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `end_time` datetime(3) DEFAULT NULL COMMENT '结束时间',
  `duration` int DEFAULT 0 COMMENT '持续时间(秒)',
  PRIMARY KEY (`id`),
  KEY `idx_pr_pipeline_id` (`pipeline_id`),
  KEY `idx_pr_status` (`status`),
  KEY `idx_pr_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='流水线运行记录';

CREATE TABLE IF NOT EXISTS `pipeline_variables` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `pipeline_id` bigint unsigned NOT NULL COMMENT '流水线ID',
  `key` varchar(100) NOT NULL COMMENT '变量名',
  `value` text COMMENT '变量值',
  `is_secret` tinyint(1) DEFAULT 0 COMMENT '是否为敏感信息',
  `description` text COMMENT '描述',
  PRIMARY KEY (`id`),
  KEY `idx_pv_pipeline_id` (`pipeline_id`),
  KEY `idx_pv_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='流水线变量';

CREATE TABLE IF NOT EXISTS `stage_runs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `pipeline_run_id` bigint unsigned NOT NULL COMMENT '流水线运行ID',
  `stage_name` varchar(100) NOT NULL COMMENT '阶段名称',
  `status` varchar(50) DEFAULT 'pending' COMMENT '状态',
  `start_time` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `end_time` datetime(3) DEFAULT NULL COMMENT '结束时间',
  `duration` int DEFAULT 0 COMMENT '持续时间(秒)',
  PRIMARY KEY (`id`),
  KEY `idx_sr_pipeline_run_id` (`pipeline_run_id`),
  KEY `idx_sr_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='阶段运行记录';

CREATE TABLE IF NOT EXISTS `step_runs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `stage_run_id` bigint unsigned NOT NULL COMMENT '阶段运行ID',
  `step_name` varchar(100) NOT NULL COMMENT '步骤名称',
  `status` varchar(50) DEFAULT 'pending' COMMENT '状态',
  `start_time` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `end_time` datetime(3) DEFAULT NULL COMMENT '结束时间',
  `duration` int DEFAULT 0 COMMENT '持续时间(秒)',
  `logs` longtext COMMENT '日志',
  PRIMARY KEY (`id`),
  KEY `idx_sr_stage_run_id` (`stage_run_id`),
  KEY `idx_sr_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='步骤运行记录';

CREATE TABLE IF NOT EXISTS `webhook_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `webhook_id` bigint unsigned NOT NULL COMMENT 'Webhook ID',
  `request_method` varchar(20) NOT NULL COMMENT '请求方法',
  `request_headers` text COMMENT '请求头',
  `request_body` longtext COMMENT '请求体',
  `response_status` int DEFAULT NULL COMMENT '响应状态码',
  `response_body` text COMMENT '响应体',
  `error_message` text COMMENT '错误信息',
  PRIMARY KEY (`id`),
  KEY `idx_wl_webhook_id` (`webhook_id`),
  KEY `idx_wl_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Webhook日志';

-- 9.2 为现有表添加缺失字段

-- alert_histories 表
ALTER TABLE `alert_histories` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_ah_deleted_at` ON `alert_histories`(`deleted_at`);

-- artifacts 表
ALTER TABLE `artifacts` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_artifacts_deleted_at` ON `artifacts`(`deleted_at`);

-- pipelines 表
ALTER TABLE `pipelines` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_pipelines_deleted_at` ON `pipelines`(`deleted_at`);

-- health_check_configs 表
ALTER TABLE `health_check_configs` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_hcc_deleted_at` ON `health_check_configs`(`deleted_at`);

-- health_check_histories 表
ALTER TABLE `health_check_histories` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_hch_deleted_at` ON `health_check_histories`(`deleted_at`);

-- app_retry_rules 表
ALTER TABLE `app_retry_rules` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_arr_deleted_at` ON `app_retry_rules`(`deleted_at`);

-- app_timeout_rules 表
ALTER TABLE `app_timeout_rules` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_atr_deleted_at` ON `app_timeout_rules`(`deleted_at`);

-- app_circuit_breaker_rules 表
ALTER TABLE `app_circuit_breaker_rules` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_acbr_deleted_at` ON `app_circuit_breaker_rules`(`deleted_at`);

-- app_rate_limit_rules 表
ALTER TABLE `app_rate_limit_rules` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_arlr_deleted_at` ON `app_rate_limit_rules`(`deleted_at`);

-- app_mirror_rules 表
ALTER TABLE `app_mirror_rules` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_amr_deleted_at` ON `app_mirror_rules`(`deleted_at`);

-- app_fault_rules 表
ALTER TABLE `app_fault_rules` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_afr_deleted_at` ON `app_fault_rules`(`deleted_at`);

-- cost_alerts 表
ALTER TABLE `cost_alerts` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_ca_deleted_at` ON `cost_alerts`(`deleted_at`);

-- cost_budgets 表
ALTER TABLE `cost_budgets` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_cb_deleted_at` ON `cost_budgets`(`deleted_at`);

-- cost_suggestions 表
ALTER TABLE `cost_suggestions` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_cs_deleted_at` ON `cost_suggestions`(`deleted_at`);

-- cost_summaries 表
ALTER TABLE `cost_summaries` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_cost_summaries_deleted_at` ON `cost_summaries`(`deleted_at`);

-- resource_costs 表
ALTER TABLE `resource_costs` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_rc_deleted_at` ON `resource_costs`(`deleted_at`);

-- resource_activities 表
ALTER TABLE `resource_activities` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_ra_deleted_at` ON `resource_activities`(`deleted_at`);

-- image_registries 表
ALTER TABLE `image_registries` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_ir_deleted_at` ON `image_registries`(`deleted_at`);

-- image_scans 表
ALTER TABLE `image_scans` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_is_deleted_at` ON `image_scans`(`deleted_at`);

-- security_audit_logs 表
ALTER TABLE `security_audit_logs` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_sal_deleted_at` ON `security_audit_logs`(`deleted_at`);

-- security_reports 表
ALTER TABLE `security_reports` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_sr_deleted_at` ON `security_reports`(`deleted_at`);

-- compliance_rules 表
ALTER TABLE `compliance_rules` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_cr_deleted_at` ON `compliance_rules`(`deleted_at`);

-- config_checks 表
ALTER TABLE `config_checks` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_cc_deleted_at` ON `config_checks`(`deleted_at`);

-- encryption_keys 表
ALTER TABLE `encryption_keys` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_ek_deleted_at` ON `encryption_keys`(`deleted_at`);

-- feishu_requests 表
ALTER TABLE `feishu_requests` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_fr_deleted_at` ON `feishu_requests`(`deleted_at`);

-- k8s_clusters 表
ALTER TABLE `k8s_clusters` ADD COLUMN IF NOT EXISTS `deleted_at` datetime(3) DEFAULT NULL COMMENT '软删除时间';
CREATE INDEX IF NOT EXISTS `idx_kc_deleted_at` ON `k8s_clusters`(`deleted_at`);

-- 9.10 补充缺失字段（2026-04-14）

-- health_check_configs 表补充字段
ALTER TABLE `health_check_configs`
  ADD COLUMN IF NOT EXISTS `last_status` varchar(20) DEFAULT 'unknown' COMMENT '最后检查状态' AFTER `enabled`,
  ADD COLUMN IF NOT EXISTS `last_checked_at` datetime(3) DEFAULT NULL COMMENT '最后检查时间' AFTER `last_status`;

CREATE INDEX IF NOT EXISTS `idx_hcc_last_status` ON `health_check_configs`(`last_status`);

-- resource_costs 表补充字段
ALTER TABLE `resource_costs`
  ADD COLUMN IF NOT EXISTS `total_cost` decimal(14,4) DEFAULT 0.0000 COMMENT '总成本' AFTER `cost`,
  ADD COLUMN IF NOT EXISTS `cpu_request` decimal(10,2) DEFAULT 0.00 COMMENT 'CPU 请求量(核)' AFTER `cpu_cost`,
  ADD COLUMN IF NOT EXISTS `memory_request` decimal(10,2) DEFAULT 0.00 COMMENT '内存请求量(GB)' AFTER `memory_cost`;

-- cost_suggestions 表补充字段
ALTER TABLE `cost_suggestions`
  ADD COLUMN IF NOT EXISTS `savings` decimal(14,4) DEFAULT 0.0000 COMMENT '预计节省金额' AFTER `estimated_saving`;

-- cost_budgets 表补充字段
ALTER TABLE `cost_budgets`
  ADD COLUMN IF NOT EXISTS `monthly_budget` decimal(14,4) DEFAULT 0.0000 COMMENT '月度预算' AFTER `amount`,
  ADD COLUMN IF NOT EXISTS `current_cost` decimal(14,4) DEFAULT 0.0000 COMMENT '当前花费' AFTER `monthly_budget`;

-- resource_activities 表补充字段
ALTER TABLE `resource_activities`
  ADD COLUMN IF NOT EXISTS `is_zombie` tinyint(1) DEFAULT 0 COMMENT '是否为僵尸资源' AFTER `operator`,
  ADD COLUMN IF NOT EXISTS `last_active_at` datetime(3) DEFAULT NULL COMMENT '最后活跃时间' AFTER `is_zombie`;

CREATE INDEX IF NOT EXISTS `idx_ra_is_zombie` ON `resource_activities`(`is_zombie`);

-- image_scans 表补充字段
ALTER TABLE `image_scans`
  ADD COLUMN IF NOT EXISTS `status` varchar(20) DEFAULT 'pending' COMMENT '扫描状态' AFTER `scan_status`;

CREATE INDEX IF NOT EXISTS `idx_is_status` ON `image_scans`(`status`);

-- config_checks 表补充字段
ALTER TABLE `config_checks`
  ADD COLUMN IF NOT EXISTS `critical_count` int DEFAULT 0 COMMENT '严重问题数' AFTER `status`,
  ADD COLUMN IF NOT EXISTS `high_count` int DEFAULT 0 COMMENT '高危问题数' AFTER `critical_count`,
  ADD COLUMN IF NOT EXISTS `medium_count` int DEFAULT 0 COMMENT '中危问题数' AFTER `high_count`,
  ADD COLUMN IF NOT EXISTS `low_count` int DEFAULT 0 COMMENT '低危问题数' AFTER `medium_count`,
  ADD COLUMN IF NOT EXISTS `passed_count` int DEFAULT 0 COMMENT '通过数' AFTER `low_count`;

CREATE INDEX IF NOT EXISTS `idx_cc_checked_at` ON `config_checks`(`checked_at`);

-- ============================================
-- 10. 修复 test.log 中的字段缺失问题（2026-04-14）
-- ============================================

-- 10.1 health_check_configs 表补充字段
ALTER TABLE `health_check_configs`
  ADD COLUMN IF NOT EXISTS `type` varchar(50) DEFAULT 'http' COMMENT '检查类型: http/tcp/ssl_cert/dns' AFTER `name`,
  ADD COLUMN IF NOT EXISTS `target_id` bigint unsigned DEFAULT 0 COMMENT '目标资源ID' AFTER `type`,
  ADD COLUMN IF NOT EXISTS `target_name` varchar(200) DEFAULT '' COMMENT '目标名称' AFTER `target_id`,
  ADD COLUMN IF NOT EXISTS `retry_count` int DEFAULT 3 COMMENT '重试次数' AFTER `timeout`,
  ADD COLUMN IF NOT EXISTS `alert_enabled` tinyint(1) DEFAULT 1 COMMENT '是否启用告警' AFTER `enabled`,
  ADD COLUMN IF NOT EXISTS `alert_platform` varchar(50) DEFAULT '' COMMENT '告警平台' AFTER `alert_enabled`,
  ADD COLUMN IF NOT EXISTS `alert_bot_id` bigint unsigned DEFAULT NULL COMMENT '告警机器人ID' AFTER `alert_platform`,
  ADD COLUMN IF NOT EXISTS `last_check_at` datetime(3) DEFAULT NULL COMMENT '最后检查时间' AFTER `alert_bot_id`,
  ADD COLUMN IF NOT EXISTS `last_error` text COMMENT '最后错误信息' AFTER `last_status`,
  ADD COLUMN IF NOT EXISTS `cert_expiry_date` datetime(3) DEFAULT NULL COMMENT '证书过期时间' AFTER `last_error`,
  ADD COLUMN IF NOT EXISTS `cert_days_remaining` int DEFAULT NULL COMMENT 'SSL证书剩余天数' AFTER `cert_expiry_date`,
  ADD COLUMN IF NOT EXISTS `cert_issuer` varchar(500) DEFAULT '' COMMENT '证书颁发者' AFTER `cert_days_remaining`,
  ADD COLUMN IF NOT EXISTS `cert_subject` varchar(500) DEFAULT '' COMMENT '证书主题' AFTER `cert_issuer`,
  ADD COLUMN IF NOT EXISTS `cert_serial_number` varchar(100) DEFAULT '' COMMENT '证书序列号' AFTER `cert_subject`,
  ADD COLUMN IF NOT EXISTS `critical_days` int DEFAULT 7 COMMENT '严重告警阈值（天）' AFTER `cert_serial_number`,
  ADD COLUMN IF NOT EXISTS `warning_days` int DEFAULT 30 COMMENT '警告告警阈值（天）' AFTER `critical_days`,
  ADD COLUMN IF NOT EXISTS `notice_days` int DEFAULT 60 COMMENT '提醒告警阈值（天）' AFTER `warning_days`,
  ADD COLUMN IF NOT EXISTS `last_alert_level` varchar(20) DEFAULT NULL COMMENT '最后告警级别: info/warning/error/critical' AFTER `notice_days`,
  ADD COLUMN IF NOT EXISTS `last_alert_at` datetime(3) DEFAULT NULL COMMENT '最后告警时间' AFTER `last_alert_level`;

CREATE INDEX IF NOT EXISTS `idx_hcc_type` ON `health_check_configs`(`type`);
CREATE INDEX IF NOT EXISTS `idx_hcc_target_id` ON `health_check_configs`(`target_id`);
CREATE INDEX IF NOT EXISTS `idx_hcc_alert_bot_id` ON `health_check_configs`(`alert_bot_id`);
CREATE INDEX IF NOT EXISTS `idx_hcc_cert_days` ON `health_check_configs`(`cert_days_remaining`);
CREATE INDEX IF NOT EXISTS `idx_hcc_alert_level` ON `health_check_configs`(`last_alert_level`);

-- 10.2 log_saved_queries 表补充字段
ALTER TABLE `log_saved_queries`
  ADD COLUMN IF NOT EXISTS `is_shared` tinyint(1) DEFAULT 0 COMMENT '是否共享给团队' AFTER `is_public`;

CREATE INDEX IF NOT EXISTS `idx_lsq_is_shared` ON `log_saved_queries`(`is_shared`);

-- 10.3 resource_costs 表补充字段
ALTER TABLE `resource_costs`
  ADD COLUMN IF NOT EXISTS `recorded_at` datetime(3) DEFAULT NULL COMMENT '成本记录时间' AFTER `period_end`,
  ADD COLUMN IF NOT EXISTS `app_name` varchar(100) DEFAULT '' COMMENT '应用名称' AFTER `resource_name`,
  ADD COLUMN IF NOT EXISTS `team_name` varchar(100) DEFAULT '' COMMENT '团队名称' AFTER `app_name`,
  ADD COLUMN IF NOT EXISTS `cpu_usage` decimal(10,2) DEFAULT 0.00 COMMENT 'CPU 实际使用量(核)' AFTER `cpu_request`,
  ADD COLUMN IF NOT EXISTS `cpu_limit` decimal(10,2) DEFAULT 0.00 COMMENT 'CPU 限制量(核)' AFTER `cpu_usage`,
  ADD COLUMN IF NOT EXISTS `memory_usage` decimal(10,2) DEFAULT 0.00 COMMENT '内存实际使用量(GB)' AFTER `memory_request`,
  ADD COLUMN IF NOT EXISTS `memory_limit` decimal(10,2) DEFAULT 0.00 COMMENT '内存限制量(GB)' AFTER `memory_usage`,
  ADD COLUMN IF NOT EXISTS `storage_size` decimal(10,2) DEFAULT 0.00 COMMENT '存储大小(GB)' AFTER `memory_limit`;

CREATE INDEX IF NOT EXISTS `idx_rc_recorded_at` ON `resource_costs`(`recorded_at`);
CREATE INDEX IF NOT EXISTS `idx_rc_app_name` ON `resource_costs`(`app_name`);
CREATE INDEX IF NOT EXISTS `idx_rc_team_name` ON `resource_costs`(`team_name`);

-- 10.4 为现有数据初始化字段值
UPDATE `health_check_configs`
SET `type` = CASE
  WHEN `url` LIKE 'https://%' OR `url` LIKE 'http://%' THEN 'http'
  WHEN `url` LIKE 'tcp://%' THEN 'tcp'
  ELSE 'http'
END
WHERE `type` IS NULL OR `type` = '';

UPDATE `resource_costs`
SET `recorded_at` = `created_at`
WHERE `recorded_at` IS NULL;

-- 10.5 pipelines 表结构与当前 Go 模型对齐
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'description';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN description TEXT COMMENT ''描述'' AFTER name',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'project_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN project_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''项目ID'' AFTER description',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'source_template_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN source_template_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''来源模板ID'' AFTER project_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'git_repo_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN git_repo_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''Git 仓库ID'' AFTER project_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'git_branch';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN git_branch VARCHAR(100) DEFAULT ''main'' COMMENT ''Git 分支'' AFTER git_repo_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'config_json';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN config_json LONGTEXT COMMENT ''流水线配置 JSON'' AFTER git_branch',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'trigger_config_json';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN trigger_config_json TEXT COMMENT ''触发配置 JSON'' AFTER config_json',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'trigger_config';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN trigger_config TEXT COMMENT ''Webhook 触发配置'' AFTER trigger_config_json',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'last_run_at';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN last_run_at DATETIME(3) DEFAULT NULL COMMENT ''最近运行时间'' AFTER status',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'last_run_status';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN last_run_status VARCHAR(20) DEFAULT '''' COMMENT ''最近运行状态'' AFTER last_run_at',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @index_exists = 0;
SELECT COUNT(*) INTO @index_exists
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND INDEX_NAME = 'idx_p_project_id';
SET @sql = IF(@index_exists = 0,
  'CREATE INDEX idx_p_project_id ON pipelines(project_id)',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @index_exists = 0;
SELECT COUNT(*) INTO @index_exists
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND INDEX_NAME = 'idx_p_source_template_id';
SET @sql = IF(@index_exists = 0,
  'CREATE INDEX idx_p_source_template_id ON pipelines(source_template_id)',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @index_exists = 0;
SELECT COUNT(*) INTO @index_exists
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND INDEX_NAME = 'idx_p_git_repo_id';
SET @sql = IF(@index_exists = 0,
  'CREATE INDEX idx_p_git_repo_id ON pipelines(git_repo_id)',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @index_exists = 0;
SELECT COUNT(*) INTO @index_exists
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND INDEX_NAME = 'idx_p_status';
SET @sql = IF(@index_exists = 0,
  'CREATE INDEX idx_p_status ON pipelines(status)',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE `pipelines`
SET `config_json` = `config`
WHERE (`config_json` IS NULL OR `config_json` = '')
  AND `config` IS NOT NULL
  AND `config` != '';

UPDATE `pipelines`
SET `git_branch` = 'main'
WHERE `git_branch` IS NULL OR `git_branch` = '';

UPDATE `pipelines`
SET `last_run_status` = `last_build_status`
WHERE (`last_run_status` IS NULL OR `last_run_status` = '')
  AND `last_build_status` IS NOT NULL
  AND `last_build_status` != '';

-- ============================================
-- 11. Telegram 通知
-- ============================================
CREATE TABLE IF NOT EXISTS `telegram_bots` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '机器人名称',
  `token` varchar(200) NOT NULL COMMENT 'Bot Token',
  `default_chat_id` varchar(100) DEFAULT '' COMMENT '默认 Chat ID',
  `api_base_url` varchar(200) DEFAULT '' COMMENT '自定义 API 地址(代理)',
  `description` text,
  `status` varchar(20) NOT NULL DEFAULT 'active' COMMENT '状态: active/inactive',
  `is_default` tinyint(1) DEFAULT 0 COMMENT '是否默认机器人',
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tg_bots_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Telegram 机器人';

CREATE TABLE IF NOT EXISTS `telegram_message_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `bot_id` bigint unsigned NOT NULL COMMENT 'Telegram 机器人ID',
  `chat_id` varchar(100) DEFAULT '' COMMENT '接收 Chat ID',
  `parse_mode` varchar(20) DEFAULT '' COMMENT '解析模式: MarkdownV2/HTML',
  `content` text,
  `source` varchar(50) DEFAULT '' COMMENT '来源: manual/alert/...',
  `status` varchar(20) NOT NULL DEFAULT 'success' COMMENT '状态: success/failed',
  `error_msg` text,
  PRIMARY KEY (`id`),
  KEY `idx_tg_logs_bot` (`bot_id`),
  KEY `idx_tg_logs_source` (`source`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Telegram 消息发送日志';


-- ============================================
-- 12. 数据库管理模块（Phase 1）
-- ============================================

-- 数据库实例表
CREATE TABLE IF NOT EXISTS `db_instances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '实例名称',
  `db_type` varchar(20) NOT NULL DEFAULT 'mysql' COMMENT '数据库类型: mysql/postgres',
  `env` varchar(20) NOT NULL DEFAULT 'dev' COMMENT '环境: dev/test/staging/prod',
  `host` varchar(200) NOT NULL COMMENT '主机',
  `port` int NOT NULL DEFAULT 3306 COMMENT '端口',
  `username` varchar(100) NOT NULL COMMENT '账号',
  `password` varchar(500) NOT NULL COMMENT '加密后的密码',
  `default_db` varchar(100) DEFAULT '' COMMENT '默认库',
  `exclude_dbs` varchar(500) DEFAULT '' COMMENT '屏蔽库列表, 逗号分隔',
  `params` varchar(500) DEFAULT '' COMMENT 'DSN 扩展参数',
  `mode` tinyint(2) NOT NULL DEFAULT 0 COMMENT '访问模式: 0写 1读 2读写',
  `status` varchar(20) NOT NULL DEFAULT 'active' COMMENT '状态: active/inactive',
  `description` text,
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_db_instance_name` (`name`),
  KEY `idx_db_instance_env` (`env`),
  KEY `idx_db_instance_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库实例';

-- 查询控制台日志
CREATE TABLE IF NOT EXISTS `db_query_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `instance_id` bigint unsigned NOT NULL,
  `username` varchar(100) NOT NULL COMMENT '执行用户',
  `schema_name` varchar(100) DEFAULT '' COMMENT '数据库',
  `sql_text` text NOT NULL COMMENT 'SQL 原文',
  `affect_rows` int NOT NULL DEFAULT 0,
  `exec_ms` int NOT NULL DEFAULT 0 COMMENT '执行耗时(ms)',
  `status` varchar(20) NOT NULL DEFAULT 'success' COMMENT '状态: success/failed/blocked',
  `error_msg` text,
  PRIMARY KEY (`id`),
  KEY `idx_dbq_instance` (`instance_id`),
  KEY `idx_dbq_user` (`username`),
  KEY `idx_dbq_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库查询日志';

-- SQL 变更工单
CREATE TABLE IF NOT EXISTS `sql_change_tickets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `work_id` varchar(64) NOT NULL COMMENT '工单编号',
  `title` varchar(200) NOT NULL COMMENT '标题',
  `description` text,
  `applicant` varchar(100) NOT NULL COMMENT '申请人',
  `real_name` varchar(100) DEFAULT '' COMMENT '申请人姓名',
  `instance_id` bigint unsigned NOT NULL COMMENT '目标实例',
  `schema_name` varchar(100) NOT NULL COMMENT '目标库',
  `change_type` tinyint(2) NOT NULL DEFAULT 1 COMMENT '类型: 0 DDL 1 DML',
  `need_backup` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否备份回滚',
  `status` tinyint(2) NOT NULL DEFAULT 0 COMMENT '状态: 0 审批中 1 已驳回 2 待执行 3 执行中 4 执行成功 5 执行失败 6 已撤回',
  `execute_time` datetime(3) DEFAULT NULL COMMENT '计划/实际执行时间',
  `delay_mode` varchar(20) DEFAULT 'none' COMMENT '延时执行: none/schedule',
  `approval_instance_id` bigint unsigned DEFAULT NULL COMMENT '关联审批实例',
  `audit_report` json DEFAULT NULL COMMENT '自动审核结果',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sct_work_id` (`work_id`),
  KEY `idx_sct_applicant` (`applicant`),
  KEY `idx_sct_instance` (`instance_id`),
  KEY `idx_sct_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL 变更工单';

-- SQL 语句明细(拆分后)
CREATE TABLE IF NOT EXISTS `sql_change_statements` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `ticket_id` bigint unsigned NOT NULL,
  `work_id` varchar(64) NOT NULL,
  `seq` int NOT NULL DEFAULT 0 COMMENT '执行顺序',
  `sql_text` longtext NOT NULL,
  `affect_rows` int NOT NULL DEFAULT 0,
  `exec_ms` int NOT NULL DEFAULT 0,
  `state` varchar(20) NOT NULL DEFAULT 'pending' COMMENT 'pending/success/failed/skipped',
  `error_msg` text,
  `executed_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_scs_ticket` (`ticket_id`),
  KEY `idx_scs_work_id` (`work_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL 变更语句明细';

-- SQL 回滚脚本
CREATE TABLE IF NOT EXISTS `sql_rollback_scripts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `ticket_id` bigint unsigned NOT NULL,
  `work_id` varchar(64) NOT NULL,
  `statement_id` bigint unsigned DEFAULT NULL,
  `rollback_sql` longtext NOT NULL,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_srs_ticket` (`ticket_id`),
  KEY `idx_srs_work_id` (`work_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL 回滚脚本';

-- SQL 审核规则集
CREATE TABLE IF NOT EXISTS `sql_audit_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '规则集名称',
  `description` text,
  `config` json NOT NULL COMMENT 'AuditRole 配置',
  `is_default` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sar_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL 审核规则集';

-- 扩展 approval_instances 支持多业务对象（若列已存在会报错, 可忽略）
ALTER TABLE `approval_instances`
  ADD COLUMN `target_type` varchar(20) NOT NULL DEFAULT 'deploy' COMMENT '业务对象: deploy/sql_change' AFTER `record_id`,
  ADD COLUMN `target_id` bigint unsigned DEFAULT NULL COMMENT '业务对象ID' AFTER `target_type`,
  ADD KEY `idx_ai_target` (`target_type`, `target_id`);


-- ============================================
-- 13. 数据库管理模块 Phase 2：SQL 变更工单工作流
-- ============================================

-- 工单工作流动作记录
CREATE TABLE IF NOT EXISTS `sql_change_workflow_details` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `ticket_id` bigint unsigned NOT NULL,
  `work_id` varchar(64) NOT NULL,
  `username` varchar(100) NOT NULL COMMENT '操作人',
  `action` varchar(50) NOT NULL COMMENT 'submit/agree/reject/execute/rollback',
  `step` int NOT NULL DEFAULT 0,
  `comment` text,
  PRIMARY KEY (`id`),
  KEY `idx_scwd_ticket` (`ticket_id`),
  KEY `idx_scwd_work_id` (`work_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL 工单工作流明细';

-- 为工单补充 Phase 1 未加的字段
ALTER TABLE `sql_change_tickets`
  ADD COLUMN `audit_config` json DEFAULT NULL COMMENT '多级审批配置 [{step_name,approvers:[]}]' AFTER `audit_report`,
  ADD COLUMN `current_step` int NOT NULL DEFAULT 0 COMMENT '当前审批步骤' AFTER `audit_config`,
  ADD COLUMN `assigned` varchar(500) DEFAULT '' COMMENT '当前处理人列表' AFTER `current_step`;


-- ============================================
-- 14. 数据库管理模块：实例 ACL 权限绑定
-- ============================================

CREATE TABLE IF NOT EXISTS `db_instance_acl` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `instance_id` bigint unsigned NOT NULL COMMENT '实例 ID',
  `subject_type` varchar(10) NOT NULL COMMENT 'user/role',
  `subject_id` bigint unsigned NOT NULL COMMENT '用户或角色 ID',
  `access_level` varchar(20) NOT NULL DEFAULT 'read' COMMENT 'read/write/owner',
  `schema_names` varchar(1000) NOT NULL DEFAULT '' COMMENT '可访问的库名(逗号分隔), 空=全部',
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ia_subject` (`instance_id`, `subject_type`, `subject_id`),
  KEY `idx_ia_instance` (`instance_id`),
  KEY `idx_ia_subject_lookup` (`subject_type`, `subject_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库实例 ACL 权限绑定';

-- ============================================================
-- 15. 环境晋级（一次编译多环境部署 / 镜像晋级）
-- ============================================================

CREATE TABLE IF NOT EXISTS `env_promotion_policies` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `application_id` bigint unsigned NOT NULL,
  `env_chain` varchar(500) NOT NULL DEFAULT '["dev","test","uat","gray","prod"]',
  `need_approval` varchar(500) NOT NULL DEFAULT '{}',
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_epp_app` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='环境晋级策略';

CREATE TABLE IF NOT EXISTS `env_promotion_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `application_id` bigint unsigned NOT NULL,
  `app_name` varchar(100) NOT NULL DEFAULT '',
  `image_url` varchar(1000) NOT NULL,
  `image_tag` varchar(500) NOT NULL,
  `git_commit` varchar(64) NOT NULL DEFAULT '',
  `git_branch` varchar(200) NOT NULL DEFAULT '',
  `source_run_id` bigint unsigned DEFAULT NULL,
  `current_env` varchar(50) NOT NULL DEFAULT 'dev',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_by` bigint unsigned NOT NULL DEFAULT 0,
  `created_by_name` varchar(100) NOT NULL DEFAULT '',
  `finished_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_epr_app` (`application_id`),
  KEY `idx_epr_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='镜像晋级记录';

CREATE TABLE IF NOT EXISTS `env_promotion_steps` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `promotion_id` bigint unsigned NOT NULL,
  `from_env` varchar(50) NOT NULL,
  `to_env` varchar(50) NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'pending',
  `deploy_record_id` bigint unsigned DEFAULT NULL,
  `approver_id` bigint unsigned DEFAULT NULL,
  `approver_name` varchar(100) NOT NULL DEFAULT '',
  `approved_at` datetime(3) DEFAULT NULL,
  `reject_reason` varchar(500) NOT NULL DEFAULT '',
  `operated_by` bigint unsigned DEFAULT NULL,
  `operated_by_name` varchar(100) NOT NULL DEFAULT '',
  `operated_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_eps_promo` (`promotion_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='晋级步骤记录';


-- ============================================================
-- 16. LDAP 认证 - 组→角色映射
-- ============================================================

CREATE TABLE IF NOT EXISTS `ldap_group_mappings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `group_dn` varchar(500) NOT NULL COMMENT 'LDAP 组 DN',
  `group_name` varchar(200) NOT NULL COMMENT '组显示名',
  `role_id` bigint unsigned NOT NULL COMMENT '映射系统角色 ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_lgm_group_dn` (`group_dn`),
  KEY `idx_lgm_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LDAP 组→角色映射';


-- ============================================================
-- 17. Nacos 配置管理 - 多实例管理
-- ============================================================

CREATE TABLE IF NOT EXISTS `nacos_instances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` varchar(100) NOT NULL COMMENT '实例名称',
  `addr` varchar(500) NOT NULL COMMENT 'Nacos 地址(含端口)',
  `username` varchar(100) DEFAULT '' COMMENT '用户名',
  `password` varchar(500) DEFAULT '' COMMENT '加密后的密码',
  `env` varchar(30) NOT NULL DEFAULT 'dev' COMMENT '环境: dev/test/uat/prod',
  `description` text,
  `status` varchar(20) NOT NULL DEFAULT 'active' COMMENT '状态: active/inactive',
  `is_default` tinyint(1) DEFAULT 0 COMMENT '是否默认实例',
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ni_name` (`name`),
  KEY `idx_ni_env` (`env`),
  KEY `idx_ni_status` (`status`),
  KEY `idx_ni_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Nacos 实例';


-- ============================================================
-- 18. 服务目录（org→project→service 三级 + 自定义环境）
-- ============================================================

CREATE TABLE IF NOT EXISTS `organizations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '组织标识',
  `display_name` varchar(200) DEFAULT '' COMMENT '显示名称',
  `description` text,
  `owner` varchar(100) DEFAULT '' COMMENT '负责人',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_org_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='组织';

CREATE TABLE IF NOT EXISTS `projects` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `organization_id` bigint unsigned NOT NULL,
  `name` varchar(100) NOT NULL COMMENT '项目标识',
  `display_name` varchar(200) DEFAULT '' COMMENT '显示名称',
  `description` text,
  `owner` varchar(100) DEFAULT '' COMMENT '负责人',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  PRIMARY KEY (`id`),
  KEY `idx_proj_org` (`organization_id`),
  UNIQUE KEY `idx_proj_org_name` (`organization_id`, `name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目';

CREATE TABLE IF NOT EXISTS `env_definitions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `name` varchar(50) NOT NULL COMMENT '环境标识',
  `display_name` varchar(100) DEFAULT '' COMMENT '显示名称',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '排序',
  `color` varchar(20) DEFAULT 'blue' COMMENT '颜色标签',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ed_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='环境定义';

-- 预置 5 个默认环境
INSERT IGNORE INTO `env_definitions` (`name`, `display_name`, `sort_order`, `color`) VALUES
  ('dev', '开发', 10, 'blue'),
  ('test', '测试', 20, 'cyan'),
  ('uat', 'UAT', 30, 'orange'),
  ('gray', '灰度', 40, 'purple'),
  ('prod', '生产', 50, 'red');

-- 为 applications 表新增 org/project 关联字段
ALTER TABLE `applications`
  ADD COLUMN IF NOT EXISTS `organization_id` bigint unsigned DEFAULT NULL COMMENT '所属组织' AFTER `description`,
  ADD COLUMN IF NOT EXISTS `project_id` bigint unsigned DEFAULT NULL COMMENT '所属项目' AFTER `organization_id`;

CREATE INDEX IF NOT EXISTS `idx_app_org` ON `applications`(`organization_id`);
CREATE INDEX IF NOT EXISTS `idx_app_proj` ON `applications`(`project_id`);

-- 19. 环境审核策略（按环境差异化审核 dev 宽松 / staging 中等 / prod 严格）
CREATE TABLE IF NOT EXISTS `env_audit_policies` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `env_name` varchar(50) NOT NULL COMMENT '环境标识',
  `display_name` varchar(100) DEFAULT '' COMMENT '显示名称',
  `risk_level` varchar(20) DEFAULT 'low' COMMENT '风险等级: low/medium/high/critical',
  `require_approval` tinyint(1) DEFAULT 0 COMMENT '是否需要审批',
  `min_approvers` int DEFAULT 1 COMMENT '最少审批人数',
  `require_chain` tinyint(1) DEFAULT 0 COMMENT '是否要求多级审批链',
  `default_chain_id` bigint unsigned DEFAULT NULL COMMENT '默认审批链ID',
  `require_deploy_window` tinyint(1) DEFAULT 0 COMMENT '是否要求发布窗口',
  `auto_reject_outside_window` tinyint(1) DEFAULT 0 COMMENT '窗口外自动拒绝',
  `require_code_review` tinyint(1) DEFAULT 0 COMMENT '是否要求代码审查',
  `require_test_pass` tinyint(1) DEFAULT 0 COMMENT '是否要求测试通过',
  `allow_emergency` tinyint(1) DEFAULT 1 COMMENT '是否允许紧急发布',
  `allow_rollback` tinyint(1) DEFAULT 1 COMMENT '是否允许回滚',
  `max_deploys_per_day` int DEFAULT 0 COMMENT '每日最大部署次数(0=不限)',
  `enabled` tinyint(1) DEFAULT 1 COMMENT '是否启用',
  `created_by` bigint unsigned DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_env_name` (`env_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='环境审核策略';

INSERT IGNORE INTO `env_audit_policies` (`env_name`, `display_name`, `risk_level`, `require_approval`, `min_approvers`, `require_chain`, `require_deploy_window`, `auto_reject_outside_window`, `require_code_review`, `require_test_pass`, `allow_emergency`, `allow_rollback`, `max_deploys_per_day`, `enabled`) VALUES
('dev', '开发环境', 'low', 0, 0, 0, 0, 0, 0, 0, 1, 1, 0, 1),
('test', '测试环境', 'low', 0, 0, 0, 0, 0, 0, 0, 1, 1, 0, 1),
('staging', '预发环境', 'medium', 1, 1, 0, 1, 0, 1, 1, 1, 1, 10, 1),
('prod', '生产环境', 'high', 1, 2, 1, 1, 1, 1, 1, 1, 1, 5, 1),
('production', '生产环境', 'high', 1, 2, 1, 1, 1, 1, 1, 1, 1, 5, 1);

-- 20. Jira 集成（需求协作对接外部 Jira）
CREATE TABLE IF NOT EXISTS `jira_instances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '实例名称',
  `base_url` varchar(500) NOT NULL COMMENT 'Jira URL',
  `username` varchar(200) DEFAULT '' COMMENT '用户名/邮箱',
  `token` varchar(500) DEFAULT '' COMMENT 'API Token(加密)',
  `auth_type` varchar(20) DEFAULT 'token' COMMENT '认证方式: token/basic',
  `is_default` tinyint(1) DEFAULT 0 COMMENT '是否默认实��',
  `status` varchar(20) DEFAULT 'active' COMMENT '状态',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Jira 实例';

CREATE TABLE IF NOT EXISTS `jira_project_mappings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `jira_instance_id` bigint unsigned NOT NULL COMMENT 'Jira 实例ID',
  `jira_project_key` varchar(50) NOT NULL COMMENT 'Jira 项目 Key',
  `jira_project_name` varchar(200) DEFAULT '' COMMENT 'Jira 项目名称',
  `devops_project_id` bigint unsigned DEFAULT NULL COMMENT 'DevOps 项目ID',
  `devops_app_id` bigint unsigned DEFAULT NULL COMMENT 'DevOps 应用ID',
  PRIMARY KEY (`id`),
  KEY `idx_instance` (`jira_instance_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Jira 项目映射';

-- 21. 值班排班与告警分配
CREATE TABLE IF NOT EXISTS `oncall_schedules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` varchar(100) NOT NULL COMMENT '排班表名称',
  `description` text COMMENT '描述',
  `timezone` varchar(50) DEFAULT 'Asia/Shanghai' COMMENT '时区',
  `rotation_type` varchar(20) DEFAULT 'weekly' COMMENT '轮转类型: daily/weekly/custom',
  `enabled` tinyint(1) DEFAULT 1 COMMENT '是否启用',
  `created_by` bigint unsigned DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='值班排班表';

CREATE TABLE IF NOT EXISTS `oncall_shifts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `schedule_id` bigint unsigned NOT NULL COMMENT '排班表ID',
  `user_id` bigint unsigned NOT NULL COMMENT '值班人ID',
  `user_name` varchar(100) DEFAULT '' COMMENT '值班人姓名',
  `start_time` datetime NOT NULL COMMENT '开始时间',
  `end_time` datetime NOT NULL COMMENT '结束时间',
  `shift_type` varchar(20) DEFAULT 'primary' COMMENT '班次类型: primary/backup',
  PRIMARY KEY (`id`),
  KEY `idx_schedule_time` (`schedule_id`, `start_time`, `end_time`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='值班班次';

CREATE TABLE IF NOT EXISTS `oncall_overrides` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `schedule_id` bigint unsigned NOT NULL COMMENT '排班表ID',
  `original_user_id` bigint unsigned NOT NULL COMMENT '原值班人ID',
  `original_user_name` varchar(100) DEFAULT '' COMMENT '原值班人姓名',
  `override_user_id` bigint unsigned NOT NULL COMMENT '替换人ID',
  `override_user_name` varchar(100) DEFAULT '' COMMENT '替换人姓名',
  `start_time` datetime NOT NULL COMMENT '开始时间',
  `end_time` datetime NOT NULL COMMENT '结束时间',
  `reason` varchar(500) DEFAULT '' COMMENT '替换原因',
  `created_by` bigint unsigned DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_schedule_user` (`schedule_id`, `original_user_id`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='值班临时替换';

CREATE TABLE IF NOT EXISTS `alert_assignments` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `alert_history_id` bigint unsigned NOT NULL COMMENT '告警历史ID',
  `assignee_id` bigint unsigned NOT NULL COMMENT '分配人ID',
  `assignee_name` varchar(100) DEFAULT '' COMMENT '分配人姓名',
  `schedule_id` bigint unsigned DEFAULT NULL COMMENT '排班表ID',
  `status` varchar(20) DEFAULT 'pending' COMMENT '状态: pending/claimed/resolved/escalated',
  `claimed_at` datetime DEFAULT NULL COMMENT '认领时间',
  `resolved_at` datetime DEFAULT NULL COMMENT '解决时间',
  `comment` text COMMENT '处理备注',
  PRIMARY KEY (`id`),
  KEY `idx_alert` (`alert_history_id`),
  KEY `idx_assignee` (`assignee_id`, `status`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='告警分配';

-- 22. SonarQube 代码质量集成
CREATE TABLE IF NOT EXISTS `sonarqube_instances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '实例名称',
  `base_url` varchar(500) NOT NULL COMMENT 'SonarQube URL',
  `token` varchar(500) DEFAULT '' COMMENT 'Token(加密)',
  `is_default` tinyint(1) DEFAULT 0 COMMENT '是否默认实例',
  `status` varchar(20) DEFAULT 'active' COMMENT '状态',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='SonarQube 实例';

CREATE TABLE IF NOT EXISTS `sonarqube_project_bindings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `sonar_qube_id` bigint unsigned NOT NULL COMMENT 'SonarQube 实例ID',
  `sonar_project_key` varchar(200) NOT NULL COMMENT 'SonarQube 项目Key',
  `sonar_project_name` varchar(200) DEFAULT '' COMMENT 'SonarQube 项目名称',
  `devops_app_id` bigint unsigned DEFAULT NULL COMMENT 'DevOps 应用ID',
  `devops_app_name` varchar(200) DEFAULT '' COMMENT 'DevOps 应用名称',
  `quality_gate_status` varchar(20) DEFAULT '' COMMENT '质量门禁状态',
  PRIMARY KEY (`id`),
  KEY `idx_sonar_instance` (`sonar_qube_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='SonarQube 项目绑定';

-- 23. Nacos 配置发布单
CREATE TABLE IF NOT EXISTS `nacos_releases` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `title` varchar(200) NOT NULL COMMENT '发布单标题',
  `nacos_instance_id` bigint unsigned NOT NULL COMMENT 'Nacos 实例ID',
  `nacos_instance_name` varchar(100) DEFAULT '' COMMENT 'Nacos 实例名称',
  `tenant` varchar(200) DEFAULT '' COMMENT '命名空间',
  `group` varchar(200) NOT NULL DEFAULT 'DEFAULT_GROUP' COMMENT '分组',
  `data_id` varchar(200) NOT NULL COMMENT '配置ID',
  `env` varchar(30) NOT NULL DEFAULT 'dev' COMMENT '环境',
  `config_type` varchar(20) DEFAULT 'yaml' COMMENT '配置类型',
  `content_before` longtext COMMENT '变更前内容',
  `content_after` longtext COMMENT '变更后内容',
  `content_hash` varchar(64) DEFAULT '' COMMENT '内容哈希',
  `service_id` bigint unsigned DEFAULT NULL COMMENT '关联服务ID',
  `service_name` varchar(100) DEFAULT '' COMMENT '关联服务名称',
  `release_id` bigint unsigned DEFAULT NULL COMMENT '关联发布主单ID',
  `status` varchar(20) NOT NULL DEFAULT 'draft' COMMENT '状态: draft/pending_approval/approved/published/rolled_back/rejected',
  `risk_level` varchar(20) DEFAULT 'low' COMMENT '风险等级: low/medium/high',
  `description` text COMMENT '描述',
  `created_by` bigint unsigned DEFAULT NULL COMMENT '创建人ID',
  `created_by_name` varchar(100) DEFAULT '' COMMENT '创建人',
  `approved_by` bigint unsigned DEFAULT NULL COMMENT '审批人ID',
  `approved_by_name` varchar(100) DEFAULT '' COMMENT '审批人',
  `approved_at` datetime(3) DEFAULT NULL COMMENT '审批时间',
  `published_at` datetime(3) DEFAULT NULL COMMENT '发布时间',
  `published_by` bigint unsigned DEFAULT NULL COMMENT '发布人ID',
  `published_by_name` varchar(100) DEFAULT '' COMMENT '发布人',
  `rollback_from_id` bigint unsigned DEFAULT NULL COMMENT '回滚来源ID',
  `reject_reason` varchar(500) DEFAULT '' COMMENT '驳回原因',
  PRIMARY KEY (`id`),
  KEY `idx_nacos_instance` (`nacos_instance_id`),
  KEY `idx_service` (`service_id`),
  KEY `idx_release` (`release_id`),
  KEY `idx_status` (`status`),
  KEY `idx_env` (`env`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Nacos 配置发布单';

-- 24. 统一发布主单
CREATE TABLE IF NOT EXISTS `releases` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `title` varchar(200) NOT NULL COMMENT '发布标题',
  `application_id` bigint unsigned DEFAULT NULL COMMENT '关联应用ID',
  `application_name` varchar(100) DEFAULT '' COMMENT '应用名称',
  `env` varchar(30) NOT NULL DEFAULT 'dev' COMMENT '环境',
  `version` varchar(100) DEFAULT '' COMMENT '版本号',
  `description` text COMMENT '描述',
  `status` varchar(20) NOT NULL DEFAULT 'draft' COMMENT '状态: draft/pending_approval/approved/publishing/published/rolled_back/rejected',
  `risk_level` varchar(20) DEFAULT 'low' COMMENT '风险等级',
  `created_by` bigint unsigned DEFAULT NULL,
  `created_by_name` varchar(100) DEFAULT '',
  `approved_by` bigint unsigned DEFAULT NULL,
  `approved_by_name` varchar(100) DEFAULT '',
  `approved_at` datetime(3) DEFAULT NULL,
  `published_at` datetime(3) DEFAULT NULL,
  `published_by` bigint unsigned DEFAULT NULL,
  `published_by_name` varchar(100) DEFAULT '',
  `rollback_at` datetime(3) DEFAULT NULL,
  `reject_reason` varchar(500) DEFAULT '',
  PRIMARY KEY (`id`),
  KEY `idx_app` (`application_id`),
  KEY `idx_env` (`env`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='统一发布主单';

CREATE TABLE IF NOT EXISTS `release_items` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `release_id` bigint unsigned NOT NULL COMMENT '发布主单ID',
  `item_type` varchar(30) NOT NULL COMMENT '子项类型: pipeline_run/nacos_release/sql_ticket',
  `item_id` bigint unsigned NOT NULL COMMENT '子项ID',
  `item_title` varchar(200) DEFAULT '' COMMENT '子项标题',
  `item_status` varchar(30) DEFAULT '' COMMENT '子项状态',
  `sort_order` int DEFAULT 0 COMMENT '排序',
  PRIMARY KEY (`id`),
  KEY `idx_release` (`release_id`),
  KEY `idx_item` (`item_type`, `item_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='发布主单关联子项';

CREATE TABLE IF NOT EXISTS `release_gate_results` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `release_id` bigint unsigned NOT NULL COMMENT 'Release ID',
  `gate_key` varchar(80) NOT NULL COMMENT 'Gate key',
  `gate_name` varchar(120) NOT NULL COMMENT 'Gate display name',
  `category` varchar(40) NOT NULL COMMENT 'change/governance/risk/security/gitops',
  `status` varchar(20) NOT NULL COMMENT 'pass/warn/block/skip',
  `severity` varchar(20) NOT NULL DEFAULT 'info' COMMENT 'info/low/medium/high/critical',
  `policy` varchar(20) NOT NULL DEFAULT 'advisory' COMMENT 'required/advisory/manual',
  `blocker` tinyint(1) NOT NULL DEFAULT 0 COMMENT 'whether this gate blocks publish',
  `message` varchar(500) DEFAULT '' COMMENT 'human-readable gate result',
  `detail` json DEFAULT NULL COMMENT 'structured detail',
  `evaluated_at` datetime(3) NOT NULL COMMENT 'evaluation timestamp',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_release_gate_release_key` (`release_id`, `gate_key`),
  KEY `idx_release_gate_category` (`category`),
  KEY `idx_release_gate_status` (`status`),
  KEY `idx_release_gate_blocker` (`blocker`),
  KEY `idx_release_gate_evaluated_at` (`evaluated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Release Gate result snapshots';

CREATE TABLE IF NOT EXISTS `application_readiness_checks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `application_id` bigint unsigned NOT NULL COMMENT 'Application ID',
  `check_key` varchar(80) NOT NULL COMMENT 'Readiness check key',
  `title` varchar(120) NOT NULL COMMENT 'Check display title',
  `description` varchar(500) DEFAULT '' COMMENT 'Check result description',
  `status` varchar(20) NOT NULL COMMENT 'pass/missing',
  `severity` varchar(20) NOT NULL DEFAULT 'info' COMMENT 'info/low/medium/high',
  `path` varchar(300) DEFAULT '' COMMENT 'Suggested remediation path',
  `score` int NOT NULL DEFAULT 0 COMMENT 'Overall readiness score at checked time',
  `level` varchar(30) NOT NULL DEFAULT 'not_ready' COMMENT 'Overall readiness level at checked time',
  `checked_at` datetime(3) NOT NULL COMMENT 'Evaluation timestamp',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_app_readiness_app_key` (`application_id`, `check_key`),
  KEY `idx_application_readiness_checks_application_id` (`application_id`),
  KEY `idx_application_readiness_checks_status` (`status`),
  KEY `idx_application_readiness_checks_severity` (`severity`),
  KEY `idx_application_readiness_checks_checked_at` (`checked_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Application readiness check snapshots';

-- 25. 统一变更事件时间线
CREATE TABLE IF NOT EXISTS `change_events` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `event_type` varchar(30) NOT NULL COMMENT '事件类型: deploy/nacos_release/sql_ticket/pipeline_run/promotion/release',
  `event_id` bigint unsigned NOT NULL COMMENT '关联事件ID',
  `title` varchar(300) NOT NULL COMMENT '事件标题',
  `description` text COMMENT '事件描述',
  `application_id` bigint unsigned DEFAULT NULL COMMENT '关联应用ID',
  `application_name` varchar(100) DEFAULT '' COMMENT '应用名称',
  `env` varchar(30) DEFAULT '' COMMENT '环境',
  `status` varchar(30) DEFAULT '' COMMENT '状态',
  `risk_level` varchar(20) DEFAULT '' COMMENT '风险等级',
  `operator` varchar(100) DEFAULT '' COMMENT '操作人',
  `operator_id` bigint unsigned DEFAULT 0 COMMENT '操作人ID',
  `metadata` text COMMENT 'JSON 扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_event_type` (`event_type`),
  KEY `idx_app` (`application_id`),
  KEY `idx_env` (`env`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='统一变更事件';

-- 26. 环境实例
CREATE TABLE IF NOT EXISTS `env_instances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `application_id` bigint unsigned NOT NULL COMMENT '应用ID',
  `application_name` varchar(100) DEFAULT '' COMMENT '应用名称',
  `env` varchar(30) NOT NULL COMMENT '环境: dev/test/uat/gray/prod',
  `cluster_id` bigint unsigned DEFAULT NULL COMMENT 'K8s 集群ID',
  `cluster_name` varchar(100) DEFAULT '' COMMENT '集群名称',
  `namespace` varchar(100) DEFAULT '' COMMENT '命名空间',
  `deployment_name` varchar(200) DEFAULT '' COMMENT 'Deployment 名称',
  `image_url` varchar(1000) DEFAULT '' COMMENT '镜像地址',
  `image_tag` varchar(500) DEFAULT '' COMMENT '镜像标签',
  `image_digest` varchar(200) DEFAULT '' COMMENT '镜像 digest (sha256)',
  `replicas` int DEFAULT 1 COMMENT '副本数',
  `status` varchar(20) DEFAULT 'unknown' COMMENT '状态: running/stopped/deploying/failed/unknown',
  `last_deploy_at` datetime(3) DEFAULT NULL COMMENT '最近部署时间',
  `last_deploy_by` varchar(100) DEFAULT '' COMMENT '最近部署人',
  `nacos_instance_id` bigint unsigned DEFAULT NULL COMMENT 'Nacos 实例ID',
  `nacos_tenant` varchar(200) DEFAULT '' COMMENT 'Nacos 命名空间',
  `nacos_group` varchar(200) DEFAULT '' COMMENT 'Nacos 分组',
  `db_instance_id` bigint unsigned DEFAULT NULL COMMENT '数据库实例ID',
  `db_instance_name` varchar(100) DEFAULT '' COMMENT '数据库实例名称',
  `config_hash` varchar(64) DEFAULT '' COMMENT '配置哈希',
  `metadata` text COMMENT 'JSON 扩展',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_env` (`application_id`, `env`),
  KEY `idx_env` (`env`),
  KEY `idx_cluster` (`cluster_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='环境实例';

-- ============================================================
-- 27. GitOps / Argo CD 集成
-- ============================================================

CREATE TABLE IF NOT EXISTS `argocd_instances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '实例名称',
  `server_url` varchar(500) NOT NULL COMMENT 'Argo CD 地址',
  `auth_token` varchar(1000) DEFAULT '' COMMENT 'Token(加密)',
  `insecure` tinyint(1) DEFAULT 0 COMMENT '跳过 TLS 证书验证',
  `is_default` tinyint(1) DEFAULT 0 COMMENT '是否默认实例',
  `status` varchar(20) DEFAULT 'active' COMMENT '状态',
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_argocd_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Argo CD 实例';

ALTER TABLE `argocd_instances`
  ADD COLUMN IF NOT EXISTS `insecure` tinyint(1) DEFAULT 0 COMMENT '跳过 TLS 证书验证' AFTER `auth_token`;

CREATE TABLE IF NOT EXISTS `argocd_applications` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `argocd_instance_id` bigint unsigned NOT NULL COMMENT '实例ID',
  `name` varchar(200) NOT NULL COMMENT '应用名称',
  `project` varchar(200) DEFAULT 'default' COMMENT 'Argo CD 项目',
  `repo_url` varchar(500) DEFAULT '' COMMENT '仓库地址',
  `repo_path` varchar(500) DEFAULT '' COMMENT '仓库路径',
  `target_revision` varchar(200) DEFAULT 'HEAD' COMMENT '目标版本',
  `dest_server` varchar(500) DEFAULT '' COMMENT '目标集群',
  `dest_namespace` varchar(200) DEFAULT '' COMMENT '目标命名空间',
  `sync_status` varchar(30) DEFAULT 'Unknown' COMMENT '同步状态: Synced/OutOfSync/Unknown',
  `health_status` varchar(30) DEFAULT 'Unknown' COMMENT '健康状态: Healthy/Degraded/Progressing/Missing/Unknown',
  `sync_policy` varchar(20) DEFAULT 'manual' COMMENT '同步策略: manual/auto',
  `last_sync_at` datetime(3) DEFAULT NULL COMMENT '最近同步时间',
  `drift_detected` tinyint(1) DEFAULT 0 COMMENT '是否检测到漂移',
  `application_id` bigint unsigned DEFAULT NULL COMMENT '关联 DevOps 应用ID',
  `application_name` varchar(100) DEFAULT '' COMMENT '关联应用名称',
  `env` varchar(30) DEFAULT '' COMMENT '环境',
  PRIMARY KEY (`id`),
  KEY `idx_argocd_inst` (`argocd_instance_id`),
  KEY `idx_argocd_sync` (`sync_status`),
  KEY `idx_argocd_health` (`health_status`),
  KEY `idx_argocd_app` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Argo CD 应用';

CREATE TABLE IF NOT EXISTS `gitops_repos` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '仓库名称',
  `repo_url` varchar(500) NOT NULL COMMENT '仓库地址',
  `branch` varchar(200) DEFAULT 'main' COMMENT '分支',
  `path` varchar(500) DEFAULT '/' COMMENT '路径',
  `auth_type` varchar(20) DEFAULT 'token' COMMENT '认证方式: token/ssh/none',
  `auth_credential` varchar(1000) DEFAULT '' COMMENT '凭证(加密)',
  `application_id` bigint unsigned DEFAULT NULL COMMENT '关联应用ID',
  `application_name` varchar(100) DEFAULT '' COMMENT '关联应用名称',
  `env` varchar(30) DEFAULT '' COMMENT '环境',
  `sync_enabled` tinyint(1) DEFAULT 1 COMMENT '是否启用同步',
  `last_commit_hash` varchar(64) DEFAULT '' COMMENT '最新提交哈希',
  `last_commit_msg` varchar(500) DEFAULT '' COMMENT '最新提交信息',
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_gitops_app` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='GitOps 部署仓库';

CREATE TABLE IF NOT EXISTS `gitops_change_requests` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `gitops_repo_id` bigint unsigned NOT NULL COMMENT 'GitOps部署仓库ID',
  `argocd_application_id` bigint unsigned DEFAULT NULL COMMENT 'ArgoCD应用ID',
  `application_id` bigint unsigned DEFAULT NULL COMMENT '关联应用ID',
  `application_name` varchar(100) DEFAULT '' COMMENT '应用名称',
  `env` varchar(30) DEFAULT '' COMMENT '环境',
  `pipeline_id` bigint unsigned DEFAULT NULL COMMENT '关联流水线ID',
  `pipeline_run_id` bigint unsigned DEFAULT NULL COMMENT '关联流水线运行ID',
  `title` varchar(200) NOT NULL COMMENT '标题',
  `description` text COMMENT '描述',
  `file_path` varchar(500) NOT NULL COMMENT '变更文件路径',
  `image_repository` varchar(500) NOT NULL COMMENT '镜像仓库',
  `image_tag` varchar(200) NOT NULL COMMENT '镜像标签',
  `helm_chart_path` varchar(500) DEFAULT '' COMMENT 'Helm Chart路径',
  `helm_values_path` varchar(500) DEFAULT '' COMMENT 'Helm values文件路径',
  `helm_release_name` varchar(200) DEFAULT '' COMMENT 'Helm Release名称',
  `replicas` int DEFAULT 0 COMMENT '副本数',
  `cpu_request` varchar(50) DEFAULT '' COMMENT 'CPU request',
  `cpu_limit` varchar(50) DEFAULT '' COMMENT 'CPU limit',
  `memory_request` varchar(50) DEFAULT '' COMMENT 'Memory request',
  `memory_limit` varchar(50) DEFAULT '' COMMENT 'Memory limit',
  `source_branch` varchar(200) DEFAULT '' COMMENT '源分支',
  `target_branch` varchar(200) DEFAULT '' COMMENT '目标分支',
  `status` varchar(30) DEFAULT 'draft' COMMENT '状态',
  `provider` varchar(30) DEFAULT '' COMMENT 'Git Provider',
  `merge_request_iid` varchar(100) DEFAULT '' COMMENT 'MR IID',
  `merge_request_url` varchar(1000) DEFAULT '' COMMENT 'MR URL',
  `last_commit_sha` varchar(100) DEFAULT '' COMMENT '最后提交SHA',
  `approval_instance_id` bigint unsigned DEFAULT NULL COMMENT '审批实例ID',
  `approval_chain_id` bigint unsigned DEFAULT NULL COMMENT '审批链ID',
  `approval_chain_name` varchar(100) DEFAULT '' COMMENT '审批链名称',
  `approval_status` varchar(30) DEFAULT 'none' COMMENT '审批状态',
  `approval_finished_at` datetime(3) DEFAULT NULL COMMENT '审批完成时间',
  `auto_merge_status` varchar(30) DEFAULT 'pending' COMMENT '自动合并状态',
  `auto_merged_at` datetime(3) DEFAULT NULL COMMENT '自动合并时间',
  `error_message` text COMMENT '错误信息',
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_gcr_gitops_repo` (`gitops_repo_id`),
  KEY `idx_gcr_argocd_app` (`argocd_application_id`),
  KEY `idx_gcr_application_id` (`application_id`),
  KEY `idx_gcr_env` (`env`),
  KEY `idx_gcr_pipeline_id` (`pipeline_id`),
  KEY `idx_gcr_pipeline_run_id` (`pipeline_run_id`),
  KEY `idx_gcr_status` (`status`),
  KEY `idx_gcr_approval_status` (`approval_status`),
  KEY `idx_gcr_auto_merge_status` (`auto_merge_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='GitOps 变更请求';

ALTER TABLE `gitops_change_requests`
  ADD COLUMN IF NOT EXISTS `helm_chart_path` varchar(500) DEFAULT '' COMMENT 'Helm Chart路径' AFTER `image_tag`,
  ADD COLUMN IF NOT EXISTS `helm_values_path` varchar(500) DEFAULT '' COMMENT 'Helm values文件路径' AFTER `helm_chart_path`,
  ADD COLUMN IF NOT EXISTS `helm_release_name` varchar(200) DEFAULT '' COMMENT 'Helm Release名称' AFTER `helm_values_path`,
  ADD COLUMN IF NOT EXISTS `replicas` int DEFAULT 0 COMMENT '副本数' AFTER `helm_release_name`,
  ADD COLUMN IF NOT EXISTS `cpu_request` varchar(50) DEFAULT '' COMMENT 'CPU request' AFTER `replicas`,
  ADD COLUMN IF NOT EXISTS `cpu_limit` varchar(50) DEFAULT '' COMMENT 'CPU limit' AFTER `cpu_request`,
  ADD COLUMN IF NOT EXISTS `memory_request` varchar(50) DEFAULT '' COMMENT 'Memory request' AFTER `cpu_limit`,
  ADD COLUMN IF NOT EXISTS `memory_limit` varchar(50) DEFAULT '' COMMENT 'Memory limit' AFTER `memory_request`;

-- ============================================================
-- 28. Prometheus 数据源管理
-- ============================================================

CREATE TABLE IF NOT EXISTS `prometheus_instances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `name` varchar(100) NOT NULL COMMENT '实例名称',
  `url` varchar(500) NOT NULL COMMENT 'Prometheus URL',
  `auth_type` varchar(20) DEFAULT 'none' COMMENT '认证方式: none/basic/bearer',
  `username` varchar(200) DEFAULT '' COMMENT '用户名',
  `password` varchar(500) DEFAULT '' COMMENT '密码/Token(加密)',
  `is_default` tinyint(1) DEFAULT 0 COMMENT '是否默认实例',
  `status` varchar(20) DEFAULT 'active' COMMENT '状态',
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_prom_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Prometheus 数据源';

-- ============================================================
-- 29. v2.x 移除飞书（物理删表，见 patch_203_drop_feishu_legacy.sql）
-- ============================================================
-- 存量库请手工执行：migrations/patch_203_drop_feishu_legacy.sql

-- ============================================================
-- 30. v2.2 移除钉钉（物理删表，见 patch_205_drop_dingtalk_legacy.sql）
-- ============================================================
-- 存量库请手工执行：migrations/patch_205_drop_dingtalk_legacy.sql

-- ============================================================
-- 31. v2.3 移除企业微信 / 统一通知渠道 / 通用 Webhook / Slack
--     （物理删表，见 patch_206_drop_wechatwork_and_unified_channels.sql）
-- ============================================================
-- 存量库请手工执行：migrations/patch_206_drop_wechatwork_and_unified_channels.sql
