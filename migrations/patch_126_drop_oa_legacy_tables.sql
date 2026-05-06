-- patch_126_drop_oa_legacy_tables.sql
--
-- v2.0 清理：移除历史遗留的 OA 模型（OAData、OAAddress 及其表）。
-- 参考：docs/adr/0001-...（GitOps 决策蓝图）、docs/roadmap/v2.0-development-plan.md（Epic 7 遗留清理）
--
-- 安全性：
--   1. 代码中已移除对 models.OAData / models.OAAddress 的全部引用
--   2. healthcheck.checkOA 已移除，相关 health_check_configs 行将被标记为 disabled
--   3. OANotifyConfig（实际为飞书默认通知配置）保留，后续 ADR-0005 重命名为 FeishuNotifyConfig
--
-- 回滚方案：此脚本包含 DROP TABLE，执行前应先完成 mysqldump 备份。
--   mysqldump -h $HOST -u $USER -p$PWD $DB oa_data oa_addresses > oa_legacy_backup.sql

-- ---------------------------------------------------------------------
-- Step 1: 禁用 type='oa' 的健康检查配置（避免运行时报错"Unknown check type"）
-- ---------------------------------------------------------------------
UPDATE health_check_configs
   SET status = 'disabled',
       updated_at = NOW()
 WHERE target_type = 'oa';

-- ---------------------------------------------------------------------
-- Step 2: 删除已无引用的数据表
-- ---------------------------------------------------------------------
DROP TABLE IF EXISTS oa_data;
DROP TABLE IF EXISTS oa_addresses;

-- ---------------------------------------------------------------------
-- Step 3（可选，留作备忘）：
--   oa_notify_configs 表保留，等 Sprint 6 / ADR-0005 统一改名为
--   feishu_notify_configs 时再做 RENAME TABLE。
-- ---------------------------------------------------------------------
