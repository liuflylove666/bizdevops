<template>
  <div class="db-query-logs">
    <div class="page-header">
      <h1>查询日志</h1>
    </div>

    <a-card :bordered="false">
      <a-form layout="inline" class="filter-bar">
        <a-form-item label="实例">
          <a-select
            v-model:value="filters.instance_id"
            allow-clear
            style="width: 220px"
            placeholder="全部实例"
            :options="instanceOptions"
            @change="loadList"
            show-search
            :filter-option="filterOption"
          />
        </a-form-item>
        <a-form-item label="用户">
          <a-input v-model:value="filters.username" allow-clear @press-enter="loadList" />
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="filters.status" allow-clear style="width: 120px" placeholder="全部" @change="loadList">
            <a-select-option value="success">success</a-select-option>
            <a-select-option value="failed">failed</a-select-option>
            <a-select-option value="blocked">blocked</a-select-option>
          </a-select>
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
            <a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag>
          </template>
          <template v-if="column.key === 'sql_text'">
            <a-typography-paragraph :ellipsis="{ rows: 2, expandable: true }">
              {{ record.sql_text ?? '' }}
            </a-typography-paragraph>
          </template>
          <template v-if="column.key === 'error_msg'">
            <a-typography-text type="danger" v-if="record.error_msg">{{ record.error_msg }}</a-typography-text>
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { dbInstanceApi, dbLogApi, type DBInstance, type DBQueryLog } from '@/services/database'

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
  { title: '时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: '实例', dataIndex: 'instance_id', key: 'instance_id', width: 100 },
  { title: '用户', dataIndex: 'username', key: 'username', width: 120 },
  { title: '库', dataIndex: 'schema_name', key: 'schema_name', width: 120 },
  { title: 'SQL', dataIndex: 'sql_text', key: 'sql_text', ellipsis: true },
  { title: '行数', dataIndex: 'affect_rows', key: 'affect_rows', width: 80 },
  { title: '耗时(ms)', dataIndex: 'exec_ms', key: 'exec_ms', width: 100 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 90 },
  { title: '错误', dataIndex: 'error_msg', key: 'error_msg', ellipsis: true }
]

const loading = ref(false)
const list = ref<DBQueryLog[]>([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const filters = reactive<{ instance_id?: number; username?: string; status?: string }>({})
const instances = ref<DBInstance[]>([])

const instanceOptions = computed(() => instances.value.map(i => ({ value: i.id, label: `${i.env}/${i.name}` })))

function filterOption(input: string, option: any) {
  return (option.label || '').toLowerCase().includes(input.toLowerCase())
}

function statusColor(s: string) {
  return { success: 'green', failed: 'red', blocked: 'orange' }[s] || 'default'
}

async function loadInstances() {
  const res = await dbInstanceApi.listAll()
  instances.value = res.data || []
}

async function loadList() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: pagination.page,
      page_size: pagination.pageSize
    }
    if (filters.instance_id) params.instance_id = filters.instance_id
    if (filters.username) params.username = filters.username
    if (filters.status) params.status = filters.status
    const res = await dbLogApi.list(params)
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
