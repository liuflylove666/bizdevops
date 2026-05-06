-- patch_404_application_repo_bindings.sql
-- 应用交付主链路：应用与标准 Git 仓库强绑定。

CREATE TABLE IF NOT EXISTS `application_repo_bindings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `application_id` bigint unsigned NOT NULL COMMENT '应用ID',
  `git_repo_id` bigint unsigned NOT NULL COMMENT '标准 Git 仓库ID',
  `role` varchar(30) NOT NULL DEFAULT 'primary' COMMENT '仓库角色: primary/secondary/config/docs',
  `is_default` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否默认主仓库',
  `created_by` bigint unsigned DEFAULT NULL COMMENT '创建人',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_repo_binding` (`application_id`, `git_repo_id`),
  KEY `idx_app_repo_binding_app` (`application_id`),
  KEY `idx_app_repo_binding_repo` (`git_repo_id`),
  KEY `idx_app_repo_binding_default` (`application_id`, `is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用与标准 Git 仓库绑定';
