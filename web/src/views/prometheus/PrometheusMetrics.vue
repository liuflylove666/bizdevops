<template>
  <div class="prometheus-page">
    <a-tabs v-model:activeKey="activeTab" @change="handleTabChange">
      <!-- 指标查询 -->
      <a-tab-pane key="explorer" :tab="t('prometheus.explorer')">
        <div style="margin-bottom: 16px; display: flex; gap: 12px; flex-wrap: wrap; align-items: flex-end">
          <div style="flex: 1; min-width: 300px">
            <div style="margin-bottom: 4px; font-weight: 500">PromQL</div>
            <a-input v-model:value="promQuery" :placeholder="t('prometheus.queryPlaceholder')" @press-enter="runQuery" />
          </div>
          <a-select v-model:value="selectedInstance" :placeholder="t('prometheus.selectInstance')" allow-clear style="width: 180px">
            <a-select-option v-for="inst in instances" :key="inst.id" :value="inst.id">{{ inst.name }}</a-select-option>
          </a-select>
          <a-radio-group v-model:value="queryMode" button-style="solid">
            <a-radio-button value="instant">Instant</a-radio-button>
            <a-radio-button value="range">Range</a-radio-button>
          </a-radio-group>
          <template v-if="queryMode === 'range'">
            <a-select v-model:value="timeRange" style="width: 120px">
              <a-select-option value="15m">15 min</a-select-option>
              <a-select-option value="1h">1 hour</a-select-option>
              <a-select-option value="3h">3 hours</a-select-option>
              <a-select-option value="6h">6 hours</a-select-option>
              <a-select-option value="12h">12 hours</a-select-option>
              <a-select-option value="24h">24 hours</a-select-option>
              <a-select-option value="7d">7 days</a-select-option>
            </a-select>
            <a-input v-model:value="stepInput" placeholder="Step (e.g. 60s)" style="width: 120px" />
          </template>
          <a-button type="primary" :loading="querying" @click="runQuery">{{ t('prometheus.execute') }}</a-button>
        </div>

        <a-spin :spinning="querying">
          <!-- Instant 结果表格 -->
          <div v-if="queryMode === 'instant' && instantResults.length > 0">
            <a-table :columns="instantColumns" :data-source="instantResults" row-key="_key" :pagination="false" size="small" />
          </div>
          <!-- Range 结果表格 (简化) -->
          <div v-if="queryMode === 'range' && rangeResults.length > 0">
            <div v-for="(series, idx) in rangeResults" :key="idx" style="margin-bottom: 16px">
              <a-tag color="blue">{{ formatMetric(series.metric) }}</a-tag>
              <a-table :columns="rangeColumns" :data-source="formatValues(series.values)" row-key="time" :pagination="{ pageSize: 20, size: 'small' }" size="small" style="margin-top: 8px" />
            </div>
          </div>
          <a-empty v-if="!querying && queryExecuted && instantResults.length === 0 && rangeResults.length === 0" />
        </a-spin>
      </a-tab-pane>

      <!-- 目标 Targets -->
      <a-tab-pane key="targets" :tab="t('prometheus.targets')">
        <div style="margin-bottom: 16px">
          <a-select v-model:value="selectedInstance" :placeholder="t('prometheus.selectInstance')" allow-clear style="width: 200px" @change="loadTargets">
            <a-select-option v-for="inst in instances" :key="inst.id" :value="inst.id">{{ inst.name }}</a-select-option>
          </a-select>
        </div>
        <a-spin :spinning="loadingTargets">
          <div v-if="targetsData && targetsData.activeTargets">
            <a-table :columns="targetColumns" :data-source="targetsData.activeTargets" row-key="scrapeUrl" :pagination="{ pageSize: 20 }" size="small">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'health'">
                  <a-tag :color="record.health === 'up' ? 'green' : 'red'">{{ record.health }}</a-tag>
                </template>
                <template v-if="column.key === 'labels'">
                  <a-tag v-for="(v, k) in record.labels" :key="k" style="margin: 2px">{{ k }}={{ v }}</a-tag>
                </template>
              </template>
            </a-table>
          </div>
          <a-empty v-else-if="!loadingTargets" />
        </a-spin>
      </a-tab-pane>

      <!-- 数据源管理 -->
      <a-tab-pane key="instances" :tab="t('prometheus.datasources')">
        <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
          <a-button type="primary" @click="openInstModal()">{{ t('common.create') }}</a-button>
        </div>
        <a-table :columns="instColumns" :data-source="instances" :loading="loadingInst" row-key="id" :pagination="false">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status }}</a-tag>
            </template>
            <template v-if="column.key === 'is_default'">
              <a-tag v-if="record.is_default" color="blue">{{ t('prometheus.default') }}</a-tag>
            </template>
            <template v-if="column.key === 'auth_type'">
              <a-tag>{{ record.auth_type }}</a-tag>
            </template>
            <template v-if="column.key === 'action'">
              <a-space>
                <a @click="handleTestConnection(record)">{{ t('prometheus.testConn') }}</a>
                <a @click="openInstModal(record)">{{ t('common.edit') }}</a>
                <a-popconfirm :title="t('common.confirmDelete')" @confirm="handleDeleteInstance(record.id)">
                  <a style="color: #ff4d4f">{{ t('common.delete') }}</a>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>

    <!-- Instance Modal -->
    <a-modal v-model:open="instModalVisible" :title="instForm.id ? t('prometheus.editDatasource') : t('prometheus.createDatasource')" @ok="handleSaveInstance" :confirm-loading="saving">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item :label="t('prometheus.instName')">
          <a-input v-model:value="instForm.name" />
        </a-form-item>
        <a-form-item label="URL">
          <a-input v-model:value="instForm.url" placeholder="http://prometheus:9090" />
        </a-form-item>
        <a-form-item :label="t('prometheus.authType')">
          <a-select v-model:value="instForm.auth_type">
            <a-select-option value="none">None</a-select-option>
            <a-select-option value="basic">Basic Auth</a-select-option>
            <a-select-option value="bearer">Bearer Token</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="instForm.auth_type === 'basic'" :label="t('prometheus.username')">
          <a-input v-model:value="instForm.username" />
        </a-form-item>
        <a-form-item v-if="instForm.auth_type !== 'none'" :label="instForm.auth_type === 'bearer' ? 'Token' : t('prometheus.password')">
          <a-input-password v-model:value="instForm.password" :placeholder="instForm.id ? t('prometheus.pwdPlaceholder') : ''" />
        </a-form-item>
        <a-form-item :label="t('prometheus.default')">
          <a-switch v-model:checked="instForm.is_default" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { prometheusApi } from '@/services/prometheus'
import type { PrometheusInstance } from '@/services/prometheus'

const { t } = useI18n()

const activeTab = ref('explorer')
const instances = ref<PrometheusInstance[]>([])
const loadingInst = ref(false)
const selectedInstance = ref<number | undefined>(undefined)

// --- Query Explorer ---
const promQuery = ref('')
const queryMode = ref<'instant' | 'range'>('instant')
const timeRange = ref('1h')
const stepInput = ref('60s')
const querying = ref(false)
const queryExecuted = ref(false)
const instantResults = ref<any[]>([])
const rangeResults = ref<any[]>([])

const instantColumns = [
  { title: 'Metric', dataIndex: 'metric', key: 'metric', ellipsis: true },
  { title: 'Value', dataIndex: 'value', key: 'value', width: 200 },
  { title: 'Timestamp', dataIndex: 'timestamp', key: 'timestamp', width: 200 }
]

const rangeColumns = [
  { title: 'Time', dataIndex: 'time', key: 'time', width: 200 },
  { title: 'Value', dataIndex: 'value', key: 'value' }
]

const parseTimeRange = () => {
  const now = Math.floor(Date.now() / 1000)
  const map: Record<string, number> = {
    '15m': 900, '1h': 3600, '3h': 10800, '6h': 21600,
    '12h': 43200, '24h': 86400, '7d': 604800
  }
  const offset = map[timeRange.value] || 3600
  return { start: String(now - offset), end: String(now) }
}

const runQuery = async () => {
  if (!promQuery.value) return
  querying.value = true
  queryExecuted.value = true
  instantResults.value = []
  rangeResults.value = []

  try {
    if (queryMode.value === 'instant') {
      const res = await prometheusApi.query({ query: promQuery.value, instance_id: selectedInstance.value })
      const data = res.data?.data
      if (data?.resultType === 'vector' && data.result) {
        instantResults.value = data.result.map((r: any, i: number) => ({
          _key: i,
          metric: formatMetric(r.metric),
          value: r.value?.[1] ?? '-',
          timestamp: r.value?.[0] ? new Date(r.value[0] * 1000).toLocaleString() : '-'
        }))
      } else if (data?.resultType === 'scalar') {
        instantResults.value = [{ _key: 0, metric: 'scalar', value: data.result?.[1], timestamp: new Date(data.result?.[0] * 1000).toLocaleString() }]
      }
    } else {
      const { start, end } = parseTimeRange()
      const res = await prometheusApi.queryRange({
        query: promQuery.value, start, end, step: stepInput.value || '60s', instance_id: selectedInstance.value
      })
      const data = res.data?.data
      if (data?.resultType === 'matrix' && data.result) {
        rangeResults.value = data.result
      }
    }
  } catch (e: any) {
    message.error(e.message || t('common.failed'))
  } finally {
    querying.value = false
  }
}

const formatMetric = (metric: Record<string, string>) => {
  if (!metric || Object.keys(metric).length === 0) return '{}'
  const name = metric.__name__ || ''
  const labels = Object.entries(metric)
    .filter(([k]) => k !== '__name__')
    .map(([k, v]) => `${k}="${v}"`)
    .join(', ')
  return name ? `${name}{${labels}}` : `{${labels}}`
}

const formatValues = (values: [number, string][]) => {
  return (values || []).map(([ts, val]) => ({
    time: new Date(ts * 1000).toLocaleString(),
    value: val
  }))
}

// --- Targets ---
const targetsData = ref<any>(null)
const loadingTargets = ref(false)

const targetColumns = [
  { title: 'Endpoint', dataIndex: 'scrapeUrl', key: 'scrapeUrl', ellipsis: true },
  { title: 'Health', key: 'health', width: 80 },
  { title: 'Labels', key: 'labels' },
  { title: 'Last Scrape', dataIndex: 'lastScrape', key: 'lastScrape', width: 200 }
]

const loadTargets = async () => {
  loadingTargets.value = true
  try {
    const res = await prometheusApi.targets({ instance_id: selectedInstance.value })
    targetsData.value = res.data?.data || null
  } catch (e: any) {
    message.error(e.message || t('common.failed'))
  } finally {
    loadingTargets.value = false
  }
}

// --- Instance CRUD ---
const instModalVisible = ref(false)
const instForm = ref<Partial<PrometheusInstance>>({})
const saving = ref(false)

const instColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('prometheus.instName'), dataIndex: 'name', key: 'name' },
  { title: 'URL', dataIndex: 'url', key: 'url' },
  { title: t('prometheus.authType'), key: 'auth_type', width: 100 },
  { title: t('prometheus.status'), dataIndex: 'status', key: 'status', width: 80 },
  { title: t('prometheus.default'), key: 'is_default', width: 80 },
  { title: t('common.action'), key: 'action', width: 200 }
]

const loadInstances = async () => {
  loadingInst.value = true
  try {
    const res = await prometheusApi.listInstances()
    instances.value = res.data?.data || []
  } finally {
    loadingInst.value = false
  }
}

const openInstModal = (record?: PrometheusInstance) => {
  instForm.value = record
    ? { ...record, password: '' }
    : { name: '', url: '', auth_type: 'none', username: '', password: '', is_default: false }
  instModalVisible.value = true
}

const handleSaveInstance = async () => {
  saving.value = true
  try {
    if (instForm.value.id) {
      await prometheusApi.updateInstance(instForm.value.id, instForm.value)
    } else {
      await prometheusApi.createInstance(instForm.value)
    }
    message.success(t('common.success'))
    instModalVisible.value = false
    loadInstances()
  } catch (e: any) {
    message.error(e.message || t('common.failed'))
  } finally {
    saving.value = false
  }
}

const handleDeleteInstance = async (id: number) => {
  await prometheusApi.deleteInstance(id)
  message.success(t('common.success'))
  loadInstances()
}

const handleTestConnection = async (record: PrometheusInstance) => {
  try {
    await prometheusApi.testConnection(record.id!)
    message.success(t('prometheus.connSuccess'))
  } catch {
    message.error(t('prometheus.connFailed'))
  }
}

const handleTabChange = (key: string) => {
  if (key === 'instances') loadInstances()
  else if (key === 'targets') { loadInstances(); loadTargets() }
  else if (key === 'explorer') loadInstances()
}

onMounted(() => {
  loadInstances()
})
</script>

<style scoped>
.prometheus-page {
  padding: 0;
}
</style>
