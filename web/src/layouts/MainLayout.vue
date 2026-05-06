<template>
  <a-layout style="min-height: 100vh" class="main-layout">
    <a-layout-sider v-model:collapsed="collapsed" :trigger="null" collapsible theme="dark" width="200" class="fixed-sider" :style="{ background: 'linear-gradient(180deg, #1a2332 0%, #2c3e50 50%, #34495e 100%)' }">
      <div class="logo">
        <!-- 使用 v-show 替代 v-if 以避免重复渲染 -->
        <span v-show="!collapsed">DevOps 管理系统</span>
        <span v-show="collapsed">DevOps</span>
      </div>
      <div v-if="!collapsed" class="menu-actions">
        <a-tooltip :title="t('layout.expandAllMenus')" placement="bottom">
          <button type="button" class="menu-action-btn" @click="expandAllMenus">
            <PlusSquareOutlined />
          </button>
        </a-tooltip>
        <a-tooltip :title="t('layout.collapseAllMenus')" placement="bottom">
          <button type="button" class="menu-action-btn" @click="collapseAllMenus">
            <MinusSquareOutlined />
          </button>
        </a-tooltip>
      </div>
      <a-menu
        v-model:selectedKeys="selectedKeys"
        v-model:openKeys="openKeys"
        theme="dark"
        mode="inline"
        @click="handleMenuClick"
        @openChange="handleOpenChange"
        :inline-collapsed="collapsed"
      >
        <template v-for="item in filteredMenuConfig" :key="item.key">
          <!-- 单级菜单 -->
          <a-menu-item v-if="!item.children || item.children.length === 0" :key="item.key">
            <template v-if="item.icon" #icon>
              <component :is="item.icon" />
            </template>
            {{ t(item.titleKey) }}
          </a-menu-item>
          
          <!-- 多级菜单 -->
          <a-sub-menu v-else :key="item.key">
            <template v-if="item.icon" #icon>
              <component :is="item.icon" />
            </template>
            <template #title>{{ t(item.titleKey) }}</template>
            
            <!-- 二级菜单 -->
            <template v-for="child in item.children" :key="child.key">
              <!-- 二级单项 -->
              <a-menu-item v-if="!child.children || child.children.length === 0" :key="child.key">
                <template v-if="child.icon" #icon>
                  <component :is="child.icon" />
                </template>
                {{ t(child.titleKey) }}
              </a-menu-item>
              
              <!-- 二级子菜单 -->
              <a-sub-menu v-else :key="child.key">
                <template #title>{{ t(child.titleKey) }}</template>
                <a-menu-item v-for="grandChild in child.children" :key="grandChild.key">
                  <template v-if="grandChild.icon" #icon>
                    <component :is="grandChild.icon" />
                  </template>
                  {{ t(grandChild.titleKey) }}
                </a-menu-item>
              </a-sub-menu>
            </template>
          </a-sub-menu>
        </template>
      </a-menu>
    </a-layout-sider>

    <a-layout class="content-layout">
      <a-layout-header class="workspace-header">
        <div class="header-left">
          <div class="trigger" @click="collapsed = !collapsed">
            <MenuUnfoldOutlined v-if="collapsed" />
            <MenuFoldOutlined v-else />
          </div>
          <div class="header-context">
            <div class="header-title">{{ currentPageTitle }}</div>
            <div class="header-subtitle">{{ currentPageSubtitle }}</div>
          </div>
        </div>
        <div class="header-right">
          <a-space size="middle" wrap>
            <a-space class="header-shortcuts">
              <a-button size="small" @click="goToMenuPath('/dashboard')">工作台</a-button>
              <a-button size="small" @click="goToMenuPath('/applications')">应用</a-button>
              <a-button size="small" type="primary" ghost @click="goToMenuPath('/releases')">发布</a-button>
            </a-space>
            <PipelineSwitcher />
            <GlobalSearch />
            <FavoriteList />
            <ThemeSwitch />
            <LanguageSwitcher />
            <a-dropdown>
              <a-space class="user-entry">
                <a-avatar>{{ userInfo?.username?.charAt(0)?.toUpperCase() || 'U' }}</a-avatar>
                <span>{{ userInfo?.username || '管理员' }}</span>
              </a-space>
              <template #overlay>
                <a-menu @click="handleDropdownClick">
                  <a-menu-item key="profile">{{ t('common.profile') }}</a-menu-item>
                  <a-menu-divider />
                  <a-menu-item key="logout">{{ t('common.logout') }}</a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </a-space>
        </div>
      </a-layout-header>

      <a-layout-content style="margin: 16px; background: #f0f2f5; min-height: calc(100vh - 80px)">
        <!-- 面包屑导航 -->
        <a-breadcrumb style="margin-bottom: 16px" v-if="breadcrumbs.length > 1">
          <a-breadcrumb-item v-for="(item, index) in breadcrumbs" :key="index">
            <router-link v-if="item.path && index < breadcrumbs.length - 1" :to="item.path">
              {{ item.title }}
            </router-link>
            <span v-else>{{ item.title }}</span>
          </a-breadcrumb-item>
        </a-breadcrumb>
        <div style="background: #fff; padding: 20px; border-radius: 4px">
          <router-view />
        </div>
      </a-layout-content>
    </a-layout>
    <!-- V2 IA 首次访问引导（固定 V2 菜单下，本地存储幂等） -->
    <V2WelcomeTour />
  </a-layout>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  DashboardOutlined,
  ThunderboltOutlined,
  CloudOutlined,
  AlertOutlined,
  AppstoreOutlined,
  SettingOutlined,
  AuditOutlined,
  DollarOutlined,
  RocketOutlined,
  FileSearchOutlined,
  ShopOutlined,
  KeyOutlined,
  ControlOutlined,
  SafetyCertificateOutlined,
  ProjectOutlined,
  ForkOutlined,
  PlusSquareOutlined,
  MinusSquareOutlined
} from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import GlobalSearch from '@/components/GlobalSearch.vue'
import FavoriteList from '@/components/FavoriteList.vue'
import PipelineSwitcher from '@/components/pipeline/PipelineSwitcher.vue'
import ThemeSwitch from '@/components/ThemeSwitch.vue'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import V2WelcomeTour from '@/components/V2WelcomeTour.vue'
import { useI18n } from 'vue-i18n'
import { getV2MenuConfig } from '@/config/menu.v2'
import { loadFeatureFlags } from '@/composables/useFeatureFlag'
import { useUserStore } from '@/stores/user'
import { usePermissionStore, type RoleFocus } from '@/stores/permission'
const router = useRouter()
const route = useRoute()
const { t } = useI18n()

// 保留 Feature Flag 加载：其它 release.* 门控仍会使用。
void loadFeatureFlags().catch(() => {})

const userStore = useUserStore()
const permissionStore = usePermissionStore()

const collapsed = ref(false)

const rolePrimaryMenuKeyMap: Record<RoleFocus, string> = {
  delivery: 'v2.apps',
  operations: 'v2.releases',
  platform: 'v2.apps',
}

const menuGroupPrefixes: Record<string, string[]> = {
  'v2.planning': ['/biz', '/jira'],
  'v2.apps': ['/applications', '/catalog', '/service-detail'],
  'v2.releases': ['/releases', '/deploy/', '/approval', '/argocd', '/nacos/releases', '/database/tickets'],
  'v2.pipelines': ['/pipeline', '/sonarqube'],
  'v2.ops': ['/alert', '/oncall', '/tracing', '/prometheus', '/logs', '/healthcheck', '/message', '/telegram', '/observability', '/incidents'],
  'v2.infra': ['/k8s', '/security', '/nacos/config', '/database'],
  'v2.platform': ['/cost', '/users', '/rbac', '/audit', '/admin', '/ai', '/system/ldap', '/system/feature-flags']
}

const getSelectedMenuKey = (path: string): string => {
  if (path.startsWith('/telegram/')) return '/telegram/message'
  if (path === '/security/image-registry') return '/security/image-registry'
  if (path.startsWith('/k8s/clusters/')) return '/k8s/clusters'
  if (path.startsWith('/pipeline/create') || path.startsWith('/pipeline/edit/')) return '/pipeline/list'
  if (path.match(/^\/pipeline\/artifacts\/\d+/)) return '/pipeline/artifacts'
  if (path.match(/^\/pipeline\/\d+$/)) return '/pipeline/list'
  if (path.match(/^\/approval\/chains\/\d+\/design$/)) return '/approval/chains'
  if (path.match(/^\/approval\/instances\/\d+$/)) return '/approval/instances'
  if (path.match(/^\/service-detail\/\d+$/)) return '/applications'
  if (path.match(/^\/biz\/goals\/\d+$/)) return '/biz/goals'
  if (path.match(/^\/biz\/requirements\/\d+$/)) return '/biz/requirements'
  if (path.match(/^\/biz\/versions\/\d+$/)) return '/biz/versions'
  if (path.startsWith('/database/tickets/')) return '/database/tickets'
  return path
}

/** v2.0 收口后，告警/日志统一入口固定启用。 */
const resolveSelectedMenuKey = (path: string): string => {
  let k = getSelectedMenuKey(path)
  const alertGroup = ['/alert/overview', '/alert/center', '/alert/config', '/alert/templates', '/alert/silence', '/alert/escalation']
  if (alertGroup.includes(k)) {
    k = '/alert/center'
  }
  if (path.startsWith('/logs/')) {
    k = '/logs/unified'
  }
  return k
}

const selectedKeys = ref<string[]>([resolveSelectedMenuKey(route.path)])

// 移动端检测 - 使用响应式变量
const windowWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1920)

const isMobile = computed(() => {
  return windowWidth.value < 768
})

// 监听窗口大小变化
const handleResize = () => {
  windowWidth.value = window.innerWidth
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
  // 初始化时检查是否为移动端
  if (isMobile.value) {
    collapsed.value = true
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})

// 面包屑配置 - 使用 i18n key
const breadcrumbMap: Record<string, { titleKey: string; parent?: string }> = {
  '/dashboard': { titleKey: 'breadcrumb.dashboard' },
  '/applications': { titleKey: 'breadcrumb.applications', parent: 'appDelivery' },
  '/deploy/timeline': { titleKey: 'breadcrumb.changeTimeline', parent: 'releaseChange' },
  '/deploy/env-instances': { titleKey: 'breadcrumb.envInstances', parent: 'releaseChange' },
  '/deploy/check': { titleKey: 'breadcrumb.deployCheck', parent: 'releaseChange' },
  '/healthcheck': { titleKey: 'breadcrumb.serviceHealth', parent: 'operations' },
  '/healthcheck/ssl-cert': { titleKey: 'breadcrumb.sslCert', parent: 'operations' },
  '/pipeline/list': { titleKey: 'breadcrumb.pipelineList', parent: 'pipeline' },
  '/pipeline/git-repos': { titleKey: 'breadcrumb.gitRepos', parent: 'pipeline' },
  '/pipeline/artifacts': { titleKey: 'breadcrumb.artifacts', parent: 'pipeline' },
  '/pipeline/credentials': { titleKey: 'breadcrumb.credentials', parent: 'pipeline' },
  '/pipeline/variables': { titleKey: 'breadcrumb.variables', parent: 'pipeline' },
  '/pipeline/templates': { titleKey: 'breadcrumb.templates', parent: 'pipeline' },
  '/sonarqube': { titleKey: 'breadcrumb.sonarqube', parent: 'pipeline' },
  '/pipeline/stats': { titleKey: 'breadcrumb.pipelineStats', parent: 'pipeline' },
  '/pipeline/create': { titleKey: 'breadcrumb.pipelineCreate', parent: 'pipeline' },
  '/k8s/overview': { titleKey: 'breadcrumb.k8sOverview', parent: 'infrastructure' },
  '/k8s/clusters': { titleKey: 'breadcrumb.k8sClusters', parent: 'infrastructure' },
  '/k8s/terminal': { titleKey: 'breadcrumb.k8sPods', parent: 'infrastructure' },
  '/security/overview': { titleKey: 'breadcrumb.securityOverview', parent: 'infrastructure' },
  '/security/image-scan': { titleKey: 'breadcrumb.imageScan', parent: 'infrastructure' },
  '/security/image-registry': { titleKey: 'menu.v2.imageRegistry', parent: 'infrastructure' },
  '/security/config-check': { titleKey: 'breadcrumb.configCheck', parent: 'infrastructure' },
  '/security/audit-log': { titleKey: 'breadcrumb.auditLog', parent: 'infrastructure' },
  '/approval/pending': { titleKey: 'breadcrumb.pending', parent: 'releaseChange' },
  '/approval/history': { titleKey: 'breadcrumb.approvalHistory', parent: 'releaseChange' },
  '/approval/chains': { titleKey: 'breadcrumb.chains', parent: 'releaseChange' },
  '/approval/instances': { titleKey: 'breadcrumb.instances', parent: 'releaseChange' },
  '/approval/rules': { titleKey: 'breadcrumb.rules', parent: 'releaseChange' },
  '/approval/windows': { titleKey: 'breadcrumb.windows', parent: 'releaseChange' },
  '/approval/env-policies': { titleKey: 'breadcrumb.envAuditPolicy', parent: 'releaseChange' },
  '/deploy/locks': { titleKey: 'breadcrumb.deployLocks', parent: 'releaseChange' },
  '/jira/integration': { titleKey: 'breadcrumb.jiraIntegration', parent: 'planning' },
  '/argocd': { titleKey: 'breadcrumb.argoCDIntegration', parent: 'releaseChange' },
  '/database/instances': { titleKey: 'breadcrumb.dbInstance', parent: 'infrastructure' },
  '/database/console': { titleKey: 'breadcrumb.dbConsole', parent: 'infrastructure' },
  '/database/logs': { titleKey: 'breadcrumb.dbQueryLogs', parent: 'infrastructure' },
  '/database/tickets': { titleKey: 'breadcrumb.dbTickets', parent: 'releaseChange' },
  '/database/tickets/create': { titleKey: 'breadcrumb.dbTicketCreate', parent: 'releaseChange' },
  '/database/rules': { titleKey: 'breadcrumb.dbAuditRule', parent: 'infrastructure' },
  '/database/statements': { titleKey: 'breadcrumb.dbStatements', parent: 'infrastructure' },
  '/telegram/message': { titleKey: 'breadcrumb.telegram', parent: 'operations' },
  '/biz/goals': { titleKey: 'menu.v2.goals', parent: 'planning' },
  '/biz/requirements': { titleKey: 'menu.v2.requirements', parent: 'planning' },
  '/biz/versions': { titleKey: 'menu.v2.versions', parent: 'planning' },
  '/alert/overview': { titleKey: 'breadcrumb.alertOverview', parent: 'operations' },
  '/alert/center': { titleKey: 'breadcrumb.alertCenter', parent: 'operations' },
  '/alert/history': { titleKey: 'breadcrumb.alertHistory', parent: 'operations' },
  '/alert/config': { titleKey: 'breadcrumb.alertConfig', parent: 'operations' },
  '/alert/silence': { titleKey: 'breadcrumb.silence', parent: 'operations' },
  '/alert/escalation': { titleKey: 'breadcrumb.escalation', parent: 'operations' },
  '/oncall': { titleKey: 'breadcrumb.oncall', parent: 'operations' },
  '/observability/event-timeline': { titleKey: 'breadcrumb.eventTimeline', parent: 'operations' },
  '/tracing': { titleKey: 'menu.v2.tracing', parent: 'operations' },
  '/prometheus': { titleKey: 'menu.v2.metrics', parent: 'operations' },
  '/cost/overview': { titleKey: 'breadcrumb.costOverview', parent: 'platformGovernance' },
  '/cost/trend': { titleKey: 'breadcrumb.costTrend', parent: 'platformGovernance' },
  '/cost/comparison': { titleKey: 'breadcrumb.costComparison', parent: 'platformGovernance' },
  '/cost/analysis': { titleKey: 'breadcrumb.costAnalysis', parent: 'platformGovernance' },
  '/cost/waste': { titleKey: 'breadcrumb.costWaste', parent: 'platformGovernance' },
  '/cost/suggestions': { titleKey: 'breadcrumb.costSuggestions', parent: 'platformGovernance' },
  '/cost/alerts': { titleKey: 'breadcrumb.costAlerts', parent: 'platformGovernance' },
  '/cost/budget': { titleKey: 'breadcrumb.costBudget', parent: 'platformGovernance' },
  '/cost/config': { titleKey: 'breadcrumb.costConfig', parent: 'platformGovernance' },
  '/logs/unified': { titleKey: 'breadcrumb.logsCenter', parent: 'operations' },
  '/logs/center': { titleKey: 'breadcrumb.logsCenter', parent: 'operations' },
  '/logs/search': { titleKey: 'breadcrumb.logsSearch', parent: 'operations' },
  '/logs/stats': { titleKey: 'breadcrumb.logsStats', parent: 'operations' },
  '/logs/compare': { titleKey: 'breadcrumb.logsCompare', parent: 'operations' },
  '/logs/alerts': { titleKey: 'breadcrumb.logsAlerts', parent: 'operations' },
  '/logs/bookmarks': { titleKey: 'breadcrumb.logsBookmarks', parent: 'operations' },
  '/logs/viewer': { titleKey: 'breadcrumb.logsViewer', parent: 'operations' },
  '/logs/export': { titleKey: 'breadcrumb.logsExport', parent: 'operations' },
  '/catalog': { titleKey: 'breadcrumb.catalog', parent: 'appDelivery' },
  '/nacos/config': { titleKey: 'breadcrumb.nacosConfig', parent: 'infrastructure' },
  '/nacos/releases': { titleKey: 'breadcrumb.nacosReleases', parent: 'releaseChange' },
  '/users': { titleKey: 'breadcrumb.users', parent: 'platformGovernance' },
  '/rbac/roles': { titleKey: 'breadcrumb.roles', parent: 'platformGovernance' },
  '/system/ldap': { titleKey: 'breadcrumb.ldap', parent: 'platformGovernance' },
  '/audit/logs': { titleKey: 'breadcrumb.auditLogs', parent: 'platformGovernance' },
  '/profile': { titleKey: 'breadcrumb.profile' },
}

const parentTitles: Record<string, { titleKey: string; path?: string }> = {
  planning: { titleKey: 'menu.v2.planning', path: '/biz/goals' },
  appDelivery: { titleKey: 'menu.v2.apps', path: '/applications' },
  releaseChange: { titleKey: 'menu.v2.releases', path: '/releases' },
  operations: { titleKey: 'menu.v2.ops', path: '/alert/center' },
  infrastructure: { titleKey: 'menu.v2.infra', path: '/k8s/overview' },
  platformGovernance: { titleKey: 'menu.v2.platform', path: '/users' },
  app: { titleKey: 'menu.v2.apps', path: '/applications' },
  healthcheck: { titleKey: 'menu.v2.ops', path: '/alert/center' },
  pipeline: { titleKey: 'menu.v2.pipelines', path: '/pipeline/list' },
  k8s: { titleKey: 'menu.v2.infra', path: '/k8s/overview' },
  approval: { titleKey: 'menu.v2.releases', path: '/approval/pending' },
  jira: { titleKey: 'menu.v2.planning', path: '/biz/goals' },
  argocd: { titleKey: 'menu.v2.releases', path: '/argocd' },
  database: { titleKey: 'menu.v2.infra', path: '/database/instances' },
  nacos: { titleKey: 'menu.v2.releases', path: '/nacos/releases' },
  message: { titleKey: 'menu.v2.ops', path: '/alert/center' },
  alert: { titleKey: 'menu.v2.ops', path: '/alert/center' },
  observability: { titleKey: 'menu.v2.ops', path: '/alert/center' },
  cost: { titleKey: 'menu.v2.platform', path: '/cost/overview' },
  logs: { titleKey: 'menu.v2.ops', path: '/alert/center' },
  system: { titleKey: 'menu.v2.platform', path: '/users' },
}

const breadcrumbs = computed(() => {
  const path = route.path
  const result: { title: string; path?: string }[] = [{ title: t('breadcrumb.home'), path: '/dashboard' }]

  // 处理动态路由，如 /pipeline/:id
  let matchedPath = path
  const config = breadcrumbMap[path]
  
  if (!config) {
    // 尝试匹配动态路由
    if (path.match(/^\/pipeline\/\d+$/)) {
      result.push({ title: t('menu.v2.pipelines'), path: '/pipeline/list' })
      result.push({ title: t('breadcrumb.pipelineDetail') })
      return result
    }
    if (path.match(/^\/pipeline\/artifacts\/\d+/)) {
      result.push({ title: t('menu.v2.pipelines'), path: '/pipeline/list' })
      result.push({ title: t('breadcrumb.artifacts'), path: '/pipeline/artifacts' })
      result.push({ title: t('breadcrumb.artifactDetail') })
      return result
    }
    if (path.match(/^\/pipeline\/edit\/\d+$/)) {
      result.push({ title: t('menu.v2.pipelines'), path: '/pipeline/list' })
      result.push({ title: t('breadcrumb.pipelineEdit') })
      return result
    }
    if (path.match(/^\/k8s\/clusters\/\d+\/resources$/)) {
      result.push({ title: t('menu.v2.infra'), path: '/k8s/overview' })
      result.push({ title: t('breadcrumb.k8sResources') })
      return result
    }
    if (path.match(/^\/k8s\/clusters\/\d+\/pods$/)) {
      result.push({ title: t('menu.v2.infra'), path: '/k8s/overview' })
      result.push({ title: t('breadcrumb.k8sPods') })
      return result
    }
    if (path.match(/^\/k8s\/clusters\/\d+\/deployments$/)) {
      result.push({ title: t('menu.v2.infra'), path: '/k8s/overview' })
      result.push({ title: t('breadcrumb.k8sDeployments') })
      return result
    }
    if (path.match(/^\/approval\/chains\/\d+\/design$/)) {
      result.push({ title: t('menu.v2.releases'), path: '/releases' })
      result.push({ title: t('breadcrumb.chains'), path: '/approval/chains' })
      result.push({ title: t('breadcrumb.chainDesign') })
      return result
    }
    if (path.match(/^\/approval\/instances\/\d+$/)) {
      result.push({ title: t('menu.v2.releases'), path: '/releases' })
      result.push({ title: t('breadcrumb.instances'), path: '/approval/instances' })
      result.push({ title: t('breadcrumb.instanceDetail') })
      return result
    }
    if (path.match(/^\/service-detail\/\d+$/)) {
      result.push({ title: t('menu.v2.apps'), path: '/applications' })
      result.push({ title: t('breadcrumb.serviceDetail') })
      return result
    }
    if (path.match(/^\/database\/tickets\/\d+$/)) {
      result.push({ title: t('menu.v2.releases'), path: '/releases' })
      result.push({ title: t('breadcrumb.dbTickets'), path: '/database/tickets' })
      result.push({ title: t('breadcrumb.dbTicketDetail') })
      return result
    }
    if (path.match(/^\/biz\/goals\/\d+$/)) {
      result.push({ title: t('menu.v2.planning'), path: '/biz/goals' })
      result.push({ title: t('menu.v2.goals'), path: '/biz/goals' })
      result.push({ title: '详情' })
      return result
    }
    if (path.match(/^\/biz\/requirements\/\d+$/)) {
      result.push({ title: t('menu.v2.planning'), path: '/biz/requirements' })
      result.push({ title: t('menu.v2.requirements'), path: '/biz/requirements' })
      result.push({ title: '详情' })
      return result
    }
    if (path.match(/^\/biz\/versions\/\d+$/)) {
      result.push({ title: t('menu.v2.planning'), path: '/biz/versions' })
      result.push({ title: t('menu.v2.versions'), path: '/biz/versions' })
      result.push({ title: '详情' })
      return result
    }
    return result
  }
  
  if (config.parent) {
    const parent = parentTitles[config.parent]
    if (parent) {
      result.push({ title: t(parent.titleKey), path: parent.path })
    }
  }
  
  result.push({ title: t(config.titleKey) })
  return result
})

const currentPageTitle = computed(() => breadcrumbs.value[breadcrumbs.value.length - 1]?.title || t('breadcrumb.home'))
const currentPageSubtitle = computed(() => {
  if (breadcrumbs.value.length > 1) {
    return breadcrumbs.value.slice(0, -1).map((item) => item.title).join(' / ')
  }
  return '按业务主线进入当前页面，减少在多级导航里反复寻找。'
})

// 根据路径获取父菜单 key
const getParentKey = (path: string): string => {
  for (const [key, prefixes] of Object.entries(menuGroupPrefixes)) {
    if (prefixes.some(prefix => path.startsWith(prefix))) {
      return key
    }
  }
  return ''
}

const getTopLevelMenuKeys = () => new Set(getV2MenuConfig().map(item => item.key))

const getAllowedOpenMenuKeys = () => {
  const allowed = new Set<string>()
  const collect = (items: MenuItemConfig[]) => {
    for (const item of items) {
      if (item.children?.length) {
        allowed.add(item.key)
        collect(item.children)
      }
    }
  }
  collect(getV2MenuConfig() as MenuItemConfig[])
  return allowed
}

const normalizeOpenKeys = (keys: string[]) => {
  const allowedOpenKeys = getAllowedOpenMenuKeys()
  return Array.from(
    new Set(keys.filter((item): item is string => Boolean(item) && allowedOpenKeys.has(item)))
  )
}

const findMenuOpenKeyPath = (items: MenuItemConfig[], selectedKey: string, parents: string[] = []): string[] => {
  for (const item of items) {
    if (item.key === selectedKey || item.path === selectedKey) {
      return parents
    }
    if (item.children?.length) {
      const found = findMenuOpenKeyPath(item.children, selectedKey, [...parents, item.key])
      if (found.length > 0) return found
    }
  }
  return []
}

const getOpenKeyPath = (path: string): string[] => {
  const selectedKey = resolveSelectedMenuKey(path)
  const menuPath = findMenuOpenKeyPath(getV2MenuConfig() as MenuItemConfig[], selectedKey)
  if (menuPath.length > 0) return menuPath

  const parentKey = getParentKey(path)
  return parentKey ? [parentKey] : []
}

// 从 localStorage 恢复菜单展开状态
const buildInitialOpenKeys = (role: RoleFocus, keys: string[]) => {
  const primary = rolePrimaryMenuKeyMap[role]
  const focused = normalizeOpenKeys(keys)
  return focused.length > 0 ? focused : normalizeOpenKeys([primary])
}

const getInitialOpenKeys = (): string[] => {
  const storedRole = permissionStore.roleGroup
  const saved = localStorage.getItem(`menuOpenKeys:${storedRole}`)
  if (saved) {
    try {
      return buildInitialOpenKeys(storedRole, [...getOpenKeyPath(route.path), ...JSON.parse(saved)])
    } catch (e) {
      console.error('Failed to parse menuOpenKeys from localStorage:', e)
    }
  }
  // 如果没有保存的状态，返回当前路由的父菜单
  return buildInitialOpenKeys(storedRole, getOpenKeyPath(route.path))
}

const userInfo = computed(() => userStore.userInfo)
const roleGroup = computed<RoleFocus>(() => permissionStore.roleGroup)

const openKeys = ref<string[]>(getInitialOpenKeys())
const openKeysBeforeCollapse = ref<string[]>([])

// ==================== 权限控制 ====================
// 权限判断逻辑统一走 usePermissionStore，见 @/stores/permission
const hasPermission = (requiredPermissions?: string[]): boolean => permissionStore.hasPermission(requiredPermissions)

// ==================== 菜单配置（带权限控制） ====================

interface MenuItemConfig {
  key: string
  icon?: any
  titleKey: string  // i18n key instead of hardcoded title
  path?: string
  children?: MenuItemConfig[]
  permission?: string[]
}

const findPathByKey = (items: MenuItemConfig[], key: string): string | null => {
  for (const item of items) {
    if (item.key === key) {
      return item.path || null
    }
    if (item.children?.length) {
      const found = findPathByKey(item.children, key)
      if (found) return found
    }
  }
  return null
}

// 定义菜单配置（包含权限信息）- 使用 i18n key
const getMenuConfig = (): MenuItemConfig[] => [
  {
    key: '/dashboard',
    icon: DashboardOutlined,
    titleKey: 'menu.workbench',
    path: '/dashboard'
    // 无 permission 字段，所有用户可见
  },
  {
    key: 'planning',
    icon: ProjectOutlined,
    titleKey: 'menu.planning',
    children: [
      { key: '/biz/goals', titleKey: 'menu.bizGoals', path: '/biz/goals' },
      { key: '/biz/requirements', titleKey: 'menu.bizRequirements', path: '/biz/requirements' },
      { key: '/biz/versions', titleKey: 'menu.bizVersions', path: '/biz/versions' },
      { key: '/jira/integration', titleKey: 'menu.jiraIntegration', path: '/jira/integration' }
    ]
  },
  {
    key: 'appDelivery',
    icon: AppstoreOutlined,
    titleKey: 'menu.appDelivery',
    permission: ['application:view'],
    children: [
      { key: '/applications', titleKey: 'menu.applications', path: '/applications' },
      { key: '/catalog', titleKey: 'menu.catalog', path: '/catalog' },
      { key: '/pipeline/list', titleKey: 'menu.pipelineList', path: '/pipeline/list' },
      { key: '/pipeline/stats', titleKey: 'menu.pipelineStats', path: '/pipeline/stats' },
      { key: '/pipeline/templates', titleKey: 'menu.templates', path: '/pipeline/templates', icon: ShopOutlined },
      { key: '/pipeline/git-repos', titleKey: 'menu.gitRepos', path: '/pipeline/git-repos' },
      { key: '/pipeline/artifacts', titleKey: 'menu.artifacts', path: '/pipeline/artifacts' },
      { key: '/pipeline/credentials', titleKey: 'menu.credentials', path: '/pipeline/credentials', icon: KeyOutlined, permission: ['pipeline:manage'] },
      { key: '/pipeline/variables', titleKey: 'menu.variables', path: '/pipeline/variables', icon: ControlOutlined, permission: ['pipeline:manage'] },
      { key: '/sonarqube', titleKey: 'menu.sonarqube', path: '/sonarqube' }
    ]
  },
  {
    key: 'releaseChange',
    icon: RocketOutlined,
    titleKey: 'menu.releaseChange',
    children: [
      { key: '/deploy/timeline', titleKey: 'menu.changeTimeline', path: '/deploy/timeline' },
      { key: '/deploy/env-instances', titleKey: 'menu.envInstances', path: '/deploy/env-instances' },
      { key: '/deploy/check', titleKey: 'menu.deployCheck', path: '/deploy/check' },
      { key: '/approval/pending', titleKey: 'menu.pending', path: '/approval/pending' },
      { key: '/approval/history', titleKey: 'menu.approvalHistory', path: '/approval/history' },
      { key: '/approval/chains', titleKey: 'menu.chains', path: '/approval/chains', permission: ['approval:manage'] },
      { key: '/approval/instances', titleKey: 'menu.instances', path: '/approval/instances' },
      { key: '/approval/rules', titleKey: 'menu.rules', path: '/approval/rules', permission: ['approval:manage'] },
      { key: '/approval/windows', titleKey: 'menu.windows', path: '/approval/windows', permission: ['approval:manage'] },
      { key: '/approval/env-policies', titleKey: 'menu.envAuditPolicy', path: '/approval/env-policies', permission: ['approval:manage'] },
      { key: '/deploy/locks', titleKey: 'menu.deployLocks', path: '/deploy/locks' },
      { key: '/argocd', titleKey: 'menu.argoCDIntegration', path: '/argocd' },
      { key: '/nacos/config', titleKey: 'menu.nacosConfig', path: '/nacos/config' },
      { key: '/nacos/releases', titleKey: 'menu.nacosReleases', path: '/nacos/releases' }
    ]
  },
  {
    key: 'operations',
    icon: AlertOutlined,
    titleKey: 'menu.operations',
    children: [
      { key: '/healthcheck', titleKey: 'menu.serviceHealth', path: '/healthcheck' },
      { key: '/healthcheck/ssl-cert', titleKey: 'menu.sslCert', path: '/healthcheck/ssl-cert', icon: SafetyCertificateOutlined },
      { key: '/alert/overview', titleKey: 'menu.alertOverview', path: '/alert/overview' },
      { key: '/alert/history', titleKey: 'menu.alertHistory', path: '/alert/history' },
      { key: '/alert/config', titleKey: 'menu.alertConfig', path: '/alert/config', permission: ['alert:manage'] },
      { key: '/alert/templates', titleKey: 'menu.alertTemplates', path: '/alert/templates', permission: ['alert:manage'] },
      { key: '/alert/gateway', titleKey: 'menu.alertGateway', path: '/alert/gateway' },
      { key: '/alert/silence', titleKey: 'menu.silence', path: '/alert/silence', permission: ['alert:manage'] },
      { key: '/alert/escalation', titleKey: 'menu.escalation', path: '/alert/escalation', permission: ['alert:manage'] },
      { key: '/oncall', titleKey: 'menu.oncall', path: '/oncall' },
      { key: '/prometheus', titleKey: 'menu.prometheusMetrics', path: '/prometheus' },
      { key: '/tracing', titleKey: 'menu.tracing', path: '/tracing' },
      { key: '/logs/center', titleKey: 'menu.logsCenter', path: '/logs/center' },
      { key: '/logs/search', titleKey: 'menu.logsSearch', path: '/logs/search' },
      { key: '/logs/stats', titleKey: 'menu.logsStats', path: '/logs/stats' },
      { key: '/logs/compare', titleKey: 'menu.logsCompare', path: '/logs/compare' },
      { key: '/logs/alerts', titleKey: 'menu.logsAlerts', path: '/logs/alerts' },
      { key: '/logs/bookmarks', titleKey: 'menu.logsBookmarks', path: '/logs/bookmarks' },
      { key: '/logs/export', titleKey: 'menu.logsExport', path: '/logs/export' },
      { key: '/telegram/message', titleKey: 'menu.message', path: '/telegram/message' }
    ]
  },
  {
    key: 'infrastructure',
    icon: SettingOutlined,
    titleKey: 'menu.infrastructure',
    children: [
      { key: '/k8s/overview', titleKey: 'menu.k8sOverview', path: '/k8s/overview' },
      { key: '/k8s/clusters', titleKey: 'menu.k8sClusters', path: '/k8s/clusters' },
      { key: '/security/overview', titleKey: 'menu.securityOverview', path: '/security/overview' },
      { key: '/security/image-registry', titleKey: 'menu.imageRegistry', path: '/security/image-registry' },
      { key: '/security/image-scan', titleKey: 'menu.imageScan', path: '/security/image-scan' },
      { key: '/security/config-check', titleKey: 'menu.configCheck', path: '/security/config-check' },
      { key: '/security/audit-log', titleKey: 'menu.auditLog', path: '/security/audit-log' },
      { key: '/database/instances', titleKey: 'menu.dbInstance', path: '/database/instances' },
      { key: '/database/console', titleKey: 'menu.dbConsole', path: '/database/console' },
      { key: '/database/tickets', titleKey: 'menu.dbTickets', path: '/database/tickets' },
      { key: '/database/rules', titleKey: 'menu.dbAuditRule', path: '/database/rules' },
      { key: '/database/statements', titleKey: 'menu.dbStatements', path: '/database/statements' },
      { key: '/database/logs', titleKey: 'menu.dbQueryLogs', path: '/database/logs' }
    ]
  },
  {
    key: 'platformGovernance',
    icon: DollarOutlined,
    titleKey: 'menu.platformGovernance',
    children: [
      { key: '/cost/overview', titleKey: 'menu.costOverview', path: '/cost/overview' },
      { key: '/cost/trend', titleKey: 'menu.costTrend', path: '/cost/trend' },
      { key: '/cost/comparison', titleKey: 'menu.costComparison', path: '/cost/comparison' },
      { key: '/cost/analysis', titleKey: 'menu.costAnalysis', path: '/cost/analysis' },
      { key: '/cost/waste', titleKey: 'menu.costWaste', path: '/cost/waste' },
      { key: '/cost/suggestions', titleKey: 'menu.costSuggestions', path: '/cost/suggestions' },
      { key: '/cost/alerts', titleKey: 'menu.costAlerts', path: '/cost/alerts' },
      { key: '/cost/budget', titleKey: 'menu.costBudget', path: '/cost/budget', permission: ['cost:manage'] },
      { key: '/cost/config', titleKey: 'menu.costConfig', path: '/cost/config', permission: ['cost:manage'] },
      { key: '/users', titleKey: 'menu.users', path: '/users', permission: ['system:manage'] },
      { key: '/rbac/roles', titleKey: 'menu.roles', path: '/rbac/roles', permission: ['system:manage'] },
      { key: '/system/ldap', titleKey: 'menu.ldap', path: '/system/ldap', permission: ['system:manage'] },
      { key: '/audit/logs', titleKey: 'menu.auditLogs', path: '/audit/logs', permission: ['system:manage'] },
    ]
  }
]

/**
 * 递归过滤菜单项，只保留有权限的菜单
 * @param items 菜单配置数组
 * @returns 过滤后的菜单配置数组
 */
const rewriteV2AlertMenuPaths = (items: MenuItemConfig[]) => {
  for (const it of items) {
    if (it.path === '/alert/overview' && it.key === '/alert/overview') {
      it.path = '/alert/center'
      it.key = '/alert/center'
    }
    if (it.children?.length) rewriteV2AlertMenuPaths(it.children)
  }
}

const rewriteV2LogsMenuPaths = (items: MenuItemConfig[]) => {
  for (const it of items) {
    if (it.path === '/logs/center' && it.key === '/logs/center') {
      it.path = '/logs/unified'
      it.key = '/logs/unified'
    }
    if (it.children?.length) rewriteV2LogsMenuPaths(it.children)
  }
}

const filterMenuByPermission = (items: MenuItemConfig[]): MenuItemConfig[] => {
  return items.filter(item => {
    // 检查当前菜单项权限
    if (!hasPermission(item.permission)) {
      return false
    }
    
    // 如果有子菜单，递归过滤
    if (item.children && item.children.length > 0) {
      item.children = filterMenuByPermission(item.children)
      // 如果过滤后没有子菜单了，也不显示父菜单
      if (item.children.length === 0) {
        return false
      }
    }
    
    return true
  })
}

// 计算过滤后的菜单配置（固定 V2：7 域精简菜单，保持声明顺序）
const filteredMenuConfig = computed(() => {
  // 保留 V1 菜单定义用于历史对照，当前固定渲染 V2。
  void getMenuConfig
  const raw = JSON.parse(JSON.stringify(getV2MenuConfig())) as MenuItemConfig[]
  rewriteV2AlertMenuPaths(raw)
  rewriteV2LogsMenuPaths(raw)
  return filterMenuByPermission(raw)
})

// 监听路由变化，更新选中状态和自动展开父菜单
watch(
  () => route.path,
  () => {
    selectedKeys.value = [resolveSelectedMenuKey(route.path)]
    openKeys.value = buildInitialOpenKeys(roleGroup.value, getOpenKeyPath(route.path))
  },
)

watch(roleGroup, (newRole) => {
  openKeys.value = buildInitialOpenKeys(newRole, getOpenKeyPath(route.path))
})

// 监听菜单展开状态变化，保存到 localStorage
watch(openKeys, (newKeys) => {
  localStorage.setItem(`menuOpenKeys:${roleGroup.value}`, JSON.stringify(newKeys))
}, { deep: true })

// 监听窗口大小变化，移动端自动折叠侧边栏
watch(isMobile, (mobile) => {
  if (mobile) {
    collapsed.value = true
  }
}, { immediate: true })

watch(collapsed, (isCollapsed) => {
  if (isCollapsed) {
    openKeysBeforeCollapse.value = [...openKeys.value]
    openKeys.value = []
    return
  }

  openKeys.value = normalizeOpenKeys(
    openKeysBeforeCollapse.value.length > 0 ? openKeysBeforeCollapse.value : getOpenKeyPath(route.path)
  )
})

const handleMenuClick = ({ key }: { key: string }) => {
  const keyString = String(key)
  const targetPath =
    keyString.startsWith('/') ? keyString : findPathByKey(filteredMenuConfig.value, keyString)

  if (!targetPath) {
    return
  }

  if (route.path !== targetPath) {
    void router.push(targetPath)
  }

  // 移动端点击菜单后自动折叠
  if (isMobile.value) {
    collapsed.value = true
  }
}

const handleOpenChange = (keys: string[]) => {
  const normalized = normalizeOpenKeys(keys)
  const topLevelKeys = getTopLevelMenuKeys()
  const latest = normalized.find(key => !openKeys.value.includes(key))
  if (latest && topLevelKeys.has(latest)) {
    openKeys.value = [latest]
    return
  }

  const activeTopLevel = [...normalized].reverse().find(key => topLevelKeys.has(key))
  openKeys.value = normalizeOpenKeys(
    activeTopLevel
      ? normalized.filter(key => !topLevelKeys.has(key) || key === activeTopLevel)
      : normalized
  )
}

const expandAllMenus = () => {
  openKeys.value = Array.from(getAllowedOpenMenuKeys())
}

const collapseAllMenus = () => {
  openKeys.value = []
}

const goToMenuPath = (path: string) => {
  router.push(path)
  if (isMobile.value) {
    collapsed.value = true
  }
}

const handleDropdownClick = ({ key }: { key: string }) => {
  if (key === 'profile') {
    router.push('/profile')
  } else if (key === 'logout') {
    handleLogout()
  }
}

const handleLogout = () => {
  userStore.clearSession()
  message.success('已退出登录')
  router.push('/login')
}
</script>

<style scoped>
/* 左侧栏固定布局 */
:deep(.ant-layout-sider) {
  position: fixed !important;
  left: 0;
  top: 0;
  bottom: 0;
  height: 100vh;
  overflow-y: auto;
  overflow-x: hidden;
  z-index: 100;
  background: linear-gradient(180deg, #1a2332 0%, #2c3e50 50%, #34495e 100%) !important;
}

/* 隐藏左侧栏滚动条 */
:deep(.ant-layout-sider)::-webkit-scrollbar {
  width: 0;
  height: 0;
}

:deep(.ant-layout-sider) {
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE 10+ */
}

/* 菜单背景透明，显示渐变色 */
:deep(.ant-menu-dark) {
  background: transparent !important;
}

:deep(.ant-menu-dark .ant-menu-sub) {
  background: rgba(0, 0, 0, 0.2) !important;
}

/* 菜单项悬停效果 */
:deep(.ant-menu-dark .ant-menu-item:hover),
:deep(.ant-menu-dark .ant-menu-submenu-title:hover) {
  background: rgba(255, 255, 255, 0.1) !important;
}

/* 菜单项选中效果 */
:deep(.ant-menu-dark .ant-menu-item-selected) {
  background: linear-gradient(90deg, rgba(24, 144, 255, 0.3) 0%, rgba(24, 144, 255, 0.1) 100%) !important;
  border-right: 3px solid #1890ff;
}

/* 主内容区域左侧留出空间 */
.content-layout {
  margin-left: 200px;
  transition: margin-left 0.2s;
}

/* 侧边栏折叠时调整主内容区域 */
:deep(.ant-layout-sider-collapsed) ~ .content-layout {
  margin-left: 80px;
}

.logo {
  height: 64px;
  line-height: 64px;
  text-align: center;
  color: #fff;
  font-size: 18px;
  font-weight: bold;
  white-space: nowrap;
  overflow: hidden;
  flex-shrink: 0;
  background: rgba(0, 0, 0, 0.2);
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
}

.menu-actions {
  display: flex;
  gap: 4px;
  justify-content: flex-end;
  padding: 6px 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.12);
}

.menu-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border: none;
  border-radius: 4px;
  color: rgba(255, 255, 255, 0.65);
  background: transparent;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
}

.menu-action-btn:hover {
  color: #fff;
  background: rgba(24, 144, 255, 0.25);
}

/* 菜单区域可滚动 */
:deep(.ant-menu) {
  border-right: 0;
}

.trigger {
  font-size: 18px;
  line-height: 64px;
  padding: 0 16px 0 0;
  cursor: pointer;
  transition: color 0.3s;
}

.trigger:hover {
  color: #1890ff;
}

.workspace-header {
  background: #fff;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #f0f0f0;
}

.header-left {
  display: flex;
  align-items: center;
  min-width: 0;
}

.header-context {
  min-width: 0;
}

.header-title {
  font-size: 18px;
  font-weight: 600;
  color: #111827;
  line-height: 1.2;
}

.header-subtitle {
  margin-top: 2px;
  color: #8c8c8c;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.header-right {
  display: flex;
  align-items: center;
}

.header-shortcuts {
  padding-right: 8px;
  border-right: 1px solid #f0f0f0;
}

.user-entry {
  cursor: pointer;
}

/* 移动端适配 */
@media (max-width: 768px) {
  .logo {
    font-size: 16px;
  }

  .trigger {
    padding: 0 12px 0 0;
  }

  :deep(.ant-layout-header) {
    padding: 0 16px !important;
  }

  .workspace-header {
    height: auto;
    min-height: 64px;
    padding: 10px 16px;
    align-items: flex-start;
    gap: 12px;
  }

  .header-left,
  .header-right {
    width: 100%;
  }

  .header-right {
    justify-content: space-between;
  }

  .header-shortcuts {
    display: none;
  }

  :deep(.ant-layout-content) {
    margin: 8px !important;
  }

  :deep(.ant-layout-content > div) {
    padding: 16px !important;
  }

  :deep(.ant-breadcrumb) {
    margin-bottom: 8px !important;
  }

  /* 移动端侧边栏覆盖在内容上方 */
  :deep(.ant-layout-sider) {
    position: fixed !important;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 999;
  }

  /* 侧边栏折叠时不占用空间 */
  :deep(.ant-layout-sider-collapsed) {
    transform: translateX(-100%);
  }

  .content-layout {
    margin-left: 0 !important;
  }

  /* 侧边栏展开时显示遮罩 */
  :deep(.ant-layout-sider:not(.ant-layout-sider-collapsed))::before {
    content: '';
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.45);
    z-index: -1;
  }
}

/* 平板设备适配 */
@media (min-width: 769px) and (max-width: 1024px) {
  .logo {
    font-size: 16px;
  }

  :deep(.ant-layout-sider) {
    width: 180px !important;
    min-width: 180px !important;
    max-width: 180px !important;
  }

  .content-layout {
    margin-left: 180px;
  }

  :deep(.ant-layout-sider-collapsed) ~ .content-layout {
    margin-left: 80px;
  }
}
</style>
