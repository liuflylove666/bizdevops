<template>
  <div class="jira-integration">
    <a-tabs v-model:activeKey="activeTab">
      <!-- 实例管理 -->
      <a-tab-pane key="instances" tab="Jira 实例">
        <a-card :bordered="false">
          <template #extra><a-button type="primary" @click="showInstModal()">添加实例</a-button></template>
          <a-table :columns="instColumns" :data-source="instances" :loading="loadingInst" row-key="id" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'status'">
                <a-badge :status="record.status === 'active' ? 'success' : 'default'" :text="record.status === 'active' ? '正常' : '停用'" />
              </template>
              <template v-if="column.key === 'is_default'">
                <a-tag v-if="record.is_default" color="blue">默认</a-tag>
              </template>
              <template v-if="column.key === 'action'">
                <a-space>
                  <a @click="handleTestConn(record.id)">测试</a>
                  <a @click="showInstModal(record)">编辑</a>
                  <a @click="showMappingModal(record)">映射</a>
                  <a-popconfirm title="确定删除？" @confirm="handleDeleteInst(record.id)"><a style="color:#ff4d4f">删除</a></a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 需求看板 -->
      <a-tab-pane key="board" tab="需求看板">
        <a-card :bordered="false">
          <template #extra>
            <a-space>
              <a-select v-model:value="selectedInstance" style="width:180px" placeholder="选择实例" @change="onInstanceChange">
                <a-select-option v-for="i in instances" :key="i.id" :value="i.id">{{ i.name }}</a-select-option>
              </a-select>
              <a-select v-model:value="selectedBoard" style="width:200px" placeholder="选择看板" allow-clear @change="onBoardChange">
                <a-select-option v-for="b in boards" :key="b.id" :value="b.id">{{ b.name }}</a-select-option>
              </a-select>
              <a-select v-model:value="selectedSprint" style="width:200px" placeholder="选择 Sprint" allow-clear @change="loadSprintIssues">
                <a-select-option v-for="s in sprints" :key="s.id" :value="s.id">{{ s.name }} ({{ s.state }})</a-select-option>
              </a-select>
            </a-space>
          </template>
          <a-spin :spinning="loadingIssues">
            <a-table :columns="issueColumns" :data-source="issues" row-key="id" :pagination="false" size="small">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'key'">
                  <a :href="getIssueUrl(record.key)" target="_blank">{{ record.key }}</a>
                </template>
                <template v-if="column.key === 'issuetype'">
                  <a-tag>{{ record.fields?.issuetype?.name || '-' }}</a-tag>
                </template>
                <template v-if="column.key === 'status'">
                  <a-tag :color="statusColor(record.fields?.status?.name)">{{ record.fields?.status?.name || '-' }}</a-tag>
                </template>
                <template v-if="column.key === 'priority'">
                  {{ record.fields?.priority?.name || '-' }}
                </template>
                <template v-if="column.key === 'assignee'">
                  {{ record.fields?.assignee?.displayName || '未分配' }}
                </template>
                <template v-if="column.key === 'action'">
                  <a @click="showIssueDetail(record.key)">详情</a>
                </template>
              </template>
            </a-table>
          </a-spin>
        </a-card>
      </a-tab-pane>

      <!-- JQL 搜索 -->
      <a-tab-pane key="search" tab="JQL 搜索">
        <a-card :bordered="false">
          <a-space style="margin-bottom:16px;width:100%" direction="vertical">
            <a-space>
              <a-select v-model:value="searchInstance" style="width:180px" placeholder="选择实例">
                <a-select-option v-for="i in instances" :key="i.id" :value="i.id">{{ i.name }}</a-select-option>
              </a-select>
              <a-input v-model:value="jqlQuery" placeholder="输入 JQL 查询，如: project = DEVOPS AND status != Done" style="width:500px" @pressEnter="doSearch" />
              <a-button type="primary" @click="doSearch" :loading="searching">搜索</a-button>
            </a-space>
          </a-space>
          <a-table :columns="issueColumns" :data-source="searchResults" row-key="id" :pagination="false" size="small">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'key'">
                <a :href="getIssueUrl(record.key)" target="_blank">{{ record.key }}</a>
              </template>
              <template v-if="column.key === 'issuetype'">
                <a-tag>{{ record.fields?.issuetype?.name || '-' }}</a-tag>
              </template>
              <template v-if="column.key === 'status'">
                <a-tag :color="statusColor(record.fields?.status?.name)">{{ record.fields?.status?.name || '-' }}</a-tag>
              </template>
              <template v-if="column.key === 'priority'">
                {{ record.fields?.priority?.name || '-' }}
              </template>
              <template v-if="column.key === 'assignee'">
                {{ record.fields?.assignee?.displayName || '未分配' }}
              </template>
              <template v-if="column.key === 'action'">
                <a @click="showIssueDetail(record.key)">详情</a>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>

    <!-- 实例编辑弹窗 -->
    <a-modal v-model:open="instModalVisible" :title="editingInst?.id ? '编辑实例' : '添加实例'" @ok="handleSaveInst" :confirmLoading="savingInst">
      <a-form :label-col="{span:5}" :wrapper-col="{span:17}">
        <a-form-item label="名称" required><a-input v-model:value="instForm.name" /></a-form-item>
        <a-form-item label="URL" required><a-input v-model:value="instForm.base_url" placeholder="https://your-domain.atlassian.net" /></a-form-item>
        <a-form-item label="认证方式">
          <a-radio-group v-model:value="instForm.auth_type">
            <a-radio value="token">API Token</a-radio>
            <a-radio value="basic">Basic Auth</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item label="用户名"><a-input v-model:value="instForm.username" placeholder="邮箱或用户名" /></a-form-item>
        <a-form-item label="Token"><a-input-password v-model:value="instForm.token" placeholder="API Token 或密码" /></a-form-item>
        <a-form-item label="默认实例"><a-switch v-model:checked="instForm.is_default" /></a-form-item>
      </a-form>
    </a-modal>

    <!-- 项目映射弹窗 -->
    <a-modal v-model:open="mappingModalVisible" title="项目映射" @ok="mappingModalVisible = false" :footer="null" width="700px">
      <a-space style="margin-bottom:12px">
        <a-button type="primary" size="small" @click="loadJiraProjects">刷新 Jira 项目</a-button>
        <a-button size="small" @click="showAddMapping">添加映射</a-button>
      </a-space>
      <a-table :columns="mappingColumns" :data-source="mappings" row-key="id" :pagination="false" size="small" :loading="loadingMappings">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'action'">
            <a-popconfirm title="确定删除？" @confirm="handleDeleteMapping(record.id)"><a style="color:#ff4d4f">删除</a></a-popconfirm>
          </template>
        </template>
      </a-table>

      <!-- 添加映射子弹窗 -->
      <a-modal v-model:open="addMappingVisible" title="添加映射" @ok="handleSaveMapping" :confirmLoading="savingMapping">
        <a-form :label-col="{span:6}" :wrapper-col="{span:16}">
          <a-form-item label="Jira 项目">
            <a-select v-model:value="mappingForm.jira_project_key" placeholder="选择 Jira 项目" show-search @change="onJiraProjectSelect">
              <a-select-option v-for="p in jiraProjects" :key="p.key" :value="p.key">{{ p.name }} ({{ p.key }})</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="DevOps 项目">
            <a-select v-model:value="mappingForm.devops_project_id" placeholder="关联 DevOps 项目" allow-clear>
              <a-select-option v-for="p in devopsProjects" :key="p.id" :value="p.id">{{ p.display_name || p.name }}</a-select-option>
            </a-select>
          </a-form-item>
        </a-form>
      </a-modal>
    </a-modal>

    <!-- Issue 详情弹窗 -->
    <a-modal v-model:open="issueDetailVisible" :title="issueDetail?.key" width="700px" :footer="null">
      <a-spin :spinning="loadingDetail">
        <template v-if="issueDetail">
          <a-descriptions :column="2" bordered size="small">
            <a-descriptions-item label="类型">{{ issueDetail.fields?.issuetype?.name }}</a-descriptions-item>
            <a-descriptions-item label="状态"><a-tag :color="statusColor(issueDetail.fields?.status?.name)">{{ issueDetail.fields?.status?.name }}</a-tag></a-descriptions-item>
            <a-descriptions-item label="优先级">{{ issueDetail.fields?.priority?.name }}</a-descriptions-item>
            <a-descriptions-item label="经办人">{{ issueDetail.fields?.assignee?.displayName || '未分配' }}</a-descriptions-item>
            <a-descriptions-item label="报告人">{{ issueDetail.fields?.reporter?.displayName || '-' }}</a-descriptions-item>
            <a-descriptions-item label="创建时间">{{ formatTime(issueDetail.fields?.created) }}</a-descriptions-item>
            <a-descriptions-item :span="2" label="标题">{{ issueDetail.fields?.summary }}</a-descriptions-item>
          </a-descriptions>
          <div style="margin-top:12px" v-if="issueDetail.fields?.description">
            <strong>描述：</strong>
            <div style="white-space:pre-wrap;background:#f5f5f5;padding:8px;border-radius:4px;max-height:300px;overflow:auto;margin-top:4px">{{ issueDetail.fields.description }}</div>
          </div>
          <div style="margin-top:12px">
            <strong>标签：</strong>
            <a-tag v-for="l in (issueDetail.fields?.labels || [])" :key="l" color="blue">{{ l }}</a-tag>
            <span v-if="!issueDetail.fields?.labels?.length">无</span>
          </div>
        </template>
      </a-spin>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { jiraApi, type JiraInstance, type JiraProjectMapping } from '@/services/jira'
import { catalogApi } from '@/services/catalog'

const activeTab = ref('instances')

// --- Instances ---
const instances = ref<JiraInstance[]>([])
const loadingInst = ref(false)
const instModalVisible = ref(false)
const savingInst = ref(false)
const editingInst = ref<JiraInstance | null>(null)
const instForm = reactive<JiraInstance>({ name: '', base_url: '', username: '', token: '', auth_type: 'token', is_default: false, status: 'active' })

const instColumns = [
  { title: '名称', dataIndex: 'name', key: 'name', width: 140 },
  { title: 'URL', dataIndex: 'base_url', key: 'base_url', ellipsis: true },
  { title: '认证方式', dataIndex: 'auth_type', key: 'auth_type', width: 100 },
  { title: '默认', key: 'is_default', width: 60 },
  { title: '状态', key: 'status', width: 80 },
  { title: '操作', key: 'action', width: 200 },
]

const loadInstances = async () => {
  loadingInst.value = true
  try { instances.value = (await jiraApi.listInstances()).data || [] } catch {} finally { loadingInst.value = false }
}
const showInstModal = (r?: JiraInstance) => {
  editingInst.value = r || null
  Object.assign(instForm, r || { name: '', base_url: '', username: '', token: '', auth_type: 'token', is_default: false, status: 'active' })
  if (r) instForm.token = ''
  instModalVisible.value = true
}
const handleSaveInst = async () => {
  if (!instForm.name || !instForm.base_url) { message.warning('请填写名称和 URL'); return }
  savingInst.value = true
  try {
    if (editingInst.value?.id) {
      await jiraApi.updateInstance(editingInst.value.id, { ...instForm })
    } else {
      await jiraApi.createInstance({ ...instForm })
    }
    message.success('保存成功'); instModalVisible.value = false; loadInstances()
  } catch (e: any) { message.error(e?.response?.data?.message || '保存失败') }
  finally { savingInst.value = false }
}
const handleDeleteInst = async (id: number) => {
  try { await jiraApi.deleteInstance(id); message.success('已删除'); loadInstances() } catch (e: any) { message.error(e?.response?.data?.message || '删除失败') }
}
const handleTestConn = async (id: number) => {
  try { await jiraApi.testConnection(id); message.success('连接成功') } catch (e: any) { message.error(e?.response?.data?.message || '连接失败') }
}

// --- Mappings ---
const mappingModalVisible = ref(false)
const addMappingVisible = ref(false)
const loadingMappings = ref(false)
const savingMapping = ref(false)
const mappings = ref<JiraProjectMapping[]>([])
const jiraProjects = ref<any[]>([])
const devopsProjects = ref<any[]>([])
const currentMappingInstance = ref<JiraInstance | null>(null)
const mappingForm = reactive<Partial<JiraProjectMapping>>({ jira_project_key: '', jira_project_name: '', devops_project_id: undefined })

const mappingColumns = [
  { title: 'Jira 项目', dataIndex: 'jira_project_key', key: 'jira_project_key', width: 120 },
  { title: 'Jira 名称', dataIndex: 'jira_project_name', key: 'jira_project_name' },
  { title: 'DevOps 项目ID', dataIndex: 'devops_project_id', key: 'devops_project_id', width: 120 },
  { title: '操作', key: 'action', width: 80 },
]

const showMappingModal = async (inst: JiraInstance) => {
  currentMappingInstance.value = inst
  mappingModalVisible.value = true
  loadMappings()
  loadDevopsProjects()
}
const loadMappings = async () => {
  if (!currentMappingInstance.value?.id) return
  loadingMappings.value = true
  try { mappings.value = (await jiraApi.listMappings(currentMappingInstance.value.id)).data || [] } catch {} finally { loadingMappings.value = false }
}
const loadJiraProjects = async () => {
  if (!currentMappingInstance.value?.id) return
  try { jiraProjects.value = (await jiraApi.listProjects(currentMappingInstance.value.id)).data || []; message.success(`获取到 ${jiraProjects.value.length} 个项目`) } catch (e: any) { message.error(e?.response?.data?.message || '获取失败') }
}
const loadDevopsProjects = async () => {
  try { devopsProjects.value = (await catalogApi.listProjects()).data || [] } catch {}
}
const showAddMapping = () => {
  Object.assign(mappingForm, { jira_project_key: '', jira_project_name: '', devops_project_id: undefined })
  addMappingVisible.value = true
  if (!jiraProjects.value.length) loadJiraProjects()
}
const onJiraProjectSelect = (key: string) => {
  const p = jiraProjects.value.find((x: any) => x.key === key)
  if (p) mappingForm.jira_project_name = p.name
}
const handleSaveMapping = async () => {
  if (!mappingForm.jira_project_key || !currentMappingInstance.value?.id) return
  savingMapping.value = true
  try {
    await jiraApi.createMapping(currentMappingInstance.value.id, { ...mappingForm })
    message.success('映射已创建'); addMappingVisible.value = false; loadMappings()
  } catch (e: any) { message.error(e?.response?.data?.message || '创建失败') }
  finally { savingMapping.value = false }
}
const handleDeleteMapping = async (id: number) => {
  try { await jiraApi.deleteMapping(id); message.success('已删除'); loadMappings() } catch {}
}

// --- Board / Sprint / Issues ---
const selectedInstance = ref<number | undefined>(undefined)
const selectedBoard = ref<number | undefined>(undefined)
const selectedSprint = ref<number | undefined>(undefined)
const boards = ref<any[]>([])
const sprints = ref<any[]>([])
const issues = ref<any[]>([])
const loadingIssues = ref(false)

const onInstanceChange = async () => {
  boards.value = []; sprints.value = []; issues.value = []
  selectedBoard.value = undefined; selectedSprint.value = undefined
  if (!selectedInstance.value) return
  try {
    const res = await jiraApi.getBoards(selectedInstance.value)
    boards.value = res.data?.values || []
  } catch {}
}
const onBoardChange = async () => {
  sprints.value = []; issues.value = []; selectedSprint.value = undefined
  if (!selectedInstance.value || !selectedBoard.value) return
  try {
    const res = await jiraApi.getSprints(selectedInstance.value, selectedBoard.value, 'active,future')
    sprints.value = res.data?.values || []
  } catch {}
}
const loadSprintIssues = async () => {
  if (!selectedInstance.value || !selectedSprint.value) return
  loadingIssues.value = true
  try {
    const res = await jiraApi.getSprintIssues(selectedInstance.value, selectedSprint.value, { max_results: 50 })
    issues.value = res.data?.issues || []
  } catch {} finally { loadingIssues.value = false }
}

// --- JQL Search ---
const searchInstance = ref<number | undefined>(undefined)
const jqlQuery = ref('')
const searchResults = ref<any[]>([])
const searching = ref(false)
const doSearch = async () => {
  if (!searchInstance.value || !jqlQuery.value) { message.warning('请选择实例并输入 JQL'); return }
  searching.value = true
  try {
    const res = await jiraApi.searchIssues(searchInstance.value, { jql: jqlQuery.value, max_results: 50 })
    searchResults.value = res.data?.issues || []
  } catch (e: any) { message.error(e?.response?.data?.message || '搜索失败') }
  finally { searching.value = false }
}

// --- Issue Detail ---
const issueDetailVisible = ref(false)
const issueDetail = ref<any>(null)
const loadingDetail = ref(false)
const showIssueDetail = async (key: string) => {
  const instId = selectedInstance.value || searchInstance.value
  if (!instId) return
  issueDetailVisible.value = true; loadingDetail.value = true; issueDetail.value = null
  try { issueDetail.value = (await jiraApi.getIssue(instId, key)).data } catch {} finally { loadingDetail.value = false }
}

// --- Common ---
const issueColumns = [
  { title: 'Key', key: 'key', dataIndex: 'key', width: 120 },
  { title: '类型', key: 'issuetype', width: 80 },
  { title: '标题', dataIndex: ['fields', 'summary'], key: 'summary', ellipsis: true },
  { title: '状态', key: 'status', width: 100 },
  { title: '优先级', key: 'priority', width: 80 },
  { title: '经办人', key: 'assignee', width: 100 },
  { title: '操作', key: 'action', width: 60 },
]

const statusColor = (s: string) => {
  if (!s) return 'default'
  const lower = s.toLowerCase()
  if (lower.includes('done') || lower.includes('closed') || lower.includes('resolved')) return 'green'
  if (lower.includes('progress') || lower.includes('review')) return 'blue'
  if (lower.includes('todo') || lower.includes('open') || lower.includes('new')) return 'default'
  return 'orange'
}
const getIssueUrl = (key: string) => {
  const inst = instances.value.find(i => i.id === (selectedInstance.value || searchInstance.value))
  if (!inst) return '#'
  return `${inst.base_url}/browse/${key}`
}
const formatTime = (t: string) => t ? new Date(t).toLocaleString('zh-CN') : '-'

onMounted(() => { loadInstances() })
</script>
