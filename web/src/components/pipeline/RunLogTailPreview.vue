<!--
  RunLogTailPreview (Sprint 2 FE-08).

  Mounts inside an `<a-popover>` content slot that wraps a run-id link in
  the runs table. Lazy-fetches the last N lines via BE-13 on first mount,
  caches per-instance so re-hovering the same row does not re-request.

  Visible only on hover (via the parent's mouse-enter-delay), so we never
  hammer the API for a casual table scroll.
-->
<template>
  <div class="rltp-root">
    <div class="rltp-header">
      <span class="rltp-title">Run #{{ runId }} 末尾日志</span>
      <a-tag v-if="data?.lines_truncated" color="orange" class="rltp-badge">截断</a-tag>
    </div>

    <a-spin :spinning="loading" size="small">
      <a-alert
        v-if="errorMessage"
        type="error"
        show-icon
        :message="errorMessage"
        class="rltp-error"
      />
      <pre v-else-if="data && data.lines.length" class="rltp-pane">{{ joinedLines }}</pre>
      <div v-else-if="!loading && data" class="rltp-empty">无日志（run 可能仍在排队）</div>
    </a-spin>

    <div v-if="data && data.lines.length" class="rltp-footer">
      共 {{ data.lines_total }} 行
      <span v-if="data.lines_truncated" class="rltp-hint">（更早内容已截断，进入详情看全文）</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { pipelineApi } from '@/services/pipeline'

interface Props {
  runId: number
  /** Number of trailing lines to fetch. Backend caps at 500. */
  tailLines?: number
}

const props = withDefaults(defineProps<Props>(), {
  tailLines: 50,
})

interface LogTailLine {
  ts?: string
  stream?: string
  line: string
}

interface LogTailEnvelope {
  run_id: number
  status: string
  lines_total: number
  lines_truncated: boolean
  lines: LogTailLine[]
}

const loading = ref(false)
const errorMessage = ref('')
const data = ref<LogTailEnvelope | null>(null)

const joinedLines = computed(() => {
  if (!data.value) return ''
  return data.value.lines.map((l) => l.line).join('\n')
})

const load = async (id: number) => {
  loading.value = true
  errorMessage.value = ''
  data.value = null
  try {
    const res: any = await pipelineApi.getRunLogTail(id, props.tailLines)
    data.value = res?.data || null
  } catch (e: any) {
    errorMessage.value = e?.message || '加载日志失败'
  } finally {
    loading.value = false
  }
}

// runId 变化（不同行 hover）就重新加载；初挂时立即加载。
watch(
  () => props.runId,
  (id) => {
    if (id) load(id)
  },
  { immediate: true },
)
</script>

<style scoped>
.rltp-root {
  width: 540px;
  max-width: 80vw;
}
.rltp-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.rltp-title {
  font-weight: 500;
  font-size: 13px;
}
.rltp-badge {
  font-size: 11px;
}
.rltp-error {
  margin-bottom: 6px;
}
.rltp-pane {
  background: #1f1f1f;
  color: #e8e8e8;
  padding: 8px 10px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 1.5;
  max-height: 300px;
  overflow: auto;
  white-space: pre;
  margin: 0;
}
.rltp-empty {
  color: #999;
  font-size: 13px;
  padding: 16px;
  text-align: center;
}
.rltp-footer {
  margin-top: 6px;
  font-size: 11px;
  color: #888;
}
.rltp-hint {
  color: #aaa;
  margin-left: 4px;
}
</style>
