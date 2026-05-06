<template>
  <div class="db-ticket-list">
    <div class="page-header">
      <h1>SQL 工单</h1>
      <a-space>
        <a-button type="primary" @click="$router.push('/database/tickets/create')">新建工单</a-button>
      </a-space>
    </div>

    <a-card :bordered="false">
      <a-form layout="inline" class="filter-bar">
        <a-form-item label="实例">
          <a-select
            v-model:value="filters.instance_id"
            allow-clear
            style="width: 220px"
            placeholder="全部"
            :options="instanceOptions"
            @change="loadList"
            show-search
            :filter-option="filterOption"
          />
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="filters.status" allow-clear style="width: 130px" placeholder="全部" @change="loadList">
            <a-select-option :value="0">审批中</a-select-option>
            <a-select-option :value="1">已驳回</a-select-option>
            <a-select-option :value="2">待执行</a-select-option>
            <a-select-option :value="3">执行中</a-select-option>
            <a-select-option :value="4">已成功</a-select-option>
            <a-select-option :value="5">已失败</a-select-option>
            <a-select-option :value="6">已撤回</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="类型">
          <a-select v-model:value="filters.change_type" allow-clear style="width: 120px" placeholder="全部" @change="loadList">
            <a-select-option :value="0">DDL</a-select-option>
            <a-select-option :value="1">DML</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="关键字">
          <a-input v-model:value="filters.keyword" allow-clear @press-enter="loadList" placeholder="标题/工单号" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" @click="loadList">查询</a-button>
        </a-form-item>
      </a-form>

      <a-table
        :columns="columns"
        :data-source="list"
        :loading="loading"
        row-key="id"
        :pagination="{ current: pagination.page, pageSize: pagination.pageSize, total: pagination.total, onChange: onPageChange }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
          </template>
          <template v-if="column.key === 'change_type'">
            <a-tag :color="record.change_type === 0 ? 'purple' : 'blue'">{{ record.change_type === 0 ? 'DDL' : 'DML' }}</a-tag>
          </template>
          <template v-if="column.key === 'work_id'">
            <a @click="$router.push(`/database/tickets/${record.id}`)">{{ record.work_id }}</a>
          </template>
          <template v-if="column.key === 'action'">
            <a-button type="link" size="small" @click="$router.push(`/database/tickets/${record.id}`)">详情</a-button>
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { dbInstanceApi, dbTicketApi, type DBInstance, type SQLChangeTicket } from '@/services/database'

const columns = [
  { title: '工单号', dataIndex: 'work_id', key: 'work_id', width: 180 },
  { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '类型', dataIndex: 'change_type', key: 'change_type', width: 80 },
  { title: '申请人', dataIndex: 'applicant', key: 'applicant', width: 120 },
  { title: '实例', dataIndex: 'instance_id', key: 'instance_id', width: 100 },
  { title: '库', dataIndex: 'schema_name', key: 'schema_name', width: 120 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: '操作', key: 'action', width: 80 }
]

const loading = ref(false)
const list = ref<SQLChangeTicket[]>([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const filters = reactive<{ instance_id?: number; status?: number; change_type?: number; keyword?: string }>({})
const instances = ref<DBInstance[]>([])

const instanceOptions = computed(() => instances.value.map(i => ({ value: i.id, label: `${i.env}/${i.name}` })))

function filterOption(input: string, option: any) {
  return (option.label || '').toLowerCase().includes(input.toLowerCase())
}

function statusColor(s: number) {
  return ['orange', 'red', 'blue', 'processing', 'green', 'red', 'default'][s] || 'default'
}
function statusText(s: number) {
  return ['审批中', '已驳回', '待执行', '执行中', '已成功', '已失败', '已撤回'][s] || '未知'
}

async function loadInstances() {
  const res = await dbInstanceApi.listAll()
  instances.value = res.data || []
}

async function loadList() {
  loading.value = true
  try {
    const params: Record<string, unknown> = { page: pagination.page, page_size: pagination.pageSize }
    if (filters.instance_id) params.instance_id = filters.instance_id
    if (filters.status !== undefined) params.status = filters.status
    if (filters.change_type !== undefined) params.change_type = filters.change_type
    if (filters.keyword) params.keyword = filters.keyword
    const res = await dbTicketApi.list(params)
    list.value = res.data?.list || []
    pagination.total = res.data?.total || 0
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function onPageChange(page: number, pageSize: number) {
  pagination.page = page
  pagination.pageSize = pageSize
  loadList()
}

onMounted(async () => {
  await loadInstances()
  loadList()
})
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header h1 { margin: 0; font-size: 20px; }
.filter-bar { margin-bottom: 16px; }
</style>
