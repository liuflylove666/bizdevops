<template>
  <div class="biz-detail-page">
    <div class="page-header">
      <a-button type="text" @click="goBack">
        <ArrowLeftOutlined />
        返回
      </a-button>
      <div class="page-header__content">
        <div class="page-header__title">
          <span>{{ detail?.goal.name || '业务目标详情' }}</span>
          <a-tag v-if="detail?.goal.status" :color="statusColor(detail.goal.status)">
            {{ statusText(detail.goal.status) }}
          </a-tag>
          <a-tag v-if="detail?.goal.priority" :color="priorityColor(detail.goal.priority)">
            {{ priorityText(detail.goal.priority) }}
          </a-tag>
        </div>
        <div class="page-header__desc">{{ detail?.goal.value_metric || detail?.goal.description || '查看目标、版本和需求的联动关系。' }}</div>
      </div>
    </div>

    <a-spin :spinning="loading">
      <a-row :gutter="16">
        <a-col :span="6">
          <a-card>
            <a-statistic title="需求总数" :value="detail?.summary.requirement_total || 0" />
          </a-card>
        </a-col>
        <a-col :span="6">
          <a-card>
            <a-statistic title="推进中需求" :value="detail?.summary.requirement_in_progress || 0" />
          </a-card>
        </a-col>
        <a-col :span="6">
          <a-card>
            <a-statistic title="已完成需求" :value="detail?.summary.requirement_done || 0" />
          </a-card>
        </a-col>
        <a-col :span="6">
          <a-card>
            <a-statistic title="关联版本" :value="detail?.summary.version_total || 0" />
          </a-card>
        </a-col>
      </a-row>

      <a-card title="目标概览" style="margin-top: 16px">
        <a-descriptions :column="2" bordered>
          <a-descriptions-item label="目标名称">{{ detail?.goal.name || '-' }}</a-descriptions-item>
          <a-descriptions-item label="目标编码">{{ detail?.goal.code || '-' }}</a-descriptions-item>
          <a-descriptions-item label="负责人">{{ detail?.goal.owner || '-' }}</a-descriptions-item>
          <a-descriptions-item label="状态">{{ detail?.goal.status ? statusText(detail.goal.status) : '-' }}</a-descriptions-item>
          <a-descriptions-item label="优先级">{{ detail?.goal.priority ? priorityText(detail.goal.priority) : '-' }}</a-descriptions-item>
          <a-descriptions-item label="价值指标">{{ detail?.goal.value_metric || '-' }}</a-descriptions-item>
          <a-descriptions-item label="创建时间">{{ formatTime(detail?.goal.created_at) }}</a-descriptions-item>
          <a-descriptions-item label="更新时间">{{ formatTime(detail?.goal.updated_at) }}</a-descriptions-item>
          <a-descriptions-item label="描述" :span="2">{{ detail?.goal.description || '-' }}</a-descriptions-item>
        </a-descriptions>
      </a-card>

      <a-row :gutter="16" style="margin-top: 16px">
        <a-col :span="12">
          <a-card title="关联版本计划">
            <a-table :columns="versionColumns" :data-source="detail?.versions || []" :pagination="false" row-key="id" size="small">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'name'">
                  <a-button type="link" style="padding: 0" @click="goVersion(record.id)">{{ record.name }}</a-button>
                </template>
                <template v-else-if="column.key === 'status'">
                  <a-tag :color="versionStatusColor(record.status)">{{ versionStatusText(record.status) }}</a-tag>
                </template>
                <template v-else-if="column.key === 'release_date'">
                  {{ formatTime(record.release_date) }}
                </template>
              </template>
            </a-table>
          </a-card>
        </a-col>
        <a-col :span="12">
          <a-card title="关联需求池">
            <template #extra>
              <a-popover trigger="click" placement="bottomRight">
                <template #content>
                  <a-checkbox-group v-model:value="selectedOptionalColumns" :options="optionalColumnOptions" />
                  <div style="margin-top: 8px; text-align: right;">
                    <a-button size="small" @click="resetOptionalColumns">恢复默认</a-button>
                  </div>
                </template>
                <a-button size="small">配置列</a-button>
              </a-popover>
            </template>
            <a-table :columns="requirementColumns" :data-source="detail?.requirements || []" :pagination="false" row-key="id" size="small">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'title'">
                  <a-button type="link" style="padding: 0" @click="goRequirement(record.id)">{{ record.title }}</a-button>
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
                <template v-else-if="column.key === 'status'">
                  <a-tag :color="requirementStatusColor(record.status)">{{ requirementStatusText(record.status) }}</a-tag>
                </template>
                <template v-else-if="column.key === 'priority'">
                  <a-tag :color="priorityColor(record.priority)">{{ priorityText(record.priority) }}</a-tag>
                </template>
              </template>
            </a-table>
          </a-card>
        </a-col>
      </a-row>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeftOutlined } from '@ant-design/icons-vue'
import { bizApi, type BizGoalDetail } from '@/services/biz'
import { resolveDefaultJiraBaseURL } from '@/services/jira'
import { useBizRequirementColumnPrefs } from '@/composables/useBizRequirementColumnPrefs'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const detail = ref<BizGoalDetail | null>(null)
const jiraBaseURL = ref('')
const OPTIONAL_COLUMN_STORAGE_KEY = 'biz.goal_detail.requirements.optional_columns'
const DEFAULT_OPTIONAL_COLUMNS = ['external_key', 'jira_epic_key']
const optionalColumnOptions = [
  { label: '外部单号', value: 'external_key' },
  { label: 'Epic', value: 'jira_epic_key' },
]
const { selectedOptionalColumns, resetOptionalColumns } = useBizRequirementColumnPrefs(
  OPTIONAL_COLUMN_STORAGE_KEY,
  optionalColumnOptions,
  DEFAULT_OPTIONAL_COLUMNS
)

const versionColumns = [
  { title: '版本名称', key: 'name' },
  { title: '关联应用', dataIndex: 'application_name', key: 'application_name', width: 140 },
  { title: '发布单', dataIndex: 'release_title', key: 'release_title', width: 140 },
  { title: '负责人', dataIndex: 'owner', key: 'owner', width: 100 },
  { title: '状态', key: 'status', width: 100 },
  { title: '计划发布时间', key: 'release_date', width: 180 }
]

const requirementColumns = computed(() => {
  const enabled = new Set(selectedOptionalColumns.value)
  return [
    { title: '需求标题', key: 'title' },
    ...(enabled.has('external_key') ? [{ title: '外部单号', key: 'external_key', width: 180 }] : []),
    ...(enabled.has('jira_epic_key') ? [{ title: 'Epic', key: 'jira_epic_key', width: 140 }] : []),
    { title: '关联应用', dataIndex: 'application_name', key: 'application_name', width: 140 },
    { title: '流水线', dataIndex: 'pipeline_name', key: 'pipeline_name', width: 140 },
    { title: '负责人', dataIndex: 'owner', key: 'owner', width: 100 },
    { title: '状态', key: 'status', width: 100 },
    { title: '优先级', key: 'priority', width: 90 },
  ]
})

const statusText = (status: string) => ({ planning: '规划中', in_progress: '推进中', done: '已完成' }[status] || status)
const statusColor = (status: string) => ({ planning: 'blue', in_progress: 'gold', done: 'green' }[status] || 'default')
const versionStatusText = (status: string) => ({ planning: '规划中', in_progress: '进行中', released: '已发布' }[status] || status)
const versionStatusColor = (status: string) => ({ planning: 'blue', in_progress: 'gold', released: 'green' }[status] || 'default')
const requirementStatusText = (status: string) => ({ backlog: '待规划', in_progress: '进行中', done: '已完成' }[status] || status)
const requirementStatusColor = (status: string) => ({ backlog: 'blue', in_progress: 'gold', done: 'green' }[status] || 'default')
const priorityText = (priority: string) => ({ high: '高', medium: '中', low: '低' }[priority] || priority)
const priorityColor = (priority: string) => ({ high: 'red', medium: 'orange', low: 'default' }[priority] || 'default')
const formatTime = (value?: string) => value ? value.replace('T', ' ').slice(0, 19) : '-'
const jiraBrowseUrl = (externalKey?: string) => {
  if (!externalKey || !jiraBaseURL.value) return ''
  return `${jiraBaseURL.value}/browse/${externalKey}`
}

const fetchDetail = async (id: number) => {
  loading.value = true
  try {
    const res = await bizApi.getGoal(id)
    detail.value = res.data
  } finally {
    loading.value = false
  }
}

const initJiraBaseURL = async () => {
  try {
    jiraBaseURL.value = await resolveDefaultJiraBaseURL()
  } catch {
    jiraBaseURL.value = ''
  }
}

const goBack = () => router.push('/biz/goals')
const goRequirement = (id: number) => router.push(`/biz/requirements/${id}`)
const goVersion = (id: number) => router.push(`/biz/versions/${id}`)

watch(
  () => Number(route.params.id),
  (id) => {
    if (id) {
      fetchDetail(id)
    }
  },
  { immediate: true }
)

void initJiraBaseURL()
</script>

<style scoped>
.biz-detail-page {
  padding: 0;
}

.page-header {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 16px;
}

.page-header__content {
  flex: 1;
}

.page-header__title {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 20px;
  font-weight: 600;
  margin-bottom: 8px;
}

.page-header__desc {
  color: #666;
}
</style>
