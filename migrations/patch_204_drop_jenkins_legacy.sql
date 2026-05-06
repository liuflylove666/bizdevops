-- patch_204_drop_jenkins_legacy.sql
--
-- 物理移除 Jenkins 集成：删除实例/构建/关联表与应用、部署记录中的 Jenkins 字段，并清理 Jenkins 权限。
-- 执行前请 mysqldump 备份相关表；不可逆。
--
-- 兼容：使用 IF EXISTS / IF NOT EXISTS，对从未创建过相关对象的环境也安全。

SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS `jenkins_builds`;
DROP TABLE IF EXISTS `jenkins_dingtalk_apps`;
DROP TABLE IF EXISTS `jenkins_wechat_work_apps`;
DROP TABLE IF EXISTS `jenkins_feishu_apps`;
DROP TABLE IF EXISTS `jenkins_instances`;

SET FOREIGN_KEY_CHECKS = 1;

-- applications：移除 Jenkins 字段与索引
ALTER TABLE `applications`
  DROP INDEX IF EXISTS `idx_jenkins_instance`;

ALTER TABLE `applications`
  DROP COLUMN IF EXISTS `jenkins_instance_id`,
  DROP COLUMN IF EXISTS `jenkins_job_name`;

-- application_envs：移除 Jenkins Job 字段
ALTER TABLE `application_envs`
  DROP COLUMN IF EXISTS `jenkins_job`;

-- deploy_records：移除 Jenkins 构建字段，并将默认部署方式改为 k8s
ALTER TABLE `deploy_records`
  DROP COLUMN IF EXISTS `jenkins_build_id`,
  DROP COLUMN IF EXISTS `jenkins_build_number`,
  DROP COLUMN IF EXISTS `jenkins_build`,
  DROP COLUMN IF EXISTS `jenkins_url`;

UPDATE `deploy_records`
SET `deploy_method` = 'k8s'
WHERE `deploy_method` = 'jenkins';

-- 清理 Jenkins 相关权限及其与角色的关联
DELETE rp FROM `role_permissions` rp
INNER JOIN `permissions` p ON rp.permission_id = p.id
WHERE p.resource = 'jenkins';

DELETE FROM `permissions` WHERE `resource` = 'jenkins';
