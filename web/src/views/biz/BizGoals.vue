<template>
  <div class="biz-page">
    <a-card :bordered="false" class="search-card">
      <a-form layout="inline">
        <a-form-item :label="t('common.keyword')">
          <a-input v-model:value="filters.keyword" :placeholder="t('menu.bizGoals')" allow-clear style="width: 240px" />
        </a-form-item>
        <a-form-item :label="t('common.status')">
          <a-select v-model:value="filters.status" allow-clear style="width: 160px">
            <a-select-option value="planning">规划中</a-select-option>
            <a-select-option value="in_progress">推进中</a-select-option>
            <a-select-option value="done">已完成</a-select-option>
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
            新建目标
          </a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card :bordered="false" style="margin-top: 16px">
      <a-table
        :columns="columns"
        :data-source="goals"
        :loading="loading"
        :pagination="paginationConfig"
        row-key="id"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <a-button type="link" style="padding: 0" @click="goDetail(record.id)">{{ record.name }}</a-button>
          </template>
          <template v-else-if="column.key === 'status'">
            <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'priority'">
            <a-tag :color="priorityColor(record.priority)">{{ priorityText(record.priority) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'updated_at'">
            {{ formatTime(record.updated_at) }}
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

    <a-modal v-model:open="modalVisible" :title="editingId ? '编辑业务目标' : '新建业务目标'" :confirm-loading="submitting" @ok="handleSubmit">
      <a-form layout="vertical">
        <a-form-item label="目标名称" required>
          <a-input v-model:value="form.name" placeholder="请输入目标名称" />
        </a-form-item>
        <a-form-item label="目标编码">
          <a-input v-model:value="form.code" placeholder="例如 GROWTH-2025-Q3" />
        </a-form-item>
        <a-form-item label="负责人">
          <a-input v-model:value="form.owner" placeholder="请输入负责人" />
        </a-form-item>
        <a-row :gutter="12">
          <a-col :span="12">
            <a-form-item label="状态">
              <a-select v-model:value="form.status">
                <a-select-option value="planning">规划中</a-select-option>
                <a-select-option value="in_progress">推进中</a-select-option>
                <a-select-option value="done">已完成</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="优先级">
              <a-select v-model:value="form.priority">
                <a-select-option value="high">高</a-select-option>
                <a-select-option value="medium">中</a-select-option>
                <a-select-option value="low">低</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="价值指标">
          <a-input v-model:value="form.value_metric" placeholder="例如 活跃用户提升 20%" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.description" :rows="4" placeholder="请输入业务目标描述" />
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
import { bizApi, type BizGoal } from '@/services/biz'

const { t } = useI18n()
const router = useRouter()
const loading = ref(false)
const submitting = ref(false)
const modalVisible = ref(false)
const editingId = ref<number | null>(null)
const goals = ref<BizGoal[]>([])

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

const form = reactive<Partial<BizGoal>>({
  name: '',
  code: '',
  owner: '',
  status: 'planning',
  priority: 'medium',
  value_metric: '',
  description: ''
})

const columns = [
  { title: '目标名称', dataIndex: 'name', key: 'name' },
  { title: '目标编码', dataIndex: 'code', key: 'code', width: 160 },
  { title: '负责人', dataIndex: 'owner', key: 'owner', width: 120 },
  { title: '状态', key: 'status', width: 110 },
  { title: '优先级', key: 'priority', width: 100 },
  { title: '价值指标', dataIndex: 'value_metric', key: 'value_metric', ellipsis: true },
  { title: '更新时间', key: 'updated_at', width: 180 },
  { title: '操作', key: 'action', width: 120 }
]

const statusText = (status: string) => ({ planning: '规划中', in_progress: '推进中', done: '已完成' }[status] || status)
const statusColor = (status: string) => ({ planning: 'blue', in_progress: 'gold', done: 'green' }[status] || 'default')
const priorityText = (priority: string) => ({ high: '高', medium: '中', low: '低' }[priority] || priority)
const priorityColor = (priority: string) => ({ high: 'red', medium: 'orange', low: 'default' }[priority] || 'default')
const formatTime = (value?: string) => value ? value.replace('T', ' ').slice(0, 19) : '-'

const resetForm = () => {
  editingId.value = null
  Object.assign(form, {
    name: '',
    code: '',
    owner: '',
    status: 'planning',
    priority: 'medium',
    value_metric: '',
    description: ''
  })
}

const fetchGoals = async () => {
  loading.value = true
  try {
    const res = await bizApi.getGoals({
      page: pagination.current,
      page_size: pagination.pageSize,
      status: filters.status,
      keyword: filters.keyword || undefined
    })
    goals.value = res.data.list || []
    pagination.total = res.data.total || 0
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  pagination.current = 1
  fetchGoals()
}

const handleReset = () => {
  filters.keyword = ''
  filters.status = undefined
  handleSearch()
}

const handleTableChange = (pager: TablePaginationConfig) => {
  pagination.current = pager.current || 1
  pagination.pageSize = pager.pageSize || 10
  fetchGoals()
}

const openCreate = () => {
  resetForm()
  modalVisible.value = true
}

const openEdit = (record: BizGoal) => {
  editingId.value = record.id
  Object.assign(form, record)
  modalVisible.value = true
}

const goDetail = (id: number) => {
  router.push(`/biz/goals/${id}`)
}

const handleSubmit = async () => {
  if (!form.name?.trim()) {
    message.error('请输入目标名称')
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await bizApi.updateGoal(editingId.value, form)
      message.success('更新成功')
    } else {
      await bizApi.createGoal(form)
      message.success('创建成功')
    }
    modalVisible.value = false
    fetchGoals()
  } finally {
    submitting.value = false
  }
}

const handleDelete = (record: BizGoal) => {
  Modal.confirm({
    title: '确认删除',
    content: `确定删除业务目标“${record.name}”吗？`,
    onOk: async () => {
      await bizApi.deleteGoal(record.id)
      message.success('删除成功')
      fetchGoals()
    }
  })
}

onMounted(fetchGoals)
</script>

<style scoped>
.biz-page {
  padding: 0;
}
</style>
