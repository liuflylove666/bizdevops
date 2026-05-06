<template>
  <div class="nacos-release">
    <a-card :bordered="false">
      <template #title>
        <a-space>
          <span>Nacos 配置发布单</span>
          <a-tag color="blue">{{ total }} 条</a-tag>
        </a-space>
      </template>
      <template #extra>
        <a-space>
          <a-select v-model:value="filter.env" placeholder="环境" allow-clear style="width: 120px" @change="loadList">
            <a-select-option value="dev">dev</a-select-option>
            <a-select-option value="test">test</a-select-option>
            <a-select-option value="uat">uat</a-select-option>
            <a-select-option value="gray">gray</a-select-option>
            <a-select-option value="prod">prod</a-select-option>
          </a-select>
          <a-select v-model:value="filter.status" placeholder="状态" allow-clear style="width: 140px" @change="loadList">
            <a-select-option value="draft">草稿</a-select-option>
            <a-select-option value="pending_approval">待审批</a-select-option>
            <a-select-option value="approved">已审批</a-select-option>
            <a-select-option value="published">已发布</a-select-option>
            <a-select-option value="rolled_back">已回滚</a-select-option>
            <a-select-option value="rejected">已驳回</a-select-option>
          </a-select>
          <a-input-search v-model:value="filter.data_id" placeholder="搜索 DataID" style="width: 200px" @search="loadList" allow-clear />
          <a-button type="primary" @click="showCreateModal">新建发布单</a-button>
        </a-space>
      </template>

      <a-table :columns="columns" :data-source="list" :loading="loading" row-key="id"
        :pagination="{ current: filter.page, pageSize: filter.page_size, total, showSizeChanger: true, showTotal: (t: number) => `共 ${t} 条` }"
        @change="handleTableChange">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
          </template>
          <template v-if="column.key === 'env'">
            <a-tag :color="envColor(record.env)">{{ record.env }}</a-tag>
          </template>
          <template v-if="column.key === 'risk_level'">
            <a-tag :color="riskColor(record.risk_level)">{{ riskText(record.risk_level) }}</a-tag>
          </template>
          <template v-if="column.key === 'created_at'">
            {{ formatTime(record.created_at) }}
          </template>
          <template v-if="column.key === 'action'">
            <a-space>
              <a @click="showDetail(record)">详情</a>
              <a v-if="record.approval_instance_id" @click="goToApprovalInstance(record.approval_instance_id)">审批详情</a>
              <a v-if="record.status === 'draft'" @click="handleSubmit(record.id)">提交审批</a>
              <!-- 已接入统一审批中心：审批通过/驳回请在审批实例内操作 -->
              <a v-if="record.status === 'pending_approval' && !record.approval_instance_id" @click="handleApprove(record.id)">通过</a>
              <a v-if="record.status === 'pending_approval' && !record.approval_instance_id" style="color: #faad14" @click="showRejectModal(record.id)">驳回</a>
              <a v-if="record.status === 'approved'" style="color: #52c41a" @click="handlePublish(record.id)">发布</a>
              <a v-if="record.status === 'published'" style="color: #ff4d4f" @click="handleRollback(record.id)">回滚</a>
              <a v-if="record.status === 'draft'" @click="showCreateModal(record)">编辑</a>
              <a-popconfirm v-if="record.status === 'draft'" title="确定删除？" @confirm="handleDelete(record.id)">
                <a style="color: #ff4d4f">删除</a>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <!-- 新建/编辑弹窗 -->
    <a-modal v-model:open="createVisible" :title="editingId ? '编辑发布单' : '新建发布单'" width="900px" @ok="handleCreateOrUpdate" :confirmLoading="submitting">
      <a-form :label-col="{ span: 4 }" :wrapper-col="{ span: 19 }">
        <a-form-item label="标题" required>
          <a-input v-model:value="form.title" placeholder="发布单标题" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="Nacos 实例" :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }" required>
              <a-select v-model:value="form.nacos_instance_id" placeholder="选择实例" @change="handleInstanceChange">
                <a-select-option v-for="inst in instances" :key="inst.id" :value="inst.id!">
                  {{ inst.name }} ({{ inst.env }})
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="环境" :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }" required>
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
            <a-form-item label="Group" :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }">
              <a-input v-model:value="form.group" placeholder="DEFAULT_GROUP" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="DataID" :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }" required>
              <a-select
                v-model:value="form.data_id"
                placeholder="自动获取 DataID"
                :loading="loadingDataIds"
                :disabled="!form.nacos_instance_id"
                show-search
                :filter-option="filterDataIdOption"
              >
                <a-select-option v-for="item in dataIdOptions" :key="item" :value="item">
                  {{ item }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="配置类型" :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }">
              <a-select v-model:value="form.config_type">
                <a-select-option value="yaml">YAML</a-select-option>
                <a-select-option value="json">JSON</a-select-option>
                <a-select-option value="properties">Properties</a-select-option>
                <a-select-option value="text">Text</a-select-option>
                <a-select-option value="xml">XML</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="风险等级" :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }">
              <a-select v-model:value="form.risk_level">
                <a-select-option value="low">低</a-select-option>
                <a-select-option value="medium">中</a-select-option>
                <a-select-option value="high">高</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="命名空间">
          <a-input v-model:value="form.tenant" placeholder="留空为 public" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.description" :rows="2" placeholder="变更描述" />
        </a-form-item>
        <a-form-item label="变更后内容" required>
          <div style="display: flex; justify-content: flex-end; margin-bottom: 8px">
            <a-button size="small" @click="fetchCurrentContent" :loading="fetchingContent">拉取当前配置</a-button>
          </div>
          <a-textarea v-model:value="form.content_after" :rows="12" placeholder="填写变更后的配置内容" style="font-family: monospace" />
        </a-form-item>
        <a-form-item v-if="form.content_before" label="当前配置">
          <a-textarea :value="form.content_before" :rows="8" disabled style="font-family: monospace" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 详情弹窗 -->
    <a-modal v-model:open="detailVisible" title="发布单详情" width="1000px" :footer="null">
      <template v-if="detailRecord">
        <a-descriptions :column="3" bordered size="small">
          <a-descriptions-item label="标题" :span="3">{{ detailRecord.title }}</a-descriptions-item>
          <a-descriptions-item label="状态">
            <a-tag :color="statusColor(detailRecord.status)">{{ statusText(detailRecord.status) }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="环境">
            <a-tag :color="envColor(detailRecord.env)">{{ detailRecord.env }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="风险等级">
            <a-tag :color="riskColor(detailRecord.risk_level)">{{ riskText(detailRecord.risk_level) }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="Nacos 实例">{{ detailRecord.nacos_instance_name }}</a-descriptions-item>
          <a-descriptions-item label="Group">{{ detailRecord.group }}</a-descriptions-item>
          <a-descriptions-item label="DataID">{{ detailRecord.data_id }}</a-descriptions-item>
          <a-descriptions-item label="命名空间" :span="3">{{ detailRecord.tenant || 'public' }}</a-descriptions-item>
          <a-descriptions-item label="创建人">{{ detailRecord.created_by_name }}</a-descriptions-item>
          <a-descriptions-item label="创建时间">{{ formatTime(detailRecord.created_at) }}</a-descriptions-item>
          <a-descriptions-item label="描述">{{ detailRecord.description }}</a-descriptions-item>
          <a-descriptions-item v-if="detailRecord.approved_by_name" label="审批人">{{ detailRecord.approved_by_name }}</a-descriptions-item>
          <a-descriptions-item v-if="detailRecord.approved_at" label="审批时间">{{ formatTime(detailRecord.approved_at) }}</a-descriptions-item>
          <a-descriptions-item v-if="detailRecord.reject_reason" label="驳回原因" :span="3">
            <span style="color: #ff4d4f">{{ detailRecord.reject_reason }}</span>
          </a-descriptions-item>
          <a-descriptions-item v-if="detailRecord.published_by_name" label="发布人">{{ detailRecord.published_by_name }}</a-descriptions-item>
          <a-descriptions-item v-if="detailRecord.published_at" label="发布时间">{{ formatTime(detailRecord.published_at) }}</a-descriptions-item>
        </a-descriptions>

        <a-divider>配置变更 Diff</a-divider>
        <div style="display: flex; gap: 16px">
          <div style="flex: 1">
            <div style="font-weight: 600; margin-bottom: 8px">变更前</div>
            <a-textarea :value="detailRecord.content_before || '(无)'" :rows="16" disabled style="font-family: monospace; font-size: 12px" />
          </div>
          <div style="flex: 1">
            <div style="font-weight: 600; margin-bottom: 8px">变更后</div>
            <a-textarea :value="detailRecord.content_after || '(无)'" :rows="16" disabled style="font-family: monospace; font-size: 12px" />
          </div>
        </div>
      </template>
    </a-modal>

    <!-- 驳回弹窗 -->
    <a-modal v-model:open="rejectVisible" title="驳回发布单" @ok="handleReject" :confirmLoading="submitting">
      <a-form-item label="驳回原因" required>
        <a-textarea v-model:value="rejectReason" :rows="3" placeholder="请填写驳回原因" />
      </a-form-item>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { nacosReleaseApi } from '@/services/nacosRelease'
import type { NacosRelease } from '@/services/nacosRelease'
import { nacosApi } from '@/services/nacos'
import type { NacosInstance } from '@/services/nacos'

const router = useRouter()

const loading = ref(false)
const submitting = ref(false)
const fetchingContent = ref(false)
const loadingDataIds = ref(false)
const list = ref<NacosRelease[]>([])
const total = ref(0)
const instances = ref<NacosInstance[]>([])
const dataIdOptions = ref<string[]>([])

const filter = reactive({
  env: undefined as string | undefined,
  status: undefined as string | undefined,
  data_id: undefined as string | undefined,
  page: 1,
  page_size: 20,
})

const createVisible = ref(false)
const detailVisible = ref(false)
const rejectVisible = ref(false)
const editingId = ref<number | null>(null)
const rejectingId = ref<number | null>(null)
const rejectReason = ref('')
const detailRecord = ref<NacosRelease | null>(null)

const form = reactive({
  title: '',
  nacos_instance_id: undefined as number | undefined,
  nacos_instance_name: '',
  tenant: '',
  group: 'DEFAULT_GROUP',
  data_id: '',
  env: 'dev',
  config_type: 'yaml',
  content_before: '',
  content_after: '',
  risk_level: 'low',
  description: '',
  service_id: undefined as number | undefined,
  service_name: '',
})

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '环境', dataIndex: 'env', key: 'env', width: 80 },
  { title: 'DataID', dataIndex: 'data_id', key: 'data_id', ellipsis: true },
  { title: 'Group', dataIndex: 'group', key: 'group', width: 140 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '风险', dataIndex: 'risk_level', key: 'risk_level', width: 70 },
  { title: '创建人', dataIndex: 'created_by_name', key: 'created_by_name', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 170 },
  { title: '操作', key: 'action', width: 240 },
]

const statusColor = (s: string) => {
  const map: Record<string, string> = {
    draft: 'default', pending_approval: 'processing', approved: 'cyan',
    published: 'success', rolled_back: 'warning', rejected: 'error',
  }
  return map[s] || 'default'
}
const statusText = (s: string) => {
  const map: Record<string, string> = {
    draft: '草稿', pending_approval: '待审批', approved: '已审批',
    published: '已发布', rolled_back: '已回滚', rejected: '已驳回',
  }
  return map[s] || s
}
const envColor = (e: string) => {
  const map: Record<string, string> = { dev: 'blue', test: 'green', uat: 'orange', gray: 'purple', prod: 'red' }
  return map[e] || 'default'
}
const riskColor = (r: string) => {
  const map: Record<string, string> = { low: 'green', medium: 'orange', high: 'red' }
  return map[r] || 'default'
}
const riskText = (r: string) => {
  const map: Record<string, string> = { low: '低', medium: '中', high: '高' }
  return map[r] || r
}
const formatTime = (t?: string) => t ? new Date(t).toLocaleString('zh-CN') : '-'

function goToApprovalInstance(approvalInstanceId: number) {
  router.push(`/approval/instances/${approvalInstanceId}`)
}

async function loadList() {
  loading.value = true
  try {
    const res = await nacosReleaseApi.list(filter)
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch {
    message.error('加载发布单列表失败')
  } finally {
    loading.value = false
  }
}

async function loadInstances() {
  try {
    const res = await nacosApi.listInstances()
    instances.value = res.data || []
  } catch { /* ignore */ }
}

function handleTableChange(pagination: any) {
  filter.page = pagination.current
  filter.page_size = pagination.pageSize
  loadList()
}

function resetForm() {
  form.title = ''
  form.nacos_instance_id = undefined
  form.nacos_instance_name = ''
  form.tenant = ''
  form.group = 'DEFAULT_GROUP'
  form.data_id = ''
  form.env = 'dev'
  form.config_type = 'yaml'
  form.content_before = ''
  form.content_after = ''
  form.risk_level = 'low'
  form.description = ''
  form.service_id = undefined
  form.service_name = ''
  dataIdOptions.value = []
}

function showCreateModal(record?: NacosRelease) {
  resetForm()
  if (record) {
    editingId.value = record.id!
    Object.assign(form, {
      title: record.title,
      nacos_instance_id: record.nacos_instance_id,
      nacos_instance_name: record.nacos_instance_name,
      tenant: record.tenant,
      group: record.group,
      data_id: record.data_id,
      env: record.env,
      config_type: record.config_type,
      content_before: record.content_before,
      content_after: record.content_after,
      risk_level: record.risk_level,
      description: record.description,
    })
    void loadDataIdOptions(record.data_id)
  } else {
    editingId.value = null
  }
  createVisible.value = true
}

function handleInstanceChange(val: number) {
  const inst = instances.value.find(i => i.id === val)
  if (inst) {
    form.nacos_instance_name = inst.name
    form.env = inst.env
  }
  form.data_id = ''
  form.content_before = ''
}

async function loadDataIdOptions(selectedDataId?: string) {
  if (!form.nacos_instance_id) {
    dataIdOptions.value = []
    return
  }
  loadingDataIds.value = true
  try {
    const res = await nacosApi.listConfigs(form.nacos_instance_id, {
      tenant: form.tenant || '',
      group: form.group || 'DEFAULT_GROUP',
      page: 1,
      page_size: 500,
    })
    const items = res.data?.pageItems || []
    const options = Array.from(new Set(items.map(item => item.dataId).filter(Boolean)))
    if (selectedDataId && !options.includes(selectedDataId)) {
      options.unshift(selectedDataId)
    }
    dataIdOptions.value = options
    if (!form.data_id && options.length > 0) {
      form.data_id = options[0]
    }
  } catch {
    dataIdOptions.value = selectedDataId ? [selectedDataId] : []
    message.error('自动获取 DataID 失败')
  } finally {
    loadingDataIds.value = false
  }
}

function filterDataIdOption(input: string, option: any) {
  const value = String(option?.value || '')
  return value.toLowerCase().includes(input.toLowerCase())
}

async function fetchCurrentContent() {
  if (!form.nacos_instance_id || !form.data_id) {
    message.warning('请先选择 Nacos 实例和 DataID')
    return
  }
  fetchingContent.value = true
  try {
    const res = await nacosReleaseApi.fetchContent({
      instance_id: form.nacos_instance_id,
      tenant: form.tenant,
      group: form.group || 'DEFAULT_GROUP',
      data_id: form.data_id,
    })
    form.content_before = typeof res.data === 'string' ? res.data : ''
    if (!form.content_after) {
      form.content_after = form.content_before
    }
    message.success('已拉取当前配置')
  } catch {
    message.error('拉取配置失败')
  } finally {
    fetchingContent.value = false
  }
}

async function handleCreateOrUpdate() {
  if (!form.title || !form.nacos_instance_id || !form.data_id) {
    message.warning('请填写必填项')
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await nacosReleaseApi.update(editingId.value, { ...form })
      message.success('更新成功')
    } else {
      await nacosReleaseApi.create({ ...form })
      message.success('创建成功')
    }
    createVisible.value = false
    loadList()
  } catch {
    message.error('操作失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmit(id: number) {
  try {
    await nacosReleaseApi.submit(id)
    message.success('已提交审批')
    loadList()
  } catch {
    message.error('提交失败')
  }
}

async function handleApprove(id: number) {
  try {
    await nacosReleaseApi.approve(id)
    message.success('审批通过')
    loadList()
  } catch {
    message.error('审批失败')
  }
}

function showRejectModal(id: number) {
  rejectingId.value = id
  rejectReason.value = ''
  rejectVisible.value = true
}

async function handleReject() {
  if (!rejectReason.value) {
    message.warning('请填写驳回原因')
    return
  }
  submitting.value = true
  try {
    await nacosReleaseApi.reject(rejectingId.value!, rejectReason.value)
    message.success('已驳回')
    rejectVisible.value = false
    loadList()
  } catch {
    message.error('驳回失败')
  } finally {
    submitting.value = false
  }
}

async function handlePublish(id: number) {
  try {
    await nacosReleaseApi.publish(id)
    message.success('发布成功')
    loadList()
  } catch {
    message.error('发布失败')
  }
}

async function handleRollback(id: number) {
  try {
    await nacosReleaseApi.rollback(id)
    message.success('回滚成功')
    loadList()
  } catch {
    message.error('回滚失败')
  }
}

async function handleDelete(id: number) {
  try {
    await nacosReleaseApi.delete(id)
    message.success('已删除')
    loadList()
  } catch {
    message.error('删除失败')
  }
}

function showDetail(record: NacosRelease) {
  detailRecord.value = record
  detailVisible.value = true
}

onMounted(() => {
  loadList()
  loadInstances()
})

watch(
  () => [form.nacos_instance_id, form.group, form.tenant],
  ([instanceId], [prevInstanceId, prevGroup, prevTenant]) => {
    if (!createVisible.value) {
      return
    }
    const changed = instanceId !== prevInstanceId || form.group !== prevGroup || form.tenant !== prevTenant
    if (!changed) {
      return
    }
    form.data_id = ''
    form.content_before = ''
    void loadDataIdOptions()
  },
)
</script>

<style scoped>
.nacos-release {
  padding: 0;
}
</style>
