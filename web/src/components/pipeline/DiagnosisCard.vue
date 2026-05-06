<!--
  DiagnosisCard (Sprint 1 FE-04).

  Embedded at the top of a failed/cancelled run's detail view. Renders the
  data-driven (zero-AI) failure diagnosis returned by GET
  /pipeline/runs/:id/diagnosis. Hides itself silently when the parent run
  is not a candidate for diagnosis (e.g., status is success/running).

  Defensive against missing fields per docs/api/diagnosis_v1.md: the
  contract uses null/empty arrays for absent data and forbids guesses, so
  this card displays explicit placeholders ("暂未识别"/"暂无") rather than
  hiding sections silently — users should know something is intentionally
  empty vs. silently broken.
-->
<template>
  <div v-if="visible" class="diagnosis-card">
    <a-spin :spinning="loading">
      <a-card
        size="small"
        :bordered="false"
        :body-style="{ padding: '12px 16px' }"
        class="diagnosis-card-inner"
      >
        <template #title>
          <div class="diag-title">
            <BugOutlined class="diag-title-icon" />
            <span>失败诊断</span>
            <a-tag
              v-if="signatureShort"
              color="volcano"
              class="diag-sig"
            >{{ signatureShort }}</a-tag>
            <a-tag
              v-if="diagnosis?.is_flaky"
              :color="flakyColor"
              class="diag-flaky"
            >
              <ThunderboltOutlined /> Flaky · {{ flakyLabel }}
            </a-tag>
            <a-tag
              v-if="diagnosis && diagnosis.failure_signature === null"
              color="default"
            >无法识别失败签名</a-tag>
          </div>
        </template>

        <!-- Empty-state when diagnosis fetch failed -->
        <a-alert
          v-if="errorMessage"
          type="error"
          :message="errorMessage"
          show-icon
          style="margin-bottom: 12px"
        />

        <!-- Degraded form: only log_tail, signature unavailable -->
        <template v-if="diagnosis && diagnosis.failure_signature === null">
          <p class="diag-hint">
            日志归一化未能产出稳定签名，仅展示原始日志末尾。
          </p>
          <DiagnosisLogTail :lines="diagnosis.log_tail" />
        </template>

        <!-- Full form -->
        <template v-else-if="diagnosis">
          <a-row :gutter="16">
            <a-col :span="12">
              <div class="diag-section">
                <div class="diag-section-label">上次成功 commit</div>
                <div v-if="diagnosis.last_success" class="diag-section-body">
                  <a-tooltip :title="diagnosis.last_success.commit">
                    <code class="diag-commit">{{ shortCommit(diagnosis.last_success.commit) }}</code>
                  </a-tooltip>
                  <span class="diag-time">· {{ formatTime(diagnosis.last_success.happened_at) }}</span>
                  <a
                    v-if="diagnosis.last_success.diff_url"
                    :href="diagnosis.last_success.diff_url"
                    target="_blank"
                    class="diag-link"
                  >查看 diff</a>
                </div>
                <div v-else class="diag-empty">暂无历史成功记录</div>
              </div>

              <div class="diag-section">
                <div class="diag-section-label">改动文件</div>
                <div v-if="diagnosis.changed_files.length" class="diag-section-body">
                  <ul class="diag-files">
                    <li v-for="f in diagnosis.changed_files" :key="f.path">
                      <code>{{ f.path }}</code>
                      <span v-if="f.additions" class="diag-add">+{{ f.additions }}</span>
                      <span v-if="f.deletions" class="diag-del">-{{ f.deletions }}</span>
                    </li>
                  </ul>
                </div>
                <div v-else class="diag-empty">暂未识别</div>
              </div>
            </a-col>

            <a-col :span="12">
              <div class="diag-section">
                <div class="diag-section-label">
                  历史相似
                  <span v-if="diagnosis.similar_runs.length" class="diag-section-count">
                    （{{ diagnosis.similar_runs.length }}）
                  </span>
                </div>
                <div v-if="diagnosis.similar_runs.length" class="diag-section-body">
                  <ul class="diag-similar">
                    <li v-for="r in diagnosis.similar_runs" :key="r.run_id">
                      <a @click="emitOpenSimilar">#{{ r.run_id }}</a>
                      <span class="diag-time">· {{ formatTime(r.happened_at) }}</span>
                      <a-tag
                        v-if="r.fixed_by_commit"
                        color="green"
                        class="diag-fix-tag"
                      >已被 {{ shortCommit(r.fixed_by_commit) }} 修复</a-tag>
                    </li>
                  </ul>
                  <a class="diag-link" @click="emitOpenSimilar">查看详情 →</a>
                </div>
                <div v-else class="diag-empty">暂无相似失败</div>
              </div>

              <div class="diag-section">
                <div class="diag-section-label">修复参考</div>
                <div v-if="diagnosis.fix_references.length" class="diag-section-body">
                  <ul class="diag-fixrefs">
                    <li v-for="(ref, idx) in diagnosis.fix_references" :key="idx">
                      <a-tag :color="fixRefColor(ref.kind)">{{ fixRefLabel(ref.kind) }}</a-tag>
                      <a v-if="ref.url" :href="ref.url" target="_blank" class="diag-fixref-title">
                        {{ ref.title }}
                      </a>
                      <span v-else class="diag-fixref-title">{{ ref.title }}</span>
                    </li>
                  </ul>
                </div>
                <div v-else class="diag-empty">暂无修复参考</div>
              </div>
            </a-col>
          </a-row>

          <a-collapse v-if="diagnosis.log_tail.length" ghost class="diag-logs-collapse">
            <a-collapse-panel key="logs" :header="`日志末尾（${diagnosis.log_tail.length} 行）`">
              <DiagnosisLogTail :lines="diagnosis.log_tail" />
            </a-collapse-panel>
          </a-collapse>

          <div v-if="diagnosis.signature_occurrences" class="diag-meta">
            该签名累计出现
            <strong>{{ diagnosis.signature_occurrences }}</strong> 次，
            涉及
            <strong>{{ diagnosis.signature_distinct_commits ?? 0 }}</strong> 个不同 commit。
            首次出现于 {{ formatTime(diagnosis.signature_first_seen_at) }}。
          </div>
        </template>
      </a-card>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref, watch } from 'vue'
import { BugOutlined, ThunderboltOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import {
  diagnosisApi,
  type FailureDiagnosis,
  type FixReferenceKind,
  type LogTailLine,
} from '@/services/diagnosis'

interface Props {
  /** Run ID to diagnose. Card hides itself if 0/undefined. */
  runId: number | null | undefined
  /** Run status from parent; only failed/cancelled triggers a fetch. */
  runStatus?: string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'open-similar', diagnosis: FailureDiagnosis): void
}>()

const loading = ref(false)
const errorMessage = ref('')
const diagnosis = ref<FailureDiagnosis | null>(null)

const visible = computed(() => {
  if (!props.runId) return false
  if (props.runStatus && props.runStatus !== 'failed' && props.runStatus !== 'cancelled') {
    return false
  }
  return true
})

const signatureShort = computed(() => {
  const sig = diagnosis.value?.failure_signature
  if (!sig) return ''
  return sig
})

const flakyColor = computed(() => {
  switch (diagnosis.value?.flaky_reason) {
    case 'same_commit_retry_succeeded': return 'green'
    case 'cross_commit_recurrence': return 'orange'
    default: return 'default'
  }
})

const flakyLabel = computed(() => {
  switch (diagnosis.value?.flaky_reason) {
    case 'same_commit_retry_succeeded': return '同 commit 重试已转绿'
    case 'cross_commit_recurrence': return '跨 commit 7d 复发'
    default: return ''
  }
})

const fixRefColor = (kind: FixReferenceKind): string => ({
  jira_issue: 'blue',
  postmortem: 'purple',
  improvement_item: 'cyan',
}[kind] || 'default')

const fixRefLabel = (kind: FixReferenceKind): string => ({
  jira_issue: 'Jira',
  postmortem: '复盘',
  improvement_item: '改进项',
}[kind] || kind)

const shortCommit = (sha: string): string => (sha || '').slice(0, 8)

const formatTime = (s?: string): string => (s ? dayjs(s).format('YYYY-MM-DD HH:mm') : '-')

const emitOpenSimilar = () => {
  if (diagnosis.value) emit('open-similar', diagnosis.value)
}

const load = async (id: number) => {
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await diagnosisApi.getRunDiagnosis(id)
    diagnosis.value = res.data
  } catch (e: any) {
    errorMessage.value = e?.message || '诊断接口请求失败'
    diagnosis.value = null
  } finally {
    loading.value = false
  }
}

// Internal helper component for consistent log rendering.
const DiagnosisLogTail = defineComponent({
  props: { lines: { type: Array as () => LogTailLine[], required: true } },
  setup(p) {
    return () =>
      h(
        'pre',
        { class: 'diag-logs' },
        p.lines.map((ln) => `${ln.ts ? `[${ln.ts}] ` : ''}${ln.stream ? `${ln.stream}: ` : ''}${ln.line}`).join('\n'),
      )
  },
})

watch(
  () => [props.runId, props.runStatus],
  () => {
    if (visible.value && props.runId) {
      load(props.runId)
    } else {
      diagnosis.value = null
      errorMessage.value = ''
    }
  },
  { immediate: true },
)

defineExpose({ refresh: () => props.runId && load(props.runId) })
</script>

<style scoped>
.diagnosis-card {
  margin-bottom: 12px;
}
.diagnosis-card-inner {
  border-left: 3px solid #fa541c;
  background: #fff7e6;
}
.diag-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.diag-title-icon {
  color: #fa541c;
}
.diag-sig {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 12px;
}
.diag-flaky {
  font-weight: 500;
}
.diag-section {
  margin-bottom: 12px;
}
.diag-section:last-child {
  margin-bottom: 0;
}
.diag-section-label {
  font-size: 12px;
  color: #888;
  margin-bottom: 4px;
}
.diag-section-count {
  color: #aaa;
  font-weight: normal;
}
.diag-section-body {
  font-size: 13px;
}
.diag-empty {
  font-size: 13px;
  color: #bbb;
  font-style: italic;
}
.diag-commit {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  background: #f0f0f0;
  padding: 0 6px;
  border-radius: 3px;
}
.diag-time {
  color: #888;
  margin-left: 6px;
}
.diag-link {
  margin-left: 8px;
}
.diag-files,
.diag-similar,
.diag-fixrefs {
  list-style: none;
  padding: 0;
  margin: 0;
}
.diag-files li,
.diag-similar li,
.diag-fixrefs li {
  margin-bottom: 3px;
  line-height: 1.6;
}
.diag-add {
  color: #52c41a;
  margin-left: 6px;
}
.diag-del {
  color: #f5222d;
  margin-left: 6px;
}
.diag-fix-tag {
  margin-left: 6px;
  font-size: 11px;
}
.diag-fixref-title {
  margin-left: 6px;
}
.diag-logs-collapse {
  margin-top: 8px;
}
.diag-logs {
  background: #1f1f1f;
  color: #e8e8e8;
  padding: 10px;
  border-radius: 4px;
  font-size: 12px;
  max-height: 240px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
.diag-meta {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px dashed #ffd591;
  font-size: 12px;
  color: #888;
}
.diag-hint {
  font-size: 13px;
  color: #888;
  margin-bottom: 8px;
}
</style>
