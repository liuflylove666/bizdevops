<!--
  ContextPill (Sprint 2 FE-10).

  Top-bar chip showing the current focus context (app / env / branch /
  pipeline) maintained by stores/context.ts. Hidden when nothing is set
  to avoid UI clutter; clicking the × clears all dimensions at once.

  This is a read-only surface — population happens in views (e.g.,
  PipelineDetail records the pipeline + branch on mount).
-->
<template>
  <a-tooltip v-if="ctx.hasAny" :title="tooltipDetail">
    <a-tag
      class="ctx-pill"
      color="blue"
      closable
      @close.prevent="onClear"
    >
      <AimOutlined class="ctx-icon" />
      <span class="ctx-summary">{{ ctx.summary }}</span>
    </a-tag>
  </a-tooltip>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { AimOutlined } from '@ant-design/icons-vue'
import { useContextStore } from '@/stores/context'

const ctx = useContextStore()

const tooltipDetail = computed(() => {
  const lines: string[] = []
  if (ctx.app) lines.push(`应用：${ctx.app.name}`)
  if (ctx.pipeline) lines.push(`流水线：${ctx.pipeline.name}`)
  if (ctx.env) lines.push(`环境：${ctx.env}`)
  if (ctx.branch) lines.push(`分支：${ctx.branch}`)
  lines.push('点击 × 清空当前上下文')
  return lines.join('\n')
})

const onClear = () => {
  ctx.clearAll()
}
</script>

<style scoped>
.ctx-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0 8px;
  height: 26px;
  border-radius: 13px;
  cursor: default;
  font-size: 12px;
  max-width: 280px;
}
.ctx-icon {
  color: #1677ff;
}
.ctx-summary {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
