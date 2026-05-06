import { createRouter, createWebHistory, RouteLocationRaw, RouteRecordRaw, type RouteLocationNormalized } from 'vue-router'
import { useUserStore } from '@/stores/user'

async function redirectAlertAdminToV2Center(to: RouteLocationNormalized, tab: string) {
  return { path: '/alert/center', query: { ...to.query, tab } }
}

async function redirectLogsToUnified(to: RouteLocationNormalized, tab: string) {
  return { path: '/logs/unified', query: { ...to.query, tab } }
}

const resolveDefaultEntry = (): RouteLocationRaw => ({
  path: '/dashboard',
  query: {
    focus: 'delivery',
  },
})

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/Login.vue'),
    meta: { title: '登录' },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    children: [
      {
        path: '',
        redirect: () => resolveDefaultEntry(),
      },
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
        meta: { title: '任务工作台' },
      },
      {
        path: 'biz/goals',
        name: 'BizGoals',
        component: () => import('@/views/biz/BizGoals.vue'),
        meta: { title: '业务目标' },
      },
      {
        path: 'biz/goals/:id',
        name: 'BizGoalDetail',
        component: () => import('@/views/biz/BizGoalDetail.vue'),
        meta: { title: '业务目标详情' },
      },
      {
        path: 'biz/requirements',
        name: 'BizRequirements',
        component: () => import('@/views/biz/BizRequirements.vue'),
        meta: { title: '需求规划' },
      },
      {
        path: 'biz/requirements/:id',
        name: 'BizRequirementDetail',
        component: () => import('@/views/biz/BizRequirementDetail.vue'),
        meta: { title: '需求详情' },
      },
      {
        path: 'biz/versions',
        name: 'BizVersions',
        component: () => import('@/views/biz/BizVersions.vue'),
        meta: { title: '版本规划' },
      },
      {
        path: 'biz/versions/:id',
        name: 'BizVersionDetail',
        component: () => import('@/views/biz/BizVersionDetail.vue'),
        meta: { title: '版本规划详情' },
      },
      {
        path: 'k8s/clusters',
        name: 'K8sClusters',
        component: () => import('@/views/k8s/K8sClusters.vue'),
        meta: { title: 'K8s 集群管理' },
      },
      {
        path: 'k8s/clusters/:id/resources',
        name: 'K8sResources',
        component: () => import('@/views/k8s/K8sResources.vue'),
        meta: { title: 'K8s 资源管理' },
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('@/views/user/UserManagement.vue'),
        meta: { title: '用户管理' },
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/views/user/Profile.vue'),
        meta: { title: '个人中心' },
      },
      {
        path: 'telegram/message',
        name: 'TelegramMessage',
        component: () => import('@/views/telegram/TelegramMessage.vue'),
        meta: { title: 'Telegram 通知' },
      },
      {
        path: 'database/instances',
        name: 'DBInstance',
        component: () => import('@/views/database/DBInstance.vue'),
        meta: { title: '数据库实例' },
      },
      {
        path: 'database/console',
        name: 'DBConsole',
        component: () => import('@/views/database/DBConsole.vue'),
        meta: { title: 'SQL 控制台' },
      },
      {
        path: 'database/logs',
        name: 'DBQueryLogs',
        component: () => import('@/views/database/DBQueryLogs.vue'),
        meta: { title: '查询日志' },
      },
      {
        path: 'database/tickets',
        name: 'DBTicketList',
        component: () => import('@/views/database/DBTicketList.vue'),
        meta: { title: 'SQL 工单' },
      },
      {
        path: 'database/tickets/create',
        name: 'DBTicketCreate',
        component: () => import('@/views/database/DBTicketCreate.vue'),
        meta: { title: '新建 SQL 工单' },
      },
      {
        path: 'database/tickets/:id',
        name: 'DBTicketDetail',
        component: () => import('@/views/database/DBTicketDetail.vue'),
        meta: { title: 'SQL 工单详情' },
      },
      {
        path: 'database/rules',
        name: 'DBAuditRule',
        component: () => import('@/views/database/DBAuditRule.vue'),
        meta: { title: 'SQL 审核规则' },
      },
      {
        path: 'database/statements',
        name: 'DBStatementList',
        component: () => import('@/views/database/DBStatementList.vue'),
        meta: { title: '执行记录' },
      },
      {
        path: 'audit/logs',
        name: 'AuditLogs',
        component: () => import('@/views/audit/AuditLogs.vue'),
        meta: { title: '操作审计' },
      },
      {
        path: 'alert/overview',
        name: 'AlertOverview',
        component: () => import('@/views/alert/AlertOverview.vue'),
        meta: { title: '告警概览' },
      },
      {
        path: 'alert/center',
        name: 'AlertCenterV2',
        component: () => import('@/views/alert/AlertCenterV2.vue'),
        meta: { title: '告警中心' },
      },
      {
        path: 'alert/config',
        name: 'AlertConfig',
        component: () => import('@/views/alert/AlertConfig.vue'),
        beforeEnter: (to) => redirectAlertAdminToV2Center(to, 'config'),
        meta: { title: '告警配置' },
      },
      {
        path: 'alert/templates',
        name: 'MessageTemplate',
        component: () => import('@/views/alert/MessageTemplate.vue'),
        beforeEnter: (to) => redirectAlertAdminToV2Center(to, 'templates'),
        meta: { title: '消息模板' },
      },
      {
        path: 'alert/gateway',
        name: 'GatewayGuide',
        component: () => import('@/views/alert/GatewayGuide.vue'),
        meta: { title: '接入指南' },
      },
      {
        path: 'alert/history',
        name: 'AlertHistory',
        component: () => import('@/views/alert/AlertHistory.vue'),
        meta: { title: '告警历史' },
      },
      {
        path: 'alert/silence',
        name: 'AlertSilence',
        component: () => import('@/views/alert/AlertSilence.vue'),
        beforeEnter: (to) => redirectAlertAdminToV2Center(to, 'silence'),
        meta: { title: '静默规则' },
      },
      {
        path: 'alert/escalation',
        name: 'AlertEscalation',
        component: () => import('@/views/alert/AlertEscalation.vue'),
        beforeEnter: (to) => redirectAlertAdminToV2Center(to, 'escalation'),
        meta: { title: '升级规则' },
      },
      {
        path: 'applications',
        name: 'Applications',
        component: () => import('@/views/application/ApplicationList.vue'),
        meta: { title: '应用管理' },
      },
      {
        path: 'applications/:id',
        name: 'ApplicationDetail',
        component: () => import('@/views/application/ApplicationDetail.vue'),
        meta: { title: '应用详情' },
      },
      // v2.0: Release 主单
      {
        path: 'releases',
        name: 'ReleaseList',
        component: () => import('@/views/release/ReleaseList.vue'),
        meta: { title: '发布主单' },
      },
      {
        path: 'releases/:id',
        name: 'ReleaseDetail',
        component: () => import('@/views/release/ReleaseDetail.vue'),
        meta: { title: '发布详情' },
      },
      // v2.1: 生产事故（DORA MTTR 真实数据源）
      {
        path: 'incidents',
        name: 'IncidentList',
        component: () => import('@/views/incident/IncidentList.vue'),
        meta: { title: '生产事故' },
      },
      // DORA 效能分析
      {
        path: 'analysis/dora',
        name: 'DORAAnalysis',
        component: () => import('@/views/analysis/DORAAnalysis.vue'),
        meta: { title: 'DORA 效能分析' },
      },
      {
        path: 'incidents/:id',
        name: 'IncidentDetail',
        component: () => import('@/views/incident/IncidentDetail.vue'),
        meta: { title: '事故详情' },
      },
      {
        path: 'observability/event-timeline',
        name: 'EventTimeline',
        component: () => import('@/views/observability/EventTimeline.vue'),
        meta: { title: '事件时间线' },
      },
      {
        path: 'deploy/timeline',
        name: 'ChangeTimeline',
        component: () => import('@/views/deploy/ChangeTimeline.vue'),
        meta: { title: '变更时间线' },
      },
      {
        path: 'deploy/env-instances',
        name: 'EnvInstances',
        component: () => import('@/views/deploy/EnvInstances.vue'),
        meta: { title: '环境实例' },
      },
      {
        path: 'healthcheck',
        name: 'HealthCheck',
        component: () => import('@/views/healthcheck/HealthCheck.vue'),
        meta: { title: '健康检查' },
      },
      {
        path: 'healthcheck/ssl-cert',
        name: 'SSLCertCheck',
        component: () => import('@/views/healthcheck/SSLCertCheck.vue'),
        meta: { title: 'SSL 证书检查' },
      },

      // 审批流程
      {
        path: 'approval/chains',
        name: 'ApprovalChainList',
        component: () => import('@/views/approval/ApprovalChainList.vue'),
        meta: { title: '审批链管理' },
      },
      {
        path: 'approval/chains/:id/design',
        name: 'ApprovalChainDesigner',
        component: () => import('@/views/approval/ApprovalChainDesigner.vue'),
        meta: { title: '审批链设计' },
      },
      {
        path: 'approval/instances',
        name: 'ApprovalInstanceList',
        component: () => import('@/views/approval/ApprovalInstanceList.vue'),
        meta: { title: '审批实例' },
      },
      {
        path: 'approval/instances/:id',
        name: 'ApprovalInstanceDetail',
        component: () => import('@/views/approval/ApprovalInstancePage.vue'),
        meta: { title: '审批实例详情' },
      },
      {
        path: 'approval/rules',
        name: 'ApprovalRules',
        component: () => import('@/views/approval/ApprovalRules.vue'),
        meta: { title: '审批规则' },
      },
      {
        path: 'approval/windows',
        name: 'DeployWindows',
        component: () => import('@/views/approval/DeployWindows.vue'),
        meta: { title: '发布窗口' },
      },
      {
        path: 'approval/pending',
        name: 'PendingApprovals',
        component: () => import('@/views/approval/PendingApprovals.vue'),
        meta: { title: '待审批' },
      },
      {
        path: 'approval/history',
        name: 'ApprovalHistory',
        component: () => import('@/views/approval/ApprovalHistory.vue'),
        meta: { title: '审批历史' },
      },
      {
        path: 'approval/env-policies',
        name: 'EnvAuditPolicy',
        component: () => import('@/views/approval/EnvAuditPolicy.vue'),
        meta: { title: '环境审核策略' },
      },
      {
        path: 'deploy/locks',
        name: 'DeployLocks',
        component: () => import('@/views/deploy/DeployLocks.vue'),
        meta: { title: '部署锁' },
      },
      // K8s 运维增强
      {
        path: 'k8s/clusters/:id/pods',
        name: 'K8sPodManagement',
        component: () => import('@/views/k8s/PodManagement.vue'),
        meta: { title: 'Pod 管理' },
      },
      {
        path: 'k8s/clusters/:id/deployments',
        name: 'K8sDeploymentManagement',
        component: () => import('@/views/k8s/DeploymentManagement.vue'),
        meta: { title: 'Deployment 管理' },
      },
      // K8s 功能增强
      {
        path: 'k8s/overview',
        name: 'K8sClusterOverview',
        component: () => import('@/views/k8s/ClusterOverview.vue'),
        meta: { title: '集群概览' },
      },
      // 部署流程优化
      {
        path: 'deploy/check',
        name: 'DeployCheck',
        component: () => import('@/views/deploy/DeployCheck.vue'),
        meta: { title: '部署检查' },
      },
      // 成本管理
      {
        path: 'cost/overview',
        name: 'CostOverview',
        component: () => import('@/views/cost/CostOverview.vue'),
        meta: { title: '成本概览' },
      },
      {
        path: 'cost/trend',
        name: 'CostTrend',
        component: () => import('@/views/cost/CostTrend.vue'),
        meta: { title: '成本趋势' },
      },
      {
        path: 'cost/waste',
        name: 'CostWaste',
        component: () => import('@/views/cost/CostWaste.vue'),
        meta: { title: '资源浪费' },
      },
      {
        path: 'cost/suggestions',
        name: 'CostSuggestions',
        component: () => import('@/views/cost/CostSuggestions.vue'),
        meta: { title: '优化建议' },
      },
      {
        path: 'cost/budget',
        name: 'CostBudget',
        component: () => import('@/views/cost/CostBudget.vue'),
        meta: { title: '预算管理' },
      },
      {
        path: 'cost/config',
        name: 'CostConfig',
        component: () => import('@/views/cost/CostConfig.vue'),
        meta: { title: '成本配置' },
      },
      {
        path: 'cost/alerts',
        name: 'CostAlerts',
        component: () => import('@/views/cost/CostAlerts.vue'),
        meta: { title: '成本告警' },
      },
      {
        path: 'cost/comparison',
        name: 'CostComparison',
        component: () => import('@/views/cost/CostComparison.vue'),
        meta: { title: '成本对比' },
      },
      {
        path: 'cost/analysis',
        name: 'CostAnalysis',
        component: () => import('@/views/cost/CostAnalysis.vue'),
        meta: { title: '多维分析' },
      },
      // 安全合规中心
      {
        path: 'security/overview',
        name: 'SecurityOverview',
        component: () => import('@/views/security/SecurityOverview.vue'),
        meta: { title: '安全概览' },
      },
      {
        path: 'security/image-scan',
        name: 'ImageScan',
        component: () => import('@/views/security/ImageScan.vue'),
        meta: { title: '镜像扫描' },
      },
      {
        path: 'security/config-check',
        name: 'ConfigCheck',
        component: () => import('@/views/security/ConfigCheck.vue'),
        meta: { title: '配置检查' },
      },
      {
        path: 'security/audit-log',
        name: 'SecurityAuditLog',
        component: () => import('@/views/security/AuditLog.vue'),
        meta: { title: '安全审计' },
      },
      {
        path: 'security/image-registry',
        name: 'ImageRegistry',
        component: () => import('@/views/security/ImageRegistry.vue'),
        meta: { title: '镜像仓库' },
      },
      // CI/CD 流水线
      {
        path: 'pipeline/list',
        name: 'PipelineList',
        component: () => import('@/views/pipeline/PipelineList.vue'),
        meta: { title: '流水线列表' },
      },

      // Pod 终端
      {
        path: 'k8s/terminal',
        name: 'PodTerminal',
        component: () => import('@/views/k8s/PodTerminal.vue'),
        meta: { title: 'Pod 终端' },
      },
      {
        path: 'pipeline/git-repos',
        name: 'GitRepos',
        component: () => import('@/views/pipeline/GitRepos.vue'),
        meta: { title: 'Git 仓库' },
      },
      {
        path: 'pipeline/artifacts',
        name: 'Artifacts',
        component: () => import('@/views/pipeline/Artifacts.vue'),
        meta: { title: '构建制品' },
      },
      {
        path: 'pipeline/artifacts/:artifactId/versions',
        name: 'ArtifactVersions',
        component: () => import('@/views/pipeline/ArtifactVersions.vue'),
        meta: { title: '制品版本' },
      },
      {
        path: 'pipeline/create',
        name: 'PipelineCreate',
        component: () => import('@/views/pipeline/PipelineEditor.vue'),
        meta: { title: '创建流水线' },
      },
      {
        path: 'pipeline/edit/:id',
        name: 'PipelineEdit',
        component: () => import('@/views/pipeline/PipelineEditor.vue'),
        meta: { title: '编辑流水线' },
      },
      {
        path: 'pipeline/:id',
        name: 'PipelineDetail',
        component: () => import('@/views/pipeline/PipelineDetail.vue'),
        meta: { title: '流水线详情' },
      },
      {
        path: 'pipeline/stats',
        name: 'PipelineStats',
        component: () => import('@/views/pipeline/PipelineStats.vue'),
        meta: { title: '执行统计' },
      },
      {
        path: 'pipeline/credentials',
        name: 'PipelineCredentials',
        component: () => import('@/views/pipeline/Credentials.vue'),
        meta: { title: '凭证管理' },
      },
      {
        path: 'pipeline/variables',
        name: 'PipelineVariables',
        component: () => import('@/views/pipeline/Variables.vue'),
        meta: { title: '变量管理' },
      },
      // 日志中心
      {
        path: 'logs/unified',
        name: 'LogsUnifiedConsole',
        component: () => import('@/views/logs/LogsUnifiedConsole.vue'),
        meta: { title: '日志中心' },
      },
      {
        path: 'logs/center',
        name: 'LogCenter',
        component: () => import('@/views/logs/LogCenter.vue'),
        beforeEnter: (to) => redirectLogsToUnified(to, 'center'),
        meta: { title: '日志中心' },
      },
      {
        path: 'logs/viewer',
        name: 'LogViewer',
        redirect: '/logs/center',
        meta: { title: '日志查看' },
      },
      {
        path: 'logs/search',
        name: 'LogSearch',
        component: () => import('@/views/logs/LogSearch.vue'),
        beforeEnter: (to) => redirectLogsToUnified(to, 'search'),
        meta: { title: '日志搜索' },
      },
      {
        path: 'logs/export',
        name: 'LogExport',
        component: () => import('@/views/logs/LogExportPage.vue'),
        beforeEnter: (to) => redirectLogsToUnified(to, 'export'),
        meta: { title: '日志导出' },
      },
      {
        path: 'logs/alerts',
        name: 'LogAlertConfig',
        component: () => import('@/views/logs/LogAlertConfig.vue'),
        beforeEnter: (to) => redirectLogsToUnified(to, 'alerts'),
        meta: { title: '日志告警' },
      },
      {
        path: 'logs/stats',
        name: 'LogStats',
        component: () => import('@/views/logs/LogStats.vue'),
        beforeEnter: (to) => redirectLogsToUnified(to, 'stats'),
        meta: { title: '日志统计' },
      },
      {
        path: 'logs/compare',
        name: 'LogCompare',
        component: () => import('@/views/logs/LogCompare.vue'),
        beforeEnter: (to) => redirectLogsToUnified(to, 'compare'),
        meta: { title: '日志对比' },
      },
      {
        path: 'logs/bookmarks',
        name: 'LogBookmarks',
        component: () => import('@/views/logs/LogBookmarks.vue'),
        beforeEnter: (to) => redirectLogsToUnified(to, 'bookmarks'),
        meta: { title: '日志书签' },
      },
      // Nacos 配置管理
      {
        path: 'nacos/config',
        name: 'NacosConfig',
        component: () => import('@/views/nacos/NacosConfig.vue'),
        meta: { title: 'Nacos 配置管理' },
      },
      {
        path: 'nacos/releases',
        name: 'NacosReleases',
        component: () => import('@/views/nacos/NacosRelease.vue'),
        meta: { title: 'Nacos 发布单' },
      },
      // 服务目录
      {
        path: 'catalog',
        name: 'ServiceCatalog',
        component: () => import('@/views/application/ServiceCatalog.vue'),
        meta: { title: '服务目录' },
      },
      {
        path: 'catalog/projects/:id',
        name: 'ProjectDetail',
        component: () => import('@/views/application/ProjectDetail.vue'),
        meta: { title: '项目详情' },
      },
      // 服务详情 BFF
      {
        path: 'service-detail/:id',
        name: 'ServiceDetail',
        redirect: to => ({
          name: 'ApplicationDetail',
          params: { id: to.params.id },
          hash: '#delivery',
        }),
        meta: { title: '服务详情' },
      },
      // Jira 集成
      {
        path: 'jira/integration',
        name: 'JiraIntegration',
        component: () => import('@/views/jira/JiraIntegration.vue'),
        meta: { title: 'Jira 集成' },
      },
      // Argo CD / GitOps
      {
        path: 'argocd',
        name: 'ArgoCDIntegration',
        component: () => import('@/views/argocd/ArgoCDIntegration.vue'),
        meta: { title: 'Argo CD' },
      },
      // SonarQube 代码质量
      {
        path: 'sonarqube',
        name: 'SonarQubeIntegration',
        component: () => import('@/views/sonarqube/SonarQubeIntegration.vue'),
        meta: { title: '代码质量' },
      },
      // 链路追踪
      {
        path: 'tracing',
        name: 'TraceList',
        component: () => import('@/views/tracing/TraceList.vue'),
        meta: { title: '链路追踪' },
      },
      // Prometheus 指标
      {
        path: 'prometheus',
        name: 'PrometheusMetrics',
        component: () => import('@/views/prometheus/PrometheusMetrics.vue'),
        meta: { title: 'Prometheus 指标' },
      },
      // 值班排班
      {
        path: 'oncall',
        name: 'OncallManagement',
        component: () => import('@/views/oncall/OncallManagement.vue'),
        meta: { title: '值班管理' },
      },
      // LDAP 认证配置
      {
        path: 'system/ldap',
        name: 'LDAPSettings',
        component: () => import('@/views/system/LDAPSettings.vue'),
        meta: { title: 'LDAP 认证' },
      },
      // 功能开关
      {
        path: 'system/feature-flags',
        name: 'FeatureFlags',
        component: () => import('@/views/system/FeatureFlags.vue'),
        meta: { title: '功能开关', requireAdmin: true },
      },
      // RBAC 角色管理
      {
        path: 'rbac/roles',
        name: 'RoleManagement',
        component: () => import('@/views/rbac/RoleManagement.vue'),
        meta: { title: '角色管理' },
      },
      // 模板市场
      {
        path: 'pipeline/templates',
        name: 'TemplateMarket',
        component: () => import('@/views/pipeline/TemplateMarket.vue'),
        meta: { title: '模板市场' },
      },
      // 制品扫描结果
      {
        path: 'pipeline/artifacts/:versionId/scan',
        name: 'ArtifactScan',
        component: () => import('@/views/pipeline/ArtifactScan.vue'),
        meta: { title: '扫描结果' },
      },
    ],
  },
  // 404 页面 - 必须放在最后
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/error/NotFound.vue'),
    meta: { title: '页面不存在' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  document.title = `${to.meta.title || 'DevOps'} - 管理系统`

  const token = useUserStore().token
  if (to.path !== '/login' && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next(resolveDefaultEntry())
  } else if (to.path === '/dashboard' && !to.query.focus) {
    next({
      path: to.path,
      query: {
        ...to.query,
        focus: 'delivery',
      },
      hash: to.hash,
      replace: true,
    })
  } else {
    next()
  }
})

// 处理路由错误（如组件加载失败）
router.onError((error) => {
  console.error('路由错误:', error)
  
  // 如果是组件加载失败，跳转到 404 页面
  if (error.message.includes('Failed to fetch dynamically imported module') ||
      error.message.includes('error loading dynamically imported module')) {
    router.push('/404')
  }
})

export default router
