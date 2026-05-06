<template>
  <div class="pipeline-detail">
    <a-page-header :title="pipeline?.name || '流水线详情'" @back="goBack">
      <template #subTitle>
        <a-space>
          <a-tag :color="pipeline?.status === 'active' ? 'green' : 'default'">
            {{ pipeline?.status === 'active' ? '启用' : '禁用' }}
          </a-tag>
          <span v-if="pipeline?.description" style="color: #999">{{ pipeline.description }}</span>
        </a-space>
      </template>
      <template #extra>
        <a-space>
          <a-button @click="runPipeline" type="primary" :disabled="pipeline?.status !== 'active'">
            <PlayCircleOutlined /> 执行
          </a-button>
          <a-button @click="editPipeline">
            <EditOutlined /> 编辑
          </a-button>
          <a-button @click="yamlExportOpen = true">
            <ExportOutlined /> 导出 YAML
          </a-button>
          <a-dropdown>
            <a-button><MoreOutlined /></a-button>
            <template #overlay>
              <a-menu>
                <a-menu-item @click="toggleStatus">
                  {{ pipeline?.status === 'active' ? '禁用' : '启用' }}
                </a-menu-item>
                <a-menu-divider />
                <a-menu-item danger @click="confirmDelete">删除</a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </a-space>
      </template>
    </a-page-header>

    <a-row :gutter="16">
      <!-- 左侧：流水线信息 -->
      <a-col :span="8">
        <a-card title="基本信息" size="small" :loading="loading">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="流水线ID">{{ pipeline?.id }}</a-descriptions-item>
            <a-descriptions-item label="Git 仓库">
              <template v-if="pipeline?.git_repo_url">
                <a :href="pipeline.git_repo_url" target="_blank">{{ pipeline.git_repo_name || getRepoName(pipeline.git_repo_url) }}</a>
              </template>
              <span v-else style="color: #999">-</span>
            </a-descriptions-item>
            <a-descriptions-item label="默认分支">{{ pipeline?.git_branch || 'main' }}</a-descriptions-item>
            <a-descriptions-item label="构建方式">GitLab Runner</a-descriptions-item>
            <a-descriptions-item label="CI 配置">{{ pipeline?.ci_config_path || '.gitlab-ci.yml' }}</a-descriptions-item>
            <a-descriptions-item label="Dockerfile">CI 运行时渲染 {{ pipeline?.dockerfile_path || '.jeridevops.Dockerfile' }}</a-descriptions-item>
            <a-descriptions-item label="创建时间">{{ formatTime(pipeline?.created_at) }}</a-descriptions-item>
            <a-descriptions-item label="更新时间">{{ formatTime(pipeline?.updated_at) }}</a-descriptions-item>
          </a-descriptions>
        </a-card>

        <a-card title="阶段配置" size="small" style="margin-top: 16px" :loading="loading">
          <a-timeline v-if="pipeline?.stages?.length">
            <a-timeline-item v-for="stage in pipeline.stages" :key="stage.id" color="blue">
              <div style="font-weight: 500">{{ stage.name }}</div>
              <div style="color: #999; font-size: 12px">{{ stage.steps?.length || 0 }} 个步骤</div>
            </a-timeline-item>
          </a-timeline>
          <a-empty v-else description="暂无阶段配置" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
        </a-card>
      </a-col>

      <!-- 右侧：执行历史 -->
      <a-col :span="16">
        <a-card title="执行历史" size="small">
          <template #extra>
            <a-button type="link" size="small" @click="loadRuns">
              <ReloadOutlined /> 刷新
            </a-button>
          </template>

          <a-table
            :columns="runColumns"
            :data-source="runs"
            :loading="runsLoading"
            :pagination="runsPagination"
            @change="handleRunsTableChange"
            row-key="id"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'id'">
                <a-popover
                  trigger="hover"
                  placement="rightTop"
                  :mouse-enter-delay="0.3"
                  :mouse-leave-delay="0.1"
                  overlay-class-name="run-tail-popover"
                >
                  <template #content>
                    <RunLogTailPreview :run-id="record.id" />
                  </template>
                  <a @click="showRunDetail(record)">#{{ record.id }}</a>
                </a-popover>
              </template>
              <template v-if="column.key === 'status'">
                <a-tag :color="getStatusColor(record.status)">{{ getStatusText(record.status) }}</a-tag>
              </template>
              <template v-if="column.key === 'trigger'">
                <span>{{ record.trigger_type }} / {{ record.trigger_by }}</span>
              </template>
              <template v-if="column.key === 'duration'">
                {{ formatDuration(record.duration) }}
              </template>
              <template v-if="column.key === 'started_at'">
                {{ record.started_at || '-' }}
              </template>
              <template v-if="column.key === 'action'">
                <a-space>
                  <a-button type="link" size="small" @click="showRunDetail(record)">日志</a-button>
                  <a-button type="link" size="small" @click="cancelRun(record)" v-if="record.status === 'running'" danger>取消</a-button>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-col>
    </a-row>

    <!-- 执行流水线弹窗 -->
    <a-modal v-model:open="showRunModal" title="执行流水线" @ok="handleRun" :confirmLoading="running">
      <!-- FE-03: 智能默认提示 -->
      <a-alert
        v-if="lastUsedConfig?.has_value"
        type="info"
        show-icon
        style="margin-bottom: 12px"
      >
        <template #message>
          <div style="display: flex; justify-content: space-between; align-items: center; gap: 12px">
            <span>
              上次运行：分支 <strong>{{ lastUsedConfig.branch || '-' }}</strong>
              <span v-if="lastUsedConfig.parameters && Object.keys(lastUsedConfig.parameters).length">
                · {{ Object.keys(lastUsedConfig.parameters).length }} 个参数
              </span>
            </span>
            <a-button type="link" size="small" @click="applyLastUsedConfig">使用上次配置</a-button>
          </div>
        </template>
      </a-alert>

      <a-form :label-col="{ span: 6 }">
        <a-form-item label="Git Ref" v-if="pipeline?.git_repo_id">
          <a-radio-group v-model:value="runForm.ref_type" style="margin-bottom: 8px">
            <a-radio-button value="branch">分支</a-radio-button>
            <a-radio-button value="tag">Tag</a-radio-button>
          </a-radio-group>
          <a-auto-complete
            v-model:value="runForm.ref"
            :options="refOptions"
            :placeholder="runForm.ref_type === 'branch' ? '选择或输入分支' : '选择或输入 Tag'"
            style="width: 100%"
          />
        </a-form-item>
        <a-form-item label="参数 (JSON)">
          <a-textarea v-model:value="runForm.parameters_json" placeholder='{"key": "value"}' :rows="4" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 执行详情抽屉 -->
    <a-drawer v-model:open="runDetailVisible" title="执行详情" width="700" :footer="null">
      <template v-if="currentRun">
        <DiagnosisCard
          :run-id="currentRun.id"
          :run-status="currentRun.status"
          @open-similar="onOpenSimilar"
        />
        <a-descriptions :column="2" size="small" style="margin-bottom: 16px">
          <a-descriptions-item label="执行ID">#{{ currentRun.id }}</a-descriptions-item>
          <a-descriptions-item label="状态">
            <a-tag :color="getStatusColor(currentRun.status)">{{ getStatusText(currentRun.status) }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="触发方式">{{ currentRun.trigger_type }}</a-descriptions-item>
          <a-descriptions-item label="触发者">{{ currentRun.trigger_by }}</a-descriptions-item>
          <a-descriptions-item label="Git 分支">{{ currentRun.git_branch || '-' }}</a-descriptions-item>
          <a-descriptions-item label="GitLab Pipeline">
            <a v-if="currentRun.external_url" :href="currentRun.external_url" target="_blank" rel="noreferrer">打开 GitLab</a>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="开始时间">{{ currentRun.started_at || '-' }}</a-descriptions-item>
          <a-descriptions-item label="耗时">{{ formatDuration(currentRun.duration) }}</a-descriptions-item>
        </a-descriptions>
        <a-alert
          v-if="currentRun.status === 'success' && currentRun.gitops_handoff?.status === 'failed'"
          type="warning"
          show-icon
          style="margin-bottom: 16px"
          message="CI 已成功，但 GitOps 自动交接失败，当前不能视为部署成功。"
          :description="currentRun.gitops_handoff.message || '请检查 GitOps 仓库凭证、目标分支和 Helm values 路径后重新执行。'"
        />

        <a-row :gutter="16" style="margin-bottom: 16px">
          <a-col :span="12">
            <a-card size="small" title="镜像扫描门禁">
              <template v-if="currentRun.image_scan">
                <a-descriptions :column="1" size="small">
                  <a-descriptions-item label="镜像">{{ currentRun.image_scan.image || '-' }}</a-descriptions-item>
                  <a-descriptions-item label="扫描状态">
                    <a-tag :color="getScanStatusColor(currentRun.image_scan.status)">{{ getScanStatusText(currentRun.image_scan.status) }}</a-tag>
                  </a-descriptions-item>
                  <a-descriptions-item label="风险等级">
                    <a-tag :color="getRiskLevelColor(currentRun.image_scan.risk_level)">{{ currentRun.image_scan.risk_level || '-' }}</a-tag>
                  </a-descriptions-item>
                  <a-descriptions-item label="漏洞统计">
                    严重 {{ currentRun.image_scan.critical || 0 }} / 高危 {{ currentRun.image_scan.high || 0 }} / 中危 {{ currentRun.image_scan.medium || 0 }} / 低危 {{ currentRun.image_scan.low || 0 }}
                  </a-descriptions-item>
                </a-descriptions>
              </template>
              <a-empty v-else description="本次运行未触发镜像扫描" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
            </a-card>
          </a-col>
          <a-col :span="12">
            <a-card size="small" title="GitOps 自动交接">
              <template v-if="currentRun.gitops_handoff">
                <a-descriptions :column="1" size="small">
                  <a-descriptions-item label="交接状态">
                    <a-tag :color="getGitOpsHandoffColor(currentRun.gitops_handoff.status)">
                      {{ getGitOpsHandoffText(currentRun.gitops_handoff.status) }}
                    </a-tag>
                  </a-descriptions-item>
                  <a-descriptions-item label="变更请求">
                    <template v-if="currentRun.gitops_handoff.change_request_id">
                      <a @click="goToGitOpsChange(currentRun.gitops_handoff.change_request_id)">
                        #{{ currentRun.gitops_handoff.change_request_id }} {{ currentRun.gitops_handoff.change_request_title || '' }}
                      </a>
                    </template>
                    <span v-else>-</span>
                  </a-descriptions-item>
                  <a-descriptions-item label="审批状态">
                    <span>{{ getApprovalStatusText(currentRun.gitops_handoff.approval_status) }}</span>
                    <a v-if="currentRun.gitops_handoff.approval_instance_id" style="margin-left: 8px" @click="goToApproval(currentRun.gitops_handoff.approval_instance_id)">审批详情</a>
                  </a-descriptions-item>
                  <a-descriptions-item label="自动合并">{{ getAutoMergeText(currentRun.gitops_handoff.auto_merge_status) }}</a-descriptions-item>
                  <a-descriptions-item label="MR">
                    <a v-if="currentRun.gitops_handoff.merge_request_url" :href="currentRun.gitops_handoff.merge_request_url" target="_blank" rel="noreferrer">查看 MR</a>
                    <span v-else>-</span>
                  </a-descriptions-item>
                  <a-descriptions-item v-if="currentRun.gitops_handoff.message" label="说明">
                    {{ currentRun.gitops_handoff.message }}
                  </a-descriptions-item>
                </a-descriptions>
              </template>
              <a-empty v-else description="本次运行未启用 GitOps 自动交接" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
            </a-card>
          </a-col>
        </a-row>

        <a-collapse v-model:activeKey="logsActiveKey" accordion>
          <a-collapse-panel v-for="stage in currentRun.stage_runs" :key="stage.id" :header="stage.stage_name">
            <template #extra>
              <a-tag :color="getStatusColor(stage.status)" size="small">{{ getStatusText(stage.status) }}</a-tag>
            </template>
            <div v-for="step in stage.step_runs" :key="step.id" class="step-log-item">
              <div class="step-header">
                <span>{{ step.step_name }}</span>
                <a-tag :color="getStatusColor(step.status)" size="small">{{ getStatusText(step.status) }}</a-tag>
              </div>
              <pre class="step-logs">{{ step.logs || '暂无日志' }}</pre>
            </div>
          </a-collapse-panel>
        </a-collapse>
      </template>
    </a-drawer>

    <!-- FE-05: 历史相似 run 抽屉 -->
    <SimilarRunsDrawer
      v-model:open="similarDrawerOpen"
      :diagnosis="lastDiagnosis"
      @jump-to-run="onJumpToSimilarRun"
    />

    <!-- FE-11: YAML 导出模态 -->
    <YamlExportModal
      v-model:open="yamlExportOpen"
      :pipeline-id="pipelineId"
      :pipeline-name="pipeline?.name"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message, Modal, Empty } from 'ant-design-vue'
import { PlayCircleOutlined, EditOutlined, MoreOutlined, ReloadOutlined, ExportOutlined } from '@ant-design/icons-vue'
import { pipelineApi, gitRepoApi } from '@/services/pipeline'
import DiagnosisCard from '@/components/pipeline/DiagnosisCard.vue'
import SimilarRunsDrawer from '@/components/pipeline/SimilarRunsDrawer.vue'
import YamlExportModal from '@/components/pipeline/YamlExportModal.vue'
import RunLogTailPreview from '@/components/pipeline/RunLogTailPreview.vue'
import type { FailureDiagnosis } from '@/services/diagnosis'
import { useRecentsStore } from '@/stores/recents'

const recentsStore = useRecentsStore()

const route = useRoute()
const router = useRouter()
const pipelineId = ref(Number(route.params.id))

const loading = ref(false)
const runsLoading = ref(false)
const running = ref(false)
const pipeline = ref<any>(null)
const runs = ref<any[]>([])
const showRunModal = ref(false)
const runDetailVisible = ref(false)
const currentRun = ref<any>(null)
const logsActiveKey = ref<number[]>([])
let runDetailTimer: number | undefined
const branches = ref<string[]>([])
const tags = ref<string[]>([])
const refOptions = ref<{value: string}[]>([])

// FE-04 / FE-05: 失败诊断卡 + 相似 run 抽屉
const similarDrawerOpen = ref(false)
const lastDiagnosis = ref<FailureDiagnosis | null>(null)

// FE-11: YAML 导出模态
const yamlExportOpen = ref(false)

// FE-03: 智能默认 — 上次运行的 ref + parameters
interface LastUsedSummary {
  has_value: boolean
  run_id?: number
  branch?: string
  commit?: string
  parameters?: Record<string, unknown>
  status?: string
  happened_at?: string
}
const lastUsedConfig = ref<LastUsedSummary | null>(null)

const runForm = reactive({
  ref_type: 'branch' as 'branch' | 'tag',
  ref: '',
  parameters_json: '{}'
})

const runsPagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0
})

const runColumns = [
  { title: '#', key: 'id', width: 80 },
  { title: '状态', key: 'status', width: 100 },
  { title: '触发', key: 'trigger', width: 150 },
  { title: '耗时', key: 'duration', width: 100 },
  { title: '开始时间', key: 'started_at', width: 180 },
  { title: '操作', key: 'action', width: 120 }
]

const getStatusColor = (status: string) => ({ success: 'green', running: 'blue', failed: 'red', cancelled: 'orange', pending: 'default' }[status] || 'default')
const getStatusText = (status: string) => ({ success: 'CI 成功', running: '运行中', failed: 'CI 失败', cancelled: '已取消', pending: '等待中' }[status] || status)
const formatDuration = (seconds: number) => {
  if (!seconds) return '-'
  if (seconds < 60) return `${seconds}秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}分${seconds % 60}秒`
  return `${Math.floor(seconds / 3600)}时${Math.floor((seconds % 3600) / 60)}分`
}
const formatTime = (time: string) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}
const getRepoName = (url: string) => {
  if (!url) return ''
  const match = url.match(/[:/]([^/:]+\/[^/.]+)(\.git)?$/)
  return match ? match[1] : url
}

const loadPipeline = async () => {
  loading.value = true
  try {
    const res = await pipelineApi.get(pipelineId.value)
    pipeline.value = res?.data || res
    if (pipeline.value?.id && pipeline.value?.name) {
      recentsStore.addRecent({
        id: pipeline.value.id,
        name: pipeline.value.name,
        description: pipeline.value.description,
      })
    }
  } catch (error) {
    message.error('加载流水线失败')
  } finally {
    loading.value = false
  }
}

const loadRuns = async () => {
  runsLoading.value = true
  try {
    const res = await pipelineApi.listRuns({
      pipeline_id: pipelineId.value,
      page: runsPagination.current,
      page_size: runsPagination.pageSize
    })
    runs.value = res?.data?.items || []
    runsPagination.total = res?.data?.total || 0
  } catch (error) {
    console.error('加载执行历史失败', error)
  } finally {
    runsLoading.value = false
  }
}

const loadBranchesAndTags = async () => {
  if (!pipeline.value?.git_repo_id) return
  try {
    const [branchRes, tagRes] = await Promise.allSettled([
      gitRepoApi.getBranches(pipeline.value.git_repo_id),
      gitRepoApi.getTags(pipeline.value.git_repo_id)
    ])
    if (branchRes.status === 'fulfilled') {
      branches.value = (branchRes.value?.data || []).map((item: any) => typeof item === 'string' ? item : item.name)
    }
    if (tagRes.status === 'fulfilled') {
      tags.value = (tagRes.value?.data || []).map((item: any) => typeof item === 'string' ? item : item.name)
    }
    refOptions.value = branches.value.map(b => ({ value: b }))
  } catch (error) {
    console.error('加载分支/Tag失败', error)
  }
}

watch(() => runForm.ref_type, (type) => {
  refOptions.value = type === 'branch' 
    ? branches.value.map(b => ({ value: b }))
    : tags.value.map(t => ({ value: t }))
  runForm.ref = type === 'branch' ? (pipeline.value?.git_branch || 'main') : (tags.value[0] || '')
})

const runPipeline = () => {
  runForm.ref = pipeline.value?.git_branch || 'main'
  runForm.ref_type = 'branch'
  runForm.parameters_json = '{}'
  showRunModal.value = true
  loadBranchesAndTags()
  loadLastUsedConfig()
}

// FE-03: 拉取上次运行配置（轻量异步，不阻塞 modal 打开）
const loadLastUsedConfig = async () => {
  lastUsedConfig.value = null
  try {
    const res: any = await pipelineApi.getLastRunConfig(pipelineId.value)
    lastUsedConfig.value = res?.data || null
  } catch {
    lastUsedConfig.value = null
  }
}

// FE-03: "使用上次配置" 按钮回填表单
const applyLastUsedConfig = () => {
  const lu = lastUsedConfig.value
  if (!lu || !lu.has_value) return
  if (lu.branch) {
    runForm.ref = lu.branch
    runForm.ref_type = 'branch'
  }
  if (lu.parameters && Object.keys(lu.parameters).length) {
    try {
      runForm.parameters_json = JSON.stringify(lu.parameters, null, 2)
    } catch {
      /* 极端情况：参数无法序列化，保留原值 */
    }
  } else {
    runForm.parameters_json = '{}'
  }
  message.success('已套用上次运行配置')
}

const handleRun = async () => {
  running.value = true
  try {
    let params = {}
    if (runForm.parameters_json) {
      params = JSON.parse(runForm.parameters_json)
    }
    await pipelineApi.run(pipelineId.value, { parameters: params, branch: runForm.ref || undefined })
    message.success('流水线已开始执行')
    showRunModal.value = false
    loadRuns()
  } catch (error: any) {
    message.error(error.message || '执行失败')
  } finally {
    running.value = false
  }
}

const isLiveRunStatus = (status?: string) => status === 'running' || status === 'pending'

const stopRunDetailPolling = () => {
  if (runDetailTimer) {
    window.clearInterval(runDetailTimer)
    runDetailTimer = undefined
  }
}

const syncLogsActiveKey = () => {
  const stages = currentRun.value?.stage_runs || []
  if (!stages.length) {
    logsActiveKey.value = []
    return
  }
  const active = Number(logsActiveKey.value?.[0])
  if (!active || !stages.some((stage: any) => Number(stage.id) === active)) {
    logsActiveKey.value = [stages[0].id]
  }
}

const refreshRunDetail = async (runId: number, silent = false) => {
  try {
    const res = await pipelineApi.getRun(runId)
    currentRun.value = res?.data || res
    syncLogsActiveKey()
  } catch (error) {
    if (!silent) message.error('加载执行详情失败')
    throw error
  }
}

const startRunDetailPolling = () => {
  stopRunDetailPolling()
  if (!runDetailVisible.value || !currentRun.value?.id || !isLiveRunStatus(currentRun.value.status)) return
  runDetailTimer = window.setInterval(async () => {
    if (!runDetailVisible.value || !currentRun.value?.id) {
      stopRunDetailPolling()
      return
    }
    await refreshRunDetail(Number(currentRun.value.id), true).catch(() => undefined)
    if (!isLiveRunStatus(currentRun.value?.status)) {
      stopRunDetailPolling()
      loadRuns()
    }
  }, 5000)
}

const showRunDetail = async (record: any) => {
  try {
    await refreshRunDetail(Number(record.id))
    runDetailVisible.value = true
    startRunDetailPolling()
  } catch {
    // refreshRunDetail already displays the user-facing error.
  }
}

const openRunFromRoute = async (value: unknown) => {
  const raw = Array.isArray(value) ? value[0] : value
  const runId = Number(raw)
  if (!raw || !Number.isFinite(runId) || runId <= 0) return
  if (runDetailVisible.value && Number(currentRun.value?.id) === runId) return
  await showRunDetail({ id: runId })
}

watch(() => route.query.run, (runId) => {
  openRunFromRoute(runId)
})

watch(runDetailVisible, (open) => {
  if (open) {
    startRunDetailPolling()
  } else {
    stopRunDetailPolling()
  }
})

// FE-04 → FE-05 桥接：DiagnosisCard 触发"查看历史相似"时打开抽屉
const onOpenSimilar = (diag: FailureDiagnosis) => {
  lastDiagnosis.value = diag
  similarDrawerOpen.value = true
}

// FE-05 → 详情：抽屉中点击 "Run #N" 时跳到该 run 的详情
const onJumpToSimilarRun = async (runId: number) => {
  await showRunDetail({ id: runId })
}

const getScanStatusColor = (status?: string) => {
  switch (status) {
    case 'completed':
    case 'success':
      return 'green'
    case 'failed':
      return 'red'
    case 'running':
      return 'blue'
    default:
      return 'default'
  }
}

const getScanStatusText = (status?: string) => {
  const map: Record<string, string> = {
    completed: '已完成',
    success: '已完成',
    failed: '失败',
    running: '扫描中'
  }
  return status ? (map[status] || status) : '-'
}

const getRiskLevelColor = (level?: string) => {
  switch ((level || '').toLowerCase()) {
    case 'critical':
    case 'high':
      return 'red'
    case 'medium':
      return 'orange'
    case 'low':
      return 'green'
    default:
      return 'default'
  }
}

const getGitOpsHandoffColor = (status?: string) => {
  switch (status) {
    case 'created':
      return 'green'
    case 'skipped':
      return 'default'
    case 'failed':
      return 'red'
    case 'pending':
      return 'blue'
    default:
      return 'default'
  }
}

const getGitOpsHandoffText = (status?: string) => {
  const map: Record<string, string> = {
    created: '已创建变更',
    skipped: '已跳过',
    failed: '交接失败',
    pending: '处理中'
  }
  return status ? (map[status] || status) : '-'
}

const getApprovalStatusText = (status?: string) => {
  const map: Record<string, string> = {
    none: '未挂接',
    pending: '待审批',
    approved: '已通过',
    rejected: '已拒绝',
    cancelled: '已取消'
  }
  return status ? (map[status] || status) : '-'
}

const getAutoMergeText = (status?: string) => {
  const map: Record<string, string> = {
    manual: '待人工合并',
    pending: '待审批后合并',
    success: '已自动合并',
    failed: '自动合并失败',
    skipped: '已跳过'
  }
  return status ? (map[status] || status) : '-'
}

const goToGitOpsChange = (changeId: number) => {
  router.push({ path: '/argocd', query: { tab: 'changes', changeId: String(changeId) } })
}

const goToApproval = (approvalInstanceId: number) => {
  router.push(`/approval/instances/${approvalInstanceId}`)
}

const cancelRun = async (record: any) => {
  Modal.confirm({
    title: '确认取消',
    content: '确定要取消此次执行吗？',
    onOk: async () => {
      try {
        await pipelineApi.cancelRun(record.id)
        message.success('已取消')
        loadRuns()
      } catch (error: any) {
        message.error(error.message || '取消失败')
      }
    }
  })
}

const editPipeline = () => router.push(`/pipeline/edit/${pipelineId.value}`)
const goBack = () => router.push('/pipeline/list')

const toggleStatus = async () => {
  try {
    await pipelineApi.toggle(pipelineId.value)
    message.success('状态已更新')
    loadPipeline()
  } catch (error: any) {
    message.error(error.message || '操作失败')
  }
}

const confirmDelete = () => {
  Modal.confirm({
    title: '确认删除',
    content: `确定要删除流水线 "${pipeline.value?.name}" 吗？`,
    okType: 'danger',
    onOk: async () => {
      try {
        await pipelineApi.delete(pipelineId.value)
        message.success('删除成功')
        router.push('/pipeline/list')
      } catch (error: any) {
        message.error(error.message || '删除失败')
      }
    }
  })
}

const handleRunsTableChange = (pag: any) => {
  runsPagination.current = pag.current
  runsPagination.pageSize = pag.pageSize
  loadRuns()
}

onMounted(() => {
  loadPipeline()
  loadRuns()
  openRunFromRoute(route.query.run)
})

onUnmounted(() => {
  stopRunDetailPolling()
})
</script>

<style scoped>
.pipeline-detail {
  padding: 0;
}
.step-log-item {
  margin-bottom: 12px;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
  overflow: hidden;
}
.step-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #fafafa;
  border-bottom: 1px solid #f0f0f0;
}
.step-logs {
  margin: 0;
  padding: 12px;
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  max-height: 300px;
  overflow: auto;
  white-space: pre-wrap;
  word-wrap: break-word;
}
</style>
