<template>
  <div class="biz-detail-page">
    <div class="page-header">
      <a-button type="text" @click="goBack">
        <ArrowLeftOutlined />
        返回
      </a-button>
      <div class="page-header__content">
        <div class="page-header__title">
          <span>{{ detail?.requirement.title || '需求详情' }}</span>
          <a-tag v-if="detail?.requirement.status" :color="statusColor(detail.requirement.status)">
            {{ statusText(detail.requirement.status) }}
          </a-tag>
          <a-tag v-if="detail?.requirement.priority" :color="priorityColor(detail.requirement.priority)">
            {{ priorityText(detail.requirement.priority) }}
          </a-tag>
        </div>
        <div class="page-header__desc">{{ detail?.requirement.description || '查看需求来源、目标归属和版本承接关系。' }}</div>
      </div>
    </div>

    <a-spin :spinning="loading">
      <a-card title="需求概览">
        <a-descriptions :column="2" bordered>
          <a-descriptions-item label="需求标题">{{ detail?.requirement.title || '-' }}</a-descriptions-item>
          <a-descriptions-item label="来源">{{ sourceText(detail?.requirement.source) }}</a-descriptions-item>
          <a-descriptions-item label="外部单号">
            <a-space v-if="detail?.requirement.external_key">
              <span>{{ detail.requirement.external_key }}</span>
              <a v-if="jiraIssueBrowseUrl" :href="jiraIssueBrowseUrl" target="_blank">打开 Jira</a>
            </a-space>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="Jira Epic">{{ detail?.requirement.jira_epic_key || '-' }}</a-descriptions-item>
          <a-descriptions-item label="Jira 标签" :span="2">
            <a-space wrap v-if="splitCsv(detail?.requirement.jira_labels).length > 0">
              <a-tag v-for="label in splitCsv(detail?.requirement.jira_labels)" :key="`detail-label-${label}`">{{ label }}</a-tag>
            </a-space>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="Jira 组件" :span="2">
            <a-space wrap v-if="splitCsv(detail?.requirement.jira_components).length > 0">
              <a-tag color="purple" v-for="comp in splitCsv(detail?.requirement.jira_components)" :key="`detail-comp-${comp}`">{{ comp }}</a-tag>
            </a-space>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="负责人">{{ detail?.requirement.owner || '-' }}</a-descriptions-item>
          <a-descriptions-item label="状态">{{ detail?.requirement.status ? statusText(detail.requirement.status) : '-' }}</a-descriptions-item>
          <a-descriptions-item label="优先级">{{ detail?.requirement.priority ? priorityText(detail.requirement.priority) : '-' }}</a-descriptions-item>
          <a-descriptions-item label="价值分">{{ detail?.requirement.value_score ?? '-' }}</a-descriptions-item>
          <a-descriptions-item label="关联业务目标">
            <a-button v-if="detail?.goal" type="link" style="padding: 0" @click="goGoal(detail.goal.id)">{{ detail.goal.name }}</a-button>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="关联版本">
            <a-button v-if="detail?.version" type="link" style="padding: 0" @click="goVersion(detail.version.id)">{{ detail.version.name }}</a-button>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="创建时间">{{ formatTime(detail?.requirement.created_at) }}</a-descriptions-item>
          <a-descriptions-item label="更新时间">{{ formatTime(detail?.requirement.updated_at) }}</a-descriptions-item>
          <a-descriptions-item label="描述" :span="2">{{ detail?.requirement.description || '-' }}</a-descriptions-item>
        </a-descriptions>
      </a-card>

      <a-row :gutter="16" style="margin-top: 16px">
        <a-col :span="12">
          <a-card title="目标承接">
            <template v-if="detail?.goal">
              <a-descriptions :column="1" size="small" bordered>
                <a-descriptions-item label="目标名称">{{ detail.goal.name }}</a-descriptions-item>
                <a-descriptions-item label="目标编码">{{ detail.goal.code || '-' }}</a-descriptions-item>
                <a-descriptions-item label="负责人">{{ detail.goal.owner || '-' }}</a-descriptions-item>
                <a-descriptions-item label="价值指标">{{ detail.goal.value_metric || '-' }}</a-descriptions-item>
              </a-descriptions>
              <a-button type="primary" style="margin-top: 12px" @click="goGoal(detail.goal.id)">查看目标详情</a-button>
            </template>
            <a-empty v-else description="未关联业务目标" />
          </a-card>
        </a-col>
        <a-col :span="12">
          <a-card title="交付映射">
            <template v-if="detail?.application || detail?.pipeline">
              <a-descriptions :column="1" size="small" bordered>
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
              </a-descriptions>
              <a-space style="margin-top: 12px">
                <a-button v-if="detail?.application" @click="goApplication(detail.application.id)">查看应用详情</a-button>
                <a-button v-if="detail?.pipeline" type="primary" @click="goPipeline(detail.pipeline.id)">查看流水线详情</a-button>
              </a-space>
            </template>
            <a-empty v-else description="未建立交付映射" />
          </a-card>
        </a-col>
      </a-row>

      <a-row :gutter="16" style="margin-top: 16px">
        <a-col :span="12">
          <a-card title="版本承接">
            <template v-if="detail?.version">
              <a-descriptions :column="1" size="small" bordered>
                <a-descriptions-item label="版本名称">{{ detail.version.name }}</a-descriptions-item>
                <a-descriptions-item label="版本编码">{{ detail.version.code || '-' }}</a-descriptions-item>
                <a-descriptions-item label="负责人">{{ detail.version.owner || '-' }}</a-descriptions-item>
                <a-descriptions-item label="发布时间">{{ formatTime(detail.version.release_date) }}</a-descriptions-item>
              </a-descriptions>
              <a-button type="primary" style="margin-top: 12px" @click="goVersion(detail.version.id)">查看版本详情</a-button>
            </template>
            <a-empty v-else description="未关联版本计划" />
          </a-card>
        </a-col>
      </a-row>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeftOutlined } from '@ant-design/icons-vue'
import { bizApi, type BizRequirementDetail } from '@/services/biz'
import { resolveDefaultJiraBaseURL } from '@/services/jira'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const detail = ref<BizRequirementDetail | null>(null)
const jiraBaseURL = ref('')

const statusText = (status: string) => ({ backlog: '待规划', in_progress: '进行中', done: '已完成' }[status] || status)
const statusColor = (status: string) => ({ backlog: 'blue', in_progress: 'gold', done: 'green' }[status] || 'default')
const priorityText = (priority: string) => ({ high: '高', medium: '中', low: '低' }[priority] || priority)
const priorityColor = (priority: string) => ({ high: 'red', medium: 'orange', low: 'default' }[priority] || 'default')
const sourceText = (source?: string) => ({ manual: '手工录入', jira: 'Jira', sales: '销售反馈' }[source || ''] || source || '-')
const formatTime = (value?: string) => value ? value.replace('T', ' ').slice(0, 19) : '-'
const splitCsv = (value?: string) => (value || '').split(',').map(item => item.trim()).filter(Boolean)
const jiraIssueBrowseUrl = computed(() => {
  const key = detail.value?.requirement.external_key
  if (!key || !jiraBaseURL.value) return ''
  return `${jiraBaseURL.value}/browse/${key}`
})

const fetchDetail = async (id: number) => {
  loading.value = true
  try {
    const res = await bizApi.getRequirement(id)
    detail.value = res.data
  } finally {
    loading.value = false
  }
}

const resolveJiraBaseURL = async () => {
  try {
    jiraBaseURL.value = await resolveDefaultJiraBaseURL()
  } catch {
    jiraBaseURL.value = ''
  }
}

const goBack = () => router.push('/biz/requirements')
const goGoal = (id: number) => router.push(`/biz/goals/${id}`)
const goVersion = (id: number) => router.push(`/biz/versions/${id}`)
const goApplication = (id: number) => router.push(`/applications/${id}`)
const goPipeline = (id: number) => router.push(`/pipeline/${id}`)

watch(
  () => Number(route.params.id),
  (id) => {
    if (id) {
      fetchDetail(id)
    }
  },
  { immediate: true }
)

onMounted(() => {
  void resolveJiraBaseURL()
})
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
