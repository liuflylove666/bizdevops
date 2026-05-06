-- 应用环境绑定 GitOps Helm 部署目标。
-- Application 只表达业务身份；每个环境决定 GitOps 部署仓库、Helm values、资源限制与副本数。

ALTER TABLE `application_envs`
  ADD COLUMN IF NOT EXISTS `gitops_repo_id` bigint unsigned DEFAULT NULL COMMENT 'GitOps部署仓库ID' AFTER `branch`,
  ADD COLUMN IF NOT EXISTS `argocd_application_id` bigint unsigned DEFAULT NULL COMMENT 'ArgoCD应用ID' AFTER `gitops_repo_id`,
  ADD COLUMN IF NOT EXISTS `gitops_branch` varchar(200) DEFAULT '' COMMENT 'GitOps目标分支' AFTER `argocd_application_id`,
  ADD COLUMN IF NOT EXISTS `gitops_path` varchar(500) DEFAULT '' COMMENT 'GitOps部署目录' AFTER `gitops_branch`,
  ADD COLUMN IF NOT EXISTS `helm_chart_path` varchar(500) DEFAULT '' COMMENT 'Helm Chart路径' AFTER `gitops_path`,
  ADD COLUMN IF NOT EXISTS `helm_values_path` varchar(500) DEFAULT '' COMMENT 'Helm values文件路径' AFTER `helm_chart_path`,
  ADD COLUMN IF NOT EXISTS `helm_release_name` varchar(200) DEFAULT '' COMMENT 'Helm Release名称' AFTER `helm_values_path`,
  ADD COLUMN IF NOT EXISTS `cpu_request` varchar(50) DEFAULT '' COMMENT 'CPU request' AFTER `replicas`,
  ADD COLUMN IF NOT EXISTS `cpu_limit` varchar(50) DEFAULT '' COMMENT 'CPU limit' AFTER `cpu_request`,
  ADD COLUMN IF NOT EXISTS `memory_request` varchar(50) DEFAULT '' COMMENT 'Memory request' AFTER `cpu_limit`,
  ADD COLUMN IF NOT EXISTS `memory_limit` varchar(50) DEFAULT '' COMMENT 'Memory limit' AFTER `memory_request`;

CREATE INDEX IF NOT EXISTS `idx_app_env_gitops_repo` ON `application_envs`(`gitops_repo_id`);
CREATE INDEX IF NOT EXISTS `idx_app_env_argocd_app` ON `application_envs`(`argocd_application_id`);

ALTER TABLE `gitops_change_requests`
  ADD COLUMN IF NOT EXISTS `helm_chart_path` varchar(500) DEFAULT '' COMMENT 'Helm Chart路径' AFTER `image_tag`,
  ADD COLUMN IF NOT EXISTS `helm_values_path` varchar(500) DEFAULT '' COMMENT 'Helm values文件路径' AFTER `helm_chart_path`,
  ADD COLUMN IF NOT EXISTS `helm_release_name` varchar(200) DEFAULT '' COMMENT 'Helm Release名称' AFTER `helm_values_path`,
  ADD COLUMN IF NOT EXISTS `replicas` int DEFAULT 0 COMMENT '副本数' AFTER `helm_release_name`,
  ADD COLUMN IF NOT EXISTS `cpu_request` varchar(50) DEFAULT '' COMMENT 'CPU request' AFTER `replicas`,
  ADD COLUMN IF NOT EXISTS `cpu_limit` varchar(50) DEFAULT '' COMMENT 'CPU limit' AFTER `cpu_request`,
  ADD COLUMN IF NOT EXISTS `memory_request` varchar(50) DEFAULT '' COMMENT 'Memory request' AFTER `cpu_limit`,
  ADD COLUMN IF NOT EXISTS `memory_limit` varchar(50) DEFAULT '' COMMENT 'Memory limit' AFTER `memory_request`;
