<template>
  <div class="dashboard">
    <a-row :gutter="[16, 16]">
      <a-col :span="24">
        <a-card :bordered="false" class="hero-card">
          <a-row :gutter="16" align="middle">
            <a-col :xs="24" :lg="14">
              <div class="hero-title">欢迎回来，{{ currentUser.username || '同学' }}</div>
              <div class="hero-subtitle">{{ projectScopeText || '今天优先处理你的高频任务，不需要先理解平台结构，也能直接开始做事。' }}</div>
              <a-space wrap class="hero-meta">
                <a-tag color="blue">{{ roleLabel }}</a-tag>
                <a-tag v-if="projectId" color="cyan">项目工作台 #{{ projectId }}</a-tag>
                <a-tag :color="getHealthColor(healthOverview.status)">{{ getHealthText(healthOverview.status) }}</a-tag>
                <a-tag color="purple">交付 {{ pipelineStats.total }}</a-tag>
                <a-tag color="orange">待审批 {{ stats.pendingApprovals || 0 }}</a-tag>
                <a-tag color="red">告警 {{ stats.alertsToday }}</a-tag>
              </a-space>
            </a-col>
            <a-col :xs="24" :lg="10">
              <div class="hero-side">
                <a-radio-group v-model:value="currentFocus" button-style="solid" size="small">
                  <a-radio-button value="delivery">交付优先</a-radio-button>
                  <a-radio-button value="operations">运行优先</a-radio-button>
                  <a-radio-button value="platform">平台优先</a-radio-button>
                </a-radio-group>
                <div class="hero-priority-card">
                  <div class="hero-priority-label">当前建议</div>
                  <div class="hero-priority-title">{{ focusHeadline.title }}</div>
                  <div class="hero-priority-desc">{{ focusHeadline.description }}</div>
                  <a-button type="primary" block @click="goTo(focusHeadline.path)">{{ focusHeadline.action }}</a-button>
                </div>
              </div>
            </a-col>
          </a-row>
        </a-card>
      </a-col>

      <a-col :xs="24" :md="12" :xl="6" v-for="item in focusSummaryCards" :key="item.title">
        <a-card hoverable :bordered="false" class="summary-card" @click="goTo(item.path)">
          <a-statistic :title="item.title" :value="item.value" :suffix="item.suffix" :value-style="{ color: item.color }">
            <template #prefix>
              <component :is="item.icon" />
            </template>
          </a-statistic>
          <div class="summary-desc">{{ item.description }}</div>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="[16, 16]" class="section-row">
      <a-col :xs="24" :xl="16">
        <a-card :bordered="false" class="delivery-chain-card full-height-card">
          <div class="delivery-chain-header section-header">
            <div>
              <div class="section-title"><RocketOutlined /> 应用交付链路</div>
              <div class="section-subtitle">{{ deliveryChainSubtitle }}</div>
            </div>
            <a-space wrap>
              <a-button type="primary" @click="goTo(primaryAppPath)">
                <AppstoreOutlined /> 进入应用
              </a-button>
              <a-button @click="goTo(projectId ? `/argocd?project_id=${projectId}` : '/argocd')">
                <RocketOutlined /> GitOps 交付
              </a-button>
            </a-space>
          </div>
          <div class="delivery-chain">
            <button
              v-for="step in deliveryFlowSteps"
              :key="step.key"
              type="button"
              class="delivery-step"
              @click="goTo(step.path)"
            >
              <span class="delivery-step-icon">
                <component :is="step.icon" />
              </span>
              <span class="delivery-step-body">
                <span class="delivery-step-top">
                  <span class="delivery-step-title">{{ step.title }}</span>
                  <a-tag :color="step.color">{{ step.status }}</a-tag>
                </span>
                <span class="delivery-step-desc">{{ step.description }}</span>
              </span>
            </button>
          </div>
        </a-card>
      </a-col>

      <a-col :xs="24" :xl="8">
        <a-card title="今天先做什么" :bordered="false" class="full-height-card" :loading="loadingActions">
          <template #extra>
            <a-button type="link" size="small" @click="fetchWorkspaceActions">刷新</a-button>
          </template>
          <a-list :data-source="actionItemsForFocus" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <div class="todo-main">
                  <div class="todo-title">{{ item.title }}</div>
                  <div class="todo-desc">{{ item.description }}</div>
                </div>
                <a-space>
                  <a-tag :color="actionPriorityColor(item.priority)">{{ actionPriorityText(item.priority) }}</a-tag>
                  <a-button type="link" @click="goTo(item.path)">{{ item.action_label || '处理' }}</a-button>
                </a-space>
              </a-list-item>
            </template>
            <template #empty>
              <a-empty description="暂无待处理行动" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
            </template>
          </a-list>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="[16, 16]" class="section-row dora-row">
      <a-col :span="24">
        <div class="dora-section-title">
          <RocketOutlined /> DORA 四指标
          <a-tag color="purple" style="margin-left: 8px">v2</a-tag>
          <span class="dora-section-hint">prod 环境过去 7 天 · 同口径环比</span>
          <a-button v-if="!doraLoading" type="link" size="small" @click="fetchDORA">刷新</a-button>
          <a-spin v-else size="small" style="margin-left: 8px" />
          <a-select
            v-model:value="doraAppId"
            size="small"
            placeholder="全部应用"
            allow-clear
            show-search
            option-filter-prop="label"
            style="margin-left: 12px; width: 180px"
            :options="doraAppOptions"
            @change="fetchDORA"
          />
          <a-switch
            v-model:checked="doraComparePrev"
            size="small"
            style="margin-left: 12px"
            checked-children="叠加上期"
            un-checked-children="仅当前"
          />
          <span v-if="doraError" class="dora-section-hint" style="color: #ff4d4f; margin-left: 8px">
            {{ doraError }}
          </span>
        </div>
      </a-col>
      <a-col :xs="24" :md="12" :xl="6" v-for="card in doraCards" :key="card.key">
        <a-card hoverable :bordered="false" class="dora-card">
          <div class="dora-card-header">
            <span class="dora-card-title">{{ card.title }}</span>
            <a-space :size="4">
              <a-tooltip v-if="card.app_vs_fleet" :title="doraFleetTooltip(card)">
                <a-tag :color="doraAppVsFleetColor(card.app_vs_fleet)" class="dora-fleet-badge">
                  {{ doraAppVsFleetIcon(card.app_vs_fleet) }} vs 全站
                </a-tag>
              </a-tooltip>
              <a-tag :color="doraBenchmarkColor(card.benchmark)">{{ card.benchmark }}</a-tag>
            </a-space>
          </div>
          <div class="dora-card-value">
            <span class="dora-value">{{ card.value }}</span>
            <span class="dora-unit">{{ card.unit }}</span>
          </div>
          <div class="dora-card-trend-row">
            <div class="dora-card-trend" :style="{ color: doraTrendColor(card) }">
              {{ doraTrendIcon(card.trend) }} {{ card.delta }}
            </div>
            <DORASparkline
              v-if="card.series && card.series.length > 0"
              :points="card.series"
              :prev-points="doraComparePrev ? card.prev_series || [] : []"
              :color="doraTrendColor(card) || '#1677ff'"
              :unit="card.unit"
              :width="120"
              :height="32"
            />
          </div>
          <div class="dora-card-desc">{{ card.description }}</div>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="[16, 16]" class="section-row">
      <a-col :xs="24" :lg="12">
        <a-card title="快速开始" :bordered="false" class="full-height-card">
          <a-space direction="vertical" style="width: 100%">
            <a-button v-for="item in quickActions" :key="item.title" block @click="goTo(item.path)">
              <component :is="item.icon" /> {{ item.title }}
            </a-button>
          </a-space>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="12">
        <a-card :title="focusPanelTitle" :bordered="false" class="full-height-card">
          <a-list :data-source="focusInsights" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <div class="todo-main">
                  <div class="todo-title">{{ item.title }}</div>
                  <div class="todo-desc">{{ item.description }}</div>
                </div>
                <a-button type="link" @click="goTo(item.path)">进入</a-button>
              </a-list-item>
            </template>
          </a-list>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="[16, 16]" class="section-row">
      <a-col :xs="24" :lg="8">
        <a-card title="系统状态" :bordered="false" class="full-height-card">
          <div class="status-grid">
            <div class="status-item">
              <div class="status-label">健康检查</div>
              <div class="status-value">{{ healthOverview.healthy }}/{{ healthOverview.total }}</div>
            </div>
            <div class="status-item">
              <div class="status-label">构建成功率</div>
              <div class="status-value">{{ pipelineStats.successRate }}%</div>
            </div>
            <div class="status-item">
              <div class="status-label">安全高风险</div>
              <div class="status-value danger">{{ securityStats.highRisk }}</div>
            </div>
            <div class="status-item">
              <div class="status-label">本月成本</div>
              <div class="status-value">¥{{ formatCost(costStats.monthCost) }}</div>
            </div>
          </div>
        </a-card>
      </a-col>

      <a-col :xs="24" :lg="8">
        <a-card title="最近构建" :bordered="false" class="full-height-card">
          <a-list :data-source="recentPipelineRuns" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <a-list-item-meta :title="item.pipeline_name || `流水线 #${item.pipeline_id}`" :description="formatTime(item.created_at)" />
                <template #actions>
                  <a-tag :color="getRunStatusColor(item.status)">{{ getRunStatusText(item.status) }}</a-tag>
                </template>
              </a-list-item>
            </template>
            <template #empty>
              <a-empty description="暂无执行记录" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
            </template>
          </a-list>
        </a-card>
      </a-col>

      <a-col :xs="24" :lg="8">
        <a-card title="最近告警" :bordered="false" class="full-height-card">
          <a-list :data-source="recentAlerts" :loading="loadingAlerts" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <a-list-item-meta :title="item.title || item.type" :description="formatTime(item.created_at)" />
                <template #actions>
                  <a-tag :color="getLevelColor(item.level)">{{ getLevelText(item.level) }}</a-tag>
                </template>
              </a-list-item>
            </template>
            <template #empty>
              <a-empty description="暂无告警" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
            </template>
          </a-list>
        </a-card>
      </a-col>

      <a-col :xs="24" :lg="8">
        <a-card title="最近操作" :bordered="false" class="full-height-card">
          <a-list :data-source="recentAudits" :loading="loadingAudits" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <a-list-item-meta :title="item.username" :description="`${getActionText(item.action)} ${item.resource} · ${formatTime(item.created_at)}`" />
              </a-list-item>
            </template>
            <template #empty>
              <a-empty description="暂无操作记录" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
            </template>
          </a-list>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Empty } from 'ant-design-vue'
import {
  AlertOutlined,
  AppstoreOutlined,
  AuditOutlined,
  CheckCircleOutlined,
  CloudOutlined,
  DollarOutlined,
  FileSearchOutlined,
  ForkOutlined,
  PlusOutlined,
  RocketOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
} from '@ant-design/icons-vue'
import { dashboardApi, type DashboardStats, type HealthOverview, type RecentAlert, type RecentAudit, type WorkspaceActionItem } from '@/services/dashboard'
import request from '@/services/api'
import { doraApi, type DORAMetric, type DORASeriesPoint } from '@/services/dora'
import DORASparkline from '@/components/DORASparkline.vue'
import { useUserStore } from '@/stores/user'

type FocusType = 'delivery' | 'operations' | 'platform'

// v2.0 DORA 卡片：固定展示，指标由后端聚合回填。
type DoraTrend = 'up' | 'down' | 'flat'
interface DoraCard {
  key: 'deploy_freq' | 'lead_time' | 'change_fail_rate' | 'mttr'
  title: string
  value: string
  unit: string
  trend: DoraTrend
  delta: string
  benchmark: 'elite' | 'high' | 'medium' | 'low'
  description: string
  series?: DORASeriesPoint[]
  prev_series?: DORASeriesPoint[]
  app_vs_fleet?: 'better' | 'worse' | 'equal' | ''
  app_vs_fleet_text?: string
  fleet_value?: number
  fleet_benchmark?: 'elite' | 'high' | 'medium' | 'low' | ''
}
const doraCards = ref<DoraCard[]>([
  {
    key: 'deploy_freq',
    title: '部署频率',
    value: '--',
    unit: '次/天',
    trend: 'flat',
    delta: '加载中',
    benchmark: 'medium',
    description: '过去 7 天完成发布的次数（占位数据）',
  },
  {
    key: 'lead_time',
    title: '变更前置时间',
    value: '--',
    unit: '小时',
    trend: 'flat',
    delta: '加载中',
    benchmark: 'medium',
    description: '从代码合入 main 到生产可用的中位时长',
  },
  {
    key: 'change_fail_rate',
    title: '变更失败率',
    value: '--',
    unit: '%',
    trend: 'flat',
    delta: '加载中',
    benchmark: 'medium',
    description: '触发回滚或线上修复的发布占比',
  },
  {
    key: 'mttr',
    title: '平均恢复时间',
    value: '--',
    unit: '分钟',
    trend: 'flat',
    delta: '加载中',
    benchmark: 'medium',
    description: '从故障发现到恢复服务的中位时长',
  },
])
// DORA 数据加载状态（v2.0 / Sprint 4）
const doraLoading = ref(false)
const doraError = ref('')
// v2.1: 是否在 Sparkline 中叠加上一周期的虚线作对比
const doraComparePrev = ref(false)
// v2.1: DORA 按应用下钻（undefined = 全部应用）
const doraAppId = ref<number | undefined>(undefined)
const doraAppOptions = ref<{ label: string; value: number }[]>([])

async function loadDORAApps() {
  try {
    const { applicationApi } = await import('@/services/application')
    const res = await applicationApi.list({ page: 1, page_size: 500 })
    const list: any[] = (res as any)?.data?.list || []
    doraAppOptions.value = list.map((a) => ({ label: a.name, value: a.id }))
  } catch (e) {
    doraAppOptions.value = []
  }
}

const titleMap: Record<string, string> = {
  deploy_freq: '部署频率',
  lead_time: '变更前置时间',
  change_fail_rate: '变更失败率',
  mttr: '平均恢复时间',
}
const descMap: Record<string, string> = {
  deploy_freq: '过去 7 天 prod 环境完成发布的频次',
  lead_time: 'created → published 中位时长',
  change_fail_rate: 'failed + rolled_back 占终态总数比例',
  mttr: 'failed → 下次 published 中位时长（近似）',
}

async function fetchDORA() {
  doraLoading.value = true
  doraError.value = ''
  try {
    const res = await doraApi.get({
      env: 'prod',
      days: 7,
      application_id: doraAppId.value,
    })
    const payload = (res as any)?.data
    if (!payload?.enabled) {
      doraError.value = payload?.message || 'DORA flag 未开启'
      return
    }
    const metrics: DORAMetric[] = payload?.snapshot?.metrics || []
    if (metrics.length === 0) return
    doraCards.value = metrics.map((m) => ({
      key: m.key,
      title: titleMap[m.key] || m.title || m.key,
      value: m.sample === 0 ? '--' : String(m.value),
      unit: m.unit,
      trend: m.trend,
      delta: m.sample === 0 ? '数据不足' : m.delta_text,
      benchmark: m.benchmark,
      description: descMap[m.key] || m.description,
      series: m.series || [],
      prev_series: m.prev_series || [],
      app_vs_fleet: m.app_vs_fleet || '',
      app_vs_fleet_text: m.app_vs_fleet_text || '',
      fleet_value: m.fleet_value || 0,
      fleet_benchmark: m.fleet_benchmark || '',
    }))
  } catch (e: any) {
    doraError.value = e?.response?.data?.message || e?.message || 'DORA 加载失败'
  } finally {
    doraLoading.value = false
  }
}

const doraBenchmarkColor = (b: DoraCard['benchmark']) =>
  ({ elite: '#52c41a', high: '#1890ff', medium: '#faad14', low: '#ff4d4f' })[b]
const doraTrendIcon = (t: DoraTrend) => ({ up: '↑', down: '↓', flat: '→' })[t]
// v2.2: 应用 vs 全站徽标
const doraAppVsFleetColor = (v: DoraCard['app_vs_fleet']) =>
  v === 'better' ? 'green' : v === 'worse' ? 'red' : 'default'
const doraAppVsFleetIcon = (v: DoraCard['app_vs_fleet']) =>
  v === 'better' ? '↑' : v === 'worse' ? '↓' : '≈'
const doraFleetTooltip = (card: DoraCard) => {
  const text = card.app_vs_fleet_text || '与全站持平'
  const fleet = card.fleet_value != null ? card.fleet_value : 0
  return `${text}（全站 ${fleet}${card.unit}${card.fleet_benchmark ? ' · ' + card.fleet_benchmark : ''}）`
}

const doraTrendColor = (card: DoraCard) => {
  if (card.trend === 'flat') return '#8c8c8c'
  // 部署频率/前置时间→up 是好；失败率/MTTR→up 是差
  const upIsGood = card.key === 'deploy_freq' || card.key === 'lead_time'
  if (card.trend === 'up') return upIsGood ? '#52c41a' : '#ff4d4f'
  return upIsGood ? '#ff4d4f' : '#52c41a'
}

const router = useRouter()
const route = useRoute()

const stats = ref<DashboardStats & { applications?: number; pendingApprovals?: number }>({
  k8sClusters: 0,
  users: 0,
  healthChecks: 0,
  alertsToday: 0,
  auditsToday: 0,
})
const healthOverview = ref<HealthOverview>({ status: 'unknown', healthy: 0, unhealthy: 0, unknown: 0, total: 0 })
const recentAlerts = ref<RecentAlert[]>([])
const recentAudits = ref<RecentAudit[]>([])
const loadingAlerts = ref(false)
const loadingAudits = ref(false)
const loadingActions = ref(false)
const currentFocus = ref<FocusType>('delivery')
const projectId = computed(() => {
  const raw = Array.isArray(route.query.project_id) ? route.query.project_id[0] : route.query.project_id
  const parsed = Number(raw || 0)
  return parsed > 0 ? parsed : undefined
})
const projectScopeText = computed(() => (
  projectId.value
    ? `当前聚焦项目 #${projectId.value}，工作台待办已切换为项目视角。`
    : ''
))
const primaryApp = ref<any | null>(null)
const workspaceActions = ref<WorkspaceActionItem[]>([])
const workspaceActionGroups = ref<Record<string, string[]>>({})
const gitopsOverview = ref({
  changeRequests: 0,
  syncedApps: 0,
  appTotal: 0,
})

const pipelineStats = ref({
  total: 0,
  todayRuns: 0,
  successRate: 0,
  avgDuration: 0,
})
const recentPipelineRuns = ref<any[]>([])

const costStats = ref({
  monthCost: 0,
  trend: 0,
  savings: 0,
  idleResources: 0,
})

const securityStats = ref({
  highRisk: 0,
  critical: 0,
  high: 0,
  medium: 0,
})

const userStore = useUserStore()
const currentUser = computed(() => userStore.userInfo || {})
const primaryAppPath = computed(() => primaryApp.value?.id ? `/applications/${primaryApp.value.id}#delivery` : '/applications')
const deliveryChainSubtitle = computed(() => {
  if (primaryApp.value?.name) {
    return `${primaryApp.value.name}：应用、流水线、GitOps 变更、审批和运行验证已在一条链路里呈现。`
  }
  return '从应用进入，再沿着流水线、GitOps、审批和运行态完成一次交付。'
})

interface DeliveryFlowStep {
  key: string
  title: string
  description: string
  status: string
  color: string
  path: string
  icon: any
}

const deliveryFlowSteps = computed<DeliveryFlowStep[]>(() => {
  const latestRun = recentPipelineRuns.value[0]
  const hasSuccessRun = recentPipelineRuns.value.some((run: any) => run.status === 'success')
  const syncedText = gitopsOverview.value.syncedApps > 0
    ? `${gitopsOverview.value.syncedApps}/${gitopsOverview.value.appTotal} 已同步`
    : gitopsOverview.value.appTotal > 0
      ? `${gitopsOverview.value.appTotal} 个应用已接入 Argo CD`
    : '待同步'

  return [
    {
      key: 'app',
      title: '应用资产',
      description: primaryApp.value?.name || '先接入应用和环境',
      status: primaryApp.value ? '已接入' : '待接入',
      color: primaryApp.value ? 'green' : 'default',
      path: primaryAppPath.value,
      icon: AppstoreOutlined,
    },
    {
      key: 'pipeline',
      title: 'CI 流水线',
      description: pipelineStats.value.total > 0 ? `共 ${pipelineStats.value.total} 条流水线` : '接入代码仓库并创建流水线',
      status: pipelineStats.value.total > 0 ? '已配置' : '待配置',
      color: pipelineStats.value.total > 0 ? 'blue' : 'default',
      path: '/pipeline/list',
      icon: ForkOutlined,
    },
    {
      key: 'gitops',
      title: 'GitOps 变更',
      description: gitopsOverview.value.changeRequests > 0 ? `已有 ${gitopsOverview.value.changeRequests} 个变更请求` : '构建后生成 GitOps 变更',
      status: gitopsOverview.value.changeRequests > 0 ? '已生成' : '待发起',
      color: gitopsOverview.value.changeRequests > 0 ? 'purple' : 'default',
      path: '/argocd?tab=changes',
      icon: RocketOutlined,
    },
    {
      key: 'approval',
      title: '审批策略',
      description: stats.value.pendingApprovals ? `${stats.value.pendingApprovals} 个待处理审批` : '当前无审批阻塞',
      status: stats.value.pendingApprovals ? '待处理' : '无阻塞',
      color: stats.value.pendingApprovals ? 'orange' : 'green',
      path: '/approval/pending',
      icon: AuditOutlined,
    },
    {
      key: 'argocd',
      title: 'Argo CD 同步',
      description: syncedText,
      status: gitopsOverview.value.syncedApps > 0 ? '已同步' : gitopsOverview.value.appTotal > 0 ? '已接入' : '待接入',
      color: gitopsOverview.value.syncedApps > 0 ? 'green' : gitopsOverview.value.appTotal > 0 ? 'blue' : 'default',
      path: '/argocd?tab=apps',
      icon: CloudOutlined,
    },
    {
      key: 'runtime',
      title: '运行验证',
      description: latestRun ? `最近运行：${getRunStatusText(latestRun.status)}` : '上线后回到应用运行态验证',
      status: hasSuccessRun ? '已验证' : '待验证',
      color: hasSuccessRun ? 'green' : 'default',
      path: primaryAppPath.value,
      icon: CheckCircleOutlined,
    },
  ]
})

const roleGroup = computed<FocusType>(() => {
  const roles: string[] = currentUser.value?.roles || []
  if (roles.includes('admin') || roles.includes('administrator')) return 'platform'
  if (roles.includes('operator')) return 'operations'
  return 'delivery'
})

const roleLabel = computed(() => {
  const map: Record<FocusType, string> = {
    delivery: '研发交付角色',
    operations: '发布运维角色',
    platform: '平台管理员',
  }
  return map[roleGroup.value]
})

const focusHeadline = computed(() => {
  if (currentFocus.value === 'operations') {
    return {
      title: stats.value.pendingApprovals ? `优先清理 ${stats.value.pendingApprovals} 个审批阻塞` : '优先确认运行风险',
      description: stats.value.pendingApprovals ? '审批链路打通后，发布推进效率会显著提升。' : '当前审批阻塞不高，建议先看告警、日志和 GitOps 同步结果。',
      action: stats.value.pendingApprovals ? '进入审批中心' : '进入运行保障',
      path: stats.value.pendingApprovals ? '/approval/pending' : '/alert/overview',
    }
  }
  if (currentFocus.value === 'platform') {
    return {
      title: securityStats.value.highRisk > 0 ? `先处理 ${securityStats.value.highRisk} 个高风险问题` : '优先检查平台底座',
      description: securityStats.value.highRisk > 0 ? '高风险安全项和平台底座问题会直接影响交付连续性。' : '当前安全风险不高，建议继续看集群、仓库和成本趋势。',
      action: securityStats.value.highRisk > 0 ? '进入安全治理' : '查看基础设施',
      path: securityStats.value.highRisk > 0 ? '/security/overview' : '/k8s/clusters',
    }
  }
  return {
    title: pipelineStats.value.total > 0 ? '先沿着交付链路继续推进' : '先完成接入起步',
    description: pipelineStats.value.total > 0 ? '应用、流水线、GitOps 和审批已经串联，可以直接继续推进。' : '如果还没有形成交付链路，先接入应用和流水线，后续页面价值才会释放。',
    action: pipelineStats.value.total > 0 ? '进入应用交付' : '去创建流水线',
    path: pipelineStats.value.total > 0 ? primaryAppPath.value : '/pipeline/create',
  }
})

const focusSummaryCards = computed(() => {
  if (currentFocus.value === 'operations') {
    return [
      { title: '待审批', value: stats.value.pendingApprovals || 0, color: '#eb2f96', icon: AuditOutlined, path: '/approval/pending', description: '优先清理阻塞中的发布审批' },
      { title: '今日告警', value: stats.value.alertsToday, color: stats.value.alertsToday > 0 ? '#fa8c16' : '#52c41a', icon: AlertOutlined, path: '/alert/overview', description: '关注高优先级告警与事件' },
      { title: '健康检查', value: `${healthOverview.value.healthy}/${healthOverview.value.total}`, color: '#1677ff', icon: FileSearchOutlined, path: '/healthcheck', description: '确认健康检查与运行状态' },
      { title: '构建成功率', value: pipelineStats.value.successRate, suffix: '%', color: '#52c41a', icon: RocketOutlined, path: '/pipeline/list', description: '确认构建是否影响发布' },
    ]
  }
  if (currentFocus.value === 'platform') {
    return [
      { title: 'K8s 集群', value: stats.value.k8sClusters, color: '#722ed1', icon: CloudOutlined, path: '/k8s/clusters', description: '维护构建与部署底座' },
      { title: '安全高风险', value: securityStats.value.highRisk, color: securityStats.value.highRisk > 0 ? '#ff4d4f' : '#52c41a', icon: SafetyCertificateOutlined, path: '/security/overview', description: '关注严重与高危风险' },
      { title: '本月成本', value: `¥${formatCost(costStats.value.monthCost)}`, color: '#1677ff', icon: DollarOutlined, path: '/cost/overview', description: '观察平台成本趋势与节省空间' },
      { title: '用户数', value: stats.value.users, color: '#13c2c2', icon: AppstoreOutlined, path: '/users', description: '管理用户、权限与审计范围' },
    ]
  }
  return [
    { title: '流水线', value: pipelineStats.value.total, color: '#1890ff', icon: RocketOutlined, path: '/pipeline/list', description: '统一查看构建与交付状态' },
    { title: '今日构建', value: pipelineStats.value.todayRuns, color: '#1677ff', icon: FileSearchOutlined, path: '/pipeline/list', description: '今天已执行的流水线任务' },
    { title: '应用', value: stats.value.applications || 0, color: '#13c2c2', icon: AppstoreOutlined, path: '/applications', description: '快速进入应用与服务管理' },
    { title: '待审批', value: stats.value.pendingApprovals || 0, color: '#eb2f96', icon: AuditOutlined, path: '/approval/pending', description: '需要协同确认的发布变更' },
  ]
})

const asFallbackAction = (item: { title: string; description: string; path: string; badge: string; color: string }): WorkspaceActionItem => ({
  id: `fallback-${item.path}-${item.title}`,
  type: 'fallback',
  title: item.title,
  description: item.description,
  status: item.badge,
  priority: item.color === 'red' || item.color === 'magenta' ? 'high' : item.color === 'orange' || item.color === 'gold' ? 'medium' : 'low',
  path: item.path,
  action_label: '去处理',
  created_at: new Date().toISOString(),
  source_id: 0,
})

const fallbackTodoItems = computed<WorkspaceActionItem[]>(() => {
  if (currentFocus.value === 'operations') {
    return [
      { title: '处理待审批发布', description: `当前有 ${stats.value.pendingApprovals || 0} 个待审批事项，建议优先清理阻塞中的发布。`, badge: `${stats.value.pendingApprovals || 0} 个`, color: 'magenta', path: '/approval/pending' },
      { title: '查看今日告警', description: `今日已产生 ${stats.value.alertsToday} 条告警，建议先确认高优先级与未处理事件。`, badge: `${stats.value.alertsToday} 条`, color: 'orange', path: '/alert/overview' },
      { title: '进入 GitOps 交付', description: '围绕 Argo CD 同步状态继续处理审批、窗口、部署锁和执行结果。', badge: 'GitOps', color: 'blue', path: '/argocd' },
    ].map(asFallbackAction)
  }
  if (currentFocus.value === 'platform') {
    return [
      { title: '检查集群与底座', description: `当前接入 ${stats.value.k8sClusters} 个集群，建议优先确认构建集群与镜像仓库状态。`, badge: `${stats.value.k8sClusters} 个`, color: 'purple', path: '/k8s/clusters' },
      { title: '处理高风险问题', description: `安全高风险共 ${securityStats.value.highRisk} 项，建议先看严重和高危项。`, badge: `${securityStats.value.highRisk} 项`, color: 'red', path: '/security/overview' },
      { title: '检查成本与权限', description: '进入平台治理查看成本异常、用户权限与审计日志。', badge: '治理', color: 'gold', path: '/cost/overview' },
    ].map(asFallbackAction)
  }
  return [
    { title: '创建或接入流水线', description: '先接 Git 仓库、选模板，再创建一条可执行流水线。', badge: '交付', color: 'blue', path: '/pipeline/create' },
    { title: '查看构建部署工作台', description: `当前共 ${pipelineStats.value.total} 条流水线，可直接查看最近构建与部署状态。`, badge: `${pipelineStats.value.total} 条`, color: 'cyan', path: '/pipeline/list' },
    { title: '进入 GitOps 交付', description: '如果已完成构建，下一步直接去 GitOps 交付页确认同步与健康状态。', badge: `${stats.value.pendingApprovals || 0} 待审批`, color: 'magenta', path: '/argocd' },
  ].map(asFallbackAction)
})

const actionItemsForFocus = computed(() => {
  if (!workspaceActions.value.length) return fallbackTodoItems.value
  const ids = new Set(workspaceActionGroups.value[currentFocus.value] || [])
  const items = workspaceActions.value.filter((item) => ids.has(item.id))
  return (items.length ? items : workspaceActions.value).slice(0, 6)
})

const quickActions = computed(() => {
  if (currentFocus.value === 'operations') {
    return [
      { title: 'GitOps 交付', path: '/argocd', icon: RocketOutlined },
      { title: '待审批', path: '/approval/pending', icon: AuditOutlined },
      { title: '运行保障', path: '/alert/overview', icon: AlertOutlined },
      { title: '日志中心', path: '/logs/center', icon: FileSearchOutlined },
    ]
  }
  if (currentFocus.value === 'platform') {
    return [
      { title: 'K8s 集群', path: '/k8s/clusters', icon: CloudOutlined },
      { title: '镜像仓库', path: '/security/image-registry', icon: SafetyCertificateOutlined },
      { title: '平台治理', path: '/cost/overview', icon: DollarOutlined },
    ]
  }
  return [
    { title: '新建流水线', path: '/pipeline/create', icon: PlusOutlined },
    { title: 'Git 仓库', path: '/pipeline/git-repos', icon: RocketOutlined },
    { title: '模板市场', path: '/pipeline/templates', icon: AppstoreOutlined },
    { title: '交付工作台', path: '/pipeline/list', icon: FileSearchOutlined },
  ]
})

const focusPanelTitle = computed(() => ({
  delivery: '交付建议',
  operations: '运行建议',
  platform: '平台建议',
}[currentFocus.value]))

const focusInsights = computed(() => {
  if (currentFocus.value === 'operations') {
    return [
      { title: '从交付进入运行', description: '先看 GitOps 交付状态，再回到运行保障确认是否有新增告警与失败部署。', path: '/argocd' },
      { title: '从告警进入日志', description: '当出现告警时，优先从运行保障页直接跳日志与链路定位。', path: '/alert/overview' },
      { title: '围绕事件协同', description: '通过值班、通知和审批，统一处理发布与运行协同。', path: '/oncall' },
    ]
  }
  if (currentFocus.value === 'platform') {
    return [
      { title: '先维护交付底座', description: '确保 K8s 集群、镜像仓库、Builder 配置稳定，避免交付链整体受阻。', path: '/k8s/clusters' },
      { title: '再处理治理风险', description: '安全、成本、用户与审计建议放在平台治理视角统一处理。', path: '/security/overview' },
      { title: '最后再看扩展能力', description: 'Jira、ArgoCD 等扩展接入应放在底座稳定之后。', path: '/cost/overview' },
    ]
  }
  return [
    { title: '先接入代码', description: '先在 Git 仓库页校验凭证和分支，再进入流水线向导。', path: '/pipeline/git-repos' },
    { title: '再选模板', description: '通过模板市场快速起步，避免从空白流水线开始配置。', path: '/pipeline/templates' },
    { title: '最后确认交付', description: '构建成功后直接进入 GitOps 交付页，确认同步状态并减少多页面切换。', path: '/argocd' },
  ]
})

const getHealthColor = (status: string) => ({ healthy: '#52c41a', unhealthy: '#ff4d4f', unknown: '#d9d9d9' }[status] || '#d9d9d9')
const getHealthText = (status: string) => ({ healthy: '系统健康', unhealthy: '存在异常', unknown: '状态未知' }[status] || '状态未知')
const getLevelColor = (level: string) => ({ info: 'blue', warning: 'orange', error: 'red', critical: 'magenta' }[level] || 'default')
const getLevelText = (level: string) => ({ info: '信息', warning: '警告', error: '错误', critical: '严重' }[level] || level)
const getActionText = (action: string) => ({ create: '创建', update: '更新', delete: '删除' }[action] || action)
const getRunStatusColor = (status: string) => ({ success: 'green', running: 'blue', failed: 'red', pending: 'default', cancelled: 'orange' }[status] || 'default')
const getRunStatusText = (status: string) => ({ success: '成功', running: '运行中', failed: '失败', pending: '等待中', cancelled: '已取消' }[status] || status)
const actionPriorityColor = (priority: string) => ({ high: 'red', medium: 'orange', low: 'blue' }[priority] || 'default')
const actionPriorityText = (priority: string) => ({ high: '高优先级', medium: '中优先级', low: '低优先级' }[priority] || '待处理')

const formatTime = (time: string) => {
  if (!time) return '-'
  const date = new Date(time)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`
  return time.replace('T', ' ').substring(5, 16)
}

const formatCost = (cost: number) => {
  if (cost >= 10000) return `${(cost / 10000).toFixed(1)}万`
  return cost.toFixed(0)
}

const goTo = (path: string) => {
  router.push(path)
}

const syncFocusFromRoute = () => {
  const focus = route.query.focus
  if (focus === 'delivery' || focus === 'operations' || focus === 'platform') {
    currentFocus.value = focus
    return
  }
  currentFocus.value = 'delivery'
}

const fetchData = async () => {
  try {
    const [statsRes, healthRes] = await Promise.all([
      dashboardApi.getStats(),
      dashboardApi.getHealthOverview(),
    ])
    if (statsRes.code === 0 && statsRes.data) stats.value = { ...stats.value, ...statsRes.data }
    if (healthRes.code === 0 && healthRes.data) healthOverview.value = healthRes.data
  } catch (error) {
    console.error('获取统计数据失败', error)
  }
}

const fetchAlerts = async () => {
  loadingAlerts.value = true
  try {
    const res = await dashboardApi.getRecentAlerts()
    if (res.code === 0 && res.data) recentAlerts.value = res.data
  } catch (error) {
    console.error('获取告警失败', error)
  } finally {
    loadingAlerts.value = false
  }
}

const fetchAudits = async () => {
  loadingAudits.value = true
  try {
    const res = await dashboardApi.getRecentAudits()
    if (res.code === 0 && res.data) recentAudits.value = res.data
  } catch (error) {
    console.error('获取审计失败', error)
  } finally {
    loadingAudits.value = false
  }
}

const fetchWorkspaceActions = async () => {
  loadingActions.value = true
  try {
    const res = await dashboardApi.getWorkspaceActions({ limit: 40, project_id: projectId.value })
    if (res.code === 0 && res.data) {
      workspaceActions.value = res.data.items || []
      workspaceActionGroups.value = res.data.groups || {}
    }
  } catch (error) {
    console.error('获取行动中心失败', error)
  } finally {
    loadingActions.value = false
  }
}

const fetchPipelineStats = async () => {
  try {
    const pipelinesRes = await request.get('/pipelines', { params: { page: 1, page_size: 1 }, skipErrorToast: true })
    pipelineStats.value.total = pipelinesRes?.data?.total || 0

    const today = new Date().toISOString().split('T')[0]
    const runsRes = await request.get('/pipelines/runs', { params: { page: 1, page_size: 8 }, skipErrorToast: true })
    const runs = runsRes?.data?.items || []
    recentPipelineRuns.value = runs.slice(0, 5)

    const todayRuns = runs.filter((r: any) => r.created_at?.startsWith(today))
    pipelineStats.value.todayRuns = todayRuns.length
    const successRuns = runs.filter((r: any) => r.status === 'success')
    pipelineStats.value.successRate = runs.length > 0 ? Math.round((successRuns.length / runs.length) * 100) : 0
    const durations = runs.filter((r: any) => r.duration > 0).map((r: any) => r.duration)
    pipelineStats.value.avgDuration = durations.length > 0 ? Math.round(durations.reduce((a: number, b: number) => a + b, 0) / durations.length / 60) : 0
  } catch (error) {
    console.error('获取流水线统计失败', error)
  }
}

const fetchDeliveryOverview = async () => {
  try {
    const [appsRes, gitopsRes, crRes, argoAppsRes] = await Promise.all([
      request.get('/app', { params: { page: 1, page_size: 1 }, skipErrorToast: true }),
      request.get('/argocd/dashboard', { skipErrorToast: true }),
      request.get('/argocd/change-requests', { params: { page: 1, page_size: 1 }, skipErrorToast: true }),
      request.get('/argocd/apps', { params: { page: 1, page_size: 100 }, skipErrorToast: true }),
    ])
    const apps = appsRes?.data?.list || []
    primaryApp.value = apps[0] || null
    if (typeof appsRes?.data?.total === 'number') {
      stats.value.applications = appsRes.data.total
    }
    const argoApps = argoAppsRes?.data?.list || []
    gitopsOverview.value = {
      changeRequests: crRes?.data?.total || 0,
      syncedApps: argoApps.length
        ? argoApps.filter((item: any) => String(item.sync_status || '').toLowerCase() === 'synced').length
        : gitopsRes?.data?.app_synced || 0,
      appTotal: argoAppsRes?.data?.total || gitopsRes?.data?.app_total || 0,
    }
  } catch (error) {
    console.error('获取交付链路概览失败', error)
  }
}

const fetchCostStats = async () => {
  try {
    const res = await request.get('/cost/overview', { skipErrorToast: true })
    if (res?.data) {
      costStats.value = {
        monthCost: res.data.month_cost || res.data.total_cost || 0,
        trend: res.data.trend || res.data.month_over_month || 0,
        savings: res.data.potential_savings || res.data.savings || 0,
        idleResources: res.data.idle_resources || res.data.idle_count || 0,
      }
    }
  } catch (error) {
    console.error('获取成本统计失败', error)
  }
}

const fetchSecurityStats = async () => {
  try {
    const res = await request.get('/security/overview', { skipErrorToast: true })
    if (res?.data) {
      securityStats.value = {
        highRisk: (res.data.critical || 0) + (res.data.high || 0),
        critical: res.data.critical || 0,
        high: res.data.high || 0,
        medium: res.data.medium || 0,
      }
    }
  } catch (error) {
    console.error('获取安全统计失败', error)
  }
}

watch(() => route.query.focus, syncFocusFromRoute)
watch(() => route.query.project_id, () => {
  fetchWorkspaceActions()
})

watch(currentFocus, (focus) => {
  if (route.query.focus === focus) return
  router.replace({
    path: route.path,
    query: {
      ...route.query,
      focus,
    },
  })
})

onMounted(() => {
  syncFocusFromRoute()
  fetchData()
  fetchAlerts()
  fetchAudits()
  fetchWorkspaceActions()
  fetchPipelineStats()
  fetchDeliveryOverview()
  fetchCostStats()
  fetchSecurityStats()
  void fetchDORA()
  void loadDORAApps()
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.hero-card {
  background: linear-gradient(135deg, #f0f5ff 0%, #ffffff 100%);
}

.hero-title {
  font-size: 28px;
  font-weight: 600;
  color: #1f1f1f;
}

.hero-subtitle {
  margin-top: 8px;
  color: #8c8c8c;
}

.hero-meta {
  margin-top: 16px;
}

.hero-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 16px;
}

.hero-priority-card {
  width: 100%;
  max-width: 320px;
  padding: 16px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid #e5e7eb;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
}

.hero-priority-label {
  font-size: 12px;
  color: #8c8c8c;
}

.hero-priority-title {
  margin-top: 6px;
  font-size: 18px;
  font-weight: 600;
  color: #111827;
}

.hero-priority-desc {
  margin: 8px 0 14px;
  font-size: 13px;
  line-height: 1.6;
  color: #6b7280;
}

.section-row {
  margin-top: 16px;
}

.section-header {
  align-items: flex-start;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 16px;
  font-weight: 600;
  color: #1f1f1f;
}

.section-subtitle {
  margin-top: 6px;
  color: #6b7280;
  font-size: 13px;
}

.summary-card,
.full-height-card {
  height: 100%;
}

.delivery-chain-card {
  overflow: hidden;
}

.delivery-chain-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}

.delivery-chain {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 10px;
}

.delivery-step {
  min-height: 118px;
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
  color: inherit;
  text-align: left;
  cursor: pointer;
  display: flex;
  gap: 10px;
  transition: border-color 0.2s, box-shadow 0.2s, transform 0.2s;
}

.delivery-step:hover {
  border-color: #1677ff;
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.08);
  transform: translateY(-1px);
}

.delivery-step-icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: #f0f7ff;
  color: #1677ff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
}

.delivery-step-body {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.delivery-step-top {
  display: flex;
  align-items: center;
  gap: 8px;
  justify-content: space-between;
}

.delivery-step-title {
  color: #111827;
  font-weight: 600;
  white-space: nowrap;
}

.delivery-step-desc {
  color: #6b7280;
  font-size: 12px;
  line-height: 1.5;
}

.summary-desc {
  margin-top: 8px;
  color: #8c8c8c;
  font-size: 12px;
}

.todo-main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.todo-title {
  font-weight: 500;
  color: #1f1f1f;
}

.todo-desc {
  color: #8c8c8c;
  font-size: 12px;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.status-item {
  padding: 14px;
  border-radius: 12px;
  background: #fafafa;
}

.status-label {
  color: #8c8c8c;
  font-size: 12px;
}

.status-value {
  margin-top: 8px;
  font-size: 22px;
  font-weight: 600;
  color: #1f1f1f;
}

.status-value.danger {
  color: #ff4d4f;
}

@media (max-width: 1400px) {
  .delivery-chain {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .hero-side,
  .delivery-chain-header {
    flex-direction: column;
    align-items: stretch;
  }

  .delivery-chain {
    grid-template-columns: 1fr;
  }
}

/* v2.0 DORA 卡片 */
.dora-row {
  margin-top: 16px;
}

.dora-section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 16px;
  font-weight: 600;
  color: #1f1f1f;
  padding: 4px 0;
}

.dora-section-hint {
  margin-left: auto;
  color: #8c8c8c;
  font-size: 12px;
  font-weight: normal;
}

.dora-card {
  height: 100%;
  border-radius: 12px;
  background: linear-gradient(180deg, #ffffff 0%, #fafbff 100%);
}

.dora-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.dora-fleet-badge {
  font-size: 11px;
  padding: 0 6px;
  line-height: 18px;
}

.dora-card-title {
  font-size: 14px;
  color: #595959;
  font-weight: 500;
}

.dora-card-value {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-bottom: 4px;
}

.dora-value {
  font-size: 28px;
  font-weight: 700;
  color: #1f1f1f;
  line-height: 1.1;
}

.dora-unit {
  color: #8c8c8c;
  font-size: 13px;
}

.dora-card-trend {
  font-size: 13px;
  font-weight: 500;
  margin-bottom: 6px;
}

.dora-card-trend-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
}

.dora-card-trend-row .dora-card-trend {
  margin-bottom: 0;
}

.dora-card-desc {
  color: #8c8c8c;
  font-size: 12px;
  line-height: 1.5;
}
</style>
