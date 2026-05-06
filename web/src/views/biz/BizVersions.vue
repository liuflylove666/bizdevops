<template>
  <div class="biz-page">
    <a-card :bordered="false" class="search-card">
      <a-form layout="inline">
        <a-form-item :label="t('common.keyword')">
          <a-input v-model:value="filters.keyword" placeholder="搜索版本名称" allow-clear style="width: 220px" />
        </a-form-item>
        <a-form-item :label="t('common.status')">
          <a-select v-model:value="filters.status" allow-clear style="width: 160px">
            <a-select-option value="planning">规划中</a-select-option>
            <a-select-option value="in_progress">进行中</a-select-option>
            <a-select-option value="released">已发布</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" @click="handleSearch">查询</a-button>
            <a-button @click="handleReset">重置</a-button>
          </a-space>
        </a-form-item>
        <a-form-item style="margin-left: auto">
          <a-button type="primary" @click="openCreate">
            <template #icon><PlusOutlined /></template>
            新建版本
          </a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card :bordered="false" style="margin-top: 16px">
      <a-table
        :columns="columns"
        :data-source="versions"
        :loading="loading"
        :pagination="paginationConfig"
        row-key="id"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <a-button type="link" style="padding: 0" @click="goDetail(record.id)">{{ record.name }}</a-button>
          </template>
          <template v-else-if="column.key === 'goal_name'">
            {{ goalName(record.goal_id) }}
          </template>
          <template v-else-if="column.key === 'application_name'">
            {{ record.application_name || applicationName(record.application_id) }}
          </template>
          <template v-else-if="column.key === 'status'">
            <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'release_date'">
            {{ formatTime(record.release_date) }}
          </template>
          <template v-else-if="column.key === 'release_title'">
            {{ record.release_title || releaseTitle(record.release_id) }}
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button type="link" size="small" @click="goDetail(record.id)">详情</a-button>
              <a-button type="link" size="small" @click="openEdit(record)">编辑</a-button>
              <a-button type="link" size="small" danger @click="handleDelete(record)">删除</a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal v-model:open="modalVisible" :title="editingId ? '编辑版本计划' : '新建版本计划'" :confirm-loading="submitting" @ok="handleSubmit">
      <a-form layout="vertical">
        <a-form-item label="版本名称" required>
          <a-input v-model:value="form.name" placeholder="请输入版本名称" />
        </a-form-item>
        <a-form-item label="版本编码">
          <a-input v-model:value="form.code" placeholder="例如 V2025.08" />
        </a-form-item>
        <a-row :gutter="12">
          <a-col :span="12">
            <a-form-item label="关联业务目标">
              <a-select v-model:value="form.goal_id" allow-clear placeholder="选择业务目标">
                <a-select-option v-for="goal in goalOptions" :key="goal.id" :value="goal.id">{{ goal.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="负责人">
              <a-input v-model:value="form.owner" placeholder="请输入负责人" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="12">
          <a-col :span="8">
            <a-form-item label="关联应用">
              <a-select v-model:value="form.application_id" allow-clear placeholder="选择应用">
                <a-select-option v-for="app in applicationOptions" :key="app.id" :value="app.id">
                  {{ app.display_name || app.name }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="关联流水线">
              <a-select v-model:value="form.pipeline_id" allow-clear placeholder="选择流水线">
                <a-select-option v-for="pipeline in pipelineOptions" :key="pipeline.id" :value="pipeline.id">
                  {{ pipeline.name }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="关联发布单">
              <a-select v-model:value="form.release_id" allow-clear placeholder="选择发布单">
                <a-select-option v-for="release in releaseOptions" :key="release.id" :value="release.id">
                  {{ release.title }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="状态">
          <a-select v-model:value="form.status">
            <a-select-option value="planning">规划中</a-select-option>
            <a-select-option value="in_progress">进行中</a-select-option>
            <a-select-option value="released">已发布</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.description" :rows="4" placeholder="请输入版本规划描述" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message, Modal } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import type { TablePaginationConfig } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { bizApi, type BizGoal, type BizVersion } from '@/services/biz'
import { applicationApi, type Application } from '@/services/application'
import { pipelineApi } from '@/services/pipeline'
import { releaseApi, type Release } from '@/services/release'

const { t } = useI18n()
const router = useRouter()
const loading = ref(false)
const submitting = ref(false)
const modalVisible = ref(false)
const editingId = ref<number | null>(null)
const versions = ref<BizVersion[]>([])
const goalOptions = ref<BizGoal[]>([])
const applicationOptions = ref<Application[]>([])
const pipelineOptions = ref<Array<{ id: number; name: string }>>([])
const releaseOptions = ref<Release[]>([])

const filters = reactive({
  keyword: '',
  status: undefined as string | undefined
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0
})

const paginationConfig = computed<TablePaginationConfig>(() => ({
  current: pagination.current,
  pageSize: pagination.pageSize,
  total: pagination.total,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`
}))

const form = reactive<Partial<BizVersion>>({
  name: '',
  code: '',
  goal_id: undefined,
  application_id: undefined,
  pipeline_id: undefined,
  release_id: undefined,
  owner: '',
  status: 'planning',
  description: ''
})

const columns = [
  { title: '版本名称', dataIndex: 'name', key: 'name' },
  { title: '版本编码', dataIndex: 'code', key: 'code', width: 150 },
  { title: '业务目标', key: 'goal_name', width: 180 },
  { title: '关联应用', key: 'application_name', width: 160 },
  { title: '发布单', key: 'release_title', width: 180 },
  { title: '负责人', dataIndex: 'owner', key: 'owner', width: 120 },
  { title: '状态', key: 'status', width: 110 },
  { title: '计划发布时间', key: 'release_date', width: 180 },
  { title: '操作', key: 'action', width: 120 }
]

const statusText = (status: string) => ({ planning: '规划中', in_progress: '进行中', released: '已发布' }[status] || status)
const statusColor = (status: string) => ({ planning: 'blue', in_progress: 'gold', released: 'green' }[status] || 'default')
const formatTime = (value?: string) => value ? value.replace('T', ' ').slice(0, 19) : '-'
const goalName = (goalId?: number) => goalOptions.value.find(item => item.id === goalId)?.name || '-'
const applicationName = (applicationId?: number) => applicationOptions.value.find(item => item.id === applicationId)?.display_name || applicationOptions.value.find(item => item.id === applicationId)?.name || '-'
const releaseTitle = (releaseId?: number) => releaseOptions.value.find(item => item.id === releaseId)?.title || '-'

const resetForm = () => {
  editingId.value = null
  Object.assign(form, {
    name: '',
    code: '',
    goal_id: undefined,
    application_id: undefined,
    pipeline_id: undefined,
    release_id: undefined,
    owner: '',
    status: 'planning',
    description: ''
  })
}

const fetchGoals = async () => {
  const [goalRes, appRes, pipelineRes, releaseRes] = await Promise.all([
    bizApi.getGoals({ page: 1, page_size: 1000 }),
    applicationApi.list({ page: 1, page_size: 1000 }),
    pipelineApi.list({ page: 1, page_size: 1000 }),
    releaseApi.list({ page: 1, page_size: 1000 })
  ])
  goalOptions.value = goalRes.data.list || []
  applicationOptions.value = appRes.data?.list || []
  pipelineOptions.value = pipelineRes.data.list || []
  releaseOptions.value = releaseRes.data?.list || releaseRes.data?.items || []
}

const fetchVersions = async () => {
  loading.value = true
  try {
    const res = await bizApi.getVersions({
      page: pagination.current,
      page_size: pagination.pageSize,
      status: filters.status,
      keyword: filters.keyword || undefined
    })
    versions.value = res.data.list || []
    pagination.total = res.data.total || 0
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  pagination.current = 1
  fetchVersions()
}

const handleReset = () => {
  filters.keyword = ''
  filters.status = undefined
  handleSearch()
}

const handleTableChange = (pager: TablePaginationConfig) => {
  pagination.current = pager.current || 1
  pagination.pageSize = pager.pageSize || 10
  fetchVersions()
}

const openCreate = () => {
  resetForm()
  modalVisible.value = true
}

const openEdit = (record: BizVersion) => {
  editingId.value = record.id
  Object.assign(form, record)
  modalVisible.value = true
}

const goDetail = (id: number) => {
  router.push(`/biz/versions/${id}`)
}

const handleSubmit = async () => {
  if (!form.name?.trim()) {
    message.error('请输入版本名称')
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await bizApi.updateVersion(editingId.value, form)
      message.success('更新成功')
    } else {
      await bizApi.createVersion(form)
      message.success('创建成功')
    }
    modalVisible.value = false
    fetchVersions()
  } finally {
    submitting.value = false
  }
}

const handleDelete = (record: BizVersion) => {
  Modal.confirm({
    title: '确认删除',
    content: `确定删除版本“${record.name}”吗？`,
    onOk: async () => {
      await bizApi.deleteVersion(record.id)
      message.success('删除成功')
      fetchVersions()
    }
  })
}

onMounted(async () => {
  await fetchGoals()
  await fetchVersions()
})
</script>

<style scoped>
.biz-page {
  padding: 0;
}
</style>
