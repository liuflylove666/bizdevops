-- patch_203_drop_feishu_legacy.sql
--
-- 物理移除飞书通知：删除 Jenkins/K8s 绑定表及 feishu_* 业务表。
-- 执行前请 mysqldump 备份相关表；不可逆。
--
-- 兼容：使用 IF EXISTS，对从未创建过飞书表的环境也安全。

SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS `jenkins_feishu_apps`;
DROP TABLE IF EXISTS `k8s_cluster_feishu_apps`;
DROP TABLE IF EXISTS `feishu_message_logs`;
DROP TABLE IF EXISTS `feishu_user_tokens`;
DROP TABLE IF EXISTS `feishu_requests`;
DROP TABLE IF EXISTS `feishu_bots`;
DROP TABLE IF EXISTS `feishu_apps`;

SET FOREIGN_KEY_CHECKS = 1;

-- 流水线内置通知模板表（若存在）：移除飞书类型行
DELETE FROM `notify_templates` WHERE `type` = 'feishu';
