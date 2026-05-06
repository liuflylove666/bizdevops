-- patch_408_cost_suggestions_columns.sql
-- 补齐 cost_suggestions 与 internal/models/monitoring/cost.go CostSuggestion 一致，
-- 修复 cost collector INSERT 报 Error 1054: Unknown column 'severity'（旧 init_tables 缺列）。
--
-- 执行：mysql -h ... -u ... -p devops < migrations/patch_408_cost_suggestions_columns.sql
-- 若某列已存在会报 Duplicate column，可忽略对应语句后重试其余列。

ALTER TABLE `cost_suggestions`
  ADD COLUMN `severity` varchar(20) NOT NULL DEFAULT '' COMMENT '严重程度' AFTER `suggestion_type`,
  ADD COLUMN `title` varchar(200) NOT NULL DEFAULT '' COMMENT '标题' AFTER `severity`,
  ADD COLUMN `current_cost` double DEFAULT 0 COMMENT '当前成本' AFTER `description`,
  ADD COLUMN `optimized_cost` double DEFAULT 0 COMMENT '优化后成本' AFTER `current_cost`,
  ADD COLUMN `savings_percent` double DEFAULT 0 COMMENT '节省百分比' AFTER `savings`,
  ADD COLUMN `current_config` text COMMENT '当前配置' AFTER `savings_percent`,
  ADD COLUMN `suggested_config` text COMMENT '建议配置' AFTER `current_config`,
  ADD COLUMN `applied_at` datetime(3) DEFAULT NULL COMMENT '应用时间' AFTER `status`,
  ADD COLUMN `applied_by` bigint unsigned DEFAULT NULL COMMENT '应用者' AFTER `applied_at`;
