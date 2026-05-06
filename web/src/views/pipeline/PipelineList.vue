<template>
  <div class="pipeline-list">
    <a-row :gutter="[16, 16]" class="delivery-overview-row">
      <a-col :span="24">
        <a-card :bordered="false" class="delivery-hero">
          <a-row :gutter="16" align="middle">
            <a-col :xs="24" :lg="16">
              <div class="hero-title">编译构建部署工作台</div>
              <div class="hero-subtitle">从代码仓库、流水线构建到发布执行，统一查看当前交付状态与下一步动作。</div>
            </a-col>
            <a-col :xs="24" :lg="8">
              <a-space wrap class="hero-actions">
                <a-button type="primary" @click="$router.push('/pipeline/create')">新建流水线</a-button>
                <a-button @click="goTo('/pipeline/git-repos')">Git 仓库</a-button>
                <a-button @click="goTo('/argocd')">GitOps 交付</a-button>
              </a-space>
            </a-col>
          </a-row>
        </a-card>
      </a-col>

      <a-col :xs="24" :md="12" :xl="6">
        <a-card :bordered="false" class="summary-card">
          <a-statistic title="流水线总数" :value="pagination.total" />
          <div class="summary-extra">当前页启用 {{ activePipelines }} 条</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :md="12" :xl="6">
        <a-card :bordered="false" class="summary-card">
          <a-statistic title="最近构建运行中" :value="runningRunsCount" :value-style="{ color: '#1677ff' }" />
          <div class="summary-extra">最近 CI 成功 {{ recentSuccessCount }} 条</div>
        </a-card>
      </a-col>

      <a-col :xs="24" :lg="16">
        <a-card title="交付流程" :bordered="false" class="workbench-card">
          <a-steps :current="deliveryFlowCurrent" size="small">
            <a-step title="源码准备" description="配置 Git 仓库和触发方式" />
            <a-step title="GitLab Runner" description="执行流水线、观察构建状态" />
            <a-step title="镜像与制品" description="确认产物、镜像和变量" />
            <a-step title="GitOps 交接" description="创建变更请求并确认同步状态" />
          </a-steps>
          <div class="flow-links">
            <a-button type="link" @click="goTo('/pipeline/git-repos')">Git 仓库</a-button>
            <a-button type="link" @click="goTo('/pipeline/templates')">模板市场</a-button>
            <a-button type="link" @click="goTo('/argocd')">GitOps 交付</a-button>
          </div>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="8">
        <a-card title="快捷操作" :bordered="false" class="workbench-card">
          <a-space direction="vertical" style="width: 100%">
            <a-button block @click="$router.push('/pipeline/create')">创建流水线</a-button>
            <a-button block @click="goTo('/pipeline/stats')">流水线统计</a-button>
            <a-button block @click="goTo('/applications')">应用交付链路</a-button>
            <a-button block @click="goTo('/deploy/check')">发布检查</a-button>
          </a-space>
        </a-card>
      </a-col>

      <a-col :xs="24">
        <a-card title="最近构建" :bordered="false">
          <a-empty v-if="recentRuns.length === 0" description="暂无最近构建" />
          <a-list v-else :data-source="recentRuns" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <a-list-item-meta :title="item.pipeline_name || `流水线 #${item.pipeline_id}`" :description="item.git_branch || item.git_ref || '未记录分支'" />
                <template #actions>
                  <a-tag :color="getRunStatusColor(item.status)">{{ getRunStatusLabel(item.status) }}</a-tag>
                </template>
              </a-list-item>
            </template>
          </a-list>
        </a-card>
      </a-col>
    </a-row>

    <a-card title="CI/CD 流水线">
      <template #extra>
        <a-space>
          <ExportButton :data="pipelines" :columns="exportColumns" filename="pipelines" />
          <a-button type="primary" @click="$router.push('/pipeline/create')">
            <template #icon><PlusOutlined /></template>
            新建流水线
          </a-button>
        </a-space>
      </template>

      <!-- 筛选 -->
      <a-form layout="inline" style="margin-bottom: 16px">
        <a-form-item>
          <a-input v-model:value="filters.name" placeholder="搜索流水线名称" allowClear style="width: 200px" @change="loadPipelines">
            <template #prefix><SearchOutlined /></template>
          </a-input>
        </a-form-item>
        <a-form-item>
          <a-select v-model:value="filters.organization_id" placeholder="组织" allowClear style="width: 160px" @change="onOrganizationChange">
            <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.display_name || org.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-select v-model:value="filters.project_id" placeholder="项目" allowClear style="width: 180px" @change="loadPipelines">
            <a-select-option v-for="project in filteredProjects" :key="project.id" :value="project.id">{{ project.display_name || project.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-select v-model:value="filters.status" placeholder="状态" allowClear style="width: 120px" @change="loadPipelines">
            <a-select-option value="active">启用</a-select-option>
            <a-select-option value="disabled">禁用</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>

      <a-table :dataSource="pipelines" :loading="loading" :pagination="pagination" @change="handleTableChange" rowKey="id">
        <a-table-column title="名称" dataIndex="name" :width="200">
          <template #default="{ record }">
            <a-space>
              <FavoriteButton type="pipeline" :id="record.id" :name="record.name" :path="`/pipeline/${record.id}`" />
              <router-link :to="`/pipeline/${record.id}`">{{ record.name }}</router-link>
            </a-space>
          </template>
        </a-table-column>
        <a-table-column title="描述" dataIndex="description" :ellipsis="true" />
        <a-table-column title="Git 仓库" :width="250">
          <template #default="{ record }">
            <template v-if="record.git_repo_url">
              <div style="font-size: 12px">
                <a :href="record.git_repo_url" target="_blank" style="color: #1890ff">
                  {{ getRepoName(record.git_repo_url) }}
                </a>
              </div>
              <div style="color: #999; font-size: 11px">
                <BranchesOutlined /> {{ record.git_branch || 'main' }}
              </div>
            </template>
            <span v-else style="color: #999">-</span>
          </template>
        </a-table-column>
        <a-table-column title="状态" dataIndex="status" :width="100">
          <template #default="{ record }">
            <a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status === 'active' ? '启用' : '禁用' }}</a-tag>
          </template>
        </a-table-column>
        <a-table-column title="最近执行" :width="180">
          <template #default="{ record }">
            <template v-if="record.last_run_at">
              <a-tag :color="getRunStatusColor(record.last_run_status)">{{ getRunStatusLabel(record.last_run_status) }}</a-tag>
              <div style="color: #999; font-size: 12px">{{ formatTime(record.last_run_at) }}</div>
            </template>
            <span v-else style="color: #999">-</span>
          </template>
        </a-table-column>
        <a-table-column title="创建时间" dataIndex="created_at" :width="180">
          <template #default="{ record }">{{ formatTime(record.created_at) }}</template>
        </a-table-column>
        <a-table-column title="操作" :width="250" fixed="right">
          <template #default="{ record }">
            <a-space>
              <a-button type="link" size="small" @click="runPipeline(record)" :disabled="record.status !== 'active'">
                <PlayCircleOutlined /> 执行
              </a-button>
              <a-button type="link" size="small" @click="viewLastLogs(record)" :disabled="!record.last_run_at">
                <FileTextOutlined /> 日志
              </a-button>
              <a-button type="link" size="small" @click="editPipeline(record)">编辑</a-button>
              <a-popconfirm title="确定删除此流水线？" @confirm="deletePipeline(record.id)">
                <a-button type="link" size="small" danger>删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </a-table-column>
      </a-table>
    </a-card>

    <!-- 新建/编辑流水线 -->
    <a-modal v-model:open="showCreateModal" :title="editingPipeline ? '编辑流水线' : '新建流水线'" width="600px" @ok="handleSave" :confirmLoading="saving">
      <a-form :model="pipelineForm" :labelCol="{ span: 4 }">
        <a-form-item label="名称" required>
          <a-input v-model:value="pipelineForm.name" placeholder="流水线名称" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="pipelineForm.description" placeholder="流水线描述" :rows="2" />
        </a-form-item>
        <a-form-item label="配置">
          <a-textarea v-model:value="pipelineForm.config_json" placeholder="流水线配置 (JSON)" :rows="10" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 执行流水线 -->
    <a-modal v-model:open="showRunModal" title="执行流水线" @ok="handleRun" :confirmLoading="running">
      <a-form :model="runForm" :labelCol="{ span: 6 }">
        <a-form-item label="流水线">{{ runForm.pipeline_name }}</a-form-item>
        <a-form-item label="Git 仓库" v-if="runForm.git_repo_url">
          <a :href="runForm.git_repo_url" target="_blank">{{ getRepoName(runForm.git_repo_url) }}</a>
        </a-form-item>
        <a-form-item label="Git Ref" v-if="runForm.git_repo_id">
          <a-radio-group v-model:value="runForm.ref_type" style="margin-bottom: 8px">
            <a-radio-button value="branch">分支 ({{ branches.length }})</a-radio-button>
            <a-radio-button value="tag">Tag ({{ tags.length }})</a-radio-button>
          </a-radio-group>
          <a-spin :spinning="branchesLoading">
            <a-auto-complete
              v-model:value="runForm.ref"
              :options="refOptions"
              :placeholder="runForm.ref_type === 'branch' ? '选择或输入分支' : '选择或输入 Tag'"
              style="width: 100%"
              allow-clear
            />
          </a-spin>
          <div style="color: #999; font-size: 12px; margin-top: 4px">
            <template v-if="runForm.ref_type === 'branch'">默认分支: {{ runForm.default_branch }}</template>
            <template v-else>{{ tags.length > 0 ? '选择一个 Tag 版本' : '暂无 Tag' }}</template>
          </div>
        </a-form-item>
        <a-form-item label="参数 (JSON)">
          <a-textarea v-model:value="runForm.parameters_json" placeholder='{"key": "value"}' :rows="4" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 执行日志弹窗 -->
    <a-modal v-model:open="showLogsModal" title="执行日志" width="900px" :footer="null">
      <div v-if="logsLoading" style="text-align: center; padding: 40px">
        <a-spin tip="加载日志中..." />
      </div>
      <template v-else-if="lastRunInfo">
        <a-descriptions :column="4" size="small" style="margin-bottom: 12px">
          <a-descriptions-item label="执行ID">#{{ lastRunInfo.id }}</a-descriptions-item>
          <a-descriptions-item label="状态">
            <a-tag :color="getRunStatusColor(lastRunInfo.status)">{{ getRunStatusLabel(lastRunInfo.status) }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="触发">{{ lastRunInfo.trigger_by }}</a-descriptions-item>
          <a-descriptions-item label="耗时">{{ formatDuration(lastRunInfo.duration) }}</a-descriptions-item>
        </a-descriptions>
        <a-collapse v-model:activeKey="logsActiveKey" accordion>
          <a-collapse-panel v-for="stage in lastRunInfo.stage_runs" :key="stage.id" :header="stage.stage_name">
            <template #extra>
              <a-tag :color="getRunStatusColor(stage.status)" size="small">{{ getRunStatusLabel(stage.status) }}</a-tag>
            </template>
            <div v-for="step in stage.step_runs" :key="step.id" class="step-log-item">
              <div class="step-header">
                <span>{{ step.step_name }}</span>
                <a-tag :color="getRunStatusColor(step.status)" size="small">{{ getRunStatusLabel(step.status) }}</a-tag>
              </div>
              <pre class="step-logs">{{ step.logs || '暂无日志' }}</pre>
            </div>
          </a-collapse-panel>
        </a-collapse>
      </template>
      <a-empty v-else description="暂无执行记录" />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { PlusOutlined, SearchOutlined, PlayCircleOutlined, FileTextOutlined, BranchesOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { pipelineApi, gitRepoApi } from '@/services/pipeline'
import { catalogApi, type Organization, type Project } from '@/services/catalog'
import FavoriteButton from '@/components/FavoriteButton.vue'
import ExportButton from '@/components/ExportButton.vue'
import dayjs from 'dayjs'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const saving = ref(false)
const running = ref(false)
const logsLoading = ref(false)
const pipelines = ref<any[]>([])
const recentRuns = ref<any[]>([])
const organizations = ref<Organization[]>([])
const projects = ref<Project[]>([])
const pagination = ref({ current: 1, pageSize: 20, total: 0 })
const filters = ref<{ name: string; status?: string; organization_id?: number; project_id?: number }>({
  name: '',
  status: undefined,
  organization_id: undefined,
  project_id: undefined,
})
const showCreateModal = ref(false)
const showRunModal = ref(false)

// 导出列配置
const exportColumns = [
  { title: '名称', dataIndex: 'name' },
  { title: '描述', dataIndex: 'description' },
  { title: '状态', dataIndex: 'status' },
  { title: '最近执行状态', dataIndex: 'last_run_status' },
  { title: '最近执行时间', dataIndex: 'last_run_at' },
  { title: '创建时间', dataIndex: 'created_at' }
]

const showLogsModal = ref(false)
const editingPipeline = ref<any>(null)

const lastRunInfo = ref<any>(null)
const logsActiveKey = ref<number[]>([])
const pipelineForm = ref({ name: '', description: '', config_json: '' })
const runForm = ref({ 
  pipeline_id: 0, 
  pipeline_name: '', 
  git_repo_id: 0,
  git_repo_url: '',
  default_branch: 'main',
  ref_type: 'branch' as 'branch' | 'tag',
  ref: '',
  parameters_json: '{}' 
})
const branches = ref<string[]>([])
const tags = ref<string[]>([])
const refOptions = ref<{value: string}[]>([])
const branchesLoading = ref(false)

const activePipelines = computed(() => pipelines.value.filter(item => item.status === 'active').length)
const runningRunsCount = computed(() => recentRuns.value.filter(item => item.status === 'running').length)
const recentSuccessCount = computed(() => recentRuns.value.filter(item => item.status === 'success').length)
const filteredProjects = computed(() => filters.value.organization_id
  ? projects.value.filter(project => project.organization_id === filters.value.organization_id)
  : projects.value)

/**
 * 交付流程 Steps 的 current（Ant Design Vue：0 基；等于步骤数时表示全部完成）
 * 原硬编码 current=1 导致永远停在「GitLab Runner」阶段。
 */
const deliveryFlowCurrent = computed(() => {
  const pls = pipelines.value || []
  const runs = recentRuns.value || []

  const hasPipelineWithRepo = pls.some(
    (p) => p && p.git_repo_id != null && Number(p.git_repo_id) > 0
  )
  if (!hasPipelineWithRepo) {
    return 0
  }

  const buildSucceeded =
    runs.some((r) => r && r.status === 'success') ||
    pls.some((p) => p && p.last_run_status === 'success')
  if (!buildSucceeded) {
    return 1
  }

  const successRuns = runs.filter((r) => r && r.status === 'success')
  const hasChangeRequest = successRuns.some(
    (r) => r.gitops_change_request_id != null && Number(r.gitops_change_request_id) > 0
  )
  if (!hasChangeRequest) {
    return 2
  }

  const handoffTerminal = successRuns.some((r) => {
    const st = String(r.gitops_handoff_status || '').toLowerCase()
    return st === 'skipped' || st === 'merged'
  })
  if (handoffTerminal) {
    return 4
  }
  return 3
})

const loadPipelines = async () => {
  loading.value = true
  try {
    const res = await pipelineApi.list({ ...filters.value, page: pagination.value.current, page_size: pagination.value.pageSize })
    pipelines.value = res?.data?.items || []
    pagination.value.total = res?.data?.total || 0
  } catch (error) {
    console.error('加载流水线失败', error)
  } finally {
    loading.value = false
  }
}

const loadCatalog = async () => {
  try {
    const [orgRes, projectRes] = await Promise.all([
      catalogApi.listOrgs(),
      catalogApi.listProjects(),
    ])
    organizations.value = orgRes.data || []
    projects.value = projectRes.data || []
  } catch (error) {
    console.error('加载组织项目失败', error)
  }
}

const onOrganizationChange = () => {
  filters.value.project_id = undefined
  loadPipelines()
}

const loadWorkbench = async () => {
  try {
    const runsRes = await pipelineApi.listRuns({ page: 1, page_size: 20 })
    recentRuns.value = runsRes?.data?.items || []
  } catch (err) {
    console.warn('加载最近构建失败', err)
  }
}

const handleTableChange = (pag: any) => {
  pagination.value.current = pag.current
  pagination.value.pageSize = pag.pageSize
  loadPipelines()
}

const editPipeline = (record: any) => {
  router.push(`/pipeline/edit/${record.id}`)
}

const goTo = (path: string) => {
  router.push(path)
}

const handleSave = async () => {
  if (!pipelineForm.value.name) {
    message.warning('请输入流水线名称')
    return
  }
  saving.value = true
  try {
    let config = { stages: [], variables: [] }
    if (pipelineForm.value.config_json) {
      config = JSON.parse(pipelineForm.value.config_json)
    }
    const data = { ...pipelineForm.value, ...config }
    if (editingPipeline.value) {
      await pipelineApi.update(editingPipeline.value.id, data)
      message.success('更新成功')
    } else {
      await pipelineApi.create(data)
      message.success('创建成功')
    }
    showCreateModal.value = false
    editingPipeline.value = null
    pipelineForm.value = { name: '', description: '', config_json: '' }
    loadPipelines()
    loadWorkbench()
  } catch (error: any) {
    message.error(error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

const runPipeline = async (record: any) => {
  runForm.value = { 
    pipeline_id: record.id, 
    pipeline_name: record.name,
    git_repo_id: record.git_repo_id || 0,
    git_repo_url: record.git_repo_url || '',
    default_branch: record.git_branch || 'main',
    ref_type: 'branch',
    ref: record.git_branch || 'main',
    parameters_json: '{}' 
  }
  showRunModal.value = true
  
  // 加载分支和 Tag 列表
  if (record.git_repo_id) {
    branchesLoading.value = true
    branches.value = []
    tags.value = []
    refOptions.value = []
    
    // 并行加载分支和 Tags
    const [branchRes, tagRes] = await Promise.allSettled([
      gitRepoApi.getBranches(record.git_repo_id),
      gitRepoApi.getTags(record.git_repo_id)
    ])
    
    // 处理分支
    if (branchRes.status === 'fulfilled') {
      const items = branchRes.value?.data || []
      branches.value = items.map((item: any) => typeof item === 'string' ? item : item.name)
    }
    if (branches.value.length === 0) {
      branches.value = [record.git_branch || 'main']
    }
    
    // 处理 Tags
    if (tagRes.status === 'fulfilled') {
      const items = tagRes.value?.data || []
      tags.value = items.map((item: any) => typeof item === 'string' ? item : item.name)
    }
    
    // 设置初始选项（分支）
    refOptions.value = branches.value.map(b => ({ value: b }))
    branchesLoading.value = false
  }
}

// 监听 ref_type 变化，切换选项
watch(() => runForm.value.ref_type, (type) => {
  if (type === 'branch') {
    refOptions.value = branches.value.map(b => ({ value: b }))
    runForm.value.ref = runForm.value.default_branch
  } else {
    refOptions.value = tags.value.map(t => ({ value: t }))
    runForm.value.ref = tags.value[0] || ''
  }
})

const handleRun = async () => {
  running.value = true
  try {
    let params = {}
    if (runForm.value.parameters_json) {
      params = JSON.parse(runForm.value.parameters_json)
    }
    // 传递选择的分支/Tag
    await pipelineApi.run(runForm.value.pipeline_id, { 
      parameters: params,
      branch: runForm.value.ref || undefined
    })
    message.success('流水线已开始执行')
    showRunModal.value = false
    loadPipelines()
    loadWorkbench()
  } catch (error: any) {
    message.error(error.message || '执行失败')
  } finally {
    running.value = false
  }
}

const deletePipeline = async (id: number) => {
  try {
    await pipelineApi.delete(id)
    message.success('删除成功')
    loadPipelines()
    loadWorkbench()
  } catch (error) {
    message.error('删除失败')
  }
}

const viewLastLogs = async (record: any) => {
  showLogsModal.value = true
  logsLoading.value = true
  lastRunInfo.value = null
  logsActiveKey.value = []
  
  try {
    // 获取该流水线最近一次执行记录
    const runsRes = await pipelineApi.listRuns({ pipeline_id: record.id, page: 1, page_size: 1 })
    const runs = runsRes?.data?.items || []
    
    if (runs.length === 0) {
      logsLoading.value = false
      return
    }
    
    // 获取执行详情（包含阶段和步骤）
    const runRes = await pipelineApi.getRun(runs[0].id)
    lastRunInfo.value = runRes?.data || runRes
    
    // 默认展开第一个阶段
    if (lastRunInfo.value?.stage_runs?.length > 0) {
      logsActiveKey.value = [lastRunInfo.value.stage_runs[0].id]
    }
  } catch (error) {
    console.error('加载日志失败:', error)
    message.error('加载日志失败')
  } finally {
    logsLoading.value = false
  }
}

const formatDuration = (seconds: number) => {
  if (!seconds) return '-'
  if (seconds < 60) return `${seconds}秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}分${seconds % 60}秒`
  return `${Math.floor(seconds / 3600)}时${Math.floor((seconds % 3600) / 60)}分`
}

const getRunStatusColor = (status: string) => ({ success: 'green', running: 'blue', failed: 'red', cancelled: 'orange', pending: 'default' }[status] || 'default')
const getRunStatusLabel = (status: string) => ({ success: 'CI 成功', running: '运行中', failed: 'CI 失败', cancelled: '已取消', pending: '等待中' }[status] || status)
const formatTime = (time: string) => time ? dayjs(time).format('YYYY-MM-DD HH:mm:ss') : '-'
const getRepoName = (url: string) => {
  if (!url) return ''
  // 从 URL 提取仓库名，如 https://github.com/user/repo.git -> user/repo
  const match = url.match(/[:/]([^/:]+\/[^/.]+)(\.git)?$/)
  return match ? match[1] : url
}

onMounted(() => {
  const projectId = Number(route.query.project_id)
  if (Number.isFinite(projectId) && projectId > 0) {
    filters.value.project_id = projectId
  }
  loadCatalog()
  loadPipelines()
  loadWorkbench()
})
</script>

<style scoped>
.delivery-overview-row {
  margin-bottom: 16px;
}

.delivery-hero {
  background: linear-gradient(135deg, #f6ffed 0%, #ffffff 100%);
}

.hero-title {
  font-size: 24px;
  font-weight: 600;
  color: #1f1f1f;
}

.hero-subtitle {
  margin-top: 8px;
  color: #8c8c8c;
}

.hero-actions {
  display: flex;
  justify-content: flex-end;
}

.summary-card,
.workbench-card {
  height: 100%;
}

.summary-extra {
  margin-top: 8px;
  color: #8c8c8c;
  font-size: 12px;
}

.flow-links {
  margin-top: 12px;
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
