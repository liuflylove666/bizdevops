-- 物理移除流水线旧 K8s 构建字段。
-- 新流水线使用 GitLab Runner；不再保留 build_cluster_id / build_namespace 兼容列。

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'build_cluster_id';
SET @sql = IF(@col_exists > 0,
  'ALTER TABLE pipelines DROP COLUMN build_cluster_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'build_namespace';
SET @sql = IF(@col_exists > 0,
  'ALTER TABLE pipelines DROP COLUMN build_namespace',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
