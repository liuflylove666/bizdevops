<template>
  <div class="k8s-clusters">
    <div class="page-header">
      <h1>K8s 集群管理</h1>
      <a-button type="primary" @click="showModal()">
        <template #icon><PlusOutlined /></template>
        新增集群
      </a-button>
    </div>

    <a-table :columns="columns" :data-source="clusters" :loading="loading" :pagination="pagination" @change="handleTableChange" row-key="id">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="record.status === 'active' ? 'green' : 'red'">
            {{ record.status === 'active' ? '启用' : '禁用' }}
          </a-tag>
        </template>
        <template v-if="column.key === 'is_default'">
          <a-tag v-if="record.is_default" color="blue">默认</a-tag>
          <span v-else>-</span>
        </template>
        <template v-if="column.key === 'name'">
          <a @click="goToResources(record)">{{ record.name }}</a>
        </template>
        <template v-if="column.key === 'action'">
          <a-space :size="4">
            <a @click="testConnection(record)" :class="{ 'testing': testingIds.has(record.id) }">
              <LoadingOutlined v-if="testingIds.has(record.id)" />
              测试连接
            </a>
            <a-divider type="vertical" />
            <a @click="goToResources(record)">资源</a>
            <a-divider type="vertical" />
            <a @click="showModal(record)">编辑</a>
            <a-divider type="vertical" />
            <a @click="setDefault(record)" v-if="!record.is_default">设为默认</a>
            <a-divider type="vertical" v-if="!record.is_default" />
            <a-popconfirm title="确定删除？" @confirm="handleDelete(record.id)">
              <a style="color: #ff4d4f">删除</a>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="modalVisible" :title="editingId ? '编辑集群' : '新增集群'" @ok="handleSubmit" :confirm-loading="submitting" width="600px">
      <a-form :model="form" :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="名称" required>
          <a-input v-model:value="form.name" placeholder="请输入名称" />
        </a-form-item>
        <a-form-item label="Kubeconfig" required>
          <a-textarea v-model:value="form.kubeconfig" placeholder="请输入 Kubeconfig 内容" :rows="6" />
        </a-form-item>
        <a-form-item label="默认命名空间">
          <a-input v-model:value="form.namespace" placeholder="default / devops-build" />
        </a-form-item>
        <a-form-item label="镜像仓库">
          <a-input v-model:value="form.registry" placeholder="例如 localhost:5001" />
        </a-form-item>
        <a-form-item label="仓库前缀">
          <a-input v-model:value="form.repository" placeholder="例如 jeridevops" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.description" placeholder="请输入描述" :rows="2" />
        </a-form-item>
        <a-form-item label="状态" required>
          <a-select v-model:value="form.status">
            <a-select-option value="active">启用</a-select-option>
            <a-select-option value="inactive">禁用</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="设为默认">
          <a-switch v-model:checked="form.is_default" />
        </a-form-item>
        <a-form-item label="跳过 TLS 校验">
          <a-switch v-model:checked="form.insecure_skip_tls" />
        </a-form-item>
        <a-form-item label="检测超时(秒)">
          <a-input-number v-model:value="form.check_timeout" :min="5" :max="600" style="width: 100%" />
        </a-form-item>
      </a-form>
    </a-modal>

  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { PlusOutlined, LoadingOutlined } from '@ant-design/icons-vue'
import { k8sClusterApi } from '@/services/k8s'
import type { K8sCluster } from '@/types'

const router = useRouter()
const loading = ref(false)
const submitting = ref(false)
const modalVisible = ref(false)
const editingId = ref<number | null>(null)
const clusters = ref<K8sCluster[]>([])
const testingIds = ref<Set<number>>(new Set())

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0
})

const form = reactive({
  name: '',
  kubeconfig: '',
  namespace: 'default',
  registry: '',
  repository: '',
  description: '',
  status: 'active',
  is_default: false,
  insecure_skip_tls: false,
  check_timeout: 180
})

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '命名空间', dataIndex: 'namespace', key: 'namespace', width: 120 },
  { title: '镜像仓库', dataIndex: 'registry', key: 'registry', ellipsis: true },
  { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
  { title: '状态', key: 'status' },
  { title: '默认', key: 'is_default' },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
  { title: '操作', key: 'action', width: 340 }
]

const goToResources = (record: K8sCluster) => {
  router.push(`/k8s/clusters/${record.id}/resources`)
}

const fetchClusters = async () => {
  loading.value = true
  try {
    const response = await k8sClusterApi.getClusters({
      page: pagination.current,
      page_size: pagination.pageSize
    })
    if (response.code === 0 && response.data) {
      const items = response.data.items
      clusters.value = items
      pagination.total = response.data.total
    }
  } catch (error: any) {
    message.error(error.message || '获取列表失败')
  } finally {
    loading.value = false
  }
}

const showModal = async (record?: K8sCluster) => {
  if (record) {
    editingId.value = record.id
    Object.assign(form, {
      name: record.name,
      kubeconfig: record.kubeconfig || '',
      namespace: record.namespace || 'default',
      registry: record.registry || '',
      repository: record.repository || '',
      description: record.description,
      status: record.status,
      is_default: record.is_default,
      insecure_skip_tls: record.insecure_skip_tls || false,
      check_timeout: record.check_timeout || 180
    })
  } else {
    editingId.value = null
    Object.assign(form, {
      name: '',
      kubeconfig: '',
      namespace: 'default',
      registry: '',
      repository: '',
      description: '',
      status: 'active',
      is_default: false,
      insecure_skip_tls: false,
      check_timeout: 180
    })
  }
  modalVisible.value = true
}

const handleSubmit = async () => {
  if (!form.name || !form.kubeconfig) {
    message.error('请填写必填项')
    return
  }

  submitting.value = true
  try {
    if (editingId.value) {
      await k8sClusterApi.updateCluster(editingId.value, form)
      message.success('更新成功')
    } else {
      await k8sClusterApi.createCluster(form)
      message.success('创建成功')
    }
    modalVisible.value = false
    fetchClusters()
  } catch (error: any) {
    message.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (id: number) => {
  try {
    await k8sClusterApi.deleteCluster(id)
    message.success('删除成功')
    fetchClusters()
  } catch (error: any) {
    message.error(error.message || '删除失败')
  }
}

const setDefault = async (record: K8sCluster) => {
  try {
    await k8sClusterApi.setDefaultCluster(record.id)
    message.success('设置成功')
    fetchClusters()
  } catch (error: any) {
    message.error(error.message || '设置失败')
  }
}

const testConnection = async (record: K8sCluster) => {
  if (testingIds.value.has(record.id)) return
  testingIds.value.add(record.id)
  try {
    const response = await k8sClusterApi.testConnection(record.id)
    if (response.data?.connected) {
      message.success(`连接成功！K8s 版本: ${response.data.server_version || '未知'}，节点数: ${response.data.node_count || 0}，响应时间: ${response.data.response_time_ms}ms`)
    } else {
      message.error(`连接失败: ${response.data?.error || '未知错误'}`)
    }
  } catch (error: any) {
    message.error(error.message || '测试连接失败')
  } finally {
    testingIds.value.delete(record.id)
  }
}

const handleTableChange = (pag: any) => {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchClusters()
}

onMounted(() => {
  fetchClusters()
})
</script>

<style scoped>
.k8s-clusters {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h1 {
  font-size: 20px;
  font-weight: 500;
  margin: 0;
}

.testing {
  color: #1890ff;
  cursor: wait;
}
</style>
