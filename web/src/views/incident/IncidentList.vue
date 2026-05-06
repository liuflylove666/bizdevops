<template>
  <div class="incident-list">
    <div class="page-header">
      <h1>生产事故</h1>
      <a-space>
        <a-button @click="fetchList">
          <template #icon><ReloadOutlined /></template>
          刷新
        </a-button>
        <a-button type="primary" @click="showCreateModal = true">
          <template #icon><PlusOutlined /></template>
          登记事故
        </a-button>
      </a-space>
    </div>

    <!-- 统计 -->
    <a-row :gutter="12" class="stat-row">
      <a-col :span="6">
        <a-card :bordered="false" class="stat-card">
          <a-statistic title="未解决" :value="stats.open" :value-style="{ color: '#ff4d4f' }" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card :bordered="false" class="stat-card">
          <a-statistic title="已止血" :value="stats.mitigated" :value-style="{ color: '#faad14' }" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card :bordered="false" class="stat-card">
          <a-statistic title="已解决" :value="stats.resolved" :value-style="{ color: '#52c41a' }" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card :bordered="false" class="stat-card">
          <a-statistic
            title="近 7 天 MTTR (分钟)"
            :value="stats.mttr"
            :precision="1"
            :value-style="{ color: '#1677ff' }"
          />
        </a-card>
      </a-col>
    </a-row>

    <!-- 筛选 -->
    <a-card :bordered="false" style="margin-bottom: 16px">
      <a-form layout="inline" @finish="onSearch">
        <a-form-item label="关键词">
          <a-input v-model:value="filter.keyword" placeholder="标题/描述" allow-clear style="width: 200px" />
        </a-form-item>
        <a-form-item label="环境">
          <a-select v-model:value="filter.env" placeholder="全部" allow-clear style="width: 120px">
            <a-select-option value="dev">dev</a-select-option>
            <a-select-option value="test">test</a-select-option>
            <a-select-option value="staging">staging</a-select-option>
            <a-select-option value="prod">prod</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="严重等级">
          <a-select v-model:value="filter.severity" placeholder="全部" allow-clear style="width: 100px">
            <a-select-option value="P0">P0</a-select-option>
            <a-select-option value="P1">P1</a-select-option>
            <a-select-option value="P2">P2</a-select-option>
            <a-select-option value="P3">P3</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="filter.status" placeholder="全部" allow-clear style="width: 120px">
            <a-select-option value="open">未解决</a-select-option>
            <a-select-option value="mitigated">已止血</a-select-option>
            <a-select-option value="resolved">已解决</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit">查询</a-button>
          <a-button style="margin-left: 8px" @click="resetFilter">重置</a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <!-- 列表 -->
    <a-card :bordered="false">
      <a-table
        :columns="columns"
        :data-source="incidents"
        :loading="loading"
        row-key="id"
        :pagination="pagination"
        @change="onTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <a @click="goDetail(record.id)">{{ record.title }}</a>
            <div class="sub-text">#{{ record.id }} · {{ record.application_name || '未指定应用' }} · {{ record.env }}</div>
          </template>

          <template v-else-if="column.key === 'severity'">
            <a-tag :color="severityColor(record.severity)">{{ record.severity }}</a-tag>
          </template>

          <template v-else-if="column.key === 'status'">
            <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
          </template>

          <template v-else-if="column.key === 'source'">
            <a-tag>{{ sourceLabel(record.source) }}</a-tag>
          </template>

          <template v-else-if="column.key === 'duration'">
            <span>{{ formatDuration(record) }}</span>
          </template>

          <template v-else-if="column.key === 'detected_at'">
            {{ formatTime(record.detected_at) }}
          </template>

          <template v-else-if="column.key === 'action'">
            <a-space>
              <a @click="goDetail(record.id)">详情</a>
              <a v-if="record.status === 'open'" @click="onMitigate(record.id)">标记止血</a>
              <a v-if="record.status !== 'resolved'" class="success" @click="openResolveModal(record)">解决</a>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <!-- 创建事故 -->
    <a-modal v-model:open="showCreateModal" title="登记事故" :confirm-loading="creating" @ok="onCreate">
      <a-form layout="vertical">
        <a-form-item label="标题" required>
          <a-input v-model:value="createForm.title" placeholder="简要描述事故，例如：支付接口 5XX 飙升" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="createForm.description" :rows="3" placeholder="现象、影响面、初判原因" />
        </a-form-item>
        <a-row :gutter="12">
          <a-col :span="12">
            <a-form-item label="环境" required>
              <a-select v-model:value="createForm.env">
                <a-select-option value="prod">prod</a-select-option>
                <a-select-option value="staging">staging</a-select-option>
                <a-select-option value="test">test</a-select-option>
                <a-select-option value="dev">dev</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="严重等级" required>
              <a-select v-model:value="createForm.severity">
                <a-select-option value="P0">P0 · 全站宕机</a-select-option>
                <a-select-option value="P1">P1 · 核心功能不可用</a-select-option>
                <a-select-option value="P2">P2 · 部分功能</a-select-option>
                <a-select-option value="P3">P3 · 体验问题</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="应用名">
          <a-input v-model:value="createForm.application_name" placeholder="可选，便于统计" />
        </a-form-item>
        <a-form-item label="关联 Release ID">
          <a-input-number v-model:value="createForm.release_id" :min="0" style="width: 100%" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 解决弹窗 -->
    <a-modal v-model:open="showResolveModal" title="解决事故" :confirm-loading="resolving" @ok="onResolve">
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
 * 生产事故列表（v2.1）。
 *
 * 数据源：
 *   - GET /incidents?env=&status=&severity=&keyword=&page=&pageSize=
 *   - 顶部统计：以当前 filter 为条件做快速 count
 */
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { incidentApi, type Incident, type IncidentFilter } from '@/services/incident'

const router = useRouter()
const route = useRoute()
const projectId = Number(route.query.project_id || 0) || undefined

const loading = ref(false)
const incidents = ref<Incident[]>([])

const filter = reactive<IncidentFilter>({
  env: undefined,
  status: undefined,
  severity: undefined,
  keyword: '',
  project_id: projectId,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const stats = reactive({ open: 0, mitigated: 0, resolved: 0, mttr: 0 })

const columns = [
  { title: '事故', key: 'title', dataIndex: 'title' },
  { title: '严重', key: 'severity', dataIndex: 'severity', width: 90 },
  { title: '状态', key: 'status', dataIndex: 'status', width: 110 },
  { title: '来源', key: 'source', dataIndex: 'source', width: 100 },
  { title: '发现时间', key: 'detected_at', dataIndex: 'detected_at', width: 170 },
  { title: '持续时长', key: 'duration', width: 110 },
  { title: '操作', key: 'action', width: 180 },
]

const showCreateModal = ref(false)
const creating = ref(false)
const createForm = reactive({
  title: '',
  description: '',
  env: 'prod',
  severity: 'P2' as 'P0' | 'P1' | 'P2' | 'P3',
  application_name: '',
  release_id: undefined as number | undefined,
})

const showResolveModal = ref(false)
const resolving = ref(false)
const resolveTargetId = ref<number | null>(null)
const resolveForm = reactive({ root_cause: '', postmortem_url: '' })

async function fetchList() {
  loading.value = true
  try {
    const res = await incidentApi.list({
      ...filter,
      page: pagination.current,
      pageSize: pagination.pageSize,
    })
    const data: any = (res as any)?.data || {}
    incidents.value = (data.list || data.data || []) as Incident[]
    pagination.total = data.total || 0
  } catch (e: any) {
    message.error(e?.response?.data?.message || '加载事故列表失败')
  } finally {
    loading.value = false
  }
}

async function fetchStats() {
  // 复用列表接口：分别查 open / mitigated / resolved 三次；MTTR 来自 DORA 接口
  try {
    const [open, mitigated, resolved] = await Promise.all([
      incidentApi.list({ status: 'open', pageSize: 1, project_id: filter.project_id }),
      incidentApi.list({ status: 'mitigated', pageSize: 1, project_id: filter.project_id }),
      incidentApi.list({ status: 'resolved', pageSize: 1, project_id: filter.project_id }),
    ])
    stats.open = ((open as any)?.data?.total) || 0
    stats.mitigated = ((mitigated as any)?.data?.total) || 0
    stats.resolved = ((resolved as any)?.data?.total) || 0
  } catch (e) {
    // 统计失败不中断列表展示
  }
}

function onSearch() {
  pagination.current = 1
  fetchList()
}
function resetFilter() {
  filter.env = undefined
  filter.status = undefined
  filter.severity = undefined
  filter.keyword = ''
  pagination.current = 1
  fetchList()
}
function onTableChange(pag: any) {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchList()
}
function goDetail(id: number) {
  router.push(`/incidents/${id}`)
}

async function onMitigate(id: number) {
  try {
    await incidentApi.mitigate(id)
    message.success('已标记止血')
    fetchList()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '操作失败')
  }
}

function openResolveModal(row: Incident) {
  resolveTargetId.value = row.id!
  resolveForm.root_cause = row.root_cause || ''
  resolveForm.postmortem_url = row.postmortem_url || ''
  showResolveModal.value = true
}
async function onResolve() {
  if (!resolveTargetId.value) return
  resolving.value = true
  try {
    await incidentApi.resolve(resolveTargetId.value, { ...resolveForm })
    message.success('已解决')
    showResolveModal.value = false
    fetchList()
    fetchStats()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '解决失败')
  } finally {
    resolving.value = false
  }
}

async function onCreate() {
  if (!createForm.title.trim()) {
    message.warning('请填写标题')
    return
  }
  creating.value = true
  try {
    await incidentApi.create({
      title: createForm.title.trim(),
      description: createForm.description,
      env: createForm.env,
      severity: createForm.severity,
      application_name: createForm.application_name || undefined,
      release_id: createForm.release_id,
      source: 'manual',
    })
    message.success('已登记')
    showCreateModal.value = false
    Object.assign(createForm, {
      title: '',
      description: '',
      env: 'prod',
      severity: 'P2',
      application_name: '',
      release_id: undefined,
    })
    fetchList()
    fetchStats()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '登记失败')
  } finally {
    creating.value = false
  }
}

function severityColor(s: string) {
  return s === 'P0' ? 'red' : s === 'P1' ? 'volcano' : s === 'P2' ? 'orange' : 'default'
}
function statusColor(s: string) {
  return s === 'resolved' ? 'green' : s === 'mitigated' ? 'gold' : 'red'
}
function statusLabel(s: string) {
  return s === 'resolved' ? '已解决' : s === 'mitigated' ? '已止血' : '未解决'
}
function sourceLabel(s: string) {
  if (s === 'alert') return '告警触发'
  if (s === 'release_failure') return '发布失败'
  return '手工登记'
}
function formatTime(t?: string) {
  return t ? dayjs(t).format('MM-DD HH:mm') : '-'
}
function formatDuration(row: Incident) {
  if (!row.detected_at) return '-'
  const end = row.resolved_at ? dayjs(row.resolved_at) : dayjs()
  const mins = end.diff(dayjs(row.detected_at), 'minute')
  if (mins < 60) return mins + ' 分钟'
  if (mins < 24 * 60) return (mins / 60).toFixed(1) + ' 小时'
  return (mins / 60 / 24).toFixed(1) + ' 天'
}

onMounted(() => {
  fetchList()
  fetchStats()
})
</script>

<style scoped>
.incident-list {
  padding: 0;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.page-header h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}
.stat-row {
  margin-bottom: 16px;
}
.stat-card {
  text-align: center;
}
.sub-text {
  color: #999;
  font-size: 12px;
  margin-top: 2px;
}
.success {
  color: #52c41a;
}
</style>
