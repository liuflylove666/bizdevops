<template>
  <div class="nacos-config">
    <a-tabs v-model:activeKey="activeTab">
      <!-- 实例管理 -->
      <a-tab-pane key="instances" tab="实例管理">
        <a-card :bordered="false">
          <template #extra>
            <a-button type="primary" @click="showInstanceModal()">新增实例</a-button>
          </template>
          <a-table :columns="instanceColumns" :data-source="instances" :loading="loadingInstances" row-key="id" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'status'">
                <a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status === 'active' ? '启用' : '停用' }}</a-tag>
              </template>
              <template v-if="column.key === 'env'">
                <a-tag :color="envColors[record.env] || 'default'">{{ record.env }}</a-tag>
              </template>
              <template v-if="column.key === 'action'">
                <a-space>
                  <a @click="handleSelectInstance(record)">管理配置</a>
                  <a @click="handleTestConnection(record.id)">测试</a>
                  <a @click="showInstanceModal(record)">编辑</a>
                  <a-popconfirm title="确定删除？" @confirm="handleDeleteInstance(record.id)">
                    <a style="color: #ff4d4f">删除</a>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 配置管理 -->
      <a-tab-pane key="configs" tab="配置管理" :disabled="!selectedInstance">
        <a-card :bordered="false">
          <template #title>
            <a-space>
              <span>{{ selectedInstance?.name }}</span>
              <a-tag :color="envColors[selectedInstance?.env || ''] || 'default'">{{ selectedInstance?.env }}</a-tag>
            </a-space>
          </template>
          <template #extra>
            <a-space>
              <a-select v-model:value="selectedNamespace" style="width: 200px" placeholder="命名空间" @change="loadConfigs">
                <a-select-option value="">public</a-select-option>
                <a-select-option v-for="ns in namespaces" :key="ns.namespace" :value="ns.namespace">{{ ns.namespaceShowName || ns.namespace }}</a-select-option>
              </a-select>
              <a-input v-model:value="searchGroup" placeholder="Group" style="width: 120px" allow-clear @pressEnter="loadConfigs" />
              <a-input v-model:value="searchDataId" placeholder="Data ID" style="width: 160px" allow-clear @pressEnter="loadConfigs" />
              <a-button @click="loadConfigs">搜索</a-button>
              <a-button type="primary" @click="showConfigEditor()">新建配置</a-button>
            </a-space>
          </template>

          <a-table :columns="configColumns" :data-source="configs" :loading="loadingConfigs" row-key="id"
            :pagination="{ current: configPage, pageSize: configPageSize, total: configTotal, onChange: onConfigPageChange }">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'action'">
                <a-space>
                  <a @click="handleViewConfig(record)">查看</a>
                  <a @click="handleEditConfig(record)">编辑</a>
                  <a @click="handleViewHistory(record)">历史</a>
                  <a-popconfirm title="确定删除？" @confirm="handleDeleteConfig(record)">
                    <a style="color: #ff4d4f">删除</a>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 跨环境对比 -->
      <a-tab-pane key="compare" tab="环境对比">
        <a-card :bordered="false">
          <a-form layout="inline" style="margin-bottom: 16px">
            <a-form-item label="源实例">
              <a-select v-model:value="compare.sourceInstanceId" style="width: 180px" placeholder="选择源实例" @change="loadSourceNamespaces">
                <a-select-option v-for="inst in instances" :key="inst.id" :value="inst.id">{{ inst.name }} ({{ inst.env }})</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="源命名空间">
              <a-select v-model:value="compare.sourceTenant" style="width: 160px" placeholder="public">
                <a-select-option value="">public</a-select-option>
                <a-select-option v-for="ns in compareSourceNs" :key="ns.namespace" :value="ns.namespace">{{ ns.namespaceShowName || ns.namespace }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="目标实例">
              <a-select v-model:value="compare.targetInstanceId" style="width: 180px" placeholder="选择目标实例" @change="loadTargetNamespaces">
                <a-select-option v-for="inst in instances" :key="inst.id" :value="inst.id">{{ inst.name }} ({{ inst.env }})</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="目标命名空间">
              <a-select v-model:value="compare.targetTenant" style="width: 160px" placeholder="public">
                <a-select-option value="">public</a-select-option>
                <a-select-option v-for="ns in compareTargetNs" :key="ns.namespace" :value="ns.namespace">{{ ns.namespaceShowName || ns.namespace }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="Group">
              <a-input v-model:value="compare.group" placeholder="DEFAULT_GROUP" style="width: 140px" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" :loading="comparing" @click="handleCompare">对比</a-button>
            </a-form-item>
          </a-form>

          <a-table :columns="compareColumns" :data-source="compareResults" :loading="comparing" row-key="data_id" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'status'">
                <a-tag v-if="record.same" color="green">一致</a-tag>
                <a-tag v-else-if="!record.source_content" color="orange">仅目标</a-tag>
                <a-tag v-else-if="!record.target_content" color="blue">仅源端</a-tag>
                <a-tag v-else color="red">不一致</a-tag>
              </template>
              <template v-if="column.key === 'action'">
                <a-popconfirm v-if="record.source_content && !record.same" title="确认将源配置同步到目标？" @confirm="handleSync(record)">
                  <a-button type="link" size="small">同步到目标</a-button>
                </a-popconfirm>
                <span v-else>-</span>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>

    <!-- 实例编辑弹窗 -->
    <a-modal v-model:open="instanceModalVisible" :title="editingInstance?.id ? '编辑实例' : '新增实例'" @ok="handleSaveInstance" :confirmLoading="savingInstance">
      <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 17 }">
        <a-form-item label="名称" required><a-input v-model:value="instanceForm.name" /></a-form-item>
        <a-form-item label="地址" required><a-input v-model:value="instanceForm.addr" placeholder="127.0.0.1:8848" /></a-form-item>
        <a-form-item label="用户名"><a-input v-model:value="instanceForm.username" placeholder="nacos" /></a-form-item>
        <a-form-item label="密码"><a-input-password v-model:value="instanceForm.password" placeholder="留空保留原密码" /></a-form-item>
        <a-form-item label="环境" required>
          <a-select v-model:value="instanceForm.env">
            <a-select-option value="dev">dev</a-select-option>
            <a-select-option value="test">test</a-select-option>
            <a-select-option value="uat">uat</a-select-option>
            <a-select-option value="gray">gray</a-select-option>
            <a-select-option value="prod">prod</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="描述"><a-textarea v-model:value="instanceForm.description" :rows="2" /></a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="instanceForm.status">
            <a-select-option value="active">启用</a-select-option>
            <a-select-option value="inactive">停用</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 配置编辑弹窗 -->
    <a-modal v-model:open="configEditorVisible" :title="configEditorTitle" width="800px" @ok="handleSaveConfig" :confirmLoading="savingConfig">
      <a-form :label-col="{ span: 4 }" :wrapper-col="{ span: 19 }">
        <a-form-item label="Group" required><a-input v-model:value="configForm.group" :disabled="!!configForm._editing" /></a-form-item>
        <a-form-item label="Data ID" required><a-input v-model:value="configForm.data_id" :disabled="!!configForm._editing" /></a-form-item>
        <a-form-item label="格式">
          <a-select v-model:value="configForm.config_type" style="width: 120px">
            <a-select-option value="">text</a-select-option>
            <a-select-option value="json">json</a-select-option>
            <a-select-option value="yaml">yaml</a-select-option>
            <a-select-option value="properties">properties</a-select-option>
            <a-select-option value="xml">xml</a-select-option>
            <a-select-option value="toml">toml</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="内容" required>
          <a-textarea v-model:value="configForm.content" :rows="16" style="font-family: monospace" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 配置查看弹窗 -->
    <a-modal v-model:open="configViewVisible" title="配置详情" width="800px" :footer="null">
      <a-descriptions :column="2" bordered size="small" style="margin-bottom: 12px">
        <a-descriptions-item label="Group">{{ viewConfig.group }}</a-descriptions-item>
        <a-descriptions-item label="Data ID">{{ viewConfig.dataId }}</a-descriptions-item>
      </a-descriptions>
      <pre style="background: #f5f5f5; padding: 12px; border-radius: 4px; max-height: 500px; overflow: auto; white-space: pre-wrap; word-break: break-all">{{ viewConfig.content }}</pre>
    </a-modal>

    <!-- 历史弹窗 -->
    <a-modal v-model:open="historyVisible" title="配置历史" width="800px" :footer="null">
      <a-table :columns="historyColumns" :data-source="historyItems" :loading="loadingHistory" row-key="nid" size="small"
        :pagination="{ current: historyPage, pageSize: 10, total: historyTotal, onChange: onHistoryPageChange }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'action'">
            <a-space>
              <a @click="handleViewHistoryDetail(record)">查看</a>
              <a-popconfirm title="确认回滚到此版本？" @confirm="handleRollback(record)">
                <a style="color: #fa8c16">回滚</a>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { nacosApi, type NacosInstance, type NacosNamespace, type NacosConfigItem, type ConfigCompareItem, type ConfigHistoryItem } from '@/services/nacos'

const activeTab = ref('instances')
const envColors: Record<string, string> = { dev: 'blue', test: 'cyan', uat: 'orange', gray: 'purple', prod: 'red' }

// --- Instances ---
const instances = ref<NacosInstance[]>([])
const loadingInstances = ref(false)
const instanceModalVisible = ref(false)
const savingInstance = ref(false)
const editingInstance = ref<NacosInstance | null>(null)
const instanceForm = reactive<NacosInstance>({ name: '', addr: '', username: '', password: '', env: 'dev', description: '', status: 'active', is_default: false })

const instanceColumns = [
  { title: '名称', dataIndex: 'name', key: 'name', width: 140 },
  { title: '地址', dataIndex: 'addr', key: 'addr', ellipsis: true },
  { title: '环境', dataIndex: 'env', key: 'env', width: 80 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 80 },
  { title: '操作', key: 'action', width: 240 },
]

const loadInstances = async () => {
  loadingInstances.value = true
  try {
    const res = await nacosApi.listInstances()
    instances.value = res.data || []
  } catch {} finally { loadingInstances.value = false }
}

const showInstanceModal = (record?: NacosInstance) => {
  editingInstance.value = record || null
  if (record) {
    Object.assign(instanceForm, { ...record, password: '' })
  } else {
    Object.assign(instanceForm, { name: '', addr: '', username: '', password: '', env: 'dev', description: '', status: 'active', is_default: false })
  }
  instanceModalVisible.value = true
}

const handleSaveInstance = async () => {
  if (!instanceForm.name || !instanceForm.addr) { message.warning('请填写名称和地址'); return }
  savingInstance.value = true
  try {
    if (editingInstance.value?.id) {
      await nacosApi.updateInstance(editingInstance.value.id, { ...instanceForm })
    } else {
      await nacosApi.createInstance({ ...instanceForm })
    }
    message.success('保存成功')
    instanceModalVisible.value = false
    loadInstances()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '保存失败')
  } finally { savingInstance.value = false }
}

const handleDeleteInstance = async (id: number) => {
  try { await nacosApi.deleteInstance(id); message.success('已删除'); loadInstances() } catch (e: any) { message.error(e?.response?.data?.message || '删除失败') }
}

const handleTestConnection = async (id: number) => {
  try { await nacosApi.testConnection(id); message.success('连接成功') } catch (e: any) { message.error(e?.response?.data?.message || '连接失败') }
}

// --- Config Management ---
const selectedInstance = ref<NacosInstance | null>(null)
const namespaces = ref<NacosNamespace[]>([])
const selectedNamespace = ref('')
const searchGroup = ref('')
const searchDataId = ref('')
const configs = ref<NacosConfigItem[]>([])
const loadingConfigs = ref(false)
const configPage = ref(1)
const configPageSize = ref(20)
const configTotal = ref(0)

const configColumns = [
  { title: 'Data ID', dataIndex: 'dataId', key: 'dataId', ellipsis: true },
  { title: 'Group', dataIndex: 'group', key: 'group', width: 160 },
  { title: '类型', dataIndex: 'type', key: 'type', width: 80 },
  { title: '操作', key: 'action', width: 200 },
]

const handleSelectInstance = async (inst: NacosInstance) => {
  selectedInstance.value = inst
  activeTab.value = 'configs'
  selectedNamespace.value = ''
  try {
    const res = await nacosApi.listNamespaces(inst.id!)
    namespaces.value = res.data || []
  } catch { namespaces.value = [] }
  loadConfigs()
}

const loadConfigs = async () => {
  if (!selectedInstance.value?.id) return
  loadingConfigs.value = true
  try {
    const res = await nacosApi.listConfigs(selectedInstance.value.id!, {
      tenant: selectedNamespace.value, group: searchGroup.value, data_id: searchDataId.value,
      page: configPage.value, page_size: configPageSize.value,
    })
    const data = res.data
    configs.value = data?.pageItems || []
    configTotal.value = data?.totalCount || 0
  } catch {} finally { loadingConfigs.value = false }
}

const onConfigPageChange = (page: number) => { configPage.value = page; loadConfigs() }

// Config editor
const configEditorVisible = ref(false)
const configEditorTitle = ref('新建配置')
const savingConfig = ref(false)
const configForm = reactive<{ group: string; data_id: string; content: string; config_type: string; _editing: boolean }>({ group: 'DEFAULT_GROUP', data_id: '', content: '', config_type: '', _editing: false })

const showConfigEditor = (item?: NacosConfigItem) => {
  if (item) {
    configEditorTitle.value = '编辑配置'
    Object.assign(configForm, { group: item.group, data_id: item.dataId, content: item.content || '', config_type: item.type || '', _editing: true })
  } else {
    configEditorTitle.value = '新建配置'
    Object.assign(configForm, { group: 'DEFAULT_GROUP', data_id: '', content: '', config_type: '', _editing: false })
  }
  configEditorVisible.value = true
}

const handleEditConfig = async (record: NacosConfigItem) => {
  try {
    const res = await nacosApi.getConfig(selectedInstance.value!.id!, { tenant: selectedNamespace.value, group: record.group, data_id: record.dataId })
    showConfigEditor({ ...record, content: res.data as unknown as string })
  } catch (e: any) { message.error('读取配置失败') }
}

const handleSaveConfig = async () => {
  if (!configForm.group || !configForm.data_id || !configForm.content) { message.warning('请填写完整'); return }
  savingConfig.value = true
  try {
    await nacosApi.publishConfig(selectedInstance.value!.id!, {
      tenant: selectedNamespace.value, group: configForm.group, data_id: configForm.data_id,
      content: configForm.content, config_type: configForm.config_type,
    })
    message.success('发布成功')
    configEditorVisible.value = false
    loadConfigs()
  } catch (e: any) { message.error(e?.response?.data?.message || '发布失败') } finally { savingConfig.value = false }
}

const handleDeleteConfig = async (record: NacosConfigItem) => {
  try {
    await nacosApi.deleteConfig(selectedInstance.value!.id!, { tenant: selectedNamespace.value, group: record.group, data_id: record.dataId })
    message.success('已删除')
    loadConfigs()
  } catch (e: any) { message.error(e?.response?.data?.message || '删除失败') }
}

// Config view
const configViewVisible = ref(false)
const viewConfig = reactive<{ group: string; dataId: string; content: string }>({ group: '', dataId: '', content: '' })

const handleViewConfig = async (record: NacosConfigItem) => {
  try {
    const res = await nacosApi.getConfig(selectedInstance.value!.id!, { tenant: selectedNamespace.value, group: record.group, data_id: record.dataId })
    Object.assign(viewConfig, { group: record.group, dataId: record.dataId, content: res.data as unknown as string })
    configViewVisible.value = true
  } catch { message.error('读取配置失败') }
}

// History
const historyVisible = ref(false)
const historyItems = ref<ConfigHistoryItem[]>([])
const loadingHistory = ref(false)
const historyPage = ref(1)
const historyTotal = ref(0)
const historyTarget = reactive<{ group: string; dataId: string }>({ group: '', dataId: '' })

const historyColumns = [
  { title: '版本 ID', dataIndex: 'nid', key: 'nid', width: 100 },
  { title: '操作类型', dataIndex: 'opType', key: 'opType', width: 80 },
  { title: '修改时间', dataIndex: 'lastModifiedTime', key: 'lastModifiedTime' },
  { title: '操作', key: 'action', width: 120 },
]

const handleViewHistory = async (record: NacosConfigItem) => {
  historyTarget.group = record.group
  historyTarget.dataId = record.dataId
  historyPage.value = 1
  historyVisible.value = true
  loadHistory()
}

const loadHistory = async () => {
  loadingHistory.value = true
  try {
    const res = await nacosApi.listConfigHistory(selectedInstance.value!.id!, {
      tenant: selectedNamespace.value, group: historyTarget.group, data_id: historyTarget.dataId,
      page: historyPage.value, page_size: 10,
    })
    historyItems.value = res.data?.pageItems || []
    historyTotal.value = res.data?.totalCount || 0
  } catch {} finally { loadingHistory.value = false }
}

const onHistoryPageChange = (page: number) => { historyPage.value = page; loadHistory() }

const handleViewHistoryDetail = async (record: ConfigHistoryItem) => {
  try {
    const res = await nacosApi.getConfigHistoryDetail(selectedInstance.value!.id!, {
      tenant: selectedNamespace.value, group: historyTarget.group, data_id: historyTarget.dataId, nid: record.nid,
    })
    const item = res.data
    Object.assign(viewConfig, { group: historyTarget.group, dataId: historyTarget.dataId, content: item?.content || '' })
    configViewVisible.value = true
  } catch { message.error('读取历史详情失败') }
}

const handleRollback = async (record: ConfigHistoryItem) => {
  try {
    const res = await nacosApi.getConfigHistoryDetail(selectedInstance.value!.id!, {
      tenant: selectedNamespace.value, group: historyTarget.group, data_id: historyTarget.dataId, nid: record.nid,
    })
    const content = res.data?.content
    if (!content) { message.error('历史版本内容为空'); return }
    await nacosApi.publishConfig(selectedInstance.value!.id!, {
      tenant: selectedNamespace.value, group: historyTarget.group, data_id: historyTarget.dataId, content,
    })
    message.success('回滚成功')
    loadHistory()
    loadConfigs()
  } catch (e: any) { message.error(e?.response?.data?.message || '回滚失败') }
}

// --- Compare ---
const compare = reactive<{ sourceInstanceId: number | undefined; targetInstanceId: number | undefined; sourceTenant: string; targetTenant: string; group: string }>({
  sourceInstanceId: undefined, targetInstanceId: undefined, sourceTenant: '', targetTenant: '', group: '',
})
const compareSourceNs = ref<NacosNamespace[]>([])
const compareTargetNs = ref<NacosNamespace[]>([])
const compareResults = ref<ConfigCompareItem[]>([])
const comparing = ref(false)

const compareColumns = [
  { title: 'Data ID', dataIndex: 'data_id', key: 'data_id', ellipsis: true },
  { title: 'Group', dataIndex: 'group', key: 'group', width: 140 },
  { title: '状态', key: 'status', width: 100 },
  { title: '操作', key: 'action', width: 120 },
]

const loadSourceNamespaces = async () => {
  if (!compare.sourceInstanceId) return
  try { const r = await nacosApi.listNamespaces(compare.sourceInstanceId); compareSourceNs.value = r.data || [] } catch { compareSourceNs.value = [] }
}
const loadTargetNamespaces = async () => {
  if (!compare.targetInstanceId) return
  try { const r = await nacosApi.listNamespaces(compare.targetInstanceId); compareTargetNs.value = r.data || [] } catch { compareTargetNs.value = [] }
}

const handleCompare = async () => {
  if (!compare.sourceInstanceId || !compare.targetInstanceId) { message.warning('请选择源和目标实例'); return }
  comparing.value = true
  try {
    const res = await nacosApi.compareConfigs({
      source_instance_id: compare.sourceInstanceId, target_instance_id: compare.targetInstanceId,
      source_tenant: compare.sourceTenant, target_tenant: compare.targetTenant, group: compare.group,
    })
    compareResults.value = res.data || []
  } catch (e: any) { message.error(e?.response?.data?.message || '对比失败') } finally { comparing.value = false }
}

const handleSync = async (record: ConfigCompareItem) => {
  if (!compare.sourceInstanceId || !compare.targetInstanceId) return
  try {
    await nacosApi.syncConfig({
      source_instance_id: compare.sourceInstanceId, target_instance_id: compare.targetInstanceId,
      source_tenant: compare.sourceTenant, target_tenant: compare.targetTenant,
      group: record.group, data_id: record.data_id,
    })
    message.success('同步成功')
    handleCompare()
  } catch (e: any) { message.error(e?.response?.data?.message || '同步失败') }
}

onMounted(() => { loadInstances() })
</script>

<style scoped>
.nacos-config { padding: 0; }
</style>
