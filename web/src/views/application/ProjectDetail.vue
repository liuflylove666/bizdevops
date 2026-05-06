<template>
  <div class="project-detail">
    <div class="page-header">
      <div>
        <a-button type="text" @click="router.back()">返回</a-button>
        <h1>{{ overview?.project.display_name || overview?.project.name || '项目详情' }}</h1>
        <div class="sub-text">{{ overview?.project.org_name || '-' }} · {{ overview?.project.owner || '未设置负责人' }}</div>
        <div class="page-subtitle">先看风险和阻塞，再看交付推进和应用覆盖，避免项目页沦为静态信息堆叠。</div>
      </div>
      <a-space wrap>
        <a-button @click="goWorkspace">项目工作台</a-button>
        <a-button @click="goApplications">查看应用</a-button>
        <a-button @click="goPipelines">查看流水线</a-button>
        <a-button type="primary" @click="goReleases">查看发布主单</a-button>
      </a-space>
    </div>

    <a-spin :spinning="loading">
      <template v-if="overview">
        <a-card :bordered="false" class="hero-card">
          <a-row :gutter="[16, 16]" align="middle">
            <a-col :xs="24" :xl="14">
              <div class="hero-title">项目当前重点：{{ projectFocusHeadline }}</div>
              <div class="hero-subtitle">{{ projectFocusDescription }}</div>
              <a-space wrap class="hero-tags">
                <a-tag color="blue">应用 {{ overview.app_count }}</a-tag>
                <a-tag color="purple">流水线 {{ overview.pipeline_count }}</a-tag>
                <a-tag color="green">达标应用 {{ overview.ready_app_count }}</a-tag>
                <a-tag :color="overview.open_incident_count > 0 ? 'red' : 'green'">事故 {{ overview.open_incident_count }}</a-tag>
                <a-tag :color="overview.pending_release_count > 0 ? 'orange' : 'green'">未完成发布 {{ overview.pending_release_count }}</a-tag>
              </a-space>
            </a-col>
            <a-col :xs="24" :xl="10">
              <a-row :gutter="[12, 12]">
                <a-col :xs="12" v-for="card in topSummaryCards" :key="card.title">
                  <a-card class="summary-card">
                    <a-statistic :title="card.title" :value="card.value" :suffix="card.suffix" :value-style="{ color: card.color }" />
                    <div class="summary-hint">{{ card.hint }}</div>
                  </a-card>
                </a-col>
              </a-row>
            </a-col>
          </a-row>
        </a-card>

        <a-row :gutter="[16, 16]" class="section-row">
          <a-col :xs="24" :xl="9">
            <a-card :bordered="false" title="项目待办" class="full-height-card">
              <a-list :data-source="overview.focus_items" size="small">
                <template #renderItem="{ item }">
                  <a-list-item>
                    <div class="list-main">
                      <div class="list-title">{{ item.title }}</div>
                      <div class="sub-text">{{ item.description }}</div>
                    </div>
                    <a-space>
                      <a-tag :color="getSeverityColor(item.severity)">{{ formatSeverity(item.severity) }}</a-tag>
                      <a-button type="link" @click="router.push(item.path)">处理</a-button>
                    </a-space>
                  </a-list-item>
                </template>
                <template #empty>
                  <a-empty description="当前项目暂无待办风险项" />
                </template>
              </a-list>
            </a-card>
          </a-col>
          <a-col :xs="24" :xl="15">
            <a-card :bordered="false" title="项目风险驾驶舱" class="full-height-card">
              <a-row :gutter="[12, 12]">
                <a-col :xs="24" :md="8" v-for="card in riskCards" :key="card.title">
                  <div class="risk-panel">
                    <div class="risk-title">{{ card.title }}</div>
                    <div class="risk-value" :style="{ color: card.color }">{{ card.value }}</div>
                    <div class="risk-hint">{{ card.hint }}</div>
                    <a-button type="link" size="small" @click="card.action">{{ card.cta }}</a-button>
                  </div>
                </a-col>
              </a-row>
            </a-card>
          </a-col>
        </a-row>

        <a-row :gutter="[16, 16]" class="section-row">
          <a-col :xs="24" :lg="14">
            <a-card :bordered="false" title="GitOps 运行态" extra="最近同步状态">
              <a-list :data-source="overview.recent_argocd_apps" size="small">
                <template #renderItem="{ item }">
                  <a-list-item>
                    <div class="list-main">
                      <div class="list-title">{{ item.application_name || item.name }}</div>
                      <div class="sub-text">{{ item.env || '-' }} · {{ item.name }}</div>
                    </div>
                    <a-space wrap>
                      <a-tag :color="getSyncStatusColor(item.sync_status)">{{ item.sync_status || '-' }}</a-tag>
                      <a-tag :color="getHealthStatusColor(item.health_status)">{{ item.health_status || '-' }}</a-tag>
                      <a-tag v-if="item.drift_detected" color="red">Drift</a-tag>
                    </a-space>
                  </a-list-item>
                </template>
                <template #empty>
                  <a-empty description="当前项目暂无 GitOps 应用" />
                </template>
              </a-list>
            </a-card>
          </a-col>
          <a-col :xs="24" :lg="10">
            <a-card :bordered="false" title="GitOps 健康概览" class="full-height-card">
              <div class="gitops-grid">
                <div class="gitops-item">
                  <div class="gitops-label">接入应用</div>
                  <div class="gitops-value">{{ overview.argocd_app_count }}</div>
                </div>
                <div class="gitops-item">
                  <div class="gitops-label">配置漂移</div>
                  <div class="gitops-value danger">{{ overview.drift_app_count }}</div>
                </div>
                <div class="gitops-item">
                  <div class="gitops-label">OutOfSync</div>
                  <div class="gitops-value warning">{{ overview.out_of_sync_app_count }}</div>
                </div>
                <div class="gitops-item">
                  <div class="gitops-label">Degraded</div>
                  <div class="gitops-value danger">{{ overview.degraded_app_count }}</div>
                </div>
              </div>
              <a-button type="primary" block class="gitops-action" @click="goArgoCD">进入 GitOps</a-button>
            </a-card>
          </a-col>
        </a-row>

        <a-row :gutter="[16, 16]" class="section-row">
          <a-col :xs="24" :lg="16">
            <a-card :bordered="false" title="项目应用">
              <a-table :data-source="overview.apps" :columns="appColumns" row-key="id" :pagination="false" size="small">
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'name'">
                    <div class="list-main">
                      <a class="list-title" @click="router.push(`/applications/${record.id}`)">{{ record.display_name || record.name }}</a>
                      <div class="sub-text">{{ record.name }}</div>
                    </div>
                  </template>
                  <template v-if="column.key === 'status'">
                    <a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status === 'active' ? '启用' : '停用' }}</a-tag>
                  </template>
                  <template v-if="column.key === 'readiness'">
                    <a-progress :percent="record.readiness_score || 0" size="small" :status="getReadinessStatus(record.readiness_score || 0)" />
                  </template>
                </template>
              </a-table>
            </a-card>
          </a-col>
          <a-col :xs="24" :lg="8">
            <a-card :bordered="false" title="项目画像" class="full-height-card">
              <a-descriptions :column="1" size="small">
                <a-descriptions-item label="项目标识">{{ overview.project.name }}</a-descriptions-item>
                <a-descriptions-item label="所属组织">{{ overview.project.org_name || '-' }}</a-descriptions-item>
                <a-descriptions-item label="负责人">{{ overview.project.owner || '-' }}</a-descriptions-item>
                <a-descriptions-item label="状态">{{ overview.project.status || '-' }}</a-descriptions-item>
                <a-descriptions-item label="描述">{{ overview.project.description || '-' }}</a-descriptions-item>
              </a-descriptions>
            </a-card>
          </a-col>
        </a-row>

        <a-row :gutter="[16, 16]" class="section-row">
          <a-col :xs="24" :lg="12">
            <a-card :bordered="false" title="最近流水线">
              <a-list :data-source="overview.recent_pipelines" size="small">
                <template #renderItem="{ item }">
                  <a-list-item>
                    <div class="list-main">
                      <a class="list-title" @click="router.push(`/pipeline/${item.id}`)">{{ item.name }}</a>
                      <div class="sub-text">{{ item.application_name || '-' }} · {{ item.env || '-' }}</div>
                    </div>
                    <a-tag :color="getPipelineStatusColor(item.last_run_status || item.status)">{{ item.last_run_status || item.status || '-' }}</a-tag>
                  </a-list-item>
                </template>
              </a-list>
              <a-empty v-if="!overview.recent_pipelines.length" description="暂无流水线" />
            </a-card>
          </a-col>
          <a-col :xs="24" :lg="12">
            <a-card :bordered="false" title="最近发布">
              <a-list :data-source="overview.recent_releases" size="small">
                <template #renderItem="{ item }">
                  <a-list-item>
                    <div class="list-main">
                      <a class="list-title" @click="router.push(`/releases/${item.id}`)">{{ item.title }}</a>
                      <div class="sub-text">{{ item.application_name }} · {{ item.env }}</div>
                    </div>
                    <a-space>
                      <a-tag :color="getReleaseStatusColor(item.status)">{{ item.status }}</a-tag>
                      <a-tag v-if="item.risk_level" color="orange">{{ item.risk_level }} / {{ item.risk_score ?? 0 }}</a-tag>
                    </a-space>
                  </a-list-item>
                </template>
              </a-list>
              <a-empty v-if="!overview.recent_releases.length" description="暂无发布主单" />
            </a-card>
          </a-col>
        </a-row>
      </template>
      <a-empty v-else-if="!loading" description="项目不存在" />
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { catalogApi, type ProjectOverview } from '@/services/catalog'

const route = useRoute()
const router = useRouter()
const projectId = Number(route.params.id)

const loading = ref(false)
const overview = ref<ProjectOverview | null>(null)

const appColumns = [
  { title: '应用', key: 'name', width: 220 },
  { title: '负责人', dataIndex: 'owner', key: 'owner', width: 120 },
  { title: '团队', dataIndex: 'team', key: 'team', width: 120 },
  { title: '环境数', dataIndex: 'env_count', key: 'env_count', width: 90 },
  { title: '流水线', dataIndex: 'pipeline_count', key: 'pipeline_count', width: 90 },
  { title: '接入完整度', key: 'readiness', width: 180 },
  { title: '状态', key: 'status', width: 90 },
]

const topSummaryCards = computed(() => {
  if (!overview.value) return []
  return [
    {
      title: '平均接入完整度',
      value: Number(overview.value.avg_readiness_score || 0).toFixed(1),
      suffix: '%',
      color: '#1677ff',
      hint: `达标应用 ${overview.value.ready_app_count} 个`,
    },
    {
      title: '发布主单',
      value: overview.value.release_count,
      color: '#722ed1',
      hint: '项目累计发布记录',
    },
    {
      title: '活跃应用',
      value: overview.value.active_app_count,
      color: '#52c41a',
      hint: '当前处于启用状态',
    },
    {
      title: 'GitOps 接入',
      value: overview.value.argocd_app_count,
      color: '#13c2c2',
      hint: '已纳入 Argo CD',
    },
  ]
})

const riskCards = computed(() => {
  if (!overview.value) return []
  return [
    {
      title: '未关闭事故',
      value: overview.value.open_incident_count,
      color: overview.value.open_incident_count > 0 ? '#cf1322' : '#52c41a',
      hint: '优先确认当前运行风险',
      cta: '查看事故',
      action: goIncidents,
    },
    {
      title: '失败流水线',
      value: overview.value.failed_pipeline_count,
      color: overview.value.failed_pipeline_count > 0 ? '#d46b08' : '#52c41a',
      hint: '恢复交付链路稳定性',
      cta: '查看流水线',
      action: goPipelines,
    },
    {
      title: '未完成发布',
      value: overview.value.pending_release_count,
      color: overview.value.pending_release_count > 0 ? '#1677ff' : '#52c41a',
      hint: '待审批 / 待合并 / 待发布',
      cta: '查看主单',
      action: goReleases,
    },
  ]
})

const projectFocusHeadline = computed(() => {
  if (!overview.value) return ''
  if (overview.value.open_incident_count > 0) return '先处理运行事故'
  if (overview.value.failed_pipeline_count > 0) return '先恢复失败流水线'
  if (overview.value.pending_release_count > 0) return '先推进未完成发布'
  return '交付链路整体稳定'
})

const projectFocusDescription = computed(() => {
  if (!overview.value) return ''
  if (overview.value.open_incident_count > 0) return `当前有 ${overview.value.open_incident_count} 个未关闭事故，建议优先回到运行面处理。`
  if (overview.value.failed_pipeline_count > 0) return `当前有 ${overview.value.failed_pipeline_count} 条失败流水线，建议先恢复构建和发布链路。`
  if (overview.value.pending_release_count > 0) return `当前有 ${overview.value.pending_release_count} 个发布仍在推进中，建议优先清理阻塞项。`
  return `当前项目已有 ${overview.value.ready_app_count} 个应用达到达标状态，可以继续优化覆盖面和交付效率。`
})

const fetchOverview = async () => {
  loading.value = true
  try {
    const res = await catalogApi.getProjectOverview(projectId)
    overview.value = res.data || null
  } catch (error: any) {
    message.error(error?.message || '加载项目详情失败')
  } finally {
    loading.value = false
  }
}

const goApplications = () => {
  if (!overview.value?.project.id) return
  router.push({
    path: '/applications',
    query: {
      organization_id: String(overview.value.project.organization_id),
      project_id: String(overview.value.project.id),
    },
  })
}

const projectRouteQuery = () => ({
  project_id: String(overview.value?.project.id || ''),
})

const goWorkspace = () => {
  if (!overview.value?.project.id) return
  router.push({
    path: '/dashboard',
    query: {
      focus: 'delivery',
      ...projectRouteQuery(),
    },
  })
}

const goPipelines = () => {
  if (!overview.value?.project.id) return
  router.push({
    path: '/pipeline/list',
    query: projectRouteQuery(),
  })
}

const goReleases = () => {
  if (!overview.value?.project.id) return
  router.push({
    path: '/releases',
    query: projectRouteQuery(),
  })
}

const goIncidents = () => {
  if (!overview.value?.project.id) return
  router.push({
    path: '/incidents',
    query: projectRouteQuery(),
  })
}

const goArgoCD = () => {
  if (!overview.value?.project.id) return
  router.push({
    path: '/argocd',
    query: projectRouteQuery(),
  })
}

const getReadinessStatus = (score: number) => {
  if (score >= 80) return 'success'
  if (score >= 50) return 'normal'
  return 'exception'
}

const getPipelineStatusColor = (status?: string) => {
  if (status === 'success' || status === 'active') return 'green'
  if (status === 'running') return 'blue'
  if (status === 'failed' || status === 'disabled') return 'red'
  return 'default'
}

const getReleaseStatusColor = (status?: string) => {
  if (status === 'published') return 'green'
  if (status === 'pending_approval') return 'gold'
  if (status === 'approved' || status === 'pr_opened' || status === 'pr_merged') return 'blue'
  if (status === 'failed' || status === 'rejected') return 'red'
  return 'default'
}

const getSeverityColor = (severity?: string) => {
  if (severity === 'high') return 'red'
  if (severity === 'medium') return 'orange'
  if (severity === 'low') return 'blue'
  return 'default'
}

const formatSeverity = (severity?: string) => {
  if (severity === 'high') return '高优先级'
  if (severity === 'medium') return '中优先级'
  if (severity === 'low') return '低优先级'
  return severity || '-'
}

const getSyncStatusColor = (status?: string) => {
  if (status === 'Synced') return 'green'
  if (status === 'OutOfSync') return 'orange'
  if (status === 'Unknown') return 'red'
  return 'default'
}

const getHealthStatusColor = (status?: string) => {
  if (status === 'Healthy') return 'green'
  if (status === 'Progressing') return 'blue'
  if (status === 'Degraded' || status === 'Missing') return 'red'
  return 'default'
}

onMounted(fetchOverview)
</script>

<style scoped>
.project-detail {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}

.page-header h1 {
  margin: 8px 0 4px;
  font-size: 22px;
}

.page-subtitle,
.sub-text {
  color: #8c8c8c;
  font-size: 12px;
}

.page-subtitle {
  margin-top: 10px;
  font-size: 13px;
}

.hero-card {
  margin-bottom: 16px;
  background: linear-gradient(135deg, #f6fbff 0%, #ffffff 55%, #f8faff 100%);
}

.hero-title {
  font-size: 24px;
  font-weight: 600;
  color: #1f1f1f;
}

.hero-subtitle {
  margin-top: 8px;
  color: #6b7280;
  line-height: 1.6;
}

.hero-tags {
  margin-top: 16px;
}

.summary-card,
.full-height-card {
  height: 100%;
}

.summary-hint {
  margin-top: 8px;
  color: #8c8c8c;
  font-size: 12px;
}

.section-row {
  margin-bottom: 16px;
}

.list-main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.list-title {
  font-weight: 500;
}

.risk-panel {
  height: 100%;
  padding: 16px;
  border-radius: 12px;
  background: #fafafa;
}

.risk-title,
.gitops-label {
  color: #8c8c8c;
  font-size: 12px;
}

.risk-value,
.gitops-value {
  margin-top: 8px;
  font-size: 26px;
  font-weight: 700;
  color: #1f1f1f;
}

.risk-hint {
  margin-top: 8px;
  color: #6b7280;
  min-height: 36px;
}

.gitops-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.gitops-item {
  padding: 16px;
  border-radius: 12px;
  background: #fafafa;
}

.danger {
  color: #cf1322;
}

.warning {
  color: #d46b08;
}

.gitops-action {
  margin-top: 16px;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
  }
}
</style>
