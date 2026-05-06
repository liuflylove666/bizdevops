<template>
  <div class="dora-analysis">
    <!-- 顶部筛选栏 -->
    <a-card :bordered="false" class="filter-card">
      <a-space size="middle" wrap>
        <span class="filter-label">环境</span>
        <a-select v-model:value="filter.env" style="width: 120px" @change="fetchData">
          <a-select-option value="prod">prod</a-select-option>
          <a-select-option value="staging">staging</a-select-option>
          <a-select-option value="pre">pre</a-select-option>
          <a-select-option value="test">test</a-select-option>
          <a-select-option value="dev">dev</a-select-option>
        </a-select>

        <span class="filter-label">时间范围</span>
        <a-radio-group v-model:value="filter.rangeMode" button-style="solid" @change="onRangeModeChange">
          <a-radio-button :value="7">7 天</a-radio-button>
          <a-radio-button :value="14">14 天</a-radio-button>
          <a-radio-button :value="30">30 天</a-radio-button>
          <a-radio-button :value="90">90 天</a-radio-button>
          <a-radio-button value="custom">自定义</a-radio-button>
        </a-radio-group>
        <a-range-picker
          v-if="filter.rangeMode === 'custom'"
          v-model:value="filter.customRange"
          format="YYYY-MM-DD"
          @change="fetchData"
        />

        <span class="filter-label">应用</span>
        <a-select
          v-model:value="filter.applicationId"
          style="min-width: 220px"
          placeholder="全部应用（全站口径）"
          show-search
          option-filter-prop="label"
          allow-clear
          :options="appOptions"
          @change="fetchData"
        />

        <a-checkbox v-model:checked="filter.comparePrev">对比上一周期</a-checkbox>

        <a-button type="primary" :loading="loading" @click="fetchData">
          <template #icon><ReloadOutlined /></template>
          刷新
        </a-button>
      </a-space>

      <div v-if="errorMsg" class="filter-error">
        <ExclamationCircleOutlined /> {{ errorMsg }}
      </div>
      <div v-else-if="snapshot" class="filter-summary">
        区间 <b>{{ formatDateOnly(snapshot.from) }}</b> ~ <b>{{ formatDateOnly(snapshot.to) }}</b>
        · 环境 <b>{{ snapshot.env }}</b>
        <span v-if="filter.applicationId"> · 应用 <b>{{ currentAppName }}</b></span>
      </div>
    </a-card>

    <!-- 4 指标卡 -->
    <a-row :gutter="[16, 16]" class="metric-row">
      <a-col :xs="24" :md="12" :xl="6" v-for="card in cards" :key="card.key">
        <a-card hoverable :bordered="false" class="metric-card" :loading="loading">
          <div class="metric-header">
            <span class="metric-title">{{ card.title }}</span>
            <a-tag :color="benchmarkColor(card.benchmark)">{{ card.benchmark }}</a-tag>
          </div>
          <div class="metric-value">
            <span class="val">{{ card.value }}</span>
            <span class="unit">{{ card.unit }}</span>
          </div>
          <div class="metric-trend" :style="{ color: trendColor(card) }">
            {{ trendIcon(card.trend) }} {{ card.delta_text || `${card.delta}` }}
          </div>
          <div class="metric-spark">
            <DORASparkline
              :points="card.series || []"
              :prev-points="filter.comparePrev ? (card.prev_series || []) : []"
              :color="trendColor(card) || '#1677ff'"
              :unit="card.unit"
              :width="240"
              :height="48"
            />
          </div>
          <div class="metric-desc">{{ card.description }}</div>
        </a-card>
      </a-col>
    </a-row>

    <!-- 4 个大趋势图 -->
    <a-row :gutter="[16, 16]" class="chart-row">
      <a-col :xs="24" :xl="12" v-for="card in cards" :key="card.key + '-chart'">
        <a-card :bordered="false" class="chart-card">
          <template #title>
            {{ card.title }} 趋势
            <a-tag :color="benchmarkColor(card.benchmark)" class="inline-tag">{{ card.benchmark }}</a-tag>
          </template>
          <template #extra>
            <span class="chart-summary" :style="{ color: trendColor(card) }">
              当前 <b>{{ card.value }}{{ card.unit }}</b>
              · {{ trendIcon(card.trend) }} {{ card.delta_text || `${card.delta}` }}
            </span>
          </template>
          <div class="big-chart" :ref="(el) => bindChart(el as HTMLElement, card.key)"></div>
        </a-card>
      </a-col>
    </a-row>

    <!-- DORA 等级说明 -->
    <a-alert type="info" show-icon class="benchmark-alert">
      <template #message>DORA 四指标等级参考</template>
      <template #description>
        <div class="benchmark-grid">
          <div><b>部署频率</b>：Elite 每天多次 · High 每周至少一次 · Medium 每月至少一次 · Low 不足每月一次</div>
          <div><b>变更前置时间</b>：Elite &lt; 1 天 · High 1 天 ~ 1 周 · Medium 1 周 ~ 1 月 · Low &gt; 1 月</div>
          <div><b>变更失败率</b>：Elite 0~15% · High 16~30% · Medium 31~45% · Low &gt; 45%</div>
          <div><b>MTTR 恢复时长</b>：Elite &lt; 1 小时 · High &lt; 1 天 · Medium &lt; 1 周 · Low &gt; 1 周</div>
        </div>
      </template>
    </a-alert>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount, nextTick, computed, watch } from 'vue'
import { ReloadOutlined, ExclamationCircleOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import dayjs, { type Dayjs } from 'dayjs'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { TooltipComponent, GridComponent, LegendComponent, DataZoomComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import DORASparkline from '@/components/DORASparkline.vue'
import { doraApi, type DORAMetric, type DORASnapshot } from '@/services/dora'

echarts.use([LineChart, TooltipComponent, GridComponent, LegendComponent, DataZoomComponent, CanvasRenderer])

type RangeMode = 7 | 14 | 30 | 90 | 'custom'

interface AppOption { label: string; value: number }

const filter = reactive({
  env: 'prod',
  rangeMode: 7 as RangeMode,
  customRange: null as [Dayjs, Dayjs] | null,
  applicationId: undefined as number | undefined,
  comparePrev: true
})

const loading = ref(false)
const errorMsg = ref('')
const snapshot = ref<DORASnapshot | null>(null)
const cards = ref<DORAMetric[]>([])
const appOptions = ref<AppOption[]>([])

const currentAppName = computed(() => {
  const opt = appOptions.value.find(o => o.value === filter.applicationId)
  return opt?.label || ''
})

// ==================== 数据获取 ====================

async function loadApplications() {
  try {
    const { applicationApi } = await import('@/services/application')
    const res = await applicationApi.list({ page: 1, page_size: 500 })
    const list = (res as any)?.data?.list || (res as any)?.data || []
    appOptions.value = (Array.isArray(list) ? list : []).map((a: any) => ({ label: a.name, value: a.id }))
  } catch {
    appOptions.value = []
  }
}

function resolveRangeParams(): { days?: number; from?: string; to?: string } {
  if (filter.rangeMode === 'custom') {
    if (!filter.customRange || filter.customRange.length !== 2) {
      return { days: 7 }
    }
    const [from, to] = filter.customRange
    return {
      from: from.startOf('day').toISOString(),
      to: to.endOf('day').toISOString()
    }
  }
  return { days: filter.rangeMode as number }
}

async function fetchData() {
  loading.value = true
  errorMsg.value = ''
  try {
    const params = {
      env: filter.env,
      ...resolveRangeParams(),
      application_id: filter.applicationId
    }
    const res: any = await doraApi.get(params)
    const payload = res?.data ?? res
    if (!payload?.enabled) {
      errorMsg.value = payload?.message || 'DORA 指标未启用'
      snapshot.value = null
      cards.value = []
      return
    }
    snapshot.value = payload.snapshot || null
    cards.value = payload.snapshot?.metrics || []
    await nextTick()
    renderAllCharts()
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.message || e?.message || 'DORA 加载失败'
  } finally {
    loading.value = false
  }
}

function onRangeModeChange() {
  if (filter.rangeMode !== 'custom') {
    fetchData()
  }
}

// ==================== ECharts 大趋势图 ====================

const chartInstances = new Map<string, echarts.ECharts>()
const chartRefs = new Map<string, HTMLElement>()

function bindChart(el: HTMLElement | null, key: string) {
  if (!el) return
  chartRefs.set(key, el)
}

function renderAllCharts() {
  cards.value.forEach(card => renderChart(card))
}

function renderChart(card: DORAMetric) {
  const el = chartRefs.get(card.key)
  if (!el) return
  let chart = chartInstances.get(card.key)
  if (!chart) {
    chart = echarts.init(el)
    chartInstances.set(card.key, chart)
  }
  const curPoints = card.series || []
  const prevPoints = filter.comparePrev ? (card.prev_series || []) : []
  const xs = curPoints.map(p => p.date)
  const curValues = curPoints.map(p => p.value)
  const prevValues = prevPoints.map(p => p.value)
  const color = trendColor(card) || '#1677ff'

  const series: any[] = [
    {
      name: '当前',
      type: 'line',
      data: curValues,
      smooth: true,
      showSymbol: false,
      lineStyle: { color, width: 2 },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: color + '40' },
          { offset: 1, color: color + '00' }
        ])
      }
    }
  ]
  if (prevPoints.length > 0) {
    series.push({
      name: '上一周期',
      type: 'line',
      data: prevValues,
      smooth: true,
      showSymbol: false,
      lineStyle: { color: '#bfbfbf', width: 1.5, type: 'dashed' }
    })
  }

  chart.setOption({
    tooltip: {
      trigger: 'axis',
      formatter: (params: any) => {
        const arr = Array.isArray(params) ? params : [params]
        const idx = arr[0].dataIndex
        const d = curPoints[idx]?.date || '-'
        let html = `<div><b>${d}</b></div>`
        arr.forEach((p: any) => {
          html += `<div>${p.marker} ${p.seriesName}: ${p.value}${card.unit}</div>`
        })
        return html
      }
    },
    legend: {
      data: prevPoints.length > 0 ? ['当前', '上一周期'] : ['当前'],
      right: 10,
      top: 0
    },
    grid: { top: 30, bottom: 24, left: 48, right: 16 },
    xAxis: {
      type: 'category',
      data: xs,
      boundaryGap: false,
      axisLabel: { fontSize: 11 }
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        fontSize: 11,
        formatter: (v: number) => `${v}${card.unit}`
      }
    },
    series
  }, true)
  chart.resize()
}

function onResize() {
  chartInstances.forEach(c => c.resize())
}

// ==================== 工具函数 ====================

function benchmarkColor(b: string): string {
  return ({ elite: 'green', high: 'blue', medium: 'orange', low: 'red' } as Record<string, string>)[b] || 'default'
}

function trendIcon(t: string): string {
  return t === 'up' ? '↑' : t === 'down' ? '↓' : '→'
}

function trendColor(card: DORAMetric): string {
  // 对"越大越好"的指标（deploy_freq）：up=绿，down=红
  // 对"越小越好"的指标（lead_time / change_fail_rate / mttr）：up=红，down=绿
  const higherIsBetter = card.key === 'deploy_freq'
  if (card.trend === 'flat') return ''
  const good = (card.trend === 'up') === higherIsBetter
  return good ? '#52c41a' : '#ff4d4f'
}

function formatDateOnly(iso: string): string {
  try {
    return dayjs(iso).format('YYYY-MM-DD')
  } catch {
    return iso
  }
}

// ==================== 生命周期 ====================

watch(() => filter.comparePrev, () => {
  nextTick(renderAllCharts)
})

onMounted(async () => {
  window.addEventListener('resize', onResize)
  await loadApplications()
  await fetchData()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  chartInstances.forEach(c => c.dispose())
  chartInstances.clear()
})
</script>

<style scoped>
.dora-analysis {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.filter-card :deep(.ant-card-body) {
  padding: 16px 20px;
}

.filter-label {
  color: #666;
  font-size: 13px;
}

.filter-error {
  margin-top: 10px;
  color: #ff4d4f;
  font-size: 13px;
}

.filter-summary {
  margin-top: 10px;
  color: #666;
  font-size: 13px;
}

.metric-card {
  display: flex;
  flex-direction: column;
  min-height: 180px;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.metric-title {
  color: #666;
  font-size: 13px;
}

.metric-value {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-bottom: 6px;
}

.metric-value .val {
  font-size: 28px;
  font-weight: 600;
  color: #1a2332;
}

.metric-value .unit {
  font-size: 14px;
  color: #999;
}

.metric-trend {
  font-size: 13px;
  margin-bottom: 6px;
}

.metric-spark {
  margin: 4px 0 8px;
}

.metric-desc {
  color: #999;
  font-size: 12px;
  line-height: 1.5;
}

.chart-card :deep(.ant-card-head) {
  min-height: 44px;
}

.inline-tag {
  margin-left: 6px;
}

.chart-summary {
  font-size: 13px;
}

.big-chart {
  width: 100%;
  height: 280px;
}

.benchmark-alert :deep(.ant-alert-description) {
  padding-top: 6px;
}

.benchmark-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 20px;
  font-size: 13px;
  color: #333;
}

@media (max-width: 900px) {
  .benchmark-grid {
    grid-template-columns: 1fr;
  }
}
</style>
