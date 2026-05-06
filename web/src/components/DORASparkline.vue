<template>
  <div ref="elRef" class="dora-sparkline" :style="{ width: width + 'px', height: height + 'px' }"></div>
</template>

<script setup lang="ts">
/**
 * DORASparkline —— DORA 指标迷你趋势图（v2.1）。
 *
 * 不显示轴、legend、网格，纯粹的走势视觉；tooltip 显示日期与数值。
 * 数据为空时渲染占位水平线，避免 ECharts 空数据告警。
 */
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { TooltipComponent, GridComponent } from 'echarts/components'
import { SVGRenderer } from 'echarts/renderers'
import type { DORASeriesPoint } from '@/services/dora'

echarts.use([LineChart, TooltipComponent, GridComponent, SVGRenderer])

const props = withDefaults(
  defineProps<{
    points: DORASeriesPoint[]
    /** 上一周期序列；提供且长度>0 时以虚线叠加，便于直观对比 */
    prevPoints?: DORASeriesPoint[]
    color?: string
    unit?: string
    width?: number
    height?: number
  }>(),
  {
    prevPoints: () => [],
    color: '#1677ff',
    unit: '',
    width: 120,
    height: 36,
  },
)

const elRef = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null

function render() {
  if (!elRef.value) return
  if (!chart) {
    chart = echarts.init(elRef.value, undefined, { renderer: 'svg' })
  }
  const pts = props.points && props.points.length > 0
    ? props.points
    : [
        { date: '', value: 0 },
        { date: '', value: 0 },
      ]
  const xs = pts.map((_, i) => i)
  const curValues = pts.map((p) => p.value)
  const prevHasData = Array.isArray(props.prevPoints) && props.prevPoints.length > 0
  const prevValues = prevHasData
    ? props.prevPoints!.map((p) => p.value)
    : []
  // x 轴按索引对齐；tooltip 时同时显示两侧日期
  const allValues = [...curValues, ...prevValues]
  const max = allValues.length > 0 ? Math.max(...allValues) : 0
  const allZero = allValues.every((v) => v === 0)

  const series: any[] = [
    {
      name: '当前',
      type: 'line',
      data: curValues,
      showSymbol: false,
      smooth: true,
      lineStyle: { color: props.color, width: 1.6 },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: props.color + '55' },
          { offset: 1, color: props.color + '00' },
        ]),
      },
      z: 3,
    },
  ]
  if (prevHasData) {
    series.push({
      name: '上一周期',
      type: 'line',
      data: prevValues,
      showSymbol: false,
      smooth: true,
      lineStyle: {
        color: '#bfbfbf',
        width: 1.2,
        type: 'dashed',
      },
      z: 1,
    })
  }

  chart.setOption(
    {
      grid: { top: 2, bottom: 2, left: 2, right: 2 },
      xAxis: { type: 'category', show: false, boundaryGap: false, data: xs },
      yAxis: {
        type: 'value',
        show: false,
        min: 0,
        max: allZero ? 1 : max * 1.1,
      },
      tooltip: {
        trigger: 'axis',
        confine: true,
        formatter: (params: any) => {
          const arr = Array.isArray(params) ? params : [params]
          const idx = arr[0].dataIndex
          const curDate = pts[idx]?.date || '-'
          const prevDate = prevHasData ? props.prevPoints![idx]?.date || '-' : ''
          const curVal = curValues[idx] ?? '-'
          const prevVal = prevHasData ? prevValues[idx] ?? '-' : null
          const u = props.unit || ''
          let html = `<div style="min-width:100px"><b>${curDate}</b>: ${curVal}${u}</div>`
          if (prevHasData) {
            html += `<div style="color:#999">上期 ${prevDate}: ${prevVal}${u}</div>`
          }
          return html
        },
        textStyle: { fontSize: 12 },
      },
      series,
    },
    true, // notMerge：避免残留上次的双线
  )
}

function resize() {
  chart?.resize()
}

onMounted(() => {
  nextTick(render)
  window.addEventListener('resize', resize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', resize)
  chart?.dispose()
  chart = null
})
watch(() => props.points, render, { deep: true })
watch(() => props.prevPoints, render, { deep: true })
watch(() => props.color, render)
</script>

<style scoped>
.dora-sparkline {
  display: inline-block;
  vertical-align: middle;
}
</style>
