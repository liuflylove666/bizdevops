<template>
  <div class="biz-page">
    <a-card :bordered="false" class="search-card">
      <a-form layout="inline">
        <a-form-item :label="t('common.keyword')">
          <a-input v-model:value="filters.keyword" placeholder="搜索需求标题" allow-clear style="width: 220px" />
        </a-form-item>
        <a-form-item :label="t('common.status')">
          <a-select v-model:value="filters.status" allow-clear style="width: 140px">
            <a-select-option value="backlog">待规划</a-select-option>
            <a-select-option value="in_progress">进行中</a-select-option>
            <a-select-option value="done">已完成</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="优先级">
          <a-select v-model:value="filters.priority" allow-clear style="width: 140px">
            <a-select-option value="high">高</a-select-option>
            <a-select-option value="medium">中</a-select-option>
            <a-select-option value="low">低</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="来源">
          <a-select v-model:value="filters.source" allow-clear style="width: 130px">
            <a-select-option value="jira">Jira</a-select-option>
            <a-select-option value="manual">手工录入</a-select-option>
            <a-select-option value="sales">销售反馈</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="Epic">
          <a-input v-model:value="filters.jiraEpicKey" placeholder="如 DEV-EPIC-1" allow-clear style="width: 170px" />
        </a-form-item>
        <a-form-item label="标签">
          <a-input v-model:value="filters.jiraLabel" placeholder="如 backend" allow-clear style="width: 150px" />
        </a-form-item>
        <a-form-item label="组件">
          <a-input v-model:value="filters.jiraComponent" placeholder="如 release" allow-clear style="width: 150px" />
        </a-form-item>
        <a-form-item label="列显示">
          <a-popover trigger="click" placement="bottomLeft">
            <template #content>
              <a-checkbox-group v-model:value="selectedOptionalColumns" :options="optionalColumnOptions" />
              <div style="margin-top: 8px; text-align: right;">
                <a-button size="small" @click="resetOptionalColumns">恢复默认</a-button>
              </div>
            </template>
            <a-button>配置列</a-button>
          </a-popover>
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" @click="handleSearch">查询</a-button>
            <a-button @click="handleReset">重置</a-button>
          </a-space>
        </a-form-item>
        <a-form-item style="margin-left: auto">
          <a-space>
            <a-button v-if="isJiraSource" @click="openJiraPullModal">Jira 拉取视图</a-button>
            <a-button type="primary" :disabled="isJiraSource" @click="openCreate">
            <template #icon><PlusOutlined /></template>
            新建需求
            </a-button>
          </a-space>
        </a-form-item>
      </a-form>
      <a-alert
        v-if="isJiraSource"
        style="margin-top: 12px"
        type="info"
        show-icon
        message="规划已切换为 Jira 权威源，本页为只读视图。可在 Jira 拉取视图中检索需求，编辑请前往 Jira。"
      />
    </a-card>

    <a-card :bordered="false" style="margin-top: 16px">
      <a-table
        :columns="columns"
        :data-source="requirements"
        :loading="loading"
        :pagination="paginationConfig"
        row-key="id"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <a-button type="link" style="padding: 0" @click="goDetail(record.id)">{{ record.title }}</a-button>
          </template>
          <template v-else-if="column.key === 'status'">
            <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'priority'">
            <a-tag :color="priorityColor(record.priority)">{{ priorityText(record.priority) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'goal_name'">
            {{ goalName(record.goal_id) }}
          </template>
          <template v-else-if="column.key === 'version_name'">
            {{ versionName(record.version_id) }}
          </template>
          <template v-else-if="column.key === 'application_name'">
            {{ record.application_name || applicationName(record.application_id) }}
          </template>
          <template v-else-if="column.key === 'pipeline_name'">
            {{ record.pipeline_name || pipelineName(record.pipeline_id) }}
          </template>
          <template v-else-if="column.key === 'external_key'">
            <a-space v-if="record.external_key">
              <span>{{ record.external_key }}</span>
              <a v-if="jiraBrowseUrl(record.external_key)" :href="jiraBrowseUrl(record.external_key)" target="_blank">打开 Jira</a>
            </a-space>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.key === 'jira_epic_key'">
            {{ record.jira_epic_key || '-' }}
          </template>
          <template v-else-if="column.key === 'jira_labels'">
            <a-space wrap>
              <a-tag v-for="label in splitCsv(record.jira_labels)" :key="`label-${record.id}-${label}`">{{ label }}</a-tag>
              <span v-if="splitCsv(record.jira_labels).length === 0">-</span>
            </a-space>
          </template>
          <template v-else-if="column.key === 'jira_components'">
            <a-space wrap>
              <a-tag color="purple" v-for="comp in splitCsv(record.jira_components)" :key="`comp-${record.id}-${comp}`">{{ comp }}</a-tag>
              <span v-if="splitCsv(record.jira_components).length === 0">-</span>
            </a-space>
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button type="link" size="small" @click="goDetail(record.id)">详情</a-button>
              <a-button type="link" size="small" :disabled="isJiraSource" @click="openEdit(record)">编辑</a-button>
              <a-button type="link" size="small" danger :disabled="isJiraSource" @click="handleDelete(record)">删除</a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal v-model:open="modalVisible" :title="editingId ? '编辑需求' : '新建需求'" :confirm-loading="submitting" @ok="handleSubmit">
      <a-form layout="vertical">
        <a-form-item label="需求标题" required>
          <a-input v-model:value="form.title" placeholder="请输入需求标题" />
        </a-form-item>
        <a-row :gutter="12">
          <a-col :span="12">
            <a-form-item label="关联目标">
              <a-select v-model:value="form.goal_id" allow-clear placeholder="选择业务目标">
                <a-select-option v-for="goal in goalOptions" :key="goal.id" :value="goal.id">{{ goal.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="关联版本">
              <a-select v-model:value="form.version_id" allow-clear placeholder="选择版本计划">
                <a-select-option v-for="version in versionOptions" :key="version.id" :value="version.id">{{ version.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="12">
          <a-col :span="8">
            <a-form-item label="来源">
              <a-select v-model:value="form.source">
                <a-select-option value="manual">手工录入</a-select-option>
                <a-select-option value="jira">Jira</a-select-option>
                <a-select-option value="sales">销售反馈</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="状态">
              <a-select v-model:value="form.status">
                <a-select-option value="backlog">待规划</a-select-option>
                <a-select-option value="in_progress">进行中</a-select-option>
                <a-select-option value="done">已完成</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="优先级">
              <a-select v-model:value="form.priority">
                <a-select-option value="high">高</a-select-option>
                <a-select-option value="medium">中</a-select-option>
                <a-select-option value="low">低</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="关联应用">
              <a-select v-model:value="form.application_id" allow-clear placeholder="选择应用">
                <a-select-option v-for="app in applicationOptions" :key="app.id" :value="app.id">
                  {{ app.display_name || app.name }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="关联流水线">
              <a-select v-model:value="form.pipeline_id" allow-clear placeholder="选择流水线">
                <a-select-option v-for="pipeline in pipelineOptions" :key="pipeline.id" :value="pipeline.id">
                  {{ pipeline.name }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="负责人">
          <a-input v-model:value="form.owner" placeholder="请输入负责人" />
        </a-form-item>
        <a-form-item label="价值分">
          <a-input-number v-model:value="form.value_score" :min="0" :max="100" style="width: 100%" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.description" :rows="4" placeholder="请输入需求描述" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="jiraPullVisible"
      title="Jira 拉取视图"
      width="1100px"
      :footer="null"
      @cancel="jiraIssues = []"
    >
      <a-space style="margin-bottom: 12px">
        <a-input
          v-model:value="jiraJql"
          style="width: 680px"
          placeholder="输入 JQL（默认按最近更新时间倒序）"
          @pressEnter="loadJiraIssues"
        />
        <a-button type="primary" :loading="jiraLoading" @click="loadJiraIssues">查询</a-button>
      </a-space>
      <a-table
        size="small"
        row-key="id"
        :loading="jiraLoading"
        :data-source="jiraIssues"
        :pagination="false"
        :columns="jiraColumns"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'key'">
            <a :href="jiraIssueUrl(record.key)" target="_blank">{{ record.key }}</a>
          </template>
          <template v-else-if="column.key === 'status'">
            <a-tag>{{ record.fields?.status?.name || '-' }}</a-tag>
          </template>
          <template v-else-if="column.key === 'assignee'">
            {{ record.fields?.assignee?.displayName || '未分配' }}
          </template>
        </template>
      </a-table>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message, Modal } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import type { TablePaginationConfig } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { bizApi, type BizGoal, type BizRequirement, type BizVersion } from '@/services/biz'
import { applicationApi, type Application } from '@/services/application'
import { pipelineApi } from '@/services/pipeline'
import { jiraApi, pickDefaultJiraInstance, resolveDefaultJiraBaseURL } from '@/services/jira'
import { useBizRequirementColumnPrefs } from '@/composables/useBizRequirementColumnPrefs'

const { t } = useI18n()
const router = useRouter()
const loading = ref(false)
const submitting = ref(false)
const modalVisible = ref(false)
const editingId = ref<number | null>(null)
const requirements = ref<BizRequirement[]>([])
const goalOptions = ref<BizGoal[]>([])
const versionOptions = ref<BizVersion[]>([])
const applicationOptions = ref<Application[]>([])
const pipelineOptions = ref<Array<{ id: number; name: string }>>([])
const planningSource = ref<'manual' | 'jira'>('manual')
const jiraPullVisible = ref(false)
const jiraLoading = ref(false)
const jiraJql = ref('project is not EMPTY ORDER BY updated DESC')
const jiraIssues = ref<any[]>([])
const jiraInstanceId = ref<number | null>(null)
const jiraBaseURL = ref('')
const OPTIONAL_COLUMN_STORAGE_KEY = 'biz.requirements.optional_columns'
const DEFAULT_OPTIONAL_COLUMNS = ['external_key', 'jira_epic_key']

const filters = reactive({
  keyword: '',
  status: undefined as string | undefined,
  priority: undefined as string | undefined,
  source: undefined as string | undefined,
  jiraEpicKey: '',
  jiraLabel: '',
  jiraComponent: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0
})

const paginationConfig = computed<TablePaginationConfig>(() => ({
  current: pagination.current,
  pageSize: pagination.pageSize,
  total: pagination.total,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`
}))
const isJiraSource = computed(() => planningSource.value === 'jira')
const optionalColumnOptions = [
  { label: '外部单号', value: 'external_key' },
  { label: 'Epic', value: 'jira_epic_key' },
  { label: '标签', value: 'jira_labels' },
  { label: '组件', value: 'jira_components' },
]
const { selectedOptionalColumns, resetOptionalColumns } = useBizRequirementColumnPrefs(
  OPTIONAL_COLUMN_STORAGE_KEY,
  optionalColumnOptions,
  DEFAULT_OPTIONAL_COLUMNS
)

const form = reactive<Partial<BizRequirement>>({
  title: '',
  goal_id: undefined,
  version_id: undefined,
  application_id: undefined,
  pipeline_id: undefined,
  source: 'manual',
  owner: '',
  priority: 'medium',
  status: 'backlog',
  value_score: 0,
  description: ''
})

const leadingColumns = [
  { title: '需求标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '业务目标', key: 'goal_name', width: 160 },
  { title: '版本计划', key: 'version_name', width: 160 },
  { title: '关联应用', key: 'application_name', width: 160 },
  { title: '关联流水线', key: 'pipeline_name', width: 160 },
]
const optionalColumns = [
  { title: '外部单号', key: 'external_key', width: 180 },
  { title: 'Epic', key: 'jira_epic_key', width: 140 },
  { title: '标签', key: 'jira_labels', width: 200 },
  { title: '组件', key: 'jira_components', width: 180 },
]
const trailingColumns = [
  { title: '状态', key: 'status', width: 100 },
  { title: '优先级', key: 'priority', width: 100 },
  { title: '负责人', dataIndex: 'owner', key: 'owner', width: 120 },
  { title: '价值分', dataIndex: 'value_score', key: 'value_score', width: 90 },
  { title: '操作', key: 'action', width: 120 },
]
const columns = computed(() => {
  const enabled = new Set(selectedOptionalColumns.value)
  return [
    ...leadingColumns,
    ...optionalColumns.filter(col => enabled.has(col.key)),
    ...trailingColumns,
  ]
})
const jiraColumns = [
  { title: 'Key', key: 'key', dataIndex: 'key', width: 120 },
  { title: '标题', key: 'summary', dataIndex: ['fields', 'summary'], ellipsis: true },
  { title: '状态', key: 'status', width: 140 },
  { title: '经办人', key: 'assignee', width: 160 },
]

const statusText = (status: string) => ({ backlog: '待规划', in_progress: '进行中', done: '已完成' }[status] || status)
const statusColor = (status: string) => ({ backlog: 'blue', in_progress: 'gold', done: 'green' }[status] || 'default')
const priorityText = (priority: string) => ({ high: '高', medium: '中', low: '低' }[priority] || priority)
const priorityColor = (priority: string) => ({ high: 'red', medium: 'orange', low: 'default' }[priority] || 'default')

const goalName = (goalId?: number) => goalOptions.value.find(item => item.id === goalId)?.name || '-'
const versionName = (versionId?: number) => versionOptions.value.find(item => item.id === versionId)?.name || '-'
const applicationName = (applicationId?: number) => applicationOptions.value.find(item => item.id === applicationId)?.display_name || applicationOptions.value.find(item => item.id === applicationId)?.name || '-'
const pipelineName = (pipelineId?: number) => pipelineOptions.value.find(item => item.id === pipelineId)?.name || '-'
const splitCsv = (value?: string) => (value || '').split(',').map(item => item.trim()).filter(Boolean)
const jiraBrowseUrl = (externalKey?: string) => {
  if (!externalKey || !jiraBaseURL.value) return ''
  return `${jiraBaseURL.value}/browse/${externalKey}`
}

const resetForm = () => {
  editingId.value = null
  Object.assign(form, {
    title: '',
    goal_id: undefined,
    version_id: undefined,
    application_id: undefined,
    pipeline_id: undefined,
    source: 'manual',
    owner: '',
    priority: 'medium',
    status: 'backlog',
    value_score: 0,
    description: ''
  })
}

const fetchOptions = async () => {
  const [goalRes, versionRes, appRes, pipelineRes] = await Promise.all([
    bizApi.getGoals({ page: 1, page_size: 1000 }),
    bizApi.getVersions({ page: 1, page_size: 1000 }),
    applicationApi.list({ page: 1, page_size: 1000 }),
    pipelineApi.list({ page: 1, page_size: 1000 })
  ])
  goalOptions.value = goalRes.data.list || []
  versionOptions.value = versionRes.data.list || []
  applicationOptions.value = appRes.data?.list || []
  pipelineOptions.value = pipelineRes.data.list || []
}

const fetchRequirements = async () => {
  loading.value = true
  try {
    const res = await bizApi.getRequirements({
      page: pagination.current,
      page_size: pagination.pageSize,
      status: filters.status,
      priority: filters.priority,
      source: filters.source,
      jira_epic_key: filters.jiraEpicKey || undefined,
      jira_label: filters.jiraLabel || undefined,
      jira_component: filters.jiraComponent || undefined,
      keyword: filters.keyword || undefined
    })
    requirements.value = res.data.list || []
    pagination.total = res.data.total || 0
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  pagination.current = 1
  fetchRequirements()
}

const handleReset = () => {
  filters.keyword = ''
  filters.status = undefined
  filters.priority = undefined
  filters.source = undefined
  filters.jiraEpicKey = ''
  filters.jiraLabel = ''
  filters.jiraComponent = ''
  handleSearch()
}

const handleTableChange = (pager: TablePaginationConfig) => {
  pagination.current = pager.current || 1
  pagination.pageSize = pager.pageSize || 10
  fetchRequirements()
}

const openCreate = () => {
  if (isJiraSource.value) {
    message.warning('Jira 权威源模式下不可在本页新建，请前往 Jira。')
    return
  }
  resetForm()
  modalVisible.value = true
}

const openEdit = (record: BizRequirement) => {
  if (isJiraSource.value) {
    message.warning('Jira 权威源模式下不可在本页编辑，请前往 Jira。')
    return
  }
  editingId.value = record.id
  Object.assign(form, record)
  modalVisible.value = true
}

const goDetail = (id: number) => {
  router.push(`/biz/requirements/${id}`)
}

const handleSubmit = async () => {
  if (isJiraSource.value) {
    message.warning('Jira 权威源模式下不可写入，请前往 Jira 编辑。')
    return
  }
  if (!form.title?.trim()) {
    message.error('请输入需求标题')
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await bizApi.updateRequirement(editingId.value, form)
      message.success('更新成功')
    } else {
      await bizApi.createRequirement(form)
      message.success('创建成功')
    }
    modalVisible.value = false
    fetchRequirements()
  } finally {
    submitting.value = false
  }
}

const handleDelete = (record: BizRequirement) => {
  if (isJiraSource.value) {
    message.warning('Jira 权威源模式下不可删除，请前往 Jira 编辑。')
    return
  }
  Modal.confirm({
    title: '确认删除',
    content: `确定删除需求“${record.title}”吗？`,
    onOk: async () => {
      await bizApi.deleteRequirement(record.id)
      message.success('删除成功')
      fetchRequirements()
    }
  })
}

const resolveJiraDefaultInstance = async () => {
  jiraBaseURL.value = await resolveDefaultJiraBaseURL()
  const res = await jiraApi.listInstances()
  const defaultOne = pickDefaultJiraInstance(res.data || [])
  jiraInstanceId.value = defaultOne?.id ?? null
}

const openJiraPullModal = async () => {
  jiraPullVisible.value = true
  if (!jiraInstanceId.value) {
    try {
      await resolveJiraDefaultInstance()
    } catch {
      message.error('未找到可用 Jira 实例，请先在 Jira 集成页面配置实例')
      return
    }
  }
  void loadJiraIssues()
}

const loadJiraIssues = async () => {
  if (!jiraInstanceId.value) {
    message.warning('未找到可用 Jira 实例')
    return
  }
  jiraLoading.value = true
  try {
    const res = await jiraApi.searchIssues(jiraInstanceId.value, {
      jql: jiraJql.value || 'project is not EMPTY ORDER BY updated DESC',
      max_results: 50,
    })
    jiraIssues.value = res.data?.issues || []
  } catch (e: any) {
    message.error(e?.response?.data?.message || '拉取 Jira 需求失败')
  } finally {
    jiraLoading.value = false
  }
}

const jiraIssueUrl = (key: string) => {
  if (jiraBaseURL.value) return `${jiraBaseURL.value}/browse/${key}`
  return '#'
}

onMounted(async () => {
  try {
    const sourceRes = await bizApi.getPlanningSource()
    planningSource.value = sourceRes.data?.source === 'jira' ? 'jira' : 'manual'
  } catch {
    planningSource.value = 'manual'
  }
  await fetchOptions()
  if (isJiraSource.value) {
    await resolveJiraDefaultInstance()
  }
  await fetchRequirements()
})
</script>

<style scoped>
.biz-page {
  padding: 0;
}
</style>
