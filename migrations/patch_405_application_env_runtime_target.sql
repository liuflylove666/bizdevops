-- 将部署目标从应用本体下沉到应用环境。
-- 应用只保留业务身份与仓库归属；K8s 集群、namespace、deployment 归属 application_envs。

ALTER TABLE `application_envs`
  ADD COLUMN IF NOT EXISTS `k8s_cluster_id` bigint unsigned DEFAULT NULL COMMENT 'K8s集群ID' AFTER `branch`;

CREATE INDEX IF NOT EXISTS `idx_app_env_k8s_cluster` ON `application_envs`(`k8s_cluster_id`);

ALTER TABLE `applications`
  DROP INDEX IF EXISTS `idx_k8s_cluster`,
  DROP COLUMN IF EXISTS `k8s_cluster_id`,
  DROP COLUMN IF EXISTS `k8s_namespace`,
  DROP COLUMN IF EXISTS `k8s_deployment`;
