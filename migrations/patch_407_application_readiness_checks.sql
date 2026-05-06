-- v2.1 Application readiness check snapshots.
-- Records the latest readiness evaluation per application/check key.

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
