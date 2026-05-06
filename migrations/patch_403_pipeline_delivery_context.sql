-- 显式串联应用、流水线运行与 GitOps 变更请求。
-- 新流水线主链路：Application -> Pipeline -> PipelineRun -> GitOpsChangeRequest。
-- 不恢复任何旧 K8s 构建字段。

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'application_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN application_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''关联应用ID'' AFTER project_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'application_name';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN application_name VARCHAR(100) DEFAULT '''' COMMENT ''关联应用名称'' AFTER application_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND COLUMN_NAME = 'env';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipelines ADD COLUMN env VARCHAR(50) DEFAULT '''' COMMENT ''交付环境'' AFTER application_name',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipeline_runs'
  AND COLUMN_NAME = 'application_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipeline_runs ADD COLUMN application_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''关联应用ID'' AFTER parameters_json',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipeline_runs'
  AND COLUMN_NAME = 'application_name';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipeline_runs ADD COLUMN application_name VARCHAR(100) DEFAULT '''' COMMENT ''关联应用名称'' AFTER application_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipeline_runs'
  AND COLUMN_NAME = 'env';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE pipeline_runs ADD COLUMN env VARCHAR(50) DEFAULT '''' COMMENT ''交付环境'' AFTER application_name',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'gitops_change_requests'
  AND COLUMN_NAME = 'application_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE gitops_change_requests ADD COLUMN application_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''关联应用ID'' AFTER argocd_application_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'gitops_change_requests'
  AND COLUMN_NAME = 'pipeline_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE gitops_change_requests ADD COLUMN pipeline_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''关联流水线ID'' AFTER env',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'gitops_change_requests'
  AND COLUMN_NAME = 'pipeline_run_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE gitops_change_requests ADD COLUMN pipeline_run_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''关联流水线运行ID'' AFTER pipeline_id',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'deploy_records'
  AND COLUMN_NAME = 'gitops_change_request_id';
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE deploy_records ADD COLUMN gitops_change_request_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''GitOps 变更请求ID'' AFTER deploy_method',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @index_exists = 0;
SELECT COUNT(*) INTO @index_exists
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipelines'
  AND INDEX_NAME = 'idx_p_application_id';
SET @sql = IF(@index_exists = 0, 'CREATE INDEX idx_p_application_id ON pipelines(application_id)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @index_exists = 0;
SELECT COUNT(*) INTO @index_exists
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'pipeline_runs'
  AND INDEX_NAME = 'idx_pr_application';
SET @sql = IF(@index_exists = 0, 'CREATE INDEX idx_pr_application ON pipeline_runs(application_id)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @index_exists = 0;
SELECT COUNT(*) INTO @index_exists
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'gitops_change_requests'
  AND INDEX_NAME = 'idx_gcr_pipeline_run_id';
SET @sql = IF(@index_exists = 0, 'CREATE INDEX idx_gcr_pipeline_run_id ON gitops_change_requests(pipeline_run_id)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
