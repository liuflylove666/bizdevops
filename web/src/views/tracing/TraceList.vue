<template>
  <div class="tracing-page">
    <div class="tracing-header">
      <a-space>
        <a-select v-model:value="selectedService" :placeholder="t('menu.selectService')" allow-clear style="width: 200px" @change="handleSearch">
          <a-select-option v-for="svc in services" :key="svc" :value="svc">{{ svc }}</a-select-option>
        </a-select>
        <a-input v-model:value="traceIdSearch" :placeholder="t('menu.traceIdPlaceholder')" allow-clear style="width: 300px" @pressEnter="handleSearch">
          <template #prefix>
            <SearchOutlined />
          </template>
        </a-input>
        <a-select v-model:value="statusFilter" :placeholder="t('menu.statusFilter')" allow-clear style="width: 120px" @change="handleSearch">
          <a-select-option value="ok">{{ t('menu.statusOk') }}</a-select-option>
          <a-select-option value="error">{{ t('menu.statusError') }}</a-select-option>
        </a-select>
        <a-range-picker v-model:value="timeRange" show-time :placeholder="[t('menu.startTime'), t('menu.endTime')]" @change="handleSearch" />
        <a-button type="primary" :loading="loading" @click="handleSearch">
          <SearchOutlined /> {{ t('common.search') }}
        </a-button>
      </a-space>
    </div>

    <a-spin :spinning="loading">
      <div class="tracing-content">
        <a-tabs v-model:activeKey="activeTab">
          <a-tab-pane key="list" :tab="t('menu.traceList')">
            <a-table
              :columns="columns"
              :data-source="traces"
              :pagination="pagination"
              :loading="loading"
              row-key="trace_id"
              @change="handleTableChange"
              size="small"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'status'">
                  <a-tag :color="record.status === 'ok' ? 'green' : 'red'">
                    {{ record.status === 'ok' ? t('menu.statusOk') : t('menu.statusError') }}
                  </a-tag>
                </template>
                <template v-if="column.key === 'duration'">
                  <span :style="{ color: getDurationColor(record.duration_ms) }">
                    {{ record.duration_ms }}ms
                  </span>
                </template>
                <template v-if="column.key === 'start_time'">
                  {{ formatTime(record.start_time) }}
                </template>
                <template v-if="column.key === 'operation'">
                  <span class="operation-name">{{ record.operation }}</span>
                </template>
                <template v-if="column.key === 'service'">
                  <a-tag color="blue">{{ record.service }}</a-tag>
                </template>
                <template v-if="column.key === 'action'">
                  <a-space>
                    <a @click="handleViewTrace(record)">{{ t('menu.viewDetail') }}</a>
                    <a @click="handleCopyTraceId(record.trace_id)">
                      <CopyOutlined />
                    </a>
                  </a-space>
                </template>
              </template>
            </a-table>
          </a-tab-pane>

          <a-tab-pane key="detail" :tab="t('menu.traceDetail')" :disabled="!currentTrace">
            <div v-if="currentTrace" class="trace-detail">
              <a-descriptions bordered :column="2" size="small">
                <a-descriptions-item :label="t('menu.traceId')">
                  <a @click="handleCopyTraceId(currentTrace.trace_id)">{{ currentTrace.trace_id }}</a>
                </a-descriptions-item>
                <a-descriptions-item :label="t('menu.spanId')">{{ currentTrace.span_id }}</a-descriptions-item>
                <a-descriptions-item :label="t('menu.operation')">{{ currentTrace.operation }}</a-descriptions-item>
                <a-descriptions-item :label="t('menu.service')">
                  <a-tag color="blue">{{ currentTrace.service }}</a-tag>
                </a-descriptions-item>
                <a-descriptions-item :label="t('menu.kind')">{{ currentTrace.kind }}</a-descriptions-item>
                <a-descriptions-item :label="t('menu.status')">
                  <a-tag :color="currentTrace.status === 'ok' ? 'green' : 'red'">
                    {{ currentTrace.status === 'ok' ? t('menu.statusOk') : t('menu.statusError') }}
                  </a-tag>
                </a-descriptions-item>
                <a-descriptions-item :label="t('menu.duration')">{{ currentTrace.duration_ms }}ms</a-descriptions-item>
                <a-descriptions-item :label="t('menu.startTime')">{{ formatTime(currentTrace.start_time) }}</a-descriptions-item>
              </a-descriptions>

              <a-divider>{{ t('menu.attributes') }}</a-divider>
              <a-table
                :columns="attrColumns"
                :data-source="formatAttributes(currentTrace.attributes)"
                :pagination="false"
                size="small"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'value'">
                    <a-tag v-if="isBoolean(record.value)">{{ String(record.value) }}</a-tag>
                    <span v-else-if="isNumber(record.value)">{{ record.value }}</span>
                    <span v-else>{{ record.value }}</span>
                  </template>
                </template>
              </a-table>

              <template v-if="currentTrace.events && currentTrace.events.length > 0">
                <a-divider>{{ t('menu.events') }}</a-divider>
                <a-timeline>
                  <a-timeline-item v-for="(event, idx) in currentTrace.events" :key="idx" :color="getEventColor(event.name)">
                    <p><strong>{{ event.name }}</strong> @ {{ formatTime(event.timestamp) }}</p>
                    <p v-if="event.attributes" class="event-attrs">
                      <span v-for="(v, k) in event.attributes" :key="k" class="event-attr">
                        {{ k }}: {{ v }}
                      </span>
                    </p>
                  </a-timeline-item>
                </a-timeline>
              </template>

              <template v-if="currentTrace.error_msg">
                <a-divider>{{ t('menu.errorInfo') }}</a-divider>
                <a-alert :message="t('menu.errorMsg')" :description="currentTrace.error_msg" type="error" show-icon />
              </template>
            </div>
            <a-empty v-else :description="t('menu.selectTraceToView')" />
          </a-tab-pane>
        </a-tabs>
      </div>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { CopyOutlined, SearchOutlined } from '@ant-design/icons-vue'
import { traceApi } from '@/services/trace'
import type { TraceRecord } from '@/services/trace'
import dayjs, { type Dayjs } from 'dayjs'

const { t } = useI18n()

const activeTab = ref('list')
const loading = ref(false)
const traces = ref<TraceRecord[]>([])
const services = ref<string[]>([])
const currentTrace = ref<TraceRecord | null>(null)

const selectedService = ref<string | undefined>(undefined)
const traceIdSearch = ref('')
const statusFilter = ref<string | undefined>(undefined)
const timeRange = ref<[Dayjs, Dayjs] | null>(null)

const pagination = ref({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total: number) => `Total ${total} items`,
})

const columns = [
  { title: 'Trace ID', dataIndex: 'trace_id', key: 'trace_id', ellipsis: true, width: 200 },
  { title: t('menu.operation'), key: 'operation', ellipsis: true, width: 200 },
  { title: t('menu.service'), key: 'service', width: 120 },
  { title: t('menu.kind'), dataIndex: 'kind', key: 'kind', width: 80 },
  { title: t('menu.status'), key: 'status', width: 100 },
  { title: t('menu.duration'), key: 'duration', width: 100 },
  { title: t('menu.startTime'), key: 'start_time', width: 180 },
  { title: t('common.action'), key: 'action', width: 120, fixed: 'right' },
]

const attrColumns = [
  { title: t('menu.attrKey'), dataIndex: 'key', key: 'key', width: 200 },
  { title: t('menu.attrValue'), key: 'value' },
]

const loadServices = async () => {
  try {
    const res = await traceApi.listServices()
    services.value = res.data?.data?.services || []
  } catch (e: any) {
    console.error('Failed to load services:', e)
  }
}

const loadTraces = async () => {
  loading.value = true
  try {
    const params: any = {
      limit: pagination.value.pageSize,
      offset: (pagination.value.current - 1) * pagination.value.pageSize,
    }
    if (selectedService.value) {
      params.service = selectedService.value
    }
    if (traceIdSearch.value) {
      params.trace_id = traceIdSearch.value
    }
    if (statusFilter.value) {
      params.status = statusFilter.value
    }
    if (timeRange.value) {
      params.start = timeRange.value[0].unix()
      params.end = timeRange.value[1].unix()
    }

    const res = await traceApi.queryTraces(params)
    const data = res.data?.data
    if (data) {
      traces.value = data.traces || []
      pagination.value.total = data.total || 0
    }
  } catch (e: any) {
    message.error(e.message || t('common.failed'))
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  pagination.value.current = 1
  loadTraces()
}

const handleTableChange = (pag: any) => {
  pagination.value.current = pag.current
  pagination.value.pageSize = pag.pageSize
  loadTraces()
}

const handleViewTrace = async (record: TraceRecord) => {
  try {
    const res = await traceApi.getTrace(record.trace_id)
    if (res.data?.data) {
      currentTrace.value = res.data.data
      activeTab.value = 'detail'
    }
  } catch (e: any) {
    message.error(t('common.failed'))
  }
}

const handleCopyTraceId = (traceId: string) => {
  navigator.clipboard.writeText(traceId)
  message.success(t('common.copied'))
}

const formatTime = (timeStr: string) => {
  if (!timeStr) return '-'
  return dayjs(timeStr).format('YYYY-MM-DD HH:mm:ss.SSS')
}

const getDurationColor = (ms: number) => {
  if (ms < 100) return '#52c41a'
  if (ms < 500) return '#faad14'
  return '#f5222d'
}

const getEventColor = (name: string) => {
  if (name.includes('error')) return 'red'
  if (name.includes('warn')) return 'orange'
  return 'blue'
}

const formatAttributes = (attrs: Record<string, any> | undefined) => {
  if (!attrs) return []
  return Object.entries(attrs).map(([key, value]) => ({ key, value }))
}

const isBoolean = (val: any) => typeof val === 'boolean'
const isNumber = (val: any) => typeof val === 'number' && !isNaN(val)

onMounted(() => {
  loadServices()
  loadTraces()
})
</script>

<style scoped>
.tracing-page {
  padding: 0;
}

.tracing-header {
  margin-bottom: 16px;
  padding: 12px;
  background: #fafafa;
  border-radius: 4px;
}

.tracing-content {
  background: #fff;
  padding: 12px;
  border-radius: 4px;
}

.operation-name {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
}

.trace-detail {
  padding: 16px;
}

.event-attrs {
  margin-top: 4px;
}

.event-attr {
  display: inline-block;
  margin-right: 12px;
  font-size: 12px;
  color: #666;
}
</style>
