-- ============================================
-- Patch 205: 移除钉钉相关历史遗留对象
--
-- 适用场景：已存在钉钉表结构的旧库升级到 v2.2+ 之后，
-- 与 patch_203_drop_feishu_legacy.sql 对应，彻底下线 DingTalk 集成。
-- 新装库由 init_tables.sql 完成，不需要执行本补丁。
-- ============================================

-- 1. 钉钉业务表
DROP TABLE IF EXISTS `dingtalk_message_logs`;
DROP TABLE IF EXISTS `dingtalk_apps`;
DROP TABLE IF EXISTS `dingtalk_bots`;

-- 2. K8s 集群关联表
DROP TABLE IF EXISTS `k8s_cluster_dingtalk_apps`;

-- 3. alert_configs 中旧有的 dingtalk_bot_id 列
ALTER TABLE `alert_configs` DROP COLUMN IF EXISTS `dingtalk_bot_id`;

-- 4. 清理钉钉相关 RBAC 权限与角色绑定
DELETE FROM `role_permissions`
WHERE `permission_id` IN (
  SELECT `id` FROM `permissions` WHERE `resource` = 'dingtalk'
);
DELETE FROM `permissions` WHERE `resource` = 'dingtalk';

-- 5. 清理通知渠道目录表中 dingtalk 记录
DELETE FROM `notification_channels` WHERE `channel_type` = 'dingtalk';

-- 6. 清理通知模板（流水线-钉钉等）
DELETE FROM `notify_templates` WHERE `type` = 'dingtalk';
DELETE FROM `message_templates` WHERE `name` LIKE '%钉钉%';

-- 7. 审计日志中 resource_type = 'dingtalk' 的记录：仅保留参考，不强制删除
-- UPDATE `audit_logs` SET `resource_type` = 'deprecated_dingtalk' WHERE `resource_type` = 'dingtalk';
