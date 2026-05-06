-- v2.1 Release Gate result snapshots.
-- Safe for existing databases; backend AutoMigrate also keeps this table in sync.

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
