<template>
  <div class="db-statements">
    <div class="page-header">
      <h1>执行记录</h1>
    </div>

    <a-card :bordered="false">
      <a-form layout="inline" class="filter-bar">
        <a-form-item label="工单编号">
          <a-input v-model:value="filters.work_id" allow-clear placeholder="工单编号" @press-enter="loadList" />
        </a-form-item>
        <a-form-item label="实例">
          <a-select v-model:value="filters.instance_id" allow-clear style="width: 180px" placeholder="全部" @change="loadList">
            <a-select-option v-for="inst in instances" :key="inst.id" :value="inst.id">{{ inst.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="filters.state" allow-clear style="width: 120px" placeholder="全部" @change="loadList">
            <a-select-option value="pending">待执行</a-select-option>
            <a-select-option value="running">执行中</a-select-option>
            <a-select-option value="success">成功</a-select-option>
            <a-select-option value="failed">失败</a-select-option>
            <a-select-option value="skipped">跳过</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="提交人">
          <a-input v-model:value="filters.applicant" allow-clear placeholder="用户名" @press-enter="loadList" />
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
        :scroll="{ x: 1400 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'state'">
            <a-tag :color="stateColor(record.state)">{{ stateLabel(record.state) }}</a-tag>
          </template>
          <template v-if="column.key === 'sql_text'">
            <a-tooltip :title="record.sql_text">
              <span class="sql-cell">{{ record.sql_text }}</span>
            </a-tooltip>
          </template>
          <template v-if="column.key === 'ticket'">
            <router-link :to="`/database/tickets/${record.ticket_id}`">{{ record.ticket_work_id }}</router-link>
            <span style="margin-left:4px;color:#999">{{ record.ticket_title }}</span>
          </template>
          <template v-if="column.key === 'ticket_status'">
            <a-tag :color="ticketStatusColor(record.ticket_status)">{{ ticketStatusLabel(record.ticket_status) }}</a-tag>
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { dbStatementApi, dbInstanceApi, type StatementListItem, type DBInstance } from '@/services/database'

const columns = [
  { title: '工单', key: 'ticket', width: 240 },
  { title: '序号', dataIndex: 'seq', width: 60 },
  { title: 'SQL', key: 'sql_text', ellipsis: true },
  { title: '状态', key: 'state', width: 90 },
  { title: '影响行', dataIndex: 'affect_rows', width: 80 },
  { title: '耗时(ms)', dataIndex: 'exec_ms', width: 90 },
  { title: '提交人', dataIndex: 'applicant', width: 100 },
  { title: '工单状态', key: 'ticket_status', width: 100 },
  { title: '执行时间', dataIndex: 'executed_at', width: 170 },
  { title: '错误信息', dataIndex: 'error_msg', ellipsis: true, width: 200 }
]

const loading = ref(false)
const list = ref<StatementListItem[]>([])
const instances = ref<DBInstance[]>([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const filters = reactive<Record<string, any>>({ work_id: '', instance_id: undefined, state: undefined, applicant: '' })

function stateColor(s: string) {
  return { pending: 'default', running: 'processing', success: 'success', failed: 'error', skipped: 'warning' }[s] || 'default'
}
function stateLabel(s: string) {
  return { pending: '待执行', running: '执行中', success: '成功', failed: '失败', skipped: '跳过' }[s] || s
}
function ticketStatusColor(s: number) {
  return { 0: 'processing', 1: 'error', 2: 'warning', 3: 'processing', 4: 'success', 5: 'error', 6: 'default' }[s] || 'default'
}
function ticketStatusLabel(s: number) {
  return { 0: '审批中', 1: '已驳回', 2: '待执行', 3: '执行中', 4: '成功', 5: '失败', 6: '已撤回' }[s] || '未知'
}

async function loadList() {
  loading.value = true
  try {
    const params: Record<string, any> = { page: pagination.page, page_size: pagination.pageSize }
    if (filters.work_id) params.work_id = filters.work_id
    if (filters.instance_id) params.instance_id = filters.instance_id
    if (filters.state) params.state = filters.state
    if (filters.applicant) params.applicant = filters.applicant
    const res = await dbStatementApi.list(params)
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

async function loadInstances() {
  try {
    const res = await dbInstanceApi.listAll()
    instances.value = res.data || []
  } catch { /* ignore */ }
}

onMounted(() => {
  loadInstances()
  loadList()
})
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header h1 { margin: 0; font-size: 20px; }
.filter-bar { margin-bottom: 16px; }
.sql-cell { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: inline-block; }
</style>
