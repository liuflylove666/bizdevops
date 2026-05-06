<template>
  <div class="release-list">
    <div class="page-header">
      <div>
        <h1>发布主单</h1>
        <div class="page-subtitle">把待审批、待发布、失败和高风险主单前置，先处理阻塞，再看全量列表。</div>
      </div>
      <a-space wrap>
        <a-button @click="fetchList">
          <template #icon><ReloadOutlined /></template>
          刷新
        </a-button>
        <a-button type="primary" @click="showCreateModal = true">
          <template #icon><PlusOutlined /></template>
          新建发布
        </a-button>
      </a-space>
    </div>

    <a-card :bordered="false" class="hero-card">
      <a-row :gutter="[16, 16]" align="middle">
        <a-col :xs="24" :xl="10">
          <div class="hero-title">{{ releaseHeadline.title }}</div>
          <div class="hero-subtitle">{{ releaseHeadline.description }}</div>
          <a-space wrap class="hero-tags">
            <a-tag color="gold">待审批 {{ pendingApprovalCount }}</a-tag>
            <a-tag color="blue">待推进 {{ inProgressCount }}</a-tag>
            <a-tag color="red">失败 / 回滚 {{ failedCount }}</a-tag>
            <a-tag color="orange">高风险 {{ highRiskCount }}</a-tag>
          </a-space>
        </a-col>
        <a-col :xs="24" :xl="14">
          <a-row :gutter="[12, 12]">
            <a-col :xs="12" :md="6" v-for="card in summaryCards" :key="card.title">
              <a-card class="summary-card" hoverable>
                <a-statistic :title="card.title" :value="card.value" :value-style="{ color: card.color }" />
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
          <div class="section-subtitle">建议先按组织、项目、环境和状态缩小范围，再进入主单处理。</div>
        </div>
        <a-space wrap>
          <a-tag v-if="hasActiveFilters" color="processing">{{ activeFilterCount }} 个筛选条件生效</a-tag>
          <a-button type="link" @click="resetFilter">清空筛选</a-button>
        </a-space>
      </div>
      <a-form layout="inline" @finish="onSearch" class="filter-form">
        <a-form-item label="标题">
          <a-input v-model:value="filter.title" placeholder="搜索标题" allow-clear style="width: 200px" />
        </a-form-item>
        <a-form-item label="环境">
          <a-select v-model:value="filter.env" placeholder="全部" allow-clear style="width: 120px">
            <a-select-option value="dev">dev</a-select-option>
            <a-select-option value="test">test</a-select-option>
            <a-select-option value="staging">staging</a-select-option>
            <a-select-option value="prod">prod</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="组织">
          <a-select v-model:value="filter.organization_id" placeholder="全部" allow-clear style="width: 160px" @change="onOrganizationChange">
            <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.display_name || org.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="项目">
          <a-select v-model:value="filter.project_id" placeholder="全部" allow-clear style="width: 180px">
            <a-select-option v-for="project in filteredProjects" :key="project.id" :value="project.id">{{ project.display_name || project.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="filter.status" placeholder="全部" allow-clear style="width: 140px">
            <a-select-option v-for="s in statusOptions" :key="s.value" :value="s.value">{{ s.label }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" html-type="submit">查询</a-button>
            <a-button @click="resetFilter">重置</a-button>
          </a-space>
        </a-form-item>
      </a-form>
      <a-space v-if="hasActiveFilters" wrap class="active-filters">
        <a-tag v-if="filter.title" closable @close.prevent="clearFilter('title')">标题：{{ filter.title }}</a-tag>
        <a-tag v-if="filter.env" closable @close.prevent="clearFilter('env')">环境：{{ filter.env }}</a-tag>
        <a-tag v-if="filter.organization_id" closable @close.prevent="clearFilter('organization_id')">组织：{{ currentOrganizationName }}</a-tag>
        <a-tag v-if="filter.project_id" closable @close.prevent="clearFilter('project_id')">项目：{{ currentProjectName }}</a-tag>
        <a-tag v-if="filter.status" closable @close.prevent="clearFilter('status')">状态：{{ statusLabel(filter.status) }}</a-tag>
      </a-space>
    </a-card>

    <a-row :gutter="[16, 16]" class="content-row">
      <a-col :xs="24" :lg="8">
        <a-card :bordered="false" title="发布建议" class="insight-card">
          <a-list :data-source="releaseSuggestions" size="small">
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
        <a-card :bordered="false" title="主单列表" class="table-card">
          <template #extra>
            <a-space wrap>
              <span class="table-summary">当前显示 {{ releases.length }} / {{ pagination.total }} 条主单</span>
              <a-button type="link" @click="fetchList">刷新</a-button>
            </a-space>
          </template>
          <a-table
            :columns="columns"
            :data-source="releases"
            :loading="loading"
            row-key="id"
            :pagination="pagination"
            @change="onTableChange"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'title'">
                <div class="release-title-cell">
                  <a class="release-link" @click="goDetail(record.id)">{{ record.title }}</a>
                  <div class="sub-text">#{{ record.id }} · {{ record.application_name }} · {{ record.project_name || '未关联项目' }} · {{ record.env }}</div>
                </div>
              </template>

              <template v-else-if="column.key === 'rollout'">
                <a-tag :color="rolloutColor(record.rollout_strategy)">
                  {{ rolloutLabel(record.rollout_strategy) }}
                </a-tag>
              </template>

              <template v-else-if="column.key === 'risk'">
                <RiskBadge :score="record.risk_score" :level="record.risk_level" />
              </template>

              <template v-else-if="column.key === 'status'">
                <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
              </template>

              <template v-else-if="column.key === 'created'">
                <div>{{ formatTime(record.created_at) }}</div>
                <div class="sub-text">{{ record.created_by_name || '-' }}</div>
              </template>

              <template v-else-if="column.key === 'action'">
                <a-space wrap>
                  <a-button type="primary" ghost size="small" @click="goDetail(record.id)">进入主单</a-button>
                  <a-popconfirm title="确认删除此发布主单？" @confirm="onDelete(record)">
                    <a-button type="link" size="small" danger>删除</a-button>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-col>
    </a-row>

    <a-modal
      v-model:open="showCreateModal"
      title="新建发布主单"
      :width="640"
      :confirm-loading="creating"
      @ok="onCreate"
    >
      <a-form :model="createForm" layout="vertical">
        <a-form-item label="标题" required>
          <a-input v-model:value="createForm.title" placeholder="如：用户中心 v1.4.2 上线" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="应用名">
              <a-input v-model:value="createForm.application_name" placeholder="user-center" />
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item label="环境" required>
              <a-select v-model:value="createForm.env">
                <a-select-option value="dev">dev</a-select-option>
                <a-select-option value="test">test</a-select-option>
                <a-select-option value="staging">staging</a-select-option>
                <a-select-option value="prod">prod</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item label="版本">
              <a-input v-model:value="createForm.version" placeholder="v1.4.2" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="发布策略" required>
              <a-select v-model:value="createForm.rollout_strategy">
                <a-select-option value="direct">直接发布</a-select-option>
                <a-select-option value="canary">金丝雀</a-select-option>
                <a-select-option value="blue_green">蓝绿</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="手工风险等级">
              <a-select v-model:value="createForm.risk_level" allow-clear>
                <a-select-option value="low">low</a-select-option>
                <a-select-option value="medium">medium</a-select-option>
                <a-select-option value="high">high</a-select-option>
                <a-select-option value="critical">critical</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="说明">
          <a-textarea v-model:value="createForm.description" :rows="3" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { releaseApi, type Release, type ReleaseFilter } from '@/services/release'
import { catalogApi, type Organization, type Project } from '@/services/catalog'
import RiskBadge from '@/components/release/RiskBadge.vue'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const releases = ref<Release[]>([])
const showCreateModal = ref(false)
const creating = ref(false)
const organizations = ref<Organization[]>([])
const projects = ref<Project[]>([])

const filter = reactive<ReleaseFilter & { organization_id?: number }>({
  title: '',
  env: undefined,
  status: undefined,
  project_id: undefined,
  organization_id: undefined,
})

const filteredProjects = computed(() => filter.organization_id ? projects.value.filter((project) => project.organization_id === filter.organization_id) : projects.value)
const currentOrganizationName = computed(() => organizations.value.find((org) => org.id === filter.organization_id)?.display_name || organizations.value.find((org) => org.id === filter.organization_id)?.name || '-')
const currentProjectName = computed(() => projects.value.find((project) => project.id === filter.project_id)?.display_name || projects.value.find((project) => project.id === filter.project_id)?.name || '-')

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`,
})

const createForm = reactive({
  title: '',
  application_name: '',
  env: 'dev',
  version: '',
  description: '',
  rollout_strategy: 'direct' as 'direct' | 'canary' | 'blue_green',
  risk_level: undefined as string | undefined,
})

const statusOptions = [
  { value: 'draft', label: '草稿' },
  { value: 'pending_approval', label: '待审批' },
  { value: 'approved', label: '已审批' },
  { value: 'rejected', label: '已驳回' },
  { value: 'pr_opened', label: 'PR 已提交' },
  { value: 'pr_merged', label: 'PR 已合并' },
  { value: 'published', label: '已发布' },
  { value: 'failed', label: '失败' },
  { value: 'rolled_back', label: '已回滚' },
]

const columns = [
  { title: '发布', key: 'title', dataIndex: 'title', width: 320 },
  { title: '策略', key: 'rollout', width: 110 },
  { title: '风险', key: 'risk', width: 130 },
  { title: '状态', key: 'status', width: 120 },
  { title: '创建', key: 'created', width: 200 },
  { title: '操作', key: 'action', width: 160 },
]

const pendingApprovalCount = computed(() => releases.value.filter((item) => item.status === 'pending_approval').length)
const inProgressCount = computed(() => releases.value.filter((item) => ['approved', 'pr_opened', 'pr_merged'].includes(item.status || '')).length)
const failedCount = computed(() => releases.value.filter((item) => ['failed', 'rolled_back', 'rejected'].includes(item.status || '')).length)
const highRiskCount = computed(() => releases.value.filter((item) => ['high', 'critical'].includes((item.risk_level || '').toLowerCase())).length)
const publishedCount = computed(() => releases.value.filter((item) => item.status === 'published').length)
const activeFilterCount = computed(() => [filter.title, filter.env, filter.status, filter.organization_id, filter.project_id].filter(Boolean).length)
const hasActiveFilters = computed(() => activeFilterCount.value > 0)

const releaseHeadline = computed(() => {
  if (pendingApprovalCount.value > 0) {
    return {
      title: `当前有 ${pendingApprovalCount.value} 个发布在等待审批`,
      description: '建议先清理审批阻塞，再继续看 PR 合并和发布推进状态。',
    }
  }
  if (failedCount.value > 0) {
    return {
      title: `当前有 ${failedCount.value} 个失败或回滚主单`,
      description: '失败和回滚主单优先级高于新增发布，应先确认恢复动作和原因。',
    }
  }
  if (inProgressCount.value > 0) {
    return {
      title: `当前有 ${inProgressCount.value} 个主单仍在推进中`,
      description: '建议先收口在途发布，再创建新的变更，避免交付链路并发过高。',
    }
  }
  return {
    title: '当前发布链路整体平稳',
    description: '没有明显阻塞时，可以继续创建新主单或按项目视角检查交付覆盖面。',
  }
})

const summaryCards = computed(() => [
  { title: '待审批', value: pendingApprovalCount.value, color: '#d48806', hint: '优先清理审批阻塞' },
  { title: '进行中', value: inProgressCount.value, color: '#1677ff', hint: '待合并 / 待发布的主单' },
  { title: '已发布', value: publishedCount.value, color: '#52c41a', hint: '已完成交付闭环' },
  { title: '高风险', value: highRiskCount.value, color: '#cf1322', hint: '建议优先复核高风险变更' },
])

const releaseSuggestions = computed(() => [
  {
    title: '先处理待审批主单',
    description: pendingApprovalCount.value > 0 ? `当前列表中有 ${pendingApprovalCount.value} 个主单仍在等待审批。` : '当前没有审批阻塞，可以继续看在途和失败主单。',
    label: pendingApprovalCount.value > 0 ? '去处理' : '看列表',
    action: () => {
      const target = releases.value.find((item) => item.status === 'pending_approval')
      if (target?.id) {
        goDetail(target.id)
        return
      }
      fetchList()
    },
  },
  {
    title: '复核高风险发布',
    description: highRiskCount.value > 0 ? `${highRiskCount.value} 个主单被标记为高风险或严重风险。` : '当前列表中没有明显的高风险主单。',
    label: highRiskCount.value > 0 ? '去复核' : '新建主单',
    action: () => {
      const target = releases.value.find((item) => ['high', 'critical'].includes((item.risk_level || '').toLowerCase()))
      if (target?.id) {
        goDetail(target.id)
        return
      }
      showCreateModal.value = true
    },
  },
  {
    title: '继续推进在途发布',
    description: inProgressCount.value > 0 ? `${inProgressCount.value} 个主单还在 PR、审批或待发布阶段。` : '当前没有明显的在途主单，可以直接创建新的发布任务。',
    label: inProgressCount.value > 0 ? '继续推进' : '创建发布',
    action: () => {
      const target = releases.value.find((item) => ['approved', 'pr_opened', 'pr_merged'].includes(item.status || ''))
      if (target?.id) {
        goDetail(target.id)
        return
      }
      showCreateModal.value = true
    },
  },
])

async function fetchList() {
  loading.value = true
  try {
    const res = await releaseApi.list({
      ...filter,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    const data: any = res?.data || {}
    releases.value = data.list || data.items || []
    pagination.total = data.total || 0
  } catch (e: any) {
    message.error(e?.message || '加载发布列表失败')
  } finally {
    loading.value = false
  }
}

async function loadCatalog() {
  try {
    const [orgRes, projectRes] = await Promise.all([catalogApi.listOrgs(), catalogApi.listProjects()])
    organizations.value = orgRes.data || []
    projects.value = projectRes.data || []
  } catch (error) {
    console.error('加载组织项目失败', error)
  }
}

function onSearch() {
  pagination.current = 1
  fetchList()
}

function onOrganizationChange() {
  filter.project_id = undefined
}

function resetFilter() {
  filter.title = ''
  filter.env = undefined
  filter.status = undefined
  filter.project_id = undefined
  filter.organization_id = undefined
  onSearch()
}

function clearFilter(key: 'title' | 'env' | 'status' | 'organization_id' | 'project_id') {
  if (key === 'organization_id') {
    filter.organization_id = undefined
    filter.project_id = undefined
  } else if (key === 'project_id') {
    filter.project_id = undefined
  } else {
    filter[key] = undefined
  }
  onSearch()
}

function onTableChange(p: any) {
  pagination.current = p.current
  pagination.pageSize = p.pageSize
  fetchList()
}

function goDetail(id?: number) {
  if (!id) return
  router.push(`/releases/${id}`)
}

async function onDelete(rec: Release) {
  if (!rec.id) return
  try {
    await releaseApi.delete(rec.id)
    message.success('已删除')
    fetchList()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '删除失败')
  }
}

async function onCreate() {
  if (!createForm.title.trim()) {
    message.warning('请填写标题')
    return
  }
  creating.value = true
  try {
    const res = await releaseApi.create({ ...createForm } as any)
    message.success('已创建')
    showCreateModal.value = false
    Object.assign(createForm, {
      title: '',
      application_name: '',
      env: 'dev',
      version: '',
      description: '',
      rollout_strategy: 'direct',
      risk_level: undefined,
    })
    if (res?.data?.id) {
      router.push(`/releases/${res.data.id}`)
    } else {
      fetchList()
    }
  } catch (e: any) {
    message.error(e?.response?.data?.message || '创建失败')
  } finally {
    creating.value = false
  }
}

function rolloutLabel(s?: string) {
  return s === 'canary' ? '金丝雀' : s === 'blue_green' ? '蓝绿' : '直接发布'
}

function rolloutColor(s?: string) {
  return s === 'canary' ? 'orange' : s === 'blue_green' ? 'geekblue' : 'default'
}

function statusLabel(s?: string) {
  return statusOptions.find((o) => o.value === s)?.label || s || '-'
}

function statusColor(s?: string) {
  const map: Record<string, string> = {
    draft: 'default',
    pending_approval: 'gold',
    approved: 'blue',
    rejected: 'red',
    pr_opened: 'cyan',
    pr_merged: 'geekblue',
    published: 'green',
    failed: 'red',
    rolled_back: 'volcano',
  }
  return map[s || ''] || 'default'
}

function formatTime(t?: string) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '-'
}

onMounted(() => {
  const projectId = Number(route.query.project_id)
  if (Number.isFinite(projectId) && projectId > 0) {
    filter.project_id = projectId
  }
  loadCatalog()
  fetchList()
})
</script>

<style scoped>
.release-list {
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
  margin: 0;
  font-size: 20px;
}

.page-subtitle {
  margin-top: 6px;
  color: #8c8c8c;
  font-size: 13px;
}

.hero-card {
  margin-bottom: 16px;
  background: linear-gradient(135deg, #fffaf0 0%, #ffffff 55%, #fffdf6 100%);
}

.hero-title {
  font-size: 22px;
  font-weight: 600;
  color: #1f1f1f;
}

.hero-subtitle {
  margin-top: 8px;
  line-height: 1.6;
  color: #6b7280;
}

.hero-tags {
  margin-top: 16px;
}

.summary-card {
  height: 100%;
}

.summary-hint {
  margin-top: 8px;
  color: #8c8c8c;
  font-size: 12px;
}

.filter-card,
.content-row {
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

.insight-card,
.table-card {
  height: 100%;
}

.insight-main,
.release-title-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.insight-title,
.release-link {
  font-weight: 500;
}

.table-summary,
.sub-text {
  font-size: 12px;
  color: #999;
}

@media (max-width: 768px) {
  .page-header,
  .section-header {
    flex-direction: column;
  }
}
</style>
