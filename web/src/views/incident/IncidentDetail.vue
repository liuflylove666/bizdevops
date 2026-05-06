<template>
  <div class="incident-detail">
    <a-page-header
      :title="incident?.title || '加载中…'"
      :sub-title="incident ? `#${incident.id} · ${incident.application_name || '-'} · ${incident.env}` : ''"
      @back="goBack"
    >
      <template #tags>
        <a-tag v-if="incident" :color="severityColor(incident.severity)">{{ incident.severity }}</a-tag>
        <a-tag v-if="incident" :color="statusColor(incident.status)">{{ statusLabel(incident.status) }}</a-tag>
      </template>
      <template #extra>
        <a-space>
          <a-button @click="fetch">
            <template #icon><ReloadOutlined /></template>
            刷新
          </a-button>
          <a-button :loading="exporting" @click="onExportPostmortem">
            <template #icon><DownloadOutlined /></template>
            导出复盘 (MD)
          </a-button>
          <a-button
            v-if="incident && incident.status === 'open'"
            type="primary"
            ghost
            :loading="acting"
            @click="onMitigate"
          >
            标记止血
          </a-button>
          <a-button
            v-if="incident && incident.status !== 'resolved'"
            type="primary"
            :loading="acting"
            @click="showResolve = true"
          >
            解决
          </a-button>
        </a-space>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-row :gutter="16">
        <a-col :span="16">
          <a-card title="时间线" :bordered="false" size="small" style="margin-bottom: 16px">
            <a-timeline>
              <a-timeline-item color="red">
                <p><strong>发现</strong> · {{ formatTime(incident?.detected_at) }}</p>
                <p class="muted">来源：{{ sourceLabel(incident?.source) }}</p>
              </a-timeline-item>
              <a-timeline-item v-if="incident?.mitigated_at" color="orange">
                <p><strong>止血</strong> · {{ formatTime(incident?.mitigated_at) }}</p>
                <p v-if="mitigateLead" class="muted">距发现 {{ mitigateLead }}</p>
              </a-timeline-item>
              <a-timeline-item v-if="incident?.resolved_at" color="green">
                <p><strong>解决</strong> · {{ formatTime(incident?.resolved_at) }} · {{ incident?.resolved_by_name || '-' }}</p>
                <p v-if="resolveLead" class="muted">MTTR {{ resolveLead }}</p>
              </a-timeline-item>
            </a-timeline>
          </a-card>

          <a-card title="现场描述" :bordered="false" size="small" style="margin-bottom: 16px">
            <pre class="text-block">{{ incident?.description || '无' }}</pre>
          </a-card>

          <a-card title="根因与复盘" :bordered="false" size="small">
            <a-descriptions :column="1" size="small">
              <a-descriptions-item label="根因">
                <pre class="text-block">{{ incident?.root_cause || '尚未填写' }}</pre>
              </a-descriptions-item>
              <a-descriptions-item label="复盘链接">
                <a v-if="incident?.postmortem_url" :href="incident.postmortem_url" target="_blank">
                  {{ incident.postmortem_url }}
                </a>
                <span v-else class="muted">未提供</span>
              </a-descriptions-item>
            </a-descriptions>
          </a-card>
        </a-col>

        <a-col :span="8">
          <a-card title="基本信息" :bordered="false" size="small">
            <a-descriptions :column="1" size="small">
              <a-descriptions-item label="应用">{{ incident?.application_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="环境">{{ incident?.env }}</a-descriptions-item>
              <a-descriptions-item label="严重等级">{{ incident?.severity }}</a-descriptions-item>
              <a-descriptions-item label="来源">{{ sourceLabel(incident?.source) }}</a-descriptions-item>
              <a-descriptions-item label="告警指纹">
                <code v-if="incident?.alert_fingerprint">{{ incident.alert_fingerprint }}</code>
                <span v-else class="muted">-</span>
              </a-descriptions-item>
              <a-descriptions-item label="关联发布">
                <a v-if="incident?.release_id" @click="goRelease(incident.release_id)">
                  #{{ incident.release_id }}
                </a>
                <span v-else class="muted">-</span>
              </a-descriptions-item>
              <a-descriptions-item label="登记人">{{ incident?.created_by_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="登记时间">{{ formatTime(incident?.created_at) }}</a-descriptions-item>
            </a-descriptions>
          </a-card>
        </a-col>
      </a-row>
    </a-spin>

    <a-modal v-model:open="showResolve" title="解决事故" :confirm-loading="acting" @ok="onResolve">
      <a-form layout="vertical">
        <a-form-item label="根因">
          <a-textarea v-model:value="resolveForm.root_cause" :rows="3" />
        </a-form-item>
        <a-form-item label="复盘文档链接">
          <a-input v-model:value="resolveForm.postmortem_url" placeholder="https://..." />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
/**
 * 事故详情页（v2.1）。
 *
 * 与 ReleaseDetail 类似：时间线 + 基本信息 + 根因/复盘 三部分。
 */
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { ReloadOutlined, DownloadOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { incidentApi, type Incident } from '@/services/incident'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const acting = ref(false)
const exporting = ref(false)
const incident = ref<Incident | null>(null)
const showResolve = ref(false)
const resolveForm = reactive({ root_cause: '', postmortem_url: '' })

const id = computed(() => Number(route.params.id))

async function fetch() {
  loading.value = true
  try {
    const res = await incidentApi.getById(id.value)
    incident.value = (res as any)?.data || null
    resolveForm.root_cause = incident.value?.root_cause || ''
    resolveForm.postmortem_url = incident.value?.postmortem_url || ''
  } catch (e: any) {
    message.error(e?.response?.data?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function onMitigate() {
  acting.value = true
  try {
    await incidentApi.mitigate(id.value)
    message.success('已标记止血')
    fetch()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '操作失败')
  } finally {
    acting.value = false
  }
}
async function onResolve() {
  acting.value = true
  try {
    await incidentApi.resolve(id.value, { ...resolveForm })
    message.success('已解决')
    showResolve.value = false
    fetch()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '解决失败')
  } finally {
    acting.value = false
  }
}

async function onExportPostmortem() {
  if (!incident.value?.id) return
  exporting.value = true
  try {
    const res = await incidentApi.exportPostmortem(incident.value.id)
    // axios responseType=blob 时，res.data 是 Blob
    const raw = (res as any)?.data ?? res
    const blob: Blob =
      raw instanceof Blob ? raw : new Blob([String(raw)], { type: 'text/markdown;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `incident-${incident.value.id}-postmortem.md`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    message.success('复盘文档已下载')
  } catch (e: any) {
    message.error(e?.response?.data?.message || e?.message || '导出失败')
  } finally {
    exporting.value = false
  }
}

function goBack() { router.push('/incidents') }
function goRelease(rid: number) { router.push(`/releases/${rid}`) }

function severityColor(s: string) {
  return s === 'P0' ? 'red' : s === 'P1' ? 'volcano' : s === 'P2' ? 'orange' : 'default'
}
function statusColor(s: string) {
  return s === 'resolved' ? 'green' : s === 'mitigated' ? 'gold' : 'red'
}
function statusLabel(s: string) {
  return s === 'resolved' ? '已解决' : s === 'mitigated' ? '已止血' : '未解决'
}
function sourceLabel(s?: string) {
  if (s === 'alert') return '告警触发'
  if (s === 'release_failure') return '发布失败'
  return '手工登记'
}
function formatTime(t?: string) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm:ss') : '-'
}
function humanDiff(a?: string, b?: string) {
  if (!a || !b) return ''
  const mins = dayjs(b).diff(dayjs(a), 'minute')
  if (mins < 60) return mins + ' 分钟'
  if (mins < 24 * 60) return (mins / 60).toFixed(1) + ' 小时'
  return (mins / 60 / 24).toFixed(1) + ' 天'
}
const mitigateLead = computed(() => humanDiff(incident.value?.detected_at, incident.value?.mitigated_at))
const resolveLead = computed(() => humanDiff(incident.value?.detected_at, incident.value?.resolved_at))

onMounted(fetch)
</script>

<style scoped>
.incident-detail {
  padding: 0;
}
.muted {
  color: #999;
  font-size: 12px;
}
.text-block {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  font-family: inherit;
}
</style>
