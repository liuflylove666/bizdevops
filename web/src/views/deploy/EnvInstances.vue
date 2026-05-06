<template>
  <div class="env-instances">
    <a-tabs v-model:activeKey="viewMode">
      <!-- 列表视图 -->
      <a-tab-pane key="list" tab="实例列表">
        <a-card :bordered="false">
          <template #extra>
            <a-space>
              <a-select v-model:value="filter.env" placeholder="环境" allow-clear style="width: 110px" @change="loadList">
                <a-select-option value="dev">dev</a-select-option>
                <a-select-option value="test">test</a-select-option>
                <a-select-option value="uat">uat</a-select-option>
                <a-select-option value="gray">gray</a-select-option>
                <a-select-option value="prod">prod</a-select-option>
              </a-select>
              <a-select v-model:value="filter.status" placeholder="状态" allow-clear style="width: 120px" @change="loadList">
                <a-select-option value="running">运行中</a-select-option>
                <a-select-option value="stopped">已停止</a-select-option>
                <a-select-option value="deploying">部署中</a-select-option>
                <a-select-option value="failed">失败</a-select-option>
                <a-select-option value="unknown">未知</a-select-option>
              </a-select>
              <a-button type="primary" @click="showCreateModal">新建实例</a-button>
            </a-space>
          </template>

          <a-table :columns="columns" :data-source="list" :loading="loading" row-key="id"
            :pagination="{ current: filter.page, pageSize: filter.page_size, total, showSizeChanger: true, showTotal: (t: number) => `共 ${t} 条` }"
            @change="handleTableChange">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'env'">
                <a-tag :color="envColor(record.env)">{{ record.env }}</a-tag>
              </template>
              <template v-if="column.key === 'status'">
                <a-badge :status="statusBadge(record.status)" :text="statusText(record.status)" />
              </template>
              <template v-if="column.key === 'image_tag'">
                <span style="font-family: monospace; font-size: 12px">{{ record.image_tag || '-' }}</span>
              </template>
              <template v-if="column.key === 'image_digest'">
                <a-tooltip v-if="record.image_digest" :title="record.image_digest">
                  <span style="font-family: monospace; font-size: 11px">{{ record.image_digest.substring(0, 16) }}...</span>
                </a-tooltip>
                <span v-else style="color: #ff4d4f">未校验</span>
              </template>
              <template v-if="column.key === 'last_deploy_at'">
                {{ formatTime(record.last_deploy_at) }}
              </template>
              <template v-if="column.key === 'action'">
                <a-space>
                  <a @click="showCreateModal(record)">编辑</a>
                  <a-popconfirm title="确定删除？" @confirm="handleDelete(record.id)">
                    <a style="color: #ff4d4f">删除</a>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 环境矩阵视图 -->
      <a-tab-pane key="matrix" tab="环境矩阵">
        <a-card :bordered="false" :loading="loadingMatrix">
          <a-table :columns="matrixColumns" :data-source="matrixData" row-key="app_name" :pagination="false" size="small" bordered>
            <template #bodyCell="{ column, record }">
              <template v-if="envList.includes(column.key as string)">
                <div v-if="record[column.key as string]" style="text-align: center">
                  <a-badge :status="statusBadge(record[column.key as string].status)" />
                  <div style="font-size: 11px; font-family: monospace">{{ record[column.key as string].image_tag || '-' }}</div>
                  <div style="font-size: 10px; color: #999">{{ record[column.key as string].replicas }}副本</div>
                </div>
                <div v-else style="text-align: center; color: #ccc">-</div>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>

    <!-- 新建/编辑弹窗 -->
    <a-modal v-model:open="createVisible" :title="editingId ? '编辑环境实例' : '新建环境实例'" width="700px" @ok="handleCreateOrUpdate" :confirmLoading="submitting">
      <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 18 }">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="应用名称" :label-col="{ span: 10 }" :wrapper-col="{ span: 14 }" required>
              <a-input v-model:value="form.application_name" placeholder="应用名称" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="环境" :label-col="{ span: 10 }" :wrapper-col="{ span: 14 }" required>
              <a-select v-model:value="form.env">
                <a-select-option value="dev">dev</a-select-option>
                <a-select-option value="test">test</a-select-option>
                <a-select-option value="uat">uat</a-select-option>
                <a-select-option value="gray">gray</a-select-option>
                <a-select-option value="prod">prod</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="集群名称" :label-col="{ span: 10 }" :wrapper-col="{ span: 14 }">
              <a-input v-model:value="form.cluster_name" placeholder="K8s 集群" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="命名空间" :label-col="{ span: 10 }" :wrapper-col="{ span: 14 }">
              <a-input v-model:value="form.namespace" placeholder="namespace" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="Deployment">
          <a-input v-model:value="form.deployment_name" placeholder="Deployment 名称" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="16">
            <a-form-item label="镜像地址" :label-col="{ span: 7 }" :wrapper-col="{ span: 17 }">
              <a-input v-model:value="form.image_url" placeholder="镜像仓库地址" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="标签" :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }">
              <a-input v-model:value="form.image_tag" placeholder="tag" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="镜像 Digest">
          <a-input v-model:value="form.image_digest" placeholder="sha256:..." />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="副本数" :label-col="{ span: 12 }" :wrapper-col="{ span: 12 }">
              <a-input-number v-model:value="form.replicas" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="状态" :label-col="{ span: 12 }" :wrapper-col="{ span: 12 }">
              <a-select v-model:value="form.status">
                <a-select-option value="running">运行中</a-select-option>
                <a-select-option value="stopped">已停止</a-select-option>
                <a-select-option value="unknown">未知</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { envInstanceApi } from '@/services/envInstance'
import type { EnvInstance } from '@/services/envInstance'

const loading = ref(false)
const submitting = ref(false)
const loadingMatrix = ref(false)
const list = ref<EnvInstance[]>([])
const total = ref(0)
const matrixRaw = ref<EnvInstance[]>([])
const viewMode = ref('list')
const createVisible = ref(false)
const editingId = ref<number | null>(null)
const envList = ['dev', 'test', 'uat', 'gray', 'prod']

const filter = reactive({
  env: undefined as string | undefined,
  status: undefined as string | undefined,
  page: 1,
  page_size: 20,
})

const form = reactive({
  application_id: undefined as number | undefined,
  application_name: '',
  env: 'dev',
  cluster_name: '',
  namespace: '',
  deployment_name: '',
  image_url: '',
  image_tag: '',
  image_digest: '',
  replicas: 1,
  status: 'unknown',
})

const columns = [
  { title: '应用', dataIndex: 'application_name', key: 'application_name', width: 140 },
  { title: '环境', dataIndex: 'env', key: 'env', width: 80 },
  { title: '集群', dataIndex: 'cluster_name', key: 'cluster_name', width: 120 },
  { title: '命名空间', dataIndex: 'namespace', key: 'namespace', width: 120 },
  { title: '镜像标签', dataIndex: 'image_tag', key: 'image_tag', width: 150 },
  { title: 'Digest', dataIndex: 'image_digest', key: 'image_digest', width: 150 },
  { title: '副本', dataIndex: 'replicas', key: 'replicas', width: 60 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 90 },
  { title: '最近部署', dataIndex: 'last_deploy_at', key: 'last_deploy_at', width: 160 },
  { title: '操作', key: 'action', width: 120 },
]

const matrixColumns = computed(() => [
  { title: '应用', dataIndex: 'app_name', key: 'app_name', width: 160, fixed: 'left' as const },
  ...envList.map(e => ({ title: e.toUpperCase(), key: e, width: 140, align: 'center' as const })),
])

const matrixData = computed(() => {
  const map: Record<string, any> = {}
  for (const inst of matrixRaw.value) {
    if (!map[inst.application_name]) {
      map[inst.application_name] = { app_name: inst.application_name }
    }
    map[inst.application_name][inst.env] = inst
  }
  return Object.values(map)
})

const envColor = (e: string) => ({ dev: 'blue', test: 'green', uat: 'orange', gray: 'purple', prod: 'red' }[e] || 'default')
const statusBadge = (s: string) => ({ running: 'success' as const, stopped: 'default' as const, deploying: 'processing' as const, failed: 'error' as const }[s] || 'warning' as const)
const statusText = (s: string) => ({ running: '运行中', stopped: '已停止', deploying: '部署中', failed: '失败', unknown: '未知' }[s] || s)
const formatTime = (t?: string) => t ? new Date(t).toLocaleString('zh-CN') : '-'

async function loadList() {
  loading.value = true
  try {
    const res = await envInstanceApi.list(filter)
    list.value = res.list || res.items || []
    total.value = res.total || 0
  } catch {
    message.error('加载失败')
  } finally {
    loading.value = false
  }
}

async function loadMatrix() {
  loadingMatrix.value = true
  try {
    const res = await envInstanceApi.matrix()
    matrixRaw.value = res || []
  } catch { matrixRaw.value = [] }
  finally { loadingMatrix.value = false }
}

function handleTableChange(pagination: any) {
  filter.page = pagination.current
  filter.page_size = pagination.pageSize
  loadList()
}

function resetForm() {
  form.application_id = undefined
  form.application_name = ''
  form.env = 'dev'
  form.cluster_name = ''
  form.namespace = ''
  form.deployment_name = ''
  form.image_url = ''
  form.image_tag = ''
  form.image_digest = ''
  form.replicas = 1
  form.status = 'unknown'
}

function showCreateModal(record?: EnvInstance) {
  resetForm()
  if (record) {
    editingId.value = record.id!
    Object.assign(form, {
      application_id: record.application_id,
      application_name: record.application_name,
      env: record.env,
      cluster_name: record.cluster_name,
      namespace: record.namespace,
      deployment_name: record.deployment_name,
      image_url: record.image_url,
      image_tag: record.image_tag,
      image_digest: record.image_digest,
      replicas: record.replicas,
      status: record.status,
    })
  } else {
    editingId.value = null
  }
  createVisible.value = true
}

async function handleCreateOrUpdate() {
  if (!form.application_name || !form.env) {
    message.warning('请填写应用名称和环境')
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await envInstanceApi.update(editingId.value, { ...form })
      message.success('更新成功')
    } else {
      await envInstanceApi.create({ ...form })
      message.success('创建成功')
    }
    createVisible.value = false
    loadList()
    loadMatrix()
  } catch {
    message.error('操作失败')
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await envInstanceApi.delete(id)
    message.success('已删除')
    loadList()
  } catch {
    message.error('删除失败')
  }
}

onMounted(() => {
  loadList()
  loadMatrix()
})
</script>

<style scoped>
.env-instances {
  padding: 0;
}
</style>
