<template>
  <div class="sonarqube-integration">
    <a-tabs v-model:activeKey="activeTab">
      <!-- 实例管理 -->
      <a-tab-pane key="instances" tab="SonarQube 实例">
        <div style="margin-bottom: 16px">
          <a-button type="primary" @click="showInstanceModal()">添加实例</a-button>
        </div>
        <a-table :dataSource="instances" :columns="instanceColumns" rowKey="id" :pagination="false" size="small">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status }}</a-tag>
            </template>
            <template v-if="column.key === 'is_default'">
              <a-tag v-if="record.is_default" color="blue">默认</a-tag>
            </template>
            <template v-if="column.key === 'action'">
              <a-space>
                <a-button type="link" size="small" @click="testConnection(record.id)" :loading="testing === record.id">测试</a-button>
                <a-button type="link" size="small" @click="selectInstance(record)">项目</a-button>
                <a-button type="link" size="small" @click="showInstanceModal(record)">编辑</a-button>
                <a-popconfirm title="确认删除?" @confirm="deleteInstance(record.id)">
                  <a-button type="link" danger size="small">删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <!-- 质量看板 -->
      <a-tab-pane key="dashboard" tab="质量看板" :disabled="!selectedInstance">
        <div v-if="selectedInstance" style="margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center">
          <a-space>
            <span style="font-weight: 600">{{ selectedInstance.name }}</span>
            <a-button size="small" @click="showBindingModal = true">绑定项目</a-button>
          </a-space>
          <a-select v-model:value="selectedProjectKey" placeholder="选择项目" style="width: 300px" show-search @change="loadProjectData">
            <a-select-option v-for="p in sonarProjects" :key="p.key" :value="p.key">{{ p.name }} ({{ p.key }})</a-select-option>
          </a-select>
        </div>

        <div v-if="selectedProjectKey">
          <!-- 质量门禁 -->
          <a-card title="质量门禁" style="margin-bottom: 16px" size="small">
            <a-result v-if="qualityGate" :status="qualityGate.status === 'OK' ? 'success' : qualityGate.status === 'WARN' ? 'warning' : 'error'" :title="qualityGate.status === 'OK' ? '通过' : qualityGate.status === 'WARN' ? '警告' : '未通过'" />
            <a-table v-if="qualityGate?.conditions" :dataSource="qualityGate.conditions" :columns="conditionColumns" rowKey="metricKey" :pagination="false" size="small">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'condStatus'">
                  <a-tag :color="record.status === 'OK' ? 'green' : record.status === 'WARN' ? 'orange' : 'red'">{{ record.status }}</a-tag>
                </template>
              </template>
            </a-table>
          </a-card>

          <!-- 度量指标 -->
          <a-row :gutter="16" style="margin-bottom: 16px">
            <a-col :span="4" v-for="m in measures" :key="m.metric">
              <a-card size="small">
                <a-statistic :title="metricLabel(m.metric)" :value="m.value" :suffix="metricSuffix(m.metric)" :value-style="{ fontSize: '20px' }" />
              </a-card>
            </a-col>
          </a-row>

          <!-- 问题列表 -->
          <a-card title="代码问题" size="small">
            <template #extra>
              <a-select v-model:value="issueSeverity" placeholder="严重程度" style="width: 140px" allow-clear @change="loadIssues">
                <a-select-option value="BLOCKER">Blocker</a-select-option>
                <a-select-option value="CRITICAL">Critical</a-select-option>
                <a-select-option value="MAJOR">Major</a-select-option>
                <a-select-option value="MINOR">Minor</a-select-option>
                <a-select-option value="INFO">Info</a-select-option>
              </a-select>
            </template>
            <a-table :dataSource="issues" :columns="issueColumns" rowKey="key" size="small" :pagination="{ pageSize: 20, total: issueTotal, onChange: onIssuePageChange }">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'severity'">
                  <a-tag :color="severityColor(record.severity)">{{ record.severity }}</a-tag>
                </template>
                <template v-if="column.key === 'type'">
                  <a-tag>{{ record.type }}</a-tag>
                </template>
              </template>
            </a-table>
          </a-card>
        </div>
        <a-empty v-else-if="selectedInstance" description="请选择一个项目查看质量数据" />
        <a-empty v-else description="请先选择 SonarQube 实例" />
      </a-tab-pane>

      <!-- 项目绑定 -->
      <a-tab-pane key="bindings" tab="项目绑定" :disabled="!selectedInstance">
        <a-table v-if="selectedInstance" :dataSource="bindings" :columns="bindingColumns" rowKey="id" :pagination="false" size="small">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'gate'">
              <a-tag :color="record.quality_gate_status === 'OK' ? 'green' : record.quality_gate_status === 'ERROR' ? 'red' : 'default'">
                {{ record.quality_gate_status || '未知' }}
              </a-tag>
            </template>
            <template v-if="column.key === 'action'">
              <a-space>
                <a-button type="link" size="small" @click="viewProject(record.sonar_project_key)">查看</a-button>
                <a-popconfirm title="确认删除?" @confirm="deleteBinding(record.id)">
                  <a-button type="link" danger size="small">删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>

    <!-- 实例表单弹窗 -->
    <a-modal v-model:open="instanceModalVisible" :title="editingInstance ? '编辑实例' : '添加实例'" @ok="saveInstance" :confirmLoading="saving">
      <a-form :label-col="{ span: 5 }">
        <a-form-item label="名称" required>
          <a-input v-model:value="instanceForm.name" placeholder="SonarQube 实例名称" />
        </a-form-item>
        <a-form-item label="URL" required>
          <a-input v-model:value="instanceForm.base_url" placeholder="https://sonarqube.example.com" />
        </a-form-item>
        <a-form-item label="Token" required>
          <a-input-password v-model:value="instanceForm.token" placeholder="SonarQube User Token" />
        </a-form-item>
        <a-form-item label="默认实例">
          <a-switch v-model:checked="instanceForm.is_default" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 绑定项目弹窗 -->
    <a-modal v-model:open="showBindingModal" title="绑定 SonarQube 项目" @ok="createBinding" :confirmLoading="saving">
      <a-form :label-col="{ span: 6 }">
        <a-form-item label="SonarQube 项目">
          <a-select v-model:value="bindingForm.sonar_project_key" placeholder="选择项目" show-search style="width: 100%">
            <a-select-option v-for="p in sonarProjects" :key="p.key" :value="p.key">{{ p.name }} ({{ p.key }})</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="关联应用名称">
          <a-input v-model:value="bindingForm.devops_app_name" placeholder="DevOps 应用名称" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { sonarqubeApi } from '@/services/sonarqube'
import type { SonarQubeInstance, SonarQubeProjectBinding, SonarProject, QualityGate, SonarMeasure, SonarIssue } from '@/services/sonarqube'

const activeTab = ref('instances')
const instances = ref<SonarQubeInstance[]>([])
const selectedInstance = ref<SonarQubeInstance | null>(null)
const sonarProjects = ref<SonarProject[]>([])
const selectedProjectKey = ref('')
const bindings = ref<SonarQubeProjectBinding[]>([])
const qualityGate = ref<QualityGate | null>(null)
const measures = ref<SonarMeasure[]>([])
const issues = ref<SonarIssue[]>([])
const issueTotal = ref(0)
const issueSeverity = ref<string | undefined>(undefined)
const issuePage = ref(1)
const saving = ref(false)
const testing = ref<number | null>(null)

const instanceModalVisible = ref(false)
const editingInstance = ref(false)
const showBindingModal = ref(false)

const instanceForm = reactive({ id: 0, name: '', base_url: '', token: '', is_default: false, status: 'active' })
const bindingForm = reactive({ sonar_project_key: '', devops_app_name: '' })

const instanceColumns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: 'URL', dataIndex: 'base_url', key: 'base_url', ellipsis: true },
  { title: '状态', key: 'status' },
  { title: '默认', key: 'is_default' },
  { title: '操作', key: 'action', width: 200 },
]

const conditionColumns = [
  { title: '指标', dataIndex: 'metricKey', key: 'metricKey' },
  { title: '状态', key: 'condStatus' },
  { title: '实际值', dataIndex: 'actualValue', key: 'actualValue' },
  { title: '阈值', dataIndex: 'errorThreshold', key: 'errorThreshold' },
  { title: '比较', dataIndex: 'comparator', key: 'comparator' },
]

const issueColumns = [
  { title: '严重程度', key: 'severity', width: 100 },
  { title: '类型', key: 'type', width: 100 },
  { title: '规则', dataIndex: 'rule', key: 'rule', width: 180, ellipsis: true },
  { title: '描述', dataIndex: 'message', key: 'message', ellipsis: true },
  { title: '文件', dataIndex: 'component', key: 'component', ellipsis: true },
  { title: '行', dataIndex: 'line', key: 'line', width: 60 },
]

const bindingColumns = [
  { title: 'SonarQube 项目', dataIndex: 'sonar_project_key', key: 'sonar_project_key' },
  { title: '项目名称', dataIndex: 'sonar_project_name', key: 'sonar_project_name' },
  { title: '关联应用', dataIndex: 'devops_app_name', key: 'devops_app_name' },
  { title: '质量门禁', key: 'gate' },
  { title: '操作', key: 'action', width: 120 },
]

const metricLabels: Record<string, string> = {
  bugs: 'Bugs', vulnerabilities: '漏洞', code_smells: '代码异味',
  coverage: '覆盖率', duplicated_lines_density: '重复率', ncloc: '代码行数',
  reliability_rating: '可靠性', security_rating: '安全性', sqale_rating: '可维护性',
  alert_status: '质量门禁',
}

const metricLabel = (m: string) => metricLabels[m] || m
const metricSuffix = (m: string) => (m === 'coverage' || m === 'duplicated_lines_density') ? '%' : ''
const severityColor = (s: string) => ({ BLOCKER: 'red', CRITICAL: 'volcano', MAJOR: 'orange', MINOR: 'blue', INFO: 'default' }[s] || 'default')

async function loadInstances() {
  try {
    const res = await sonarqubeApi.listInstances()
    instances.value = res.data?.data || []
  } catch { message.error('加载实例失败') }
}

function selectInstance(inst: SonarQubeInstance) {
  selectedInstance.value = inst
  activeTab.value = 'dashboard'
  selectedProjectKey.value = ''
  qualityGate.value = null
  measures.value = []
  issues.value = []
  loadSonarProjects()
  loadBindings()
}

async function loadSonarProjects() {
  if (!selectedInstance.value) return
  try {
    const res = await sonarqubeApi.listProjects(selectedInstance.value.id, 1, 200)
    sonarProjects.value = res.data?.data?.projects || []
  } catch { /* ignore */ }
}

async function loadBindings() {
  if (!selectedInstance.value) return
  try {
    const res = await sonarqubeApi.listBindings(selectedInstance.value.id)
    bindings.value = res.data?.data || []
  } catch { /* ignore */ }
}

async function loadProjectData() {
  if (!selectedInstance.value || !selectedProjectKey.value) return
  loadQualityGate()
  loadMeasures()
  loadIssues()
}

async function loadQualityGate() {
  try {
    const res = await sonarqubeApi.getQualityGate(selectedInstance.value!.id, selectedProjectKey.value)
    qualityGate.value = res.data?.data || null
  } catch { qualityGate.value = null }
}

async function loadMeasures() {
  try {
    const res = await sonarqubeApi.getMeasures(selectedInstance.value!.id, selectedProjectKey.value)
    measures.value = res.data?.data || []
  } catch { measures.value = [] }
}

async function loadIssues() {
  try {
    const res = await sonarqubeApi.getIssues(selectedInstance.value!.id, selectedProjectKey.value, issuePage.value, 20, issueSeverity.value)
    issues.value = res.data?.data?.issues || []
    issueTotal.value = res.data?.data?.total || 0
  } catch { issues.value = []; issueTotal.value = 0 }
}

function onIssuePageChange(page: number) {
  issuePage.value = page
  loadIssues()
}

function showInstanceModal(inst?: SonarQubeInstance) {
  if (inst) {
    editingInstance.value = true
    Object.assign(instanceForm, { id: inst.id, name: inst.name, base_url: inst.base_url, token: '', is_default: inst.is_default, status: inst.status })
  } else {
    editingInstance.value = false
    Object.assign(instanceForm, { id: 0, name: '', base_url: '', token: '', is_default: false, status: 'active' })
  }
  instanceModalVisible.value = true
}

async function saveInstance() {
  if (!instanceForm.name || !instanceForm.base_url) { message.warning('请填写名称和 URL'); return }
  saving.value = true
  try {
    if (editingInstance.value) {
      await sonarqubeApi.updateInstance(instanceForm.id, instanceForm)
      message.success('更新成功')
    } else {
      await sonarqubeApi.createInstance(instanceForm)
      message.success('创建成功')
    }
    instanceModalVisible.value = false
    loadInstances()
  } catch { message.error('保存失败') } finally { saving.value = false }
}

async function deleteInstance(id: number) {
  try {
    await sonarqubeApi.deleteInstance(id)
    message.success('已删除')
    loadInstances()
  } catch { message.error('删除失败') }
}

async function testConnection(id: number) {
  testing.value = id
  try {
    const res = await sonarqubeApi.testConnection(id)
    const data = res.data?.data
    message.success(`连接成功: ${data?.status || 'OK'}`)
  } catch (e: any) {
    message.error('连接失败: ' + (e?.response?.data?.message || e.message))
  } finally { testing.value = null }
}

async function createBinding() {
  if (!bindingForm.sonar_project_key) { message.warning('请选择项目'); return }
  const project = sonarProjects.value.find(p => p.key === bindingForm.sonar_project_key)
  saving.value = true
  try {
    await sonarqubeApi.createBinding(selectedInstance.value!.id, {
      sonar_project_key: bindingForm.sonar_project_key,
      sonar_project_name: project?.name || '',
      devops_app_name: bindingForm.devops_app_name,
    })
    message.success('绑定成功')
    showBindingModal.value = false
    loadBindings()
  } catch { message.error('绑定失败') } finally { saving.value = false }
}

async function deleteBinding(id: number) {
  try {
    await sonarqubeApi.deleteBinding(id)
    message.success('已删除')
    loadBindings()
  } catch { message.error('删除失败') }
}

function viewProject(projectKey: string) {
  selectedProjectKey.value = projectKey
  activeTab.value = 'dashboard'
  loadProjectData()
}

onMounted(() => { loadInstances() })
</script>

<style scoped>
.sonarqube-integration {
  padding: 0;
}
</style>
