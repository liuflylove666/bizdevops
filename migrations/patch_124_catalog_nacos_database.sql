-- 已有数据卷时手工执行（库名按 compose 为准，示例 devops）：
-- docker exec -i devops-mysql mysql -uroot -p"\$MYSQL_ROOT_PASSWORD" devops < migrations/patch_124_catalog_nacos_database.sql

-- ============================================
-- 124. 数据库管理 / Nacos / 服务目录（此前仅在 upgrades.sql，Docker init 未执行导致页面报错）
-- ============================================

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
  `audit_config` json DEFAULT NULL COMMENT '多级审批配置',
  `current_step` int NOT NULL DEFAULT 0 COMMENT '当前审批步骤',
  `assigned` varchar(500) DEFAULT '' COMMENT '当前处理人列表',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sct_work_id` (`work_id`),
  KEY `idx_sct_applicant` (`applicant`),
  KEY `idx_sct_instance` (`instance_id`),
  KEY `idx_sct_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL 变更工单';

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

INSERT IGNORE INTO `env_definitions` (`name`, `display_name`, `sort_order`, `color`) VALUES
  ('dev', '开发', 10, 'blue'),
  ('test', '测试', 20, 'cyan'),
  ('uat', 'UAT', 30, 'orange'),
  ('gray', '灰度', 40, 'purple'),
  ('prod', '生产', 50, 'red');

CREATE TABLE IF NOT EXISTS `ldap_group_mappings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `group_dn` varchar(500) NOT NULL COMMENT 'LDAP 组 DN',
  `group_name` varchar(200) NOT NULL COMMENT '组显示名',
  `role_id` bigint unsigned NOT NULL COMMENT '映射系统角色 ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_lgm_group_dn` (`group_dn`),
  KEY `idx_lgm_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LDAP 组→角色映射';

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
