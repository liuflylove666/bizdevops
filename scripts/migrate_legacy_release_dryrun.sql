-- E7-04 · DeployRecord / Promotion → Release 聚合（DRY-RUN 参考脚本）
-- 执行前务必备份数据库；默认全部为注释，避免误跑。
--
-- 用法（示例，仅展示意图）：
--   mysqldump ... > backup-$(date +%F).sql
--   在测试库手工解开下列语句并加 LIMIT / 事务回滚验证

-- ---------------------------------------------------------------------------
-- 1) 为 Release 表补充来自旧 deploy_records 的占位映射（示例字段名需与 ORM 对齐）
-- ---------------------------------------------------------------------------
-- START TRANSACTION;
-- INSERT INTO releases (application_id, env, version, title, status, description, created_at, updated_at)
-- SELECT application_id, env_name, COALESCE(image_tag, version, 'unknown'), CONCAT('migrated-deploy-', id), status, description, created_at, updated_at
-- FROM deploy_records
-- WHERE id NOT IN (SELECT source_deploy_id FROM release_migrations WHERE source_deploy_id IS NOT NULL)
-- LIMIT 0;
-- ROLLBACK;

-- ---------------------------------------------------------------------------
-- 2) Promotion 路径写入 release_items / promotion_path JSON（示意）
-- ---------------------------------------------------------------------------
-- UPDATE releases r
-- INNER JOIN env_promotion_records p ON p.application_id = r.application_id
-- SET r.promotion_path = JSON_ARRAY(p.id)
-- WHERE 1=0;

-- 生产切换清单：双写期 → 读新写新 → 观察 30d → 删旧表（另开工单）。
