<template>
  <div class="biz-detail-page">
    <div class="page-header">
      <a-button type="text" @click="goBack">
        <ArrowLeftOutlined />
        返回
      </a-button>
      <div class="page-header__content">
        <div class="page-header__title">
          <span>{{ detail?.version.name || '版本详情' }}</span>
          <a-tag v-if="detail?.version.status" :color="statusColor(detail.version.status)">
            {{ statusText(detail.version.status) }}
          </a-tag>
        </div>
        <div class="page-header__desc">{{ detail?.version.description || '查看版本范围、承接目标和需求装配情况。' }}</div>
      </div>
    </div>

    <a-spin :spinning="loading">
      <a-row :gutter="16">
        <a-col :span="8">
          <a-card>
            <a-statistic title="关联需求" :value="detail?.summary.requirement_total || 0" />
          </a-card>
        </a-col>
        <a-col :span="8">
          <a-card>
            <a-statistic title="进行中需求" :value="detail?.summary.requirement_in_progress || 0" />
          </a-card>
        </a-col>
        <a-col :span="8">
          <a-card>
            <a-statistic title="已完成需求" :value="detail?.summary.requirement_done || 0" />
          </a-card>
        </a-col>
      </a-row>

      <a-card title="版本概览" style="margin-top: 16px">
        <a-descriptions :column="2" bordered>
          <a-descriptions-item label="版本名称">{{ detail?.version.name || '-' }}</a-descriptions-item>
          <a-descriptions-item label="版本编码">{{ detail?.version.code || '-' }}</a-descriptions-item>
          <a-descriptions-item label="负责人">{{ detail?.version.owner || '-' }}</a-descriptions-item>
          <a-descriptions-item label="状态">{{ detail?.version.status ? statusText(detail.version.status) : '-' }}</a-descriptions-item>
          <a-descriptions-item label="关联目标">
            <a-button v-if="detail?.goal" type="link" style="padding: 0" @click="goGoal(detail.goal.id)">{{ detail.goal.name }}</a-button>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="关联应用">
            <a-button v-if="detail?.application" type="link" style="padding: 0" @click="goApplication(detail.application.id)">
              {{ detail.application.display_name || detail.application.name }}
            </a-button>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="关联流水线">
            <a-button v-if="detail?.pipeline" type="link" style="padding: 0" @click="goPipeline(detail.pipeline.id)">
              {{ detail.pipeline.name }}
            </a-button>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="关联发布单">
            <span v-if="detail?.release">{{ detail.release.title }}</span>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="计划发布时间">{{ formatTime(detail?.version.release_date) }}</a-descriptions-item>
          <a-descriptions-item label="开始时间">{{ formatTime(detail?.version.start_date) }}</a-descriptions-item>
          <a-descriptions-item label="更新时间">{{ formatTime(detail?.version.updated_at) }}</a-descriptions-item>
          <a-descriptions-item label="描述" :span="2">{{ detail?.version.description || '-' }}</a-descriptions-item>
        </a-descriptions>
      </a-card>

      <a-row :gutter="16" style="margin-top: 16px">
        <a-col :span="8">
          <a-card title="应用承接">
            <template v-if="detail?.application">
              <a-descriptions :column="1" size="small" bordered>
                <a-descriptions-item label="应用">{{ detail.application.display_name || detail.application.name }}</a-descriptions-item>
                <a-descriptions-item label="负责人">{{ detail.application.owner || '-' }}</a-descriptions-item>
                <a-descriptions-item label="团队">{{ detail.application.team || '-' }}</a-descriptions-item>
              </a-descriptions>
              <a-button type="primary" style="margin-top: 12px" @click="goApplication(detail.application.id)">查看应用详情</a-button>
            </template>
            <a-empty v-else description="未关联应用" />
          </a-card>
        </a-col>
        <a-col :span="8">
          <a-card title="流水线承接">
            <template v-if="detail?.pipeline">
              <a-descriptions :column="1" size="small" bordered>
                <a-descriptions-item label="流水线">{{ detail.pipeline.name }}</a-descriptions-item>
                <a-descriptions-item label="状态">{{ detail.pipeline.status || '-' }}</a-descriptions-item>
                <a-descriptions-item label="默认分支">{{ detail.pipeline.git_branch || '-' }}</a-descriptions-item>
              </a-descriptions>
              <a-button type="primary" style="margin-top: 12px" @click="goPipeline(detail.pipeline.id)">查看流水线详情</a-button>
            </template>
            <a-empty v-else description="未关联流水线" />
          </a-card>
        </a-col>
        <a-col :span="8">
          <a-card title="发布单承接">
            <template v-if="detail?.release">
              <a-descriptions :column="1" size="small" bordered>
                <a-descriptions-item label="发布单">{{ detail.release.title }}</a-descriptions-item>
                <a-descriptions-item label="环境">{{ detail.release.env }}</a-descriptions-item>
                <a-descriptions-item label="状态">{{ detail.release.status }}</a-descriptions-item>
              </a-descriptions>
              <a-button style="margin-top: 12px" @click="goReleaseList">前往 GitOps 交付</a-button>
            </template>
            <a-empty v-else description="未关联发布单" />
          </a-card>
        </a-col>
      </a-row>

      <a-card title="版本需求清单" style="margin-top: 16px">
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
        <a-table :columns="requirementColumns" :data-source="detail?.requirements || []" :pagination="false" row-key="id">
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
            <template v-else-if="column.key === 'goal_name'">
              <a-button v-if="record.goal_id" type="link" style="padding: 0" @click="goGoal(record.goal_id)">{{ record.goal_name || '-' }}</a-button>
              <span v-else>-</span>
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
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeftOutlined } from '@ant-design/icons-vue'
import { bizApi, type BizVersionDetail } from '@/services/biz'
import { resolveDefaultJiraBaseURL } from '@/services/jira'
import { useBizRequirementColumnPrefs } from '@/composables/useBizRequirementColumnPrefs'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const detail = ref<BizVersionDetail | null>(null)
const jiraBaseURL = ref('')
const OPTIONAL_COLUMN_STORAGE_KEY = 'biz.version_detail.requirements.optional_columns'
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

const requirementColumns = computed(() => {
  const enabled = new Set(selectedOptionalColumns.value)
  return [
    { title: '需求标题', key: 'title' },
    ...(enabled.has('external_key') ? [{ title: '外部单号', key: 'external_key', width: 180 }] : []),
    ...(enabled.has('jira_epic_key') ? [{ title: 'Epic', key: 'jira_epic_key', width: 140 }] : []),
    { title: '业务目标', key: 'goal_name', width: 160 },
    { title: '负责人', dataIndex: 'owner', key: 'owner', width: 100 },
    { title: '状态', key: 'status', width: 100 },
    { title: '优先级', key: 'priority', width: 90 },
  ]
})

const statusText = (status: string) => ({ planning: '规划中', in_progress: '进行中', released: '已发布' }[status] || status)
const statusColor = (status: string) => ({ planning: 'blue', in_progress: 'gold', released: 'green' }[status] || 'default')
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
    const res = await bizApi.getVersion(id)
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

const goBack = () => router.push('/biz/versions')
const goGoal = (id: number) => router.push(`/biz/goals/${id}`)
const goRequirement = (id: number) => router.push(`/biz/requirements/${id}`)
const goApplication = (id: number) => router.push(`/applications/${id}`)
const goPipeline = (id: number) => router.push(`/pipeline/${id}`)
const goReleaseList = () => router.push('/argocd')

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
