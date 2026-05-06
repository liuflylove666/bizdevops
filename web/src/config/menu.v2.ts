/**
 * v2.0 菜单配置（交付主线 + 平台底座）。
 *
 * 当前作为主导航默认菜单（V2）。
 * V1 菜单定义仅作为历史对照保留在 `web/src/layouts/MainLayout.vue`。
 *
 * 设计原则（见 产品优化方案 + ADR-0001~0007）：
 *   1. 主线 = 应用 → CI/CD → 发布主单 → GitOps/Nacos/SQL 变更 → 审批 → 运行态
 *   2. 业务变更入口放在"发布与变更"，底座配置和治理入口放在"基础设施"
 *   3. 以 "Release" 为发布主单（ADR-0002），"GitOps PR" 为审批对象（ADR-0001）
 *   5. 已下线的历史能力不再保留产品入口
 *
 * 路径策略：**保持 V2 菜单引用的路径与 V1 完全相同**。新功能用新路径（如 /releases），
 * 已下线的历史路径已移除，不再暴露产品入口。
 */

import type { Component } from 'vue'
import {
  DashboardOutlined,
  ProjectOutlined,
  AppstoreOutlined,
  RocketOutlined,
  ForkOutlined,
  AlertOutlined,
  CloudOutlined,
  SettingOutlined,
  ControlOutlined,
} from '@ant-design/icons-vue'

// ---- 类型 -----------------------------------------------------------------
export interface MenuItemConfigV2 {
  key: string
  icon?: Component
  titleKey: string
  path?: string
  children?: MenuItemConfigV2[]
  permission?: string[]
  /** 标记为 v2 新增/强化项（便于后续统计与灰度）。 */
  v2New?: boolean
}

// ---- 菜单主体 -------------------------------------------------------------
/**
 * 返回 v2.0 菜单。
 * 纯函数，便于单测；图标在函数外部引入一次即可。
 */
export function getV2MenuConfig(): MenuItemConfigV2[] {
  return [
    // ------------------------------------------------------------------- 工作台
    {
      key: '/dashboard',
      icon: DashboardOutlined,
      titleKey: 'menu.v2.workspace',
      path: '/dashboard',
    },

    // ------------------------------------------------------------------- 1. 应用中心：平台的交付主入口
    {
      key: 'v2.apps',
      icon: AppstoreOutlined,
      titleKey: 'menu.v2.apps',
      permission: ['application:view'],
      children: [
        { key: '/applications', titleKey: 'menu.v2.appList', path: '/applications' },
        { key: '/catalog', titleKey: 'menu.v2.catalog', path: '/catalog' },
      ],
    },

    // ------------------------------------------------------------------- 2. CI/CD：代码、构建、制品、构建资源与 CI 治理
    {
      key: 'v2.pipelines',
      icon: ForkOutlined,
      titleKey: 'menu.v2.pipelines',
      permission: ['application:view'],
      children: [
        { key: '/pipeline/list', titleKey: 'menu.v2.pipelineList', path: '/pipeline/list' },
        { key: '/pipeline/git-repos', titleKey: 'menu.v2.gitRepos', path: '/pipeline/git-repos' },
        { key: '/pipeline/templates', titleKey: 'menu.v2.pipelineTemplates', path: '/pipeline/templates' },
        { key: '/pipeline/artifacts', titleKey: 'menu.v2.artifacts', path: '/pipeline/artifacts' },
        { key: '/pipeline/stats', titleKey: 'menu.v2.pipelineStats', path: '/pipeline/stats' },
        {
          key: 'v2.ciGovernance',
          titleKey: 'menu.v2.ciGovernance',
          children: [
            { key: '/pipeline/credentials', titleKey: 'menu.v2.credentials', path: '/pipeline/credentials', permission: ['pipeline:manage'] },
            { key: '/pipeline/variables', titleKey: 'menu.v2.variables', path: '/pipeline/variables', permission: ['pipeline:manage'] },
            { key: '/sonarqube', titleKey: 'menu.v2.sonarqube', path: '/sonarqube' },
          ],
        },
      ],
    },

    // ------------------------------------------------------------------- 3. 发布与变更：所有需要审批、合并、发布的业务变更
    {
      key: 'v2.releases',
      icon: RocketOutlined,
      titleKey: 'menu.v2.releases',
      children: [
        { key: '/releases', titleKey: 'menu.v2.releaseList', path: '/releases', v2New: true },
        { key: '/argocd', titleKey: 'menu.v2.gitopsPRs', path: '/argocd', v2New: true },
        { key: '/nacos/releases', titleKey: 'menu.v2.configChanges', path: '/nacos/releases' },
        { key: '/database/tickets', titleKey: 'menu.v2.sqlChanges', path: '/database/tickets' },
        { key: '/deploy/check', titleKey: 'menu.v2.deployCheck', path: '/deploy/check' },
        {
          key: 'v2.releaseGovernance',
          titleKey: 'menu.v2.releaseGovernance',
          children: [
            { key: '/approval/pending', titleKey: 'menu.v2.approvalCenter', path: '/approval/pending' },
            { key: '/approval/history', titleKey: 'menu.v2.approvalHistory', path: '/approval/history' },
            { key: '/approval/instances', titleKey: 'menu.v2.approvalInstances', path: '/approval/instances' },
            { key: '/approval/chains', titleKey: 'menu.v2.approvalChains', path: '/approval/chains', permission: ['approval:manage'] },
            { key: '/approval/env-policies', titleKey: 'menu.v2.envAuditPolicy', path: '/approval/env-policies', permission: ['approval:manage'] },
            { key: '/deploy/locks', titleKey: 'menu.v2.deployLocks', path: '/deploy/locks' },
          ],
        },
        {
          key: 'v2.releaseInsights',
          titleKey: 'menu.v2.releaseInsights',
          children: [
            { key: '/deploy/timeline', titleKey: 'menu.v2.changeTimeline', path: '/deploy/timeline' },
            { key: '/analysis/dora', titleKey: 'menu.v2.doraAnalysis', path: '/analysis/dora', v2New: true },
          ],
        },
      ],
    },

    // ------------------------------------------------------------------- 4. 运行观测：变更上线后的反馈闭环
    {
      key: 'v2.ops',
      icon: AlertOutlined,
      titleKey: 'menu.v2.ops',
      children: [
        { key: '/alert/overview', titleKey: 'menu.v2.alertCenter', path: '/alert/overview' },
        { key: '/logs/center', titleKey: 'menu.v2.logs', path: '/logs/center' },
        {
          key: '/observability/event-timeline',
          titleKey: 'menu.v2.eventTimeline',
          path: '/observability/event-timeline',
          v2New: true,
        },
        { key: '/incidents', titleKey: 'menu.v2.incidents', path: '/incidents', v2New: true },
        {
          key: 'v2.healthChecks',
          titleKey: 'menu.v2.healthChecks',
          children: [
            { key: '/healthcheck', titleKey: 'menu.v2.healthcheck', path: '/healthcheck' },
            { key: '/healthcheck/ssl-cert', titleKey: 'menu.v2.sslCert', path: '/healthcheck/ssl-cert' },
          ],
        },
        {
          key: 'v2.opsTelemetry',
          titleKey: 'menu.v2.opsTelemetry',
          children: [
            { key: '/prometheus', titleKey: 'menu.v2.metrics', path: '/prometheus' },
            { key: '/tracing', titleKey: 'menu.v2.tracing', path: '/tracing' },
          ],
        },
        { key: '/oncall', titleKey: 'menu.v2.oncall', path: '/oncall' },
        { key: '/telegram/message', titleKey: 'menu.v2.notificationCenter', path: '/telegram/message' },
      ],
    },

    // ------------------------------------------------------------------- 5. 基础设施：交付底座与数据底座
    {
      key: 'v2.infra',
      icon: CloudOutlined,
      titleKey: 'menu.v2.infra',
      children: [
        { key: '/k8s/overview', titleKey: 'menu.v2.k8sOverview', path: '/k8s/overview' },
        { key: '/k8s/clusters', titleKey: 'menu.v2.clusters', path: '/k8s/clusters' },
        { key: '/nacos/config', titleKey: 'menu.v2.nacosConfig', path: '/nacos/config' },
        {
          key: 'v2.dataGovernance',
          titleKey: 'menu.v2.dataGovernance',
          children: [
            { key: '/database/instances', titleKey: 'menu.v2.dbInstances', path: '/database/instances' },
            { key: '/database/rules', titleKey: 'menu.v2.dbAuditRule', path: '/database/rules' },
            { key: '/database/statements', titleKey: 'menu.v2.dbStatements', path: '/database/statements' },
            { key: '/database/console', titleKey: 'menu.v2.dbConsole', path: '/database/console' },
            { key: '/database/logs', titleKey: 'menu.v2.dbQueryLogs', path: '/database/logs' },
          ],
        },
        {
          key: 'v2.securityGovernance',
          titleKey: 'menu.v2.securityGovernance',
          children: [
            { key: '/security/overview', titleKey: 'menu.v2.security', path: '/security/overview' },
            { key: '/security/image-registry', titleKey: 'menu.v2.imageRegistry', path: '/security/image-registry' },
            { key: '/security/image-scan', titleKey: 'menu.v2.imageScan', path: '/security/image-scan' },
            { key: '/security/config-check', titleKey: 'menu.v2.configCheck', path: '/security/config-check' },
            { key: '/security/audit-log', titleKey: 'menu.v2.securityAudit', path: '/security/audit-log' },
          ],
        },
      ],
    },

    // ------------------------------------------------------------------- 6. 平台管理：账号、治理、审计
    {
      key: 'v2.platform',
      icon: SettingOutlined,
      titleKey: 'menu.v2.platform',
      children: [
        { key: '/users', titleKey: 'menu.v2.users', path: '/users', permission: ['system:manage'] },
        { key: '/rbac/roles', titleKey: 'menu.v2.roles', path: '/rbac/roles', permission: ['system:manage'] },
        {
          key: 'v2.costGovernance',
          titleKey: 'menu.v2.costGovernance',
          children: [
            { key: '/cost/overview', titleKey: 'menu.v2.costOverview', path: '/cost/overview' },
            { key: '/cost/trend', titleKey: 'menu.v2.costTrend', path: '/cost/trend' },
            { key: '/cost/comparison', titleKey: 'menu.v2.costComparison', path: '/cost/comparison' },
            { key: '/cost/analysis', titleKey: 'menu.v2.costAnalysis', path: '/cost/analysis' },
            { key: '/cost/waste', titleKey: 'menu.v2.costWaste', path: '/cost/waste' },
            { key: '/cost/suggestions', titleKey: 'menu.v2.costSuggestions', path: '/cost/suggestions' },
            { key: '/cost/alerts', titleKey: 'menu.v2.costAlerts', path: '/cost/alerts' },
            { key: '/cost/budget', titleKey: 'menu.v2.costBudget', path: '/cost/budget' },
            { key: '/cost/config', titleKey: 'menu.v2.costConfig', path: '/cost/config' },
          ],
        },
        { key: '/audit/logs', titleKey: 'menu.v2.audit', path: '/audit/logs', permission: ['system:manage'] },
        { key: '/system/ldap', titleKey: 'menu.v2.ldap', path: '/system/ldap', permission: ['system:manage'] },
        {
          key: '/system/feature-flags',
          titleKey: 'menu.v2.featureFlags',
          path: '/system/feature-flags',
          permission: ['system:manage'],
          icon: ControlOutlined,
          v2New: true,
        },
      ],
    },

    // ------------------------------------------------------------------- 7. 规划：仍保留，但从交付链路之后进入
    {
      key: 'v2.planning',
      icon: ProjectOutlined,
      titleKey: 'menu.v2.planning',
      children: [
        { key: '/biz/goals', titleKey: 'menu.v2.goals', path: '/biz/goals' },
        { key: '/biz/requirements', titleKey: 'menu.v2.requirements', path: '/biz/requirements' },
        { key: '/biz/versions', titleKey: 'menu.v2.versions', path: '/biz/versions' },
        { key: '/jira/integration', titleKey: 'menu.v2.jira', path: '/jira/integration' },
      ],
    },
  ]
}

/** 仅用于单测/调试：统计顶级 + 子项总数。 */
export function countV2MenuItems(items = getV2MenuConfig()): { groups: number; total: number } {
  let total = 0
  const count = (list: MenuItemConfigV2[]) => {
    for (const it of list) {
      total++
      if (it.children && it.children.length > 0) count(it.children)
    }
  }
  count(items)
  return { groups: items.length, total }
}
