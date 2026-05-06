<template>
  <div class="risk-radar-wrapper" :style="{ height: heightStyle }">
    <div ref="chartEl" class="chart"></div>
    <div v-if="!hasData" class="empty-overlay">
      <a-empty description="暂无风险数据" />
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * RiskRadar
 *
 * 把 risk_factors.hits（命中规则集合）渲染成雷达图。
 *
 * 维度从规则 key 提取并分组到 5 个固定维度，避免维度过多导致图形难读：
 *   - 环境 (env.*)
 *   - 策略 (strategy.*)
 *   - 变更内容 (items.*)
 *   - 时间窗口 (time.*)
 *   - 人工标记 (manual.*)
 *
 * 每个维度的值 = 命中规则的累计权重；最大值取规则集理论上限或固定 30
 */
import { ref, watch, onMounted, onBeforeUnmount, computed } from 'vue'
import * as echarts from 'echarts'
import type { ECharts } from 'echarts'

interface Hit {
  key: string
  name: string
  weight: number
  detail?: string
}

const props = defineProps<{
  hits?: Hit[] | null
  height?: number | string
}>()

const chartEl = ref<HTMLDivElement | null>(null)
let chart: ECharts | null = null

const hasData = computed(() => Array.isArray(props.hits) && props.hits.length > 0)

const heightStyle = computed(() => {
  if (typeof props.height === 'number') return `${props.height}px`
  return props.height || '260px'
})

const dimensions = [
  { key: 'env', label: '环境' },
  { key: 'strategy', label: '策略' },
  { key: 'items', label: '变更内容' },
  { key: 'time', label: '时间窗口' },
  { key: 'manual', label: '人工标记' },
]

function dimensionOf(key: string): string {
  const prefix = key.split('.')[0]
  return dimensions.find((d) => d.key === prefix)?.key || 'items'
}

function aggregate(hits: Hit[]) {
  const sums: Record<string, number> = {}
  dimensions.forEach((d) => (sums[d.key] = 0))
  for (const h of hits) {
    sums[dimensionOf(h.key)] += h.weight
  }
  return dimensions.map((d) => sums[d.key])
}

function renderChart() {
  if (!chartEl.value) return
  if (!chart) {
    chart = echarts.init(chartEl.value)
  }
  const values = aggregate(props.hits || [])
  const option: any = {
    tooltip: {
      trigger: 'item',
      formatter: () => {
        const lines = dimensions.map((d, i) => `${d.label}: ${values[i]}`).join('<br/>')
        return `<b>风险维度分布</b><br/>${lines}`
      },
    },
    radar: {
      indicator: dimensions.map((d) => ({ name: d.label, max: 30 })),
      shape: 'polygon',
      splitNumber: 4,
      axisName: { color: '#666', fontSize: 12 },
      splitArea: { areaStyle: { color: ['rgba(245,247,250,0.6)', '#fff'] } },
    },
    series: [
      {
        type: 'radar',
        symbolSize: 6,
        data: [
          {
            value: values,
            name: '本次发布',
            areaStyle: { color: 'rgba(250, 140, 22, 0.25)' },
            lineStyle: { color: '#fa8c16', width: 2 },
            itemStyle: { color: '#fa8c16' },
          },
        ],
      },
    ],
  }
  chart.setOption(option, true)
}

function handleResize() {
  chart?.resize()
}

watch(() => props.hits, renderChart, { deep: true })

onMounted(() => {
  renderChart()
  window.addEventListener('resize', handleResize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  chart?.dispose()
  chart = null
})
</script>

<style scoped>
.risk-radar-wrapper {
  position: relative;
  width: 100%;
}
.chart {
  width: 100%;
  height: 100%;
}
.empty-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.85);
}
</style>
