<template>
  <div class="service-detail">
    <a-spin :spinning="loading">
      <template v-if="overview">
        <!-- 基本信息 -->
        <a-row :gutter="16" style="margin-bottom:16px">
          <a-col :span="16">
            <a-card :bordered="false">
              <a-descriptions :title="overview.app?.display_name || overview.app?.name" :column="3" bordered size="small">
                <a-descriptions-item label="应用标识">{{ overview.app?.name }}</a-descriptions-item>
                <a-descriptions-item label="组织">{{ overview.org_name || '-' }}</a-descriptions-item>
                <a-descriptions-item label="项目">{{ overview.project_name || '-' }}</a-descriptions-item>
                <a-descriptions-item label="开发语言">{{ overview.app?.language || '-' }}</a-descriptions-item>
                <a-descriptions-item label="仓库">
                  <a v-if="overview.app?.repo_url" :href="overview.app.repo_url" target="_blank">{{ overview.app.repo_url }}</a>
                  <span v-else>-</span>
                </a-descriptions-item>
                <a-descriptions-item label="负责人">{{ overview.app?.owner || '-' }}</a-descriptions-item>
              </a-descriptions>
            </a-card>
          </a-col>
          <a-col :span="8">
            <a-card :bordered="false" style="height:100%">
              <a-row :gutter="16">
                <a-col :span="8" class="stat-item">
                  <a-statistic title="健康状态" :value="healthLabel" :value-style="{ color: healthColor }" />
                </a-col>
                <a-col :span="8" class="stat-item">
                  <a-statistic title="近7日告警" :value="overview.alert_count" :value-style="{ color: overview.alert_count > 0 ? '#ff4d4f' : '#52c41a' }" />
                </a-col>
                <a-col :span="8" class="stat-item">
                  <a-statistic title="交付成功率" :value="successRate" suffix="%" :value-style="{ color: successRate >= 90 ? '#52c41a' : '#faad14' }" />
                </a-col>
              </a-row>
            </a-card>
          </a-col>
        </a-row>

        <!-- 环境配置 -->
        <a-card :bordered="false" title="环境配置" style="margin-bottom:16px" v-if="overview.envs && overview.envs.length">
          <a-table :columns="envColumns" :data-source="overview.envs" row-key="id" :pagination="false" size="small">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'env_name'">
                <a-tag :color="record.env_color || 'blue'">{{ record.env_display_name || record.env_name }}</a-tag>
              </template>
              <template v-if="column.key === 'status'">
                <a-badge :status="record.status === 'running' ? 'success' : record.status === 'stopped' ? 'default' : 'processing'" :text="record.status || '-'" />
              </template>
            </template>
          </a-table>
        </a-card>

        <!-- 交付统计 + 近期交付 -->
        <a-row :gutter="16" style="margin-bottom:16px">
          <a-col :span="8">
            <a-card :bordered="false" title="30天交付统计">
              <a-descriptions :column="1" size="small" bordered>
                <a-descriptions-item label="总交付次数">{{ overview.delivery_stats?.total || 0 }}</a-descriptions-item>
                <a-descriptions-item label="成功次数">{{ overview.delivery_stats?.success || 0 }}</a-descriptions-item>
                <a-descriptions-item label="失败次数">{{ overview.delivery_stats?.failed || 0 }}</a-descriptions-item>
                <a-descriptions-item label="平均耗时">{{ avgDuration }}</a-descriptions-item>
              </a-descriptions>
            </a-card>
          </a-col>
          <a-col :span="16">
            <a-card :bordered="false" title="近期交付">
              <a-table :columns="deployColumns" :data-source="overview.recent_delivery_records" row-key="id" :pagination="false" size="small">
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'status'">
                    <a-tag :color="record.status === 'success' ? 'green' : record.status === 'failed' ? 'red' : 'blue'">{{ record.status }}</a-tag>
                  </template>
                  <template v-if="column.key === 'env_name'">
                    <a-tag>{{ record.env_name || '-' }}</a-tag>
                  </template>
                  <template v-if="column.key === 'created_at'">
                    {{ formatTime(record.created_at) }}
                  </template>
                </template>
              </a-table>
            </a-card>
          </a-col>
        </a-row>

        <!-- 告警 + 健康检查 -->
        <a-row :gutter="16" style="margin-bottom: 16px">
          <a-col :span="12">
            <a-card :bordered="false" title="近期告警" :loading="loadingAlerts">
              <a-table :columns="alertColumns" :data-source="alerts" row-key="id" :pagination="false" size="small">
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'severity'">
                    <a-tag :color="record.severity === 'critical' ? 'red' : record.severity === 'warning' ? 'orange' : 'blue'">{{ record.severity }}</a-tag>
                  </template>
                  <template v-if="column.key === 'created_at'">
                    {{ formatTime(record.created_at) }}
                  </template>
                </template>
              </a-table>
            </a-card>
          </a-col>
          <a-col :span="12">
            <a-card :bordered="false" title="健康检查" :loading="loadingHealth">
              <a-table :columns="healthColumns" :data-source="healthChecks" row-key="id" :pagination="false" size="small">
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'last_status'">
                    <a-badge :status="record.last_status === 'healthy' ? 'success' : record.last_status === 'unhealthy' ? 'error' : 'default'" :text="record.last_status || 'unknown'" />
                  </template>
                  <template v-if="column.key === 'last_checked_at'">
                    {{ formatTime(record.last_checked_at) }}
                  </template>
                </template>
              </a-table>
            </a-card>
          </a-col>
        </a-row>

        <!-- 扩展 Tab：Nacos 配置 / 变更时间线 / SQL 工单 -->
        <a-card :bordered="false">
          <a-tabs v-model:activeKey="detailTab" @change="handleTabChange">
            <!-- Nacos 配置发布 -->
            <a-tab-pane key="nacos" tab="Nacos 配置">
              <a-table :columns="nacosColumns" :data-source="nacosReleases" row-key="id" :pagination="false" size="small" :loading="loadingNacos">
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'status'">
                    <a-tag :color="nacosStatusColor(record.status)">{{ nacosStatusText(record.status) }}</a-tag>
                  </template>
                  <template v-if="column.key === 'env'">
                    <a-tag :color="envColorMap(record.env)">{{ record.env }}</a-tag>
                  </template>
                  <template v-if="column.key === 'created_at'">
                    {{ formatTime(record.created_at) }}
                  </template>
                </template>
              </a-table>
              <a-empty v-if="!loadingNacos && nacosReleases.length === 0" description="暂无 Nacos 配置发布记录" />
            </a-tab-pane>

            <!-- 变更时间线 -->
            <a-tab-pane key="timeline" tab="变更时间线">
              <a-timeline v-if="changeEvents.length > 0">
                <a-timeline-item v-for="evt in changeEvents" :key="evt.id" :color="eventColor(evt.event_type)">
                  <div style="display: flex; justify-content: space-between">
                    <div>
                      <a-tag :color="eventBadge(evt.event_type)" size="small">{{ eventLabel(evt.event_type) }}</a-tag>
                      <span style="font-weight: 500">{{ evt.title }}</span>
                      <a-tag v-if="evt.env" :color="envColorMap(evt.env)" size="small" style="margin-left: 4px">{{ evt.env }}</a-tag>
                    </div>
                    <span style="color: #999; font-size: 12px">{{ evt.operator }} · {{ formatTime(evt.created_at) }}</span>
                  </div>
                  <div v-if="evt.description" style="color: #666; font-size: 13px; margin-top: 2px">{{ evt.description }}</div>
                </a-timeline-item>
              </a-timeline>
              <a-empty v-else-if="!loadingEvents" description="暂无变更事件" />
              <a-spin v-else />
            </a-tab-pane>

            <!-- SQL 工单 -->
            <a-tab-pane key="sql" tab="SQL 工单">
              <a-empty description="暂未关联 SQL 工单（将在 Release 聚合中自动关联）" />
            </a-tab-pane>
          </a-tabs>
        </a-card>
      </template>
      <a-empty v-else-if="!loading" description="应用不存在" />
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { serviceDetailApi, type ServiceOverview } from '@/services/catalog'
import { nacosReleaseApi } from '@/services/nacosRelease'
import type { NacosRelease } from '@/services/nacosRelease'
import { changeEventApi } from '@/services/changeEvent'
import type { ChangeEvent } from '@/services/changeEvent'

const route = useRoute()
const appId = Number(route.params.id)

const loading = ref(false)
const overview = ref<ServiceOverview | null>(null)
const loadingAlerts = ref(false)
const loadingHealth = ref(false)
const loadingNacos = ref(false)
const loadingEvents = ref(false)
const alerts = ref<any[]>([])
const healthChecks = ref<any[]>([])
const nacosReleases = ref<NacosRelease[]>([])
const changeEvents = ref<ChangeEvent[]>([])
const detailTab = ref('nacos')

const healthLabel = computed(() => {
  const s = overview.value?.health_status
  if (s === 'healthy') return 'Healthy'
  if (s === 'unhealthy') return 'Unhealthy'
  return 'Unknown'
})
const healthColor = computed(() => {
  const s = overview.value?.health_status
  if (s === 'healthy') return '#52c41a'
  if (s === 'unhealthy') return '#ff4d4f'
  return '#999'
})
const successRate = computed(() => {
  const stats = overview.value?.delivery_stats
  if (!stats || !stats.total) return 0
  return Math.round(stats.success_rate * 100) / 100
})
const avgDuration = computed(() => {
  const d = overview.value?.delivery_stats?.avg_duration
  if (!d) return '-'
  if (d < 60) return `${Math.round(d)}s`
  return `${Math.round(d / 60)}m`
})

const envColumns = [
  { title: '环境', key: 'env_name', dataIndex: 'env_name', width: 120 },
  { title: '镜像/版本', dataIndex: 'image', key: 'image' },
  { title: '副本数', dataIndex: 'replicas', key: 'replicas', width: 80 },
  { title: '集群', dataIndex: 'cluster_name', key: 'cluster_name', width: 120 },
  { title: '命名空间', dataIndex: 'namespace', key: 'namespace', width: 120 },
  { title: '状态', key: 'status', width: 80 },
]
const deployColumns = [
  { title: '版本', dataIndex: 'version', key: 'version', width: 120 },
  { title: '环境', key: 'env_name', width: 80 },
  { title: '状态', key: 'status', width: 80 },
  { title: '操作人', dataIndex: 'operator', key: 'operator', width: 80 },
  { title: '时间', key: 'created_at', width: 160 },
]
const alertColumns = [
  { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '级别', key: 'severity', width: 80 },
  { title: '时间', key: 'created_at', width: 160 },
]
const healthColumns = [
  { title: '目标', dataIndex: 'target_name', key: 'target_name' },
  { title: 'URL', dataIndex: 'target_url', key: 'target_url', ellipsis: true },
  { title: '状态', key: 'last_status', width: 100 },
  { title: '检查时间', key: 'last_checked_at', width: 160 },
]
const nacosColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '环境', dataIndex: 'env', key: 'env', width: 80 },
  { title: 'DataID', dataIndex: 'data_id', key: 'data_id', ellipsis: true },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '创建人', dataIndex: 'created_by_name', key: 'created_by_name', width: 100 },
  { title: '时间', dataIndex: 'created_at', key: 'created_at', width: 160 },
]
const nacosStatusColor = (s: string) => ({
  draft: 'default', pending_approval: 'processing', approved: 'cyan',
  published: 'success', rolled_back: 'warning', rejected: 'error',
}[s] || 'default')
const nacosStatusText = (s: string) => ({
  draft: '草稿', pending_approval: '待审批', approved: '已审批',
  published: '已发布', rolled_back: '已回滚', rejected: '已驳回',
}[s] || s)
const envColorMap = (e: string) => ({ dev: 'blue', test: 'green', uat: 'orange', gray: 'purple', prod: 'red' }[e] || 'default')
const eventColor = (t: string) => ({ deploy: 'green', nacos_release: 'purple', sql_ticket: 'orange', pipeline_run: 'blue', promotion: 'cyan', release: 'pink' }[t] || 'gray')
const eventBadge = (t: string) => ({ deploy: 'green', nacos_release: 'purple', sql_ticket: 'orange', pipeline_run: 'blue', promotion: 'cyan', release: 'magenta' }[t] || 'default')
const eventLabel = (t: string) => ({ deploy: '部署', nacos_release: 'Nacos', sql_ticket: 'SQL', pipeline_run: '流水线', promotion: '晋级', release: '发布' }[t] || t)

const formatTime = (t: string) => {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

const loadOverview = async () => {
  loading.value = true
  try {
    const res = await serviceDetailApi.getOverview(appId)
    overview.value = res.data || null
  } catch { overview.value = null }
  finally { loading.value = false }
}

const loadAlerts = async () => {
  loadingAlerts.value = true
  try {
    const res = await serviceDetailApi.getAlerts(appId, { page: 1, page_size: 5 })
    alerts.value = res.data?.list || res.data || []
  } catch { alerts.value = [] }
  finally { loadingAlerts.value = false }
}

const loadHealth = async () => {
  loadingHealth.value = true
  try {
    const res = await serviceDetailApi.getHealth(appId)
    healthChecks.value = res.data || []
  } catch { healthChecks.value = [] }
  finally { loadingHealth.value = false }
}

const loadNacosReleases = async () => {
  loadingNacos.value = true
  try {
    const res = await nacosReleaseApi.listByService(appId, 20)
    nacosReleases.value = res.data?.data || []
  } catch { nacosReleases.value = [] }
  finally { loadingNacos.value = false }
}

const loadChangeEvents = async () => {
  loadingEvents.value = true
  try {
    const res = await changeEventApi.listByApp(appId, 50)
    changeEvents.value = res || []
  } catch { changeEvents.value = [] }
  finally { loadingEvents.value = false }
}

const handleTabChange = (key: string) => {
  if (key === 'nacos' && nacosReleases.value.length === 0) loadNacosReleases()
  if (key === 'timeline' && changeEvents.value.length === 0) loadChangeEvents()
}

onMounted(() => { loadOverview(); loadAlerts(); loadHealth(); loadNacosReleases() })
</script>

<style scoped>
.stat-item { text-align: center; }
</style>
