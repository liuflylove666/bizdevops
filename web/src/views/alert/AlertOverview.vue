<template>
  <div class="alert-overview">
    <a-row :gutter="[16, 16]" class="ops-overview-row">
      <a-col :span="24">
        <a-card :bordered="false" class="ops-hero">
          <a-row :gutter="16" align="middle">
            <a-col :xs="24" :lg="16">
              <div class="hero-title">运行保障工作台</div>
              <div class="hero-subtitle">从告警发现问题，再快速进入日志、链路、值班和通知，形成统一的故障响应入口。</div>
              <div class="hero-priority">
                <div class="hero-priority-label">当前重点</div>
                <div class="hero-priority-title">{{ heroPriorityTitle }}</div>
                <div class="hero-priority-desc">{{ heroPriorityDescription }}</div>
              </div>
            </a-col>
            <a-col :xs="24" :lg="8">
              <a-space wrap class="hero-actions">
                <a-button type="primary" @click="goTo('/alert/history', { ack_status: 'pending' })">处理告警</a-button>
                <a-button @click="goTo('/logs/center')">查看日志</a-button>
                <a-button @click="goTo('/tracing/list')">查看链路</a-button>
                <a-button @click="goTo('/telegram/message')">通知中心</a-button>
              </a-space>
            </a-col>
          </a-row>
        </a-card>
      </a-col>

      <a-col :xs="24" :md="12" :xl="6">
        <a-card :bordered="false" class="summary-card">
          <a-statistic title="待响应告警" :value="stats.pending_count" :value-style="{ color: '#cf1322' }">
            <template #prefix><ExclamationCircleOutlined /></template>
          </a-statistic>
          <div class="summary-extra">建议优先处理高等级和未确认告警</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :md="12" :xl="6">
        <a-card :bordered="false" class="summary-card">
          <a-statistic title="今日告警总量" :value="stats.today_count" :value-style="{ color: '#fa8c16' }">
            <template #prefix><AlertOutlined /></template>
          </a-statistic>
          <div class="summary-extra">近 7 天累计 {{ weekCount }} 条</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :md="12" :xl="6">
        <a-card :bordered="false" class="summary-card">
          <a-statistic title="值班排班" :value="oncallSummary.enabledSchedules" :value-style="{ color: '#1677ff' }">
            <template #prefix><TeamOutlined /></template>
          </a-statistic>
          <div class="summary-extra">总排班 {{ oncallSummary.totalSchedules }}，当前值班 {{ oncallSummary.currentOncallLabel }}</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :md="12" :xl="6">
        <a-card :bordered="false" class="summary-card">
          <a-statistic title="链路追踪状态" :value="traceSummary.enabled ? 1 : 0" :value-style="{ color: traceSummary.enabled ? '#52c41a' : '#8c8c8c' }">
            <template #prefix><ApartmentOutlined /></template>
          </a-statistic>
          <div class="summary-extra">{{ traceSummary.enabled ? `已启用，服务 ${traceSummary.services} 个` : '当前未启用链路追踪' }}</div>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="[16, 16]" class="ops-overview-row">
      <a-col :xs="24" :lg="16">
        <a-card title="处置流程" :bordered="false" class="workbench-card">
          <a-steps :current="1" size="small">
            <a-step title="发现告警" description="先定位待处理和高等级告警" />
            <a-step title="日志排查" description="进入日志中心检索上下文与关键错误" />
            <a-step title="链路确认" description="通过 Trace 确认调用链和慢点位置" />
            <a-step title="通知协同" description="值班与通知中心协调处理与升级" />
          </a-steps>
          <div class="flow-links">
            <a-button type="link" @click="goTo('/alert/history')">告警历史</a-button>
            <a-button type="link" @click="goTo('/logs/center')">日志中心</a-button>
            <a-button type="link" @click="goTo('/tracing/list')">链路列表</a-button>
            <a-button type="link" @click="goTo('/oncall')">值班管理</a-button>
          </div>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="8">
        <a-card title="快速入口" :bordered="false" class="workbench-card">
          <a-space direction="vertical" style="width: 100%">
            <a-button block @click="goAlertPolicy('config')">告警规则</a-button>
            <a-button block @click="goAlertPolicy('silence')">静默管理</a-button>
            <a-button block @click="goAlertPolicy('escalation')">升级规则</a-button>
            <a-button block @click="goTo('/telegram/message')">通知中心</a-button>
            <a-button block @click="goTo('/oncall')">值班管理</a-button>
          </a-space>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="16">
      <!-- 告警趋势 -->
      <a-col :span="16">
        <a-card title="告警趋势（近7天）" :bordered="false" :loading="loadingTrend">
          <div ref="trendChartRef" style="height: 280px"></div>
        </a-card>
      </a-col>
      <!-- 告警分布 -->
      <a-col :span="8">
        <a-card title="告警类型分布" :bordered="false" :loading="loadingStats">
          <div ref="typeChartRef" style="height: 280px"></div>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="16" style="margin-top: 16px">
      <!-- 级别分布 -->
      <a-col :span="8">
        <a-card title="告警级别分布" :bordered="false" :loading="loadingStats">
          <div ref="levelChartRef" style="height: 240px"></div>
        </a-card>
      </a-col>
      <!-- 最近告警 -->
      <a-col :span="16">
        <a-card title="最近告警" :bordered="false" :loading="loadingRecent">
          <template #extra>
            <a-button type="link" @click="goTo('/alert/history')">查看全部</a-button>
          </template>
          <a-table :columns="recentColumns" :data-source="recentAlerts" row-key="id" size="small" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'created_at'">{{ formatTime(record.created_at) }}</template>
              <template v-if="column.key === 'type'">
                <a-tag :color="getTypeColor(record.type)" size="small">{{ getTypeLabel(record.type) }}</a-tag>
              </template>
              <template v-if="column.key === 'level'">
                <a-tag :color="getLevelColor(record.level)" size="small">{{ getLevelLabel(record.level) }}</a-tag>
              </template>
              <template v-if="column.key === 'ack_status'">
                <a-badge :status="getAckStatusBadge(record.ack_status)" :text="getAckStatusLabel(record.ack_status)" />
              </template>
              <template v-if="column.key === 'action'">
                <a-space size="small">
                  <a-button v-if="record.ack_status === 'pending'" type="link" size="small" @click="ackAlert(record.id)">确认</a-button>
                  <a-button type="link" size="small" @click="goTo('/logs/center', { keyword: record.title })">日志</a-button>
                  <a-button type="link" size="small" @click="goTo('/tracing/list', { keyword: record.title })">链路</a-button>
                  <a-button type="link" size="small" @click="goTo('/oncall')">值班</a-button>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { AlertOutlined, CheckCircleOutlined, StopOutlined, ArrowUpOutlined, ExclamationCircleOutlined, CalendarOutlined, TeamOutlined, ApartmentOutlined } from '@ant-design/icons-vue'
import * as echarts from 'echarts'
import { alertApi, type AlertStats, type AlertHistory } from '@/services/alert'
import { oncallApi } from '@/services/oncall'
import { traceApi } from '@/services/trace'

const router = useRouter()
const loadingStats = ref(false)
const loadingTrend = ref(false)
const loadingRecent = ref(false)

const stats = ref<AlertStats>({ type_stats: [], level_stats: [], ack_stats: [], today_count: 0, pending_count: 0, enabled_count: 0, active_silence_count: 0, enabled_escalation_count: 0 })
const recentAlerts = ref<AlertHistory[]>([])
const weekCount = ref(0)
const trendData = ref<{ date: string; count: number }[]>([])
const oncallSummary = reactive({
  totalSchedules: 0,
  enabledSchedules: 0,
  currentOncallLabel: '未配置'
})
const traceSummary = reactive({
  enabled: false,
  services: 0
})

const trendChartRef = ref<HTMLElement>()
const typeChartRef = ref<HTMLElement>()
const levelChartRef = ref<HTMLElement>()
let trendChart: echarts.ECharts | null = null
let typeChart: echarts.ECharts | null = null
let levelChart: echarts.ECharts | null = null

const recentColumns = [
  { title: '时间', key: 'created_at', width: 140 },
  { title: '类型', key: 'type', width: 100 },
  { title: '级别', key: 'level', width: 70 },
  { title: '标题', dataIndex: 'title', ellipsis: true },
  { title: '状态', key: 'ack_status', width: 80 },
  { title: '操作', key: 'action', width: 70 }
]

const typeLabels: Record<string, string> = { k8s_pod: 'K8s Pod', health_check: '健康检查' }
const typeColors: Record<string, string> = { k8s_pod: '#722ed1', health_check: '#1890ff' }
const levelLabels: Record<string, string> = { info: '信息', warning: '警告', error: '错误', critical: '严重' }
const levelColors: Record<string, string> = { info: '#1890ff', warning: '#faad14', error: '#f5222d', critical: '#eb2f96' }
const ackStatusLabels: Record<string, string> = { pending: '待处理', acked: '已确认', resolved: '已解决' }

const heroPriorityTitle = computed(() => {
  if (stats.value.pending_count > 0) return `先处理 ${stats.value.pending_count} 条待响应告警`
  if (stats.value.today_count > 0) return '先回看今日新增告警'
  return '当前运行告警压力较低'
})

const heroPriorityDescription = computed(() => {
  if (stats.value.pending_count > 0) return '建议优先确认高等级和未确认告警，再进入日志与链路定位。'
  if (stats.value.today_count > 0) return '当前没有堆积的待处理告警，但今天仍有告警产生，建议继续做趋势复盘。'
  return '当前没有明显的告警堆积，可以继续维护告警规则、静默和通知策略。'
})

const getTypeLabel = (type: string) => typeLabels[type] || type
const getTypeColor = (type: string) => typeColors[type] || 'default'
const getLevelLabel = (level: string) => levelLabels[level] || level
const getLevelColor = (level: string) => levelColors[level] || 'default'
const getAckStatusLabel = (status: string) => ackStatusLabels[status] || status
const getAckStatusBadge = (status: string) => status === 'resolved' ? 'success' : status === 'acked' ? 'processing' : 'warning'
const formatTime = (time: string) => time ? time.replace('T', ' ').substring(0, 16) : '-'

const goTo = (path: string, query?: Record<string, string>) => {
  router.push({ path, query })
}

const goAlertPolicy = (tab: 'config' | 'templates' | 'silence' | 'escalation') => {
  void router.push({ path: '/alert/center', query: { tab } })
}

const fetchStats = async () => {
  loadingStats.value = true
  try {
    const res = await alertApi.getStats()
    if (res.code === 0 && res.data) {
      stats.value = res.data
      await nextTick()
      renderTypeChart()
      renderLevelChart()
    }
  } finally { loadingStats.value = false }
}

const fetchTrend = async () => {
  loadingTrend.value = true
  try {
    // 从历史记录统计近7天数据
    const res = await alertApi.getTrend({ days: 7 })
    if (res.code === 0 && res.data) {
      trendData.value = res.data.items || []
      weekCount.value = res.data.total || 0
    } else {
      // 如果接口不存在，从历史记录计算
      const histRes = await alertApi.listHistories({ page: 1, page_size: 1000 })
      if (histRes.code === 0 && histRes.data) {
        const now = new Date()
        const weekAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
        const items = histRes.data.list || []
        const weekItems = items.filter(item => new Date(item.created_at) >= weekAgo)
        weekCount.value = weekItems.length
        
        // 按日期分组
        const dateMap = new Map<string, number>()
        for (let i = 6; i >= 0; i--) {
          const d = new Date(now.getTime() - i * 24 * 60 * 60 * 1000)
          dateMap.set(d.toISOString().substring(5, 10), 0)
        }
        weekItems.forEach(item => {
          const date = item.created_at.substring(5, 10)
          if (dateMap.has(date)) dateMap.set(date, (dateMap.get(date) || 0) + 1)
        })
        trendData.value = Array.from(dateMap.entries()).map(([date, count]) => ({ date, count }))
      }
    }
    await nextTick()
    renderTrendChart()
  } finally { loadingTrend.value = false }
}

const fetchRecent = async () => {
  loadingRecent.value = true
  try {
    const res = await alertApi.listHistories({ page: 1, page_size: 8 })
    if (res.code === 0 && res.data) recentAlerts.value = res.data.list || []
  } finally { loadingRecent.value = false }
}

const fetchWorkbench = async () => {
  const [scheduleRes, traceStatusRes, traceServicesRes] = await Promise.allSettled([
    oncallApi.listSchedules(),
    traceApi.getStatus(),
    traceApi.listServices()
  ])

  if (scheduleRes.status === 'fulfilled') {
    const raw = scheduleRes.value?.data
    const schedules = Array.isArray(raw) ? raw : (raw?.list || [])
    oncallSummary.totalSchedules = schedules.length
    oncallSummary.enabledSchedules = schedules.filter((item: any) => item.enabled).length
    const firstEnabled = schedules.find((item: any) => item.enabled)
    oncallSummary.currentOncallLabel = firstEnabled?.name || (schedules.length > 0 ? schedules[0].name : '未配置')
  }

  if (traceStatusRes.status === 'fulfilled' && traceStatusRes.value.data?.code === 0) {
    const payload = traceStatusRes.value.data
    traceSummary.enabled = !!payload?.data?.enabled
  }

  if (traceServicesRes.status === 'fulfilled' && traceServicesRes.value.data?.code === 0) {
    const payload = traceServicesRes.value.data
    traceSummary.services = payload?.data?.services?.length || 0
  }
}

const ackAlert = async (id: number) => {
  try {
    const res = await alertApi.ackHistory(id)
    if (res.code === 0) { message.success('已确认'); fetchRecent(); fetchStats() }
  } catch (e: any) { message.error(e.message || '操作失败') }
}

const renderTrendChart = () => {
  if (!trendChartRef.value) return
  if (!trendChart) trendChart = echarts.init(trendChartRef.value)
  trendChart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 40, right: 20, top: 20, bottom: 30 },
    xAxis: { type: 'category', data: trendData.value.map(d => d.date) },
    yAxis: { type: 'value', minInterval: 1 },
    series: [{
      type: 'line',
      data: trendData.value.map(d => d.count),
      smooth: true,
      areaStyle: { opacity: 0.3 },
      itemStyle: { color: '#f5222d' }
    }]
  })
}

const renderTypeChart = () => {
  if (!typeChartRef.value || !stats.value.type_stats?.length) return
  if (!typeChart) typeChart = echarts.init(typeChartRef.value)
  typeChart.setOption({
    tooltip: { trigger: 'item' },
    series: [{
      type: 'pie',
      radius: ['45%', '70%'],
      label: { show: true, formatter: '{b}: {c}' },
      data: stats.value.type_stats.map(s => ({
        name: typeLabels[s.name] || s.name,
        value: s.count,
        itemStyle: { color: typeColors[s.name] || '#999' }
      }))
    }]
  })
}

const renderLevelChart = () => {
  if (!levelChartRef.value || !stats.value.level_stats?.length) return
  if (!levelChart) levelChart = echarts.init(levelChartRef.value)
  levelChart.setOption({
    tooltip: { trigger: 'item' },
    series: [{
      type: 'pie',
      radius: ['45%', '70%'],
      label: { show: true, formatter: '{b}: {c}' },
      data: stats.value.level_stats.map(s => ({
        name: levelLabels[s.name] || s.name,
        value: s.count,
        itemStyle: { color: levelColors[s.name] || '#999' }
      }))
    }]
  })
}

onMounted(() => {
  fetchStats()
  fetchTrend()
  fetchRecent()
  fetchWorkbench()
})
</script>

<style scoped>
.ops-overview-row {
  margin-bottom: 16px;
}

.ops-hero {
  background: linear-gradient(135deg, #fff1f0 0%, #ffffff 100%);
}

.hero-title {
  font-size: 24px;
  font-weight: 600;
  color: #1f1f1f;
}

.hero-subtitle {
  margin-top: 8px;
  color: #8c8c8c;
}

.hero-priority {
  margin-top: 16px;
  padding: 14px 16px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid #ffe1de;
}

.hero-priority-label {
  color: #8c8c8c;
  font-size: 12px;
}

.hero-priority-title {
  margin-top: 6px;
  font-size: 18px;
  font-weight: 600;
  color: #1f1f1f;
}

.hero-priority-desc {
  margin-top: 6px;
  font-size: 13px;
  color: #6b7280;
  line-height: 1.6;
}

.hero-actions {
  display: flex;
  justify-content: flex-end;
}

.summary-card,
.workbench-card {
  height: 100%;
}

.summary-extra {
  margin-top: 8px;
  font-size: 12px;
  color: #8c8c8c;
}

.flow-links {
  margin-top: 12px;
}

.alert-overview :deep(.ant-card-hoverable) {
  cursor: pointer;
}
</style>
