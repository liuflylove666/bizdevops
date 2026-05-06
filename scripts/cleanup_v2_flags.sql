-- JeriDevOps v2.0: 清理已下线的 v2 Feature Flags（执行前请先在目标库 DRY-RUN 校验）
-- 适用场景：
--   1) E7-06 已完成代码清理后，数据库中仍存在历史 flag 记录
--   2) 需要保持 feature_flags 表与当前代码清单一致
--
-- 使用方式：
--   1) 先执行预检 SELECT，确认影响范围
--   2) 再执行 DELETE（建议事务内）
--   3) 执行后复查 SELECT 应返回 0 行

START TRANSACTION;

-- 预检：将被清理的历史 v2 flags
SELECT id, name, is_enabled, rollout_percentage, updated_at
FROM feature_flags
WHERE name IN (
  'notify.channel_unification',
  'obs.unified_logs',
  'obs.alert_center_v2',
  'obs.incident_timeline',
  'ai.global_copilot',
  'ai.auto_context',
  'app.unified_detail',
  'app.traffic_policy_facade',
  'platform.workspace_dora',
  'planning.jira_as_source',
  'legacy.oa_module',
  'platform.ia_v2',
  'release.legacy_jenkins_deploy',
  'release.legacy_k8s_apply',
  'release.gitops_enabled',
  'release.risk_scoring'
)
ORDER BY name;

-- 实际清理
DELETE FROM feature_flags
WHERE name IN (
  'notify.channel_unification',
  'obs.unified_logs',
  'obs.alert_center_v2',
  'obs.incident_timeline',
  'ai.global_copilot',
  'ai.auto_context',
  'app.unified_detail',
  'app.traffic_policy_facade',
  'platform.workspace_dora',
  'planning.jira_as_source',
  'legacy.oa_module',
  'platform.ia_v2',
  'release.legacy_jenkins_deploy',
  'release.legacy_k8s_apply',
  'release.gitops_enabled',
  'release.risk_scoring'
);

-- 复查：应为 0
SELECT COUNT(1) AS remaining_v2_flag_rows
FROM feature_flags
WHERE name IN (
  'notify.channel_unification',
  'obs.unified_logs',
  'obs.alert_center_v2',
  'obs.incident_timeline',
  'ai.global_copilot',
  'ai.auto_context',
  'app.unified_detail',
  'app.traffic_policy_facade',
  'platform.workspace_dora',
  'planning.jira_as_source',
  'legacy.oa_module',
  'platform.ia_v2',
  'release.legacy_jenkins_deploy',
  'release.legacy_k8s_apply',
  'release.gitops_enabled',
  'release.risk_scoring'
);

COMMIT;
