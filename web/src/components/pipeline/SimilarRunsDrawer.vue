<!--
  SimilarRunsDrawer (Sprint 1 FE-05).

  Right-side drawer that expands the "历史相似" preview from DiagnosisCard
  with richer per-item context: timestamp, fix-by-commit chip, "open this
  run" / "view fix diff" actions.

  Stateless: receives the parent's already-loaded FailureDiagnosis and a
  v:open binding. Re-fetching is the parent's responsibility (avoids
  double-fetch when DiagnosisCard already has the data).
-->
<template>
  <a-drawer
    :open="open"
    :title="title"
    width="560"
    placement="right"
    :body-style="{ padding: '16px' }"
    @close="emit('update:open', false)"
  >
    <a-empty
      v-if="!diagnosis || !diagnosis.similar_runs.length"
      description="暂无相似失败"
    />

    <template v-else>
      <a-alert
        v-if="diagnosis.signature_occurrences"
        type="info"
        show-icon
        :message="signatureSummary"
        style="margin-bottom: 12px"
      />

      <a-list :data-source="diagnosis.similar_runs" item-layout="vertical" :split="true">
        <template #renderItem="{ item }">
          <a-list-item>
            <a-list-item-meta>
              <template #title>
                <a class="run-link" @click="onJumpToRun(item.run_id)">
                  Run #{{ item.run_id }}
                </a>
                <a-tag
                  v-if="item.fixed_by_commit"
                  color="green"
                  style="margin-left: 8px"
                >
                  <CheckCircleOutlined /> 已修复
                </a-tag>
              </template>
              <template #description>
                <span class="run-meta">
                  {{ formatTime(item.happened_at) }}
                  <span class="run-meta-rel">·  {{ relativeTime(item.happened_at) }}</span>
                </span>
              </template>
            </a-list-item-meta>

            <div v-if="item.fixed_by_commit" class="fix-row">
              <span class="fix-label">修复 commit:</span>
              <a-tooltip :title="item.fixed_by_commit">
                <code class="fix-commit">{{ shortCommit(item.fixed_by_commit) }}</code>
              </a-tooltip>
              <a
                v-if="item.fix_diff_url"
                :href="item.fix_diff_url"
                target="_blank"
                class="fix-diff-link"
              >查看 diff →</a>
            </div>

            <template #actions>
              <a @click="onJumpToRun(item.run_id)">查看该 run</a>
              <a v-if="item.fix_diff_url" :href="item.fix_diff_url" target="_blank">查看修复 diff</a>
            </template>
          </a-list-item>
        </template>
      </a-list>
    </template>
  </a-drawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { CheckCircleOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import relativeTimePlugin from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'
import type { FailureDiagnosis } from '@/services/diagnosis'

dayjs.extend(relativeTimePlugin)
dayjs.locale('zh-cn')

interface Props {
  open: boolean
  diagnosis: FailureDiagnosis | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:open', v: boolean): void
  (e: 'jump-to-run', runId: number): void
}>()

const title = computed(() => {
  const sigShort = props.diagnosis?.failure_signature
  if (!sigShort) return '历史相似失败'
  return `历史相似 · ${sigShort}`
})

const signatureSummary = computed(() => {
  const d = props.diagnosis
  if (!d) return ''
  return `该签名累计出现 ${d.signature_occurrences ?? 0} 次，涉及 ${
    d.signature_distinct_commits ?? 0
  } 个不同 commit。下方为最近 30 天内最多 3 条相似 run。`
})

const formatTime = (s?: string): string => (s ? dayjs(s).format('YYYY-MM-DD HH:mm') : '-')
const relativeTime = (s?: string): string => (s ? dayjs(s).fromNow() : '')
const shortCommit = (sha?: string): string => (sha || '').slice(0, 8)

const onJumpToRun = (runId: number) => {
  emit('jump-to-run', runId)
  emit('update:open', false)
}
</script>

<style scoped>
.run-link {
  font-weight: 500;
  font-size: 14px;
}
.run-meta {
  font-size: 12px;
  color: #666;
}
.run-meta-rel {
  margin-left: 4px;
  color: #999;
}
.fix-row {
  margin-top: 8px;
  font-size: 13px;
}
.fix-label {
  color: #888;
  margin-right: 6px;
}
.fix-commit {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  background: #f0f0f0;
  padding: 0 6px;
  border-radius: 3px;
}
.fix-diff-link {
  margin-left: 8px;
}
</style>
