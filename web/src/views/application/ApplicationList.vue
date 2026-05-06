<template>
  <div class="application-list">
    <div class="page-header">
      <div>
        <h1>应用管理</h1>
        <div class="page-subtitle">把应用资产、项目归属和交付接入放在同一个工作面板里处理。</div>
      </div>
      <a-space wrap>
        <a-button @click="showAppModal()">
          <template #icon><PlusOutlined /></template>
          添加应用
        </a-button>
        <a-button type="primary" @click="showOnboardingWizard()">
          <template #icon><RocketOutlined /></template>
          接入向导
        </a-button>
      </a-space>
    </div>

    <a-card :bordered="false" class="overview-card">
      <a-row :gutter="[16, 16]" align="middle">
        <a-col :xs="24" :xl="10">
          <div class="overview-title">先判断哪些应用能直接交付，哪些还需要补齐接入，再进入明细处理。</div>
          <div class="overview-subtitle">{{ filterSummaryText }}</div>
          <a-space wrap class="overview-tags">
            <a-tag color="blue">总应用 {{ stats.app_count }}</a-tag>
            <a-tag color="green">可发布 {{ readyAppCount }}</a-tag>
            <a-tag color="orange">待补齐 {{ incompleteAppCount }}</a-tag>
            <a-tag color="purple">未归属项目 {{ appsWithoutProjectCount }}</a-tag>
          </a-space>
        </a-col>
        <a-col :xs="24" :xl="14">
          <a-row :gutter="[12, 12]">
            <a-col :xs="12" :md="6" v-for="card in summaryCards" :key="card.title">
              <a-card hoverable class="summary-card">
                <a-statistic :title="card.title" :value="card.value" :suffix="card.suffix" :precision="card.precision" :value-style="{ color: card.color }">
                  <template #prefix>
                    <component :is="card.icon" />
                  </template>
                </a-statistic>
                <div class="summary-hint">{{ card.hint }}</div>
              </a-card>
            </a-col>
          </a-row>
        </a-col>
      </a-row>
    </a-card>

    <a-card :bordered="false" class="filter-card">
      <div class="section-header">
        <div>
          <div class="section-title">筛选与定位</div>
          <div class="section-subtitle">优先按组织、项目和状态收敛范围，缩短查找和切换成本。</div>
        </div>
        <a-space wrap>
          <a-tag v-if="hasActiveFilters" color="processing">{{ activeFilterCount }} 个筛选条件生效</a-tag>
          <a-button type="link" @click="resetFilter">清空筛选</a-button>
        </a-space>
      </div>

      <a-form layout="inline" class="filter-form">
        <a-form-item label="应用名">
          <a-input v-model:value="filter.name" placeholder="搜索应用" allow-clear style="width: 160px" @pressEnter="fetchApps" />
        </a-form-item>
        <a-form-item label="团队">
          <a-select v-model:value="filter.team" placeholder="全部" allow-clear style="width: 130px">
            <a-select-option v-for="team in teams" :key="team" :value="team">{{ team }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="语言">
          <a-select v-model:value="filter.language" placeholder="全部" allow-clear style="width: 110px">
            <a-select-option value="go">Go</a-select-option>
            <a-select-option value="java">Java</a-select-option>
            <a-select-option value="python">Python</a-select-option>
            <a-select-option value="nodejs">Node.js</a-select-option>
            <a-select-option value="php">PHP</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="组织">
          <a-select v-model:value="filter.organization_id" placeholder="全部" allow-clear style="width: 170px" @change="onOrganizationFilterChange">
            <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.display_name || org.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="项目">
          <a-select v-model:value="filter.project_id" placeholder="全部" allow-clear style="width: 200px">
            <a-select-option v-for="project in filteredProjects" :key="project.id" :value="project.id">{{ project.display_name || project.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="filter.status" placeholder="全部" allow-clear style="width: 100px">
            <a-select-option value="active">启用</a-select-option>
            <a-select-option value="inactive">禁用</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" @click="fetchApps">查询</a-button>
            <a-button @click="resetFilter">重置</a-button>
          </a-space>
        </a-form-item>
      </a-form>

      <a-space v-if="hasActiveFilters" wrap class="active-filters">
        <a-tag v-if="filter.name" closable @close.prevent="clearFilter('name')">应用：{{ filter.name }}</a-tag>
        <a-tag v-if="filter.team" closable @close.prevent="clearFilter('team')">团队：{{ filter.team }}</a-tag>
        <a-tag v-if="filter.language" closable @close.prevent="clearFilter('language')">语言：{{ filter.language }}</a-tag>
        <a-tag v-if="filter.organization_id" closable @close.prevent="clearFilter('organization_id')">组织：{{ currentOrganizationName }}</a-tag>
        <a-tag v-if="filter.project_id" closable @close.prevent="clearFilter('project_id')">项目：{{ currentProjectName }}</a-tag>
        <a-tag v-if="filter.status" closable @close.prevent="clearFilter('status')">状态：{{ filter.status === 'active' ? '启用' : '禁用' }}</a-tag>
      </a-space>
    </a-card>

    <a-row :gutter="[16, 16]" class="content-row">
      <a-col :xs="24" :lg="8">
        <a-card :bordered="false" title="接入建议" class="insight-card">
          <a-list :data-source="onboardingSuggestions" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <div class="insight-main">
                  <div class="insight-title">{{ item.title }}</div>
                  <div class="sub-text">{{ item.description }}</div>
                </div>
                <a-button type="link" @click="item.action()">{{ item.label }}</a-button>
              </a-list-item>
            </template>
          </a-list>
        </a-card>
      </a-col>

      <a-col :xs="24" :lg="16">
        <a-card :bordered="false" title="应用列表" class="table-card">
          <template #extra>
            <a-space wrap>
              <span class="table-summary">当前显示 {{ apps.length }} / {{ pagination.total }} 个应用</span>
              <a-button type="link" @click="fetchApps">刷新</a-button>
            </a-space>
          </template>

          <a-table :columns="columns" :data-source="apps" :loading="loading" row-key="id" :pagination="pagination" @change="onTableChange">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="app-name-cell">
                  <a class="app-link" @click="goDelivery(record)">{{ record.display_name || record.name }}</a>
                  <div class="sub-text">{{ record.name }}</div>
                </div>
              </template>
              <template v-if="column.key === 'language'">
                <a-tag v-if="record.language" :color="getLangColor(record.language)">{{ record.language }}</a-tag>
                <span v-else>-</span>
              </template>
              <template v-if="column.key === 'team'">
                <a-tag v-if="record.team" color="blue">{{ record.team }}</a-tag>
                <span v-else>-</span>
              </template>
              <template v-if="column.key === 'project'">
                <a-space direction="vertical" size="small">
                  <span>{{ record.project_name || '未关联项目' }}</span>
                  <span class="sub-text">{{ record.org_name || '未关联组织' }}</span>
                </a-space>
              </template>
              <template v-if="column.key === 'status'">
                <a-badge :status="record.status === 'active' ? 'success' : 'default'" :text="record.status === 'active' ? '启用' : '禁用'" />
              </template>
              <template v-if="column.key === 'readiness'">
                <a-space direction="vertical" size="small" style="width: 100%">
                  <a-progress
                    :percent="readinessMap[record.id || 0]?.score || 0"
                    size="small"
                    :status="getReadinessProgressStatus(readinessMap[record.id || 0]?.score || 0)"
                  />
                  <a-space size="small" wrap>
                    <a-tag :color="getReadinessColor(readinessMap[record.id || 0]?.score || 0)">
                      {{ getReadinessText(readinessMap[record.id || 0]) }}
                    </a-tag>
                    <span class="sub-text">{{ getReadinessHint(readinessMap[record.id || 0]) }}</span>
                    <a v-if="readinessMap[record.id || 0]?.next_actions?.length" @click="goReadinessAction(record)">补齐</a>
                  </a-space>
                </a-space>
              </template>
              <template v-if="column.key === 'action'">
                <a-space wrap>
                  <a-button type="primary" ghost size="small" @click="goDelivery(record)">进入交付</a-button>
                  <a-button type="link" size="small" @click="goReadinessAction(record)">补齐接入</a-button>
                  <a-button type="link" size="small" @click="viewApp(record)">详情</a-button>
                  <a-dropdown>
                    <a-button type="link" size="small">更多</a-button>
                    <template #overlay>
                      <a-menu>
                        <a-menu-item @click="showAppModal(record)">编辑基础信息</a-menu-item>
                        <a-menu-item>
                          <a-popconfirm title="确定删除？" @confirm="deleteApp(record.id)">
                            <span class="danger-text">删除应用</span>
                          </a-popconfirm>
                        </a-menu-item>
                      </a-menu>
                    </template>
                  </a-dropdown>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-col>
    </a-row>

    <a-modal v-model:open="appModalVisible" :title="editingAppId ? '编辑应用' : '添加应用'" @ok="saveApp" :confirm-loading="savingApp" width="700px">
      <a-form :model="editingApp" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="应用名称" required>
              <a-input v-model:value="editingApp.name" placeholder="如：user-service" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="显示名称">
              <a-input v-model:value="editingApp.display_name" placeholder="如：用户服务" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="开发语言">
              <a-select v-model:value="editingApp.language" placeholder="选择语言" allow-clear>
                <a-select-option value="go">Go</a-select-option>
                <a-select-option value="java">Java</a-select-option>
                <a-select-option value="python">Python</a-select-option>
                <a-select-option value="nodejs">Node.js</a-select-option>
                <a-select-option value="php">PHP</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="框架">
              <a-input v-model:value="editingApp.framework" placeholder="如：gin, spring" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="团队">
              <a-auto-complete v-model:value="editingApp.team" :options="teamOptions" placeholder="所属团队" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="所属组织">
              <a-select v-model:value="editingApp.organization_id" placeholder="选择组织" allow-clear @change="onEditingOrganizationChange">
                <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.display_name || org.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="所属项目">
              <a-select v-model:value="editingApp.project_id" placeholder="选择项目" allow-clear>
                <a-select-option v-for="project in editingProjects" :key="project.id" :value="project.id">{{ project.display_name || project.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="负责人">
              <a-input v-model:value="editingApp.owner" placeholder="负责人姓名" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="标准 Git 仓库">
              <a-select
                v-model:value="selectedGitRepoId"
                placeholder="选择已接入的 GitLab 仓库"
                allow-clear
                show-search
                option-filter-prop="label"
              >
                <a-select-option
                  v-for="repo in gitRepos"
                  :key="repo.id"
                  :value="repo.id"
                  :label="`${repo.name} ${repo.url}`"
                >
                  {{ repo.name }}
                  <span class="repo-url">{{ repo.url }}</span>
                </a-select-option>
              </a-select>
              <div class="form-help">
                这里会同步为应用默认仓库，后续创建流水线将自动带出。
                <a @click="router.push('/pipeline/git-repos')">管理 Git 仓库</a>
              </div>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="描述">
          <a-textarea v-model:value="editingApp.description" :rows="2" />
        </a-form-item>
      </a-form>
    </a-modal>

    <AppOnboardingWizard v-model:open="onboardingVisible" @success="onOnboardingSuccess" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { PlusOutlined, AppstoreOutlined, RocketOutlined, BarChartOutlined, CheckCircleOutlined } from '@ant-design/icons-vue'
import { applicationApi, type Application, type AppStats, type ApplicationReadiness, type ApplicationOnboardingResponse } from '@/services/application'
import { gitRepoApi } from '@/services/pipeline'
import { catalogApi, type Organization, type Project } from '@/services/catalog'
import AppOnboardingWizard from './components/AppOnboardingWizard.vue'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const savingApp = ref(false)
const appModalVisible = ref(false)
const onboardingVisible = ref(false)
const editingAppId = ref<number | undefined>(undefined)

const apps = ref<Application[]>([])
const teams = ref<string[]>([])
const organizations = ref<Organization[]>([])
const projects = ref<Project[]>([])
const stats = ref<AppStats>({ app_count: 0, team_stats: [], lang_stats: [], today_deliveries: 0, week_deliveries: 0, success_rate: 0 })
const gitRepos = ref<any[]>([])
const selectedGitRepoId = ref<number | undefined>(undefined)
const readinessMap = ref<Record<number, ApplicationReadiness>>({})

const filter = reactive<{ name: string; team: string; language: string; status: string; organization_id?: number; project_id?: number }>({
  name: '',
  team: '',
  language: '',
  status: '',
  organization_id: undefined,
  project_id: undefined,
})
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showSizeChanger: true })

const editingApp = reactive<Partial<Application>>({
  name: '',
  display_name: '',
  description: '',
  git_repo: '',
  language: '',
  framework: '',
  team: '',
  owner: '',
  status: 'active',
  organization_id: undefined,
  project_id: undefined,
})

const teamOptions = computed(() => teams.value.map((t) => ({ value: t })))
const filteredProjects = computed(() => filter.organization_id ? projects.value.filter((project) => project.organization_id === filter.organization_id) : projects.value)
const editingProjects = computed(() => editingApp.organization_id ? projects.value.filter((project) => project.organization_id === editingApp.organization_id) : projects.value)
const currentOrganizationName = computed(() => organizations.value.find((org) => org.id === filter.organization_id)?.display_name || organizations.value.find((org) => org.id === filter.organization_id)?.name || '-')
const currentProjectName = computed(() => projects.value.find((project) => project.id === filter.project_id)?.display_name || projects.value.find((project) => project.id === filter.project_id)?.name || '-')

const summaryCards = computed(() => [
  { title: '应用总数', value: stats.value.app_count, color: '#1890ff', icon: AppstoreOutlined, hint: '纳入平台管理的应用资产' },
  { title: '今日交付', value: stats.value.today_deliveries, color: '#52c41a', icon: RocketOutlined, hint: '今天已完成的交付动作' },
  { title: '本周交付', value: stats.value.week_deliveries, color: '#722ed1', icon: BarChartOutlined, hint: '观察项目交付节奏' },
  { title: '成功率', value: stats.value.success_rate, suffix: '%', precision: 1, color: stats.value.success_rate >= 90 ? '#52c41a' : '#fa8c16', icon: CheckCircleOutlined, hint: '最近交付结果稳定性' },
])

const readyAppCount = computed(() => apps.value.filter((app) => (readinessMap.value[app.id || 0]?.score || 0) >= 80).length)
const incompleteAppCount = computed(() => apps.value.filter((app) => {
  const score = readinessMap.value[app.id || 0]?.score || 0
  return score > 0 && score < 80
}).length)
const appsWithoutProjectCount = computed(() => apps.value.filter((app) => !app.project_id).length)
const activeFilterCount = computed(() => [filter.name, filter.team, filter.language, filter.status, filter.organization_id, filter.project_id].filter(Boolean).length)
const hasActiveFilters = computed(() => activeFilterCount.value > 0)
const filterSummaryText = computed(() => {
  if (!hasActiveFilters.value) {
    return '当前查看全量应用，建议先按组织或项目缩小范围，再集中处理待补齐项。'
  }
  const segments = [
    filter.organization_id ? `组织：${currentOrganizationName.value}` : '',
    filter.project_id ? `项目：${currentProjectName.value}` : '',
    filter.team ? `团队：${filter.team}` : '',
    filter.language ? `语言：${filter.language}` : '',
    filter.status ? `状态：${filter.status === 'active' ? '启用' : '禁用'}` : '',
    filter.name ? `关键词：${filter.name}` : '',
  ].filter(Boolean)
  return `当前聚焦 ${segments.join(' / ')}。`
})

const columns = [
  { title: '应用', dataIndex: 'name', key: 'name' },
  { title: '语言', dataIndex: 'language', key: 'language', width: 90 },
  { title: '团队', dataIndex: 'team', key: 'team', width: 100 },
  { title: '所属项目', key: 'project', width: 180 },
  { title: '接入完整度', key: 'readiness', width: 220 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 90 },
  { title: '操作', key: 'action', width: 280 },
]

const langColors: Record<string, string> = { go: 'cyan', java: 'orange', python: 'blue', nodejs: 'green', php: 'purple' }
const getLangColor = (lang: string) => langColors[lang] || 'default'

const fetchApps = async () => {
  loading.value = true
  try {
    const response = await applicationApi.list({ page: pagination.current, page_size: pagination.pageSize, ...filter })
    if (response.code === 0 && response.data) {
      apps.value = response.data.list || []
      pagination.total = response.data.total
      await fetchReadinessForApps(apps.value)
    }
  } catch (error) {
    console.error('获取应用列表失败', error)
  } finally {
    loading.value = false
  }
}

const fetchReadinessForApps = async (items: Application[]) => {
  const ids = items.map((item) => item.id).filter(Boolean) as number[]
  if (!ids.length) return
  const results = await Promise.allSettled(ids.map((id) => applicationApi.getReadiness(id)))
  const next = { ...readinessMap.value }
  results.forEach((result, index) => {
    if (result.status === 'fulfilled' && result.value.code === 0 && result.value.data) {
      next[ids[index]] = result.value.data
    }
  })
  readinessMap.value = next
}

const fetchStats = async () => {
  try {
    const response = await applicationApi.getStats()
    if (response.code === 0 && response.data) {
      stats.value = response.data
    }
  } catch (error) {
    console.error('获取统计失败', error)
  }
}

const fetchCatalog = async () => {
  try {
    const [orgRes, projectRes] = await Promise.all([catalogApi.listOrgs(), catalogApi.listProjects()])
    organizations.value = orgRes.data || []
    projects.value = projectRes.data || []
  } catch (error) {
    console.error('获取组织项目目录失败', error)
  }
}

const fetchTeams = async () => {
  try {
    const response = await applicationApi.getTeams()
    if (response.code === 0 && response.data) {
      teams.value = response.data
    }
  } catch (error) {
    console.error('获取团队列表失败', error)
  }
}

const fetchGitRepos = async () => {
  try {
    const response: any = await gitRepoApi.list({ provider: 'gitlab', page: 1, page_size: 200 })
    const data = response?.data || {}
    gitRepos.value = data.list || data.items || []
  } catch (error) {
    console.error('获取 Git 仓库失败', error)
  }
}

const onTableChange = (pag: any) => {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchApps()
}

const onOrganizationFilterChange = () => {
  filter.project_id = undefined
}

const onEditingOrganizationChange = () => {
  const currentProjectId = editingApp.project_id
  if (!currentProjectId) return
  const matched = projects.value.find((project) => project.id === currentProjectId)
  if (!matched || matched.organization_id !== editingApp.organization_id) {
    editingApp.project_id = undefined
  }
}

const resetFilter = () => {
  filter.name = ''
  filter.team = ''
  filter.language = ''
  filter.status = ''
  filter.organization_id = undefined
  filter.project_id = undefined
  pagination.current = 1
  fetchApps()
}

const clearFilter = (key: 'name' | 'team' | 'language' | 'status' | 'organization_id' | 'project_id') => {
  if (key === 'organization_id') {
    filter.organization_id = undefined
    filter.project_id = undefined
  } else if (key === 'project_id') {
    filter.project_id = undefined
  } else {
    filter[key] = ''
  }
  pagination.current = 1
  fetchApps()
}

const showOnboardingWizard = () => {
  onboardingVisible.value = true
}

const onOnboardingSuccess = async (result: ApplicationOnboardingResponse) => {
  if (result.readiness) {
    readinessMap.value = { ...readinessMap.value, [result.application_id]: result.readiness }
  }
  await Promise.all([fetchApps(), fetchStats(), fetchTeams()])
}

const showAppModal = (app?: Application) => {
  if (app) {
    editingAppId.value = app.id
    Object.assign(editingApp, app)
    selectedGitRepoId.value = undefined
    if (app.id) {
      loadDefaultRepoBinding(app.id)
    }
  } else {
    editingAppId.value = undefined
    Object.assign(editingApp, {
      name: '',
      display_name: '',
      description: '',
      git_repo: '',
      language: '',
      framework: '',
      team: '',
      owner: '',
      status: 'active',
      organization_id: undefined,
      project_id: undefined,
    })
    selectedGitRepoId.value = undefined
  }
  fetchGitRepos()
  appModalVisible.value = true
}

const loadDefaultRepoBinding = async (appId: number) => {
  try {
    const response = await applicationApi.get(appId)
    selectedGitRepoId.value = response.data?.default_repo_binding?.git_repo_id
  } catch (error) {
    console.error('获取应用默认仓库失败', error)
  }
}

const saveApp = async () => {
  if (!editingApp.name) {
    message.error('请填写应用名称')
    return
  }
  savingApp.value = true
  try {
    const selectedRepo = gitRepos.value.find((repo) => repo.id === selectedGitRepoId.value)
    const payload = {
      name: editingApp.name,
      display_name: editingApp.display_name,
      description: editingApp.description,
      organization_id: editingApp.organization_id,
      project_id: editingApp.project_id,
      language: editingApp.language,
      framework: editingApp.framework,
      team: editingApp.team,
      owner: editingApp.owner,
      status: editingApp.status,
      git_repo: selectedRepo?.url || editingApp.git_repo || '',
    }
    const response = editingAppId.value
      ? await applicationApi.update(editingAppId.value, payload)
      : await applicationApi.create(payload)
    if (response.code === 0) {
      const appId = editingAppId.value || response.data?.id
      if (appId && selectedGitRepoId.value) {
        await applicationApi.bindRepo(appId, {
          git_repo_id: selectedGitRepoId.value,
          role: 'primary',
          is_default: true,
        })
      }
      message.success(editingAppId.value ? '更新成功' : '添加成功')
      appModalVisible.value = false
      fetchApps()
      fetchStats()
      fetchTeams()
    } else {
      message.error(response.message || '保存失败')
    }
  } catch (error: any) {
    message.error(error.message || '保存失败')
  } finally {
    savingApp.value = false
  }
}

const deleteApp = async (id: number) => {
  try {
    const response = await applicationApi.delete(id)
    if (response.code === 0) {
      message.success('删除成功')
      fetchApps()
      fetchStats()
    } else {
      message.error(response.message || '删除失败')
    }
  } catch (error: any) {
    message.error(error.message || '删除失败')
  }
}

const viewApp = (app: Application) => {
  router.push(`/applications/${app.id}`)
}

const goDelivery = (app: Application) => {
  router.push(`/applications/${app.id}#delivery`)
}

const goReadinessAction = (app: Application) => {
  const readiness = readinessMap.value[app.id || 0]
  const action = readiness?.next_actions?.[0]
  router.push(action?.path || `/applications/${app.id}#delivery`)
}

const onboardingSuggestions = computed(() => [
  {
    title: '先处理待补齐应用',
    description: incompleteAppCount.value > 0 ? `当前列表中有 ${incompleteAppCount.value} 个应用还没达到可发布标准。` : '当前列表没有明显的接入缺口，可以继续检查新增应用。',
    label: incompleteAppCount.value > 0 ? '去补齐' : '看列表',
    action: () => {
      const target = apps.value.find((app) => {
        const score = readinessMap.value[app.id || 0]?.score || 0
        return score > 0 && score < 80
      })
      if (target) {
        goReadinessAction(target)
        return
      }
      fetchApps()
    },
  },
  {
    title: '补全项目归属',
    description: appsWithoutProjectCount.value > 0 ? `${appsWithoutProjectCount.value} 个应用还没有关联项目，会影响项目驾驶舱和交付闭环。` : '当前列表中的应用都已归属到项目。',
    label: appsWithoutProjectCount.value > 0 ? '去编辑' : '看目录',
    action: () => {
      const target = apps.value.find((app) => !app.project_id)
      if (target) {
        showAppModal(target)
        return
      }
      router.push('/catalog')
    },
  },
  {
    title: '新增标准接入',
    description: '通过接入向导一次性完成应用、仓库、环境和流水线的基础绑定。',
    label: '打开向导',
    action: () => showOnboardingWizard(),
  },
])

const getReadinessProgressStatus = (score: number) => score >= 80 ? 'success' : score >= 50 ? 'normal' : 'exception'
const getReadinessColor = (score: number) => score >= 80 ? 'green' : score >= 50 ? 'orange' : 'red'
const getReadinessText = (readiness?: ApplicationReadiness) => {
  if (!readiness) return '检查中'
  if (readiness.score >= 90) return '可发布'
  if (readiness.score >= 70) return '接近完成'
  if (readiness.score >= 40) return '待补齐'
  return '未接入'
}
const getReadinessHint = (readiness?: ApplicationReadiness) => {
  if (!readiness) return '正在计算接入状态'
  return `${readiness.completed}/${readiness.total} 项已完成`
}

onMounted(() => {
  const orgId = Number(route.query.organization_id)
  const projectId = Number(route.query.project_id)
  if (Number.isFinite(orgId) && orgId > 0) {
    filter.organization_id = orgId
  }
  if (Number.isFinite(projectId) && projectId > 0) {
    filter.project_id = projectId
  }
  fetchApps()
  fetchStats()
  fetchTeams()
  fetchGitRepos()
  fetchCatalog()
})
</script>

<style scoped>
.application-list {
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
  font-size: 20px;
  font-weight: 500;
  margin: 0;
}

.page-subtitle {
  margin-top: 6px;
  color: #8c8c8c;
  font-size: 13px;
}

.sub-text {
  color: #999;
  font-size: 12px;
}

.repo-url {
  color: #999;
  font-size: 12px;
  margin-left: 8px;
}

.form-help {
  color: #999;
  font-size: 12px;
  margin-top: 6px;
  line-height: 1.6;
}

.overview-card {
  margin-bottom: 16px;
  background: linear-gradient(135deg, #f7fbff 0%, #ffffff 55%, #f5f8ff 100%);
}

.overview-title {
  font-size: 22px;
  line-height: 1.5;
  font-weight: 600;
  color: #1f1f1f;
}

.overview-subtitle {
  margin-top: 8px;
  color: #6b7280;
  line-height: 1.6;
}

.overview-tags {
  margin-top: 16px;
}

.summary-card {
  height: 100%;
  border-radius: 12px;
}

.summary-hint {
  margin-top: 8px;
  color: #8c8c8c;
  font-size: 12px;
}

.filter-card {
  margin-bottom: 16px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 12px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1f1f1f;
}

.section-subtitle {
  margin-top: 4px;
  color: #8c8c8c;
  font-size: 12px;
}

.filter-form {
  row-gap: 12px;
}

.active-filters {
  margin-top: 16px;
}

.content-row {
  margin-bottom: 16px;
}

.insight-card,
.table-card {
  height: 100%;
}

.insight-main,
.app-name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.insight-title,
.app-link {
  font-weight: 500;
}

.table-summary {
  color: #8c8c8c;
  font-size: 12px;
}

.danger-text {
  color: #ff4d4f;
}

@media (max-width: 768px) {
  .page-header,
  .section-header {
    flex-direction: column;
  }
}
</style>
