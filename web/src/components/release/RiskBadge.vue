<template>
  <a-tooltip v-if="hasScore" :title="tooltipText">
    <a-tag :color="color" class="risk-badge">
      <span class="dot" :style="{ background: dotColor }"></span>
      <span class="level">{{ levelLabel }}</span>
      <span class="score">{{ score }}</span>
    </a-tag>
  </a-tooltip>
  <span v-else class="muted">-</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  score?: number
  level?: string
}>()

const hasScore = computed(() => typeof props.score === 'number' || !!props.level)

const effectiveLevel = computed(() => {
  if (props.level) return props.level
  const s = props.score ?? 0
  if (s <= 20) return 'low'
  if (s <= 50) return 'medium'
  if (s <= 80) return 'high'
  return 'critical'
})

const levelLabel = computed(() => {
  const map: Record<string, string> = {
    low: '低', medium: '中', high: '高', critical: '严重'
  }
  return map[effectiveLevel.value] || effectiveLevel.value
})

const color = computed(() => {
  const map: Record<string, string> = {
    low: 'green', medium: 'gold', high: 'orange', critical: 'red'
  }
  return map[effectiveLevel.value] || 'default'
})

const dotColor = computed(() => {
  const map: Record<string, string> = {
    low: '#52c41a', medium: '#faad14', high: '#fa8c16', critical: '#f5222d'
  }
  return map[effectiveLevel.value] || '#d9d9d9'
})

const score = computed(() => props.score ?? 0)

const tooltipText = computed(() => `风险评分 ${score.value} · ${effectiveLevel.value}`)
</script>

<style scoped>
.risk-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 1px 8px;
}
.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.score {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.muted {
  color: #bbb;
}
</style>
