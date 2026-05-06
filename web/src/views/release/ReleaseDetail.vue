<template>
  <div class="release-detail">
    <!-- 顶部 -->
    <a-page-header
      :title="loading ? '加载中…' : (release?.title || '未知发布')"
      :sub-title="release ? `#${release.id} · ${release.application_name || '-'} · ${release.env || '-'}` : ''"
      @back="goBack"
    >
      <template #tags>
        <a-tag v-if="release" :color="statusColor(release.status)">
          {{ statusLabel(release.status) }}
        </a-tag>
        <a-tag v-if="release" :color="rolloutColor(release.rollout_strategy)">
          {{ rolloutLabel(release.rollout_strategy) }}
        </a-tag>
      </template>
      <template #extra>
        <a-space>
          <a-button @click="fetchAll">
            <template #icon><ReloadOutlined /></template>
            刷新
          </a-button>

          <template v-if="release">
            <a-button v-if="release.status === 'draft'" type="primary" :loading="acting" @click="onSubmit">
              提交审批
            </a-button>
            <template v-if="release.status === 'pending_approval'">
              <a-popconfirm title="确认通过审批？" @confirm="onApprove">
                <a-button type="primary" :loading="acting">通过</a-button>
              </a-popconfirm>
              <a-button danger :loading="acting" @click="showRejectModal = true">驳回</a-button>
            </template>

            <ReleaseGitOpsPRButton
              v-if="release.status === 'approved' || release.status === 'pr_opened'"
              :release-id="release.id!"
              :release-status="release.status"
              @opened="onPROpened"
            />

            <a-popconfirm
              v-if="release.status === 'pr_merged' || release.status === 'approved'"
              :title="gateSummary?.blocked ? gateSummary.next_action : '确认发布？'"
              :disabled="gateSummary?.blocked"
              @confirm="onPublish"
            >
              <a-button type="primary" :loading="acting" :disabled="gateSummary?.blocked">发布</a-button>
            </a-popconfirm>
          </template>
        </a-space>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-card
        v-if="overview"
        title="发布总览"
        :bordered="false"
        size="small"
        class="overview-card"
      >
        <template #extra>
          <a-space>
            <a-tag :color="overview.blocked ? 'red' : 'blue'">
              {{ overview.blocked ? '存在阻塞' : '下一步' }}
            </a-tag>
            <span class="overview-next">{{ overview.blocked_reason || overview.next_action }}</span>
          </a-space>
        </template>
        <a-row :gutter="12">
          <a-col
            v-for="stage in overview.stages"
            :key="stage.key"
            flex="1"
          >
            <div class="overview-stage" :class="`stage-${stage.status}`">
              <div class="stage-label">{{ stage.label }}</div>
              <div class="stage-message">{{ stage.message || overviewStageText(stage.status) }}</div>
            </div>
          </a-col>
        </a-row>
        <a-row :gutter="16" class="overview-facts">
          <a-col :span="8">
            <span class="muted">审批</span>
            <a-tag :color="approvalStatusColor(overview.approval.status)">
              {{ approvalStatusLabel(overview.approval.status) }}
            </a-tag>
            <a
              v-if="overview.approval.instance_id"
              @click="router.push(`/approval/instances/${overview.approval.instance_id}`)"
            >
              #{{ overview.approval.instance_id }}
            </a>
          </a-col>
          <a-col :span="8">
            <span class="muted">GitOps PR</span>
            <a-tag :color="gitopsStatusColor(overview.gitops.status)">
              {{ overview.gitops.status || 'none' }}
            </a-tag>
            <a
              v-if="overview.gitops.change_request_id"
              @click="goCR(overview.gitops.change_request_id)"
            >
              #{{ overview.gitops.change_request_id }}
            </a>
          </a-col>
          <a-col :span="8">
            <span class="muted">ArgoCD</span>
            <a-tag :color="argoSyncColor(overview.argocd.sync_status)">
              {{ overview.argocd.sync_status || '未同步' }}
            </a-tag>
            <a-tag v-if="overview.argocd.health_status">
              {{ overview.argocd.health_status }}
            </a-tag>
          </a-col>
        </a-row>
      </a-card>

      <a-row :gutter="16">
        <a-col :span="16">
          <a-card title="Gate Checklist" :bordered="false" size="small" style="margin-bottom: 16px" :loading="gatesLoading">
            <template #extra>
              <a-space>
                <a-tag v-if="gateSummary" :color="gateStatusColor(gateSummary.status)">
                  {{ gateStatusText(gateSummary.status) }}
                </a-tag>
                <span v-if="gateSummary" class="muted">{{ gateSummary.next_action }}</span>
                <a-button size="small" @click="refreshGates">刷新 Gate</a-button>
              </a-space>
            </template>
            <a-list
              size="small"
              :data-source="gateSummary?.items || []"
              :locale="{ emptyText: '暂无 Gate 结果' }"
            >
              <template #renderItem="{ item }">
                <a-list-item>
                  <a-list-item-meta>
                    <template #title>
                      <a-space wrap>
                        <span>{{ item.name }}</span>
                        <a-tag :color="gateStatusColor(item.status)">{{ gateStatusText(item.status) }}</a-tag>
                        <a-tag v-if="item.policy">{{ gatePolicyText(item.policy) }}</a-tag>
                      </a-space>
                    </template>
                    <template #description>
                      <span>{{ item.message }}</span>
                    </template>
                  </a-list-item-meta>
                </a-list-item>
              </template>
            </a-list>
          </a-card>

          <!-- 基本信息 -->
          <a-card title="基本信息" :bordered="false" size="small" style="margin-bottom: 16px">
            <a-descriptions :column="2" size="small">
              <a-descriptions-item label="版本">{{ release?.version || '-' }}</a-descriptions-item>
              <a-descriptions-item label="环境">{{ release?.env || '-' }}</a-descriptions-item>
              <a-descriptions-item label="应用">{{ release?.application_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="发布策略">{{ rolloutLabel(release?.rollout_strategy) }}</a-descriptions-item>
              <a-descriptions-item label="创建人">{{ release?.created_by_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="创建时间">{{ formatTime(release?.created_at) }}</a-descriptions-item>
              <a-descriptions-item label="审批人">{{ release?.approved_by_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="审批时间">{{ formatTime(release?.approved_at) }}</a-descriptions-item>
              <a-descriptions-item label="GitOps ChangeRequest" :span="2">
                <a v-if="release?.gitops_change_request_id" @click="goCR(release.gitops_change_request_id)">
                  #{{ release.gitops_change_request_id }}
                </a>
                <span v-else class="muted">未提交</span>
              </a-descriptions-item>
              <a-descriptions-item label="ArgoCD App">{{ release?.argo_app_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="同步状态">{{ release?.argo_sync_status || '-' }}</a-descriptions-item>
              <a-descriptions-item label="说明" :span="2">
                <pre class="desc-text">{{ release?.description || '无' }}</pre>
              </a-descriptions-item>
              <a-descriptions-item v-if="release?.reject_reason" label="驳回原因" :span="2">
                <a-alert :message="release.reject_reason" type="error" show-icon />
              </a-descriptions-item>
            </a-descriptions>
          </a-card>

          <!-- 变更项 / 关联事故 Tabs -->
          <a-card :bordered="false" size="small">
            <a-tabs v-model:activeKey="activeBottomTab" size="small">
              <a-tab-pane key="items">
                <template #tab>
                  <span>
                    变更项
                    <a-badge :count="items.length" :number-style="{ backgroundColor: '#91caff' }" />
                  </span>
                </template>
                <template #extra v-if="activeBottomTab === 'items'">
                  <a-space>
                    <span class="muted">共 {{ items.length }} 项</span>
                    <a-button
                      v-if="canEditItems"
                      size="small"
                      type="primary"
                      ghost
                      @click="showAddItem = true"
                    >
                      <template #icon><PlusOutlined /></template>
                      新增变更项
                    </a-button>
                  </a-space>
                </template>
                <a-table
                  :columns="itemColumns"
                  :data-source="items"
                  :pagination="false"
                  size="small"
                  row-key="id"
                  :empty-text="'暂无变更项；新建后可关联流水线/Nacos/数据库工单等'"
                >
                  <template #bodyCell="{ column, record }">
                    <template v-if="column.key === 'item_type'">
                      <a-tag :color="itemTypeColor(record.item_type)">{{ itemTypeLabel(record.item_type) }}</a-tag>
                    </template>
                    <template v-else-if="column.key === 'item_status'">
                      <a-tag>{{ record.item_status || '-' }}</a-tag>
                    </template>
                    <template v-else-if="column.key === 'action'">
                      <a-popconfirm
                        title="确认从发布单中移除此项？"
                        @confirm="onRemoveItem(record.id!)"
                      >
                        <a class="danger">移除</a>
                      </a-popconfirm>
                    </template>
                  </template>
                </a-table>
              </a-tab-pane>

              <a-tab-pane key="incidents">
                <template #tab>
                  <span>
                    关联事故
                    <a-badge
                      v-if="linkedIncidents.length > 0"
                      :count="linkedIncidents.length"
                      :number-style="{
                        backgroundColor: hasOpenIncident ? '#ff4d4f' : '#52c41a',
                      }"
                    />
                  </span>
                </template>
                <a-table
                  :columns="incidentColumns"
                  :data-source="linkedIncidents"
                  :loading="incidentsLoading"
                  :pagination="false"
                  size="small"
                  row-key="id"
                  :empty-text="'该发布尚未关联任何事故'"
                >
                  <template #bodyCell="{ column, record }">
                    <template v-if="column.key === 'title'">
                      <a @click="goIncident(record.id)">{{ record.title }}</a>
                    </template>
                    <template v-else-if="column.key === 'severity'">
                      <a-tag :color="incidentSeverityColor(record.severity)">{{ record.severity }}</a-tag>
                    </template>
                    <template v-else-if="column.key === 'status'">
                      <a-tag :color="incidentStatusColor(record.status)">{{ incidentStatusLabel(record.status) }}</a-tag>
                    </template>
                    <template v-else-if="column.key === 'detected_at'">
                      {{ formatTime(record.detected_at) }}
                    </template>
                    <template v-else-if="column.key === 'duration'">
                      {{ incidentDuration(record) }}
                    </template>
                  </template>
                </a-table>
                <div class="incidents-footer">
                  <a-alert
                    v-if="hasOpenIncident"
                    type="error"
                    show-icon
                    message="本次发布存在未解决事故，请优先处理"
                  />
                  <a-alert
                    v-else-if="linkedIncidents.length > 0"
                    type="success"
                    show-icon
                    :message="`关联 ${linkedIncidents.length} 条事故，均已解决`"
                  />
                  <a-button
                    v-if="release && release.status === 'published'"
                    size="small"
                    type="link"
                    @click="showQuickIncident = true"
                  >
                    + 登记新事故
                  </a-button>
                </div>
              </a-tab-pane>
            </a-tabs>
          </a-card>
        </a-col>

        <!-- 右侧：风险评分 -->
        <a-col :span="8">
          <a-card title="风险评分" :bordered="false" size="small" style="margin-bottom: 16px">
            <template #extra>
              <RiskBadge :score="release?.risk_score" :level="release?.risk_level" />
            </template>
            <RiskRadar :hits="release?.risk_factors?.hits || []" :height="240" />
          </a-card>

          <a-card title="命中规则" :bordered="false" size="small">
            <a-list
              size="small"
              :data-source="release?.risk_factors?.hits || []"
              :locale="{ emptyText: '暂无命中规则' }"
            >
              <template #renderItem="{ item }">
                <a-list-item>
                  <a-list-item-meta>
                    <template #title>
                      <span>{{ item.name }}</span>
                      <a-tag color="orange" style="margin-left: 8px">+{{ item.weight }}</a-tag>
                    </template>
                    <template #description>
                      <code class="rule-key">{{ item.key }}</code>
                      <span v-if="item.detail" class="rule-detail">{{ item.detail }}</span>
                    </template>
                  </a-list-item-meta>
                </a-list-item>
              </template>
            </a-list>
          </a-card>
        </a-col>
      </a-row>
    </a-spin>

    <!-- 驳回弹窗 -->
    <a-modal
      v-model:open="showRejectModal"
      title="驳回原因"
      :confirm-loading="acting"
      @ok="onReject"
    >
      <a-textarea v-model:value="rejectReason" :rows="4" placeholder="请说明驳回原因（必填）" />
    </a-modal>

    <!-- 新增变更项 -->
    <ReleaseAddItemModal
      v-if="release"
      v-model:open="showAddItem"
      :release-id="release.id!"
      :default-app-id="release.application_id"
      :default-env="release.env"
      @added="fetchItems"
    />

    <!-- v2.2: 登记关联事故（内嵌，免去跳转） -->
    <a-modal
      v-model:open="showQuickIncident"
      title="登记关联事故"
      :confirm-loading="quickIncidentSubmitting"
      ok-text="创建"
      cancel-text="取消"
      @ok="onSubmitQuickIncident"
      @cancel="resetQuickIncident"
    >
      <a-form layout="vertical" :model="quickIncidentForm">
        <a-form-item label="标题" required>
          <a-input v-model:value="quickIncidentForm.title" placeholder="如：支付回调超时，影响下单转化" />
        </a-form-item>
        <a-form-item label="严重等级" required>
          <a-radio-group v-model:value="quickIncidentForm.severity">
            <a-radio-button value="P0">P0 紧急</a-radio-button>
            <a-radio-button value="P1">P1 高</a-radio-button>
            <a-radio-button value="P2">P2 中</a-radio-button>
            <a-radio-button value="P3">P3 低</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item label="影响描述">
          <a-textarea
            v-model:value="quickIncidentForm.description"
            :rows="3"
            placeholder="现象、影响面、已排查步骤等"
          />
        </a-form-item>
        <a-form-item label="发现时间">
          <a-date-picker
            v-model:value="quickIncidentForm.detected_at"
            show-time
            style="width: 100%"
            :disabled-date="(d: any) => d && d.valueOf() > Date.now()"
          />
        </a-form-item>
        <a-alert
          v-if="release"
          type="info"
          show-icon
          :message="`将自动关联发布 #${release.id} ${release.title}，环境 ${release.env}`"
        />
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
/**
 * Release 详情页（v2.0）
 *
 * 数据：
 *   - GET /releases/:id        基本信息（含 v2 字段：rollout_strategy / risk_score / risk_factors / gitops_change_request_id）
 *   - GET /releases/:id/items  变更项明细
 *
 * 操作：
 *   - draft → 提交审批
 *   - pending_approval → 通过 / 驳回
 *   - approved → GitOps PR（dry-run + 真实），Publish
 *   - pr_merged → Publish
 */
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { ReloadOutlined, PlusOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { releaseApi, type Release, type ReleaseItem, type ReleaseOverview, type ReleaseGateSummary } from '@/services/release'
import { incidentApi, type Incident } from '@/services/incident'
import RiskBadge from '@/components/release/RiskBadge.vue'
import RiskRadar from '@/components/release/RiskRadar.vue'
import ReleaseGitOpsPRButton from '@/components/release/ReleaseGitOpsPRButton.vue'
import ReleaseAddItemModal from '@/components/release/ReleaseAddItemModal.vue'

const route = useRoute()
const router = useRouter()

const release = ref<Release | null>(null)
const overview = ref<ReleaseOverview | null>(null)
const gateSummary = ref<ReleaseGateSummary | null>(null)
const items = ref<ReleaseItem[]>([])
const loading = ref(false)
const gatesLoading = ref(false)
const acting = ref(false)
const showRejectModal = ref(false)
const rejectReason = ref('')
const showAddItem = ref(false)

// 仅在 draft/pending_approval 允许编辑变更项，避免审批后再被动改动
const canEditItems = computed(() => {
  const s = release.value?.status
  return s === 'draft' || s === 'pending_approval'
})

// v2.1: 关联事故（通过 incidents?release_id=... 查询）
const activeBottomTab = ref<'items' | 'incidents'>('items')
const linkedIncidents = ref<Incident[]>([])
const incidentsLoading = ref(false)
const hasOpenIncident = computed(() =>
  linkedIncidents.value.some((i) => i.status !== 'resolved'),
)

const incidentColumns = [
  { title: '标题', key: 'title', dataIndex: 'title' },
  { title: '严重', key: 'severity', dataIndex: 'severity', width: 80 },
  { title: '状态', key: 'status', dataIndex: 'status', width: 100 },
  { title: '发现时间', key: 'detected_at', dataIndex: 'detected_at', width: 160 },
  { title: '持续时长', key: 'duration', width: 110 },
]

const idRef = computed(() => Number(route.params.id))

const itemColumns = [
  { title: '类型', key: 'item_type', dataIndex: 'item_type', width: 120 },
  { title: '标题', key: 'item_title', dataIndex: 'item_title' },
  { title: 'ItemID', key: 'item_id', dataIndex: 'item_id', width: 90 },
  { title: '状态', key: 'item_status', width: 100 },
  { title: '操作', key: 'action', width: 80 },
]

async function fetchRelease() {
  loading.value = true
  try {
    const res = await releaseApi.getById(idRef.value)
    release.value = (res as any)?.data || null
  } catch (e: any) {
    message.error(e?.response?.data?.message || '加载发布主单失败')
  } finally {
    loading.value = false
  }
}

async function fetchOverview() {
  try {
    const res = await releaseApi.getOverview(idRef.value)
    overview.value = (res as any)?.data || null
  } catch (e) {
    overview.value = null
  }
}

async function fetchGates() {
  gatesLoading.value = true
  try {
    const res = await releaseApi.getGates(idRef.value)
    gateSummary.value = (res as any)?.data || null
  } catch (e) {
    gateSummary.value = null
  } finally {
    gatesLoading.value = false
  }
}

async function refreshGates() {
  gatesLoading.value = true
  try {
    const res = await releaseApi.refreshGates(idRef.value)
    gateSummary.value = (res as any)?.data || null
    message.success('Gate 已刷新')
  } catch (e: any) {
    message.error(e?.response?.data?.message || '刷新 Gate 失败')
  } finally {
    gatesLoading.value = false
  }
}

async function fetchItems() {
  try {
    const res = await releaseApi.listItems(idRef.value)
    items.value = ((res as any)?.data || []) as ReleaseItem[]
  } catch (e) {
    items.value = []
  }
}

async function fetchIncidents() {
  if (!idRef.value) return
  incidentsLoading.value = true
  try {
    const res = await incidentApi.list({ release_id: idRef.value, pageSize: 100 })
    const data: any = (res as any)?.data || {}
    linkedIncidents.value = (data.list || data.data || []) as Incident[]
  } catch (e) {
    linkedIncidents.value = []
  } finally {
    incidentsLoading.value = false
  }
}

async function fetchAll() {
  await Promise.all([fetchRelease(), fetchOverview(), fetchGates(), fetchItems(), fetchIncidents()])
}

function goIncident(id: number) {
  router.push(`/incidents/${id}`)
}
function onLinkIncident() {
  // 跳转到登记事故并预填 release_id（备用入口，已由 showQuickIncident 代替）
  router.push({ path: '/incidents', query: { new: '1', release_id: idRef.value } })
}

// v2.2: 内嵌登记关联事故，避免跳转丢失发布上下文
const showQuickIncident = ref(false)
const quickIncidentSubmitting = ref(false)
interface QuickIncidentForm {
  title: string
  severity: 'P0' | 'P1' | 'P2' | 'P3'
  description: string
  detected_at: any // dayjs
}
const quickIncidentForm = ref<QuickIncidentForm>({
  title: '',
  severity: 'P2',
  description: '',
  detected_at: dayjs(),
})
function resetQuickIncident() {
  quickIncidentForm.value = {
    title: '',
    severity: 'P2',
    description: '',
    detected_at: dayjs(),
  }
}
async function onSubmitQuickIncident() {
  if (!quickIncidentForm.value.title.trim()) {
    message.warning('请填写事故标题')
    return
  }
  if (!release.value) return
  quickIncidentSubmitting.value = true
  try {
    await incidentApi.create({
      title: quickIncidentForm.value.title.trim(),
      description: quickIncidentForm.value.description.trim(),
      severity: quickIncidentForm.value.severity,
      env: release.value.env,
      application_id: release.value.application_id,
      application_name: release.value.application_name,
      release_id: release.value.id,
      detected_at: quickIncidentForm.value.detected_at
        ? quickIncidentForm.value.detected_at.toISOString()
        : undefined,
      source: 'manual',
    } as any)
    message.success('已登记事故，并关联到本发布')
    showQuickIncident.value = false
    resetQuickIncident()
    await fetchIncidents()
    activeBottomTab.value = 'incidents'
  } catch (e: any) {
    message.error(e?.response?.data?.message || '登记事故失败')
  } finally {
    quickIncidentSubmitting.value = false
  }
}

function incidentSeverityColor(s: string) {
  return s === 'P0' ? 'red' : s === 'P1' ? 'volcano' : s === 'P2' ? 'orange' : 'default'
}
function incidentStatusColor(s: string) {
  return s === 'resolved' ? 'green' : s === 'mitigated' ? 'gold' : 'red'
}
function incidentStatusLabel(s: string) {
  return s === 'resolved' ? '已解决' : s === 'mitigated' ? '已止血' : '未解决'
}
function incidentDuration(row: Incident) {
  if (!row.detected_at) return '-'
  const end = row.resolved_at ? dayjs(row.resolved_at) : dayjs()
  const mins = end.diff(dayjs(row.detected_at), 'minute')
  if (mins < 60) return mins + ' 分钟'
  if (mins < 24 * 60) return (mins / 60).toFixed(1) + ' 小时'
  return (mins / 60 / 24).toFixed(1) + ' 天'
}

async function onSubmit() {
  acting.value = true
  try {
    await releaseApi.submit(idRef.value)
    message.success('已提交审批')
    fetchAll()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '提交失败')
  } finally {
    acting.value = false
  }
}

async function onApprove() {
  acting.value = true
  try {
    await releaseApi.approve(idRef.value)
    message.success('已通过')
    fetchAll()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '审批失败')
  } finally {
    acting.value = false
  }
}

async function onReject() {
  if (!rejectReason.value.trim()) {
    message.warning('请填写驳回原因')
    return
  }
  acting.value = true
  try {
    await releaseApi.reject(idRef.value, rejectReason.value)
    message.success('已驳回')
    showRejectModal.value = false
    rejectReason.value = ''
    fetchAll()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '驳回失败')
  } finally {
    acting.value = false
  }
}

async function onPublish() {
  if (gateSummary.value?.blocked) {
    message.error(gateSummary.value.next_action || '发布 Gate 未通过')
    return
  }
  acting.value = true
  try {
    await releaseApi.publish(idRef.value)
    message.success('已发布')
    fetchAll()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '发布失败')
  } finally {
    acting.value = false
  }
}

async function onRemoveItem(itemId: number) {
  try {
    await releaseApi.removeItem(idRef.value, itemId)
    message.success('已移除')
    fetchAll()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '移除失败')
  }
}

function onPROpened() {
  fetchAll()
}

function goBack() {
  router.push('/releases')
}

function goCR(id: number) {
  router.push({ path: '/argocd', query: { tab: 'changes', changeId: String(id) } })
}

function rolloutLabel(s?: string) {
  return s === 'canary' ? '金丝雀' : s === 'blue_green' ? '蓝绿' : '直接发布'
}
function rolloutColor(s?: string) {
  return s === 'canary' ? 'orange' : s === 'blue_green' ? 'geekblue' : 'default'
}

function overviewStageText(s?: string) {
  return s === 'finish' ? '已完成' : s === 'process' ? '进行中' : s === 'error' ? '阻塞' : '等待中'
}

function approvalStatusLabel(s?: string) {
  const map: Record<string, string> = {
    none: '无需审批',
    pending: '待审批',
    approved: '已通过',
    rejected: '已拒绝',
    cancelled: '已取消',
  }
  return s ? (map[s] || s) : '-'
}

function approvalStatusColor(s?: string) {
  return s === 'approved' ? 'green' : s === 'pending' ? 'gold' : s === 'rejected' ? 'red' : 'default'
}

function gitopsStatusColor(s?: string) {
  return s === 'failed' ? 'red' : s === 'open' || s === 'submitted' ? 'blue' : s === 'merged' ? 'green' : 'default'
}

function argoSyncColor(s?: string) {
  return s === 'Synced' ? 'green' : s === 'OutOfSync' ? 'orange' : s ? 'default' : 'default'
}

function gateStatusColor(s?: string) {
  return s === 'pass' ? 'green' : s === 'warn' ? 'orange' : s === 'block' ? 'red' : s === 'skip' ? 'default' : 'default'
}
function gateStatusText(s?: string) {
  return ({ pass: '通过', warn: '提醒', block: '阻断', skip: '跳过' } as Record<string, string>)[s || ''] || s || '-'
}
function gatePolicyText(s?: string) {
  return ({ required: '强校验', advisory: '观察', manual: '人工确认' } as Record<string, string>)[s || ''] || s || '-'
}

const statusOptions: Record<string, { label: string; color: string }> = {
  draft: { label: '草稿', color: 'default' },
  pending_approval: { label: '待审批', color: 'gold' },
  approved: { label: '已审批', color: 'blue' },
  rejected: { label: '已驳回', color: 'red' },
  pr_opened: { label: 'PR 已提交', color: 'cyan' },
  pr_merged: { label: 'PR 已合并', color: 'geekblue' },
  published: { label: '已发布', color: 'green' },
  failed: { label: '失败', color: 'red' },
  rolled_back: { label: '已回滚', color: 'volcano' },
}
function statusLabel(s?: string) {
  return statusOptions[s || '']?.label || s || '-'
}
function statusColor(s?: string) {
  return statusOptions[s || '']?.color || 'default'
}

const itemTypeOptions: Record<string, { label: string; color: string }> = {
  pipeline_run: { label: '流水线', color: 'blue' },
  deployment: { label: '部署', color: 'green' },
  nacos_release: { label: 'Nacos 配置', color: 'purple' },
  database: { label: '数据库变更', color: 'red' },
  sql_ticket: { label: 'SQL 工单', color: 'red' },
  manual: { label: '手工任务', color: 'default' },
}
function itemTypeLabel(t?: string) {
  return itemTypeOptions[t || '']?.label || t || '-'
}
function itemTypeColor(t?: string) {
  return itemTypeOptions[t || '']?.color || 'default'
}

function formatTime(t?: string) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm:ss') : '-'
}

watch(idRef, (id) => {
  if (id > 0) fetchAll()
})

onMounted(fetchAll)
</script>

<style scoped>
.release-detail {
  padding: 0;
}
.muted {
  color: #999;
}
.danger {
  color: #ff4d4f;
}
.overview-card {
  margin-bottom: 16px;
}
.overview-next {
  color: #555;
}
.overview-stage {
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  padding: 10px 12px;
  background: #fafafa;
}
.overview-stage.stage-finish {
  background: #f6ffed;
  border-color: #b7eb8f;
}
.overview-stage.stage-process {
  background: #e6f4ff;
  border-color: #91caff;
}
.overview-stage.stage-error {
  background: #fff1f0;
  border-color: #ffa39e;
}
.stage-label {
  font-weight: 600;
}
.stage-message {
  margin-top: 4px;
  color: #666;
  font-size: 12px;
}
.overview-facts {
  margin-top: 12px;
}
.overview-facts .muted {
  margin-right: 8px;
}
.desc-text {
  margin: 0;
  white-space: pre-wrap;
  font-family: inherit;
  font-size: 13px;
  color: #444;
}
.rule-key {
  font-size: 12px;
  color: #888;
  background: #fafafa;
  padding: 1px 6px;
  border-radius: 3px;
  margin-right: 8px;
}
.rule-detail {
  font-size: 12px;
  color: #666;
}
.incidents-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-top: 12px;
}
.incidents-footer .ant-alert {
  flex: 1;
}
</style>
