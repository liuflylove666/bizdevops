<template>
  <div class="event-timeline-page">
    <a-page-header title="事件时间线" sub-title="事故、告警、审批、变更事件与发布主单按时间融合（E4-03）" />

    <a-card :bordered="false" style="margin-bottom: 16px">
      <a-form layout="inline" :model="filters" @finish="load">
        <a-form-item label="应用">
          <a-select
            v-model:value="filters.application_id"
            allow-clear
            show-search
            option-filter-prop="label"
            placeholder="全部应用"
            style="min-width: 220px"
            :options="appOptions"
            :filter-option="filterAppOption"
          />
        </a-form-item>
        <a-form-item label="环境">
          <a-select v-model:value="filters.env" allow-clear placeholder="全部" style="width: 120px">
            <a-select-option value="dev">dev</a-select-option>
            <a-select-option value="test">test</a-select-option>
            <a-select-option value="staging">staging</a-select-option>
            <a-select-option value="prod">prod</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" :loading="loading">查询</a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card :bordered="false" :loading="loading">
      <template v-if="meta.truncated">
        <a-alert type="info" show-icon style="margin-bottom: 12px" message="结果已按条数上限截断，请缩小时间范围或加应用筛选。" />
      </template>
      <a-empty v-if="!items.length && !loading" description="暂无事件" />
      <a-timeline v-else mode="left">
        <a-timeline-item v-for="it in items" :key="`${it.kind}-${it.id}`" :color="colorFor(it)">
          <p class="tl-time">{{ formatAt(it.at) }}</p>
          <p class="tl-title">
            <a-tag>{{ labelKind(it.kind) }}</a-tag>
            <router-link v-if="it.ref" :to="it.ref">{{ it.title }}</router-link>
            <span v-else>{{ it.title }}</span>
          </p>
          <p v-if="it.summary" class="tl-sum">{{ it.summary }}</p>
          <p class="tl-meta">
            <a-tag v-if="it.env" size="small">{{ it.env }}</a-tag>
            <a-tag v-if="it.status" size="small" :color="statusColor(it)">{{ it.status }}</a-tag>
            <a-tag v-if="it.severity" size="small" :color="severityColor(it.severity)">{{ it.severity }}</a-tag>
          </p>
        </a-timeline-item>
      </a-timeline>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { observabilityTimelineApi, type TimelineItem } from '@/services/observabilityTimeline'
import { applicationApi } from '@/services/application'

const route = useRoute()

const loading = ref(false)
const items = ref<TimelineItem[]>([])
const meta = reactive({ truncated: false })

const filters = reactive<{
  application_id?: number
  env?: string
}>({})

const appOptions = ref<{ label: string; value: number }[]>([])

const filterAppOption = (input: string, option: { label: string }) =>
  option.label.toLowerCase().includes(input.toLowerCase())

const labelKind = (k: string) => {
  if (k === 'incident') return '事故'
  if (k === 'alert') return '告警'
  if (k === 'approval') return '审批'
  if (k === 'change_event') return '变更'
  if (k === 'release') return '发布'
  return k
}

const colorFor = (it: TimelineItem) => {
  if (it.kind === 'incident') return 'red'
  if (it.kind === 'alert') return 'orange'
  if (it.kind === 'approval') return 'purple'
  if (it.kind === 'release') return 'green'
  return 'blue'
}

const statusColor = (it: TimelineItem) => {
  const status = (it.status || '').toLowerCase()
  if (it.kind === 'approval') {
    if (status === 'approved') return 'green'
    if (status === 'rejected' || status === 'failed' || status === 'cancelled') return 'red'
    if (status === 'pending') return 'gold'
  }
  if (it.kind === 'alert') {
    if (status === 'resolved' || status === 'acked') return 'green'
    if (status === 'failed') return 'red'
    if (status === 'pending') return 'gold'
  }
  if (status === 'success' || status === 'synced') return 'green'
  if (status === 'failed' || status === 'error') return 'red'
  return 'blue'
}

const severityColor = (s: string) => {
  if (s === 'P0' || s === 'P1') return 'red'
  if (s === 'P2') return 'orange'
  return 'default'
}

const formatAt = (iso: string) => {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

const load = async () => {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      limit: 200,
    }
    if (filters.application_id != null) params.application_id = filters.application_id
    if (filters.env) params.env = filters.env

    const res = await observabilityTimelineApi.get(params as any)
    if (res.code !== 0) {
      message.error(res.message || '加载失败')
      return
    }
    const d = res.data
    if (!d) {
      items.value = []
      return
    }
    items.value = d.items || []
    meta.truncated = !!d.truncated
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const bootstrapFromRoute = () => {
  const q = route.query.application_id
  if (typeof q === 'string' && /^\d+$/.test(q)) {
    filters.application_id = Number(q)
  }
}

onMounted(async () => {
  try {
    const res = await applicationApi.list({ page: 1, page_size: 500 })
    if (res.code === 0 && res.data?.list) {
      appOptions.value = res.data.list.map((a) => ({
        value: a.id as number,
        label: `${a.display_name || a.name} (#${a.id})`,
      }))
    }
  } catch {
    /* ignore */
  }
  bootstrapFromRoute()
  await load()
})
</script>

<style scoped>
.event-timeline-page {
  padding: 0 8px 24px;
}
.tl-time {
  color: #8c8c8c;
  font-size: 12px;
  margin-bottom: 4px;
}
.tl-title {
  font-weight: 500;
  margin-bottom: 4px;
}
.tl-sum {
  color: #595959;
  font-size: 13px;
  margin-bottom: 4px;
}
.tl-meta {
  margin: 0;
}
</style>
