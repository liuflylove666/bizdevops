-- ============================================
-- Patch 206: 移除企业微信 / 统一通知渠道 / 通用 Webhook / Slack 历史遗留对象
--
-- 适用场景：已存在 wechat_work_* 或 notification_channels 的旧库升级到 v2.3+ 之后。
-- v2.3 起通知渠道统一收敛为 Telegram 专属流程，企业微信 / 通用 Webhook / Slack
-- 不再支持，前端也不再提供统一通道管理入口。
-- 新装库由 init_tables.sql 完成，不需要执行本补丁。
-- ============================================

-- 1. 企业微信业务表
DROP TABLE IF EXISTS `wechat_work_message_logs`;
DROP TABLE IF EXISTS `wechat_work_bots`;
DROP TABLE IF EXISTS `wechat_work_apps`;

-- 2. K8s 集群关联表
DROP TABLE IF EXISTS `k8s_cluster_wechat_work_apps`;

-- 3. 统一通知渠道目录表（ADR-0005 在 v2.3 后被废弃）
DROP TABLE IF EXISTS `notification_channels`;

-- 4. alert_configs 中旧有的 wechatwork_bot_id 列
ALTER TABLE `alert_configs` DROP COLUMN IF EXISTS `wechatwork_bot_id`;

-- 5. 清理企业微信相关 RBAC 权限与角色绑定
DELETE FROM `role_permissions`
WHERE `permission_id` IN (
  SELECT `id` FROM `permissions` WHERE `resource` = 'wechatwork'
);
DELETE FROM `permissions` WHERE `resource` = 'wechatwork';

-- 6. 清理通知模板（流水线-企业微信 / 通用 Webhook / Slack 等）
DELETE FROM `notify_templates` WHERE `type` IN ('wechatwork', 'webhook', 'slack');
DELETE FROM `message_templates` WHERE `name` LIKE '%企业微信%' OR `name` LIKE '%Slack%';

-- 7. 审计日志中 resource_type = 'wechatwork' 的记录：仅保留参考，不强制删除
-- UPDATE `audit_logs` SET `resource_type` = 'deprecated_wechatwork' WHERE `resource_type` = 'wechatwork';
