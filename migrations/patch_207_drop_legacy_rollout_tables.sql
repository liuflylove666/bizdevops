-- patch_207_drop_legacy_rollout_tables.sql
--
-- 物理移除自建金丝雀 / 蓝绿发布的遗留表，迁移至 Argo Rollouts（见 ADR-0003）。
-- 执行前请 mysqldump 备份相关表；不可逆。
--
-- 兼容：使用 IF EXISTS，对从未创建过相关对象的环境也安全。

SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS `canary_releases`;
DROP TABLE IF EXISTS `blue_green_deployments`;

SET FOREIGN_KEY_CHECKS = 1;
