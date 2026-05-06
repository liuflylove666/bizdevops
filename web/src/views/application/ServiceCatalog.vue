<template>
  <div class="service-catalog">
    <a-tabs v-model:activeKey="activeTab">
      <!-- 组织管理 -->
      <a-tab-pane key="orgs" tab="组织管理">
        <a-card :bordered="false">
          <template #extra><a-button type="primary" @click="showOrgModal()">新增组织</a-button></template>
          <a-table :columns="orgColumns" :data-source="orgs" :loading="loadingOrgs" row-key="id" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'status'">
                <a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status === 'active' ? '启用' : '停用' }}</a-tag>
              </template>
              <template v-if="column.key === 'action'">
                <a-space>
                  <a @click="showOrgModal(record)">编辑</a>
                  <a-popconfirm title="确定删除？" @confirm="handleDeleteOrg(record.id)"><a style="color:#ff4d4f">删除</a></a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 项目管理 -->
      <a-tab-pane key="projects" tab="项目管理">
        <a-card :bordered="false">
          <template #extra>
            <a-space>
              <a-select v-model:value="projOrgFilter" style="width:180px" placeholder="按组织筛选" allow-clear @change="loadProjects">
                <a-select-option v-for="o in orgs" :key="o.id" :value="o.id">{{ o.display_name || o.name }}</a-select-option>
              </a-select>
              <a-button type="primary" @click="showProjModal()">新增项目</a-button>
            </a-space>
          </template>
          <a-table :columns="projColumns" :data-source="projects" :loading="loadingProjects" row-key="id" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'action'">
                <a-space>
                  <a @click="goProjectDetail(record)">项目概览</a>
                  <a @click="goProjectApplications(record)">查看应用</a>
                  <a @click="showProjModal(record)">编辑</a>
                  <a-popconfirm title="确定删除？" @confirm="handleDeleteProject(record.id)"><a style="color:#ff4d4f">删除</a></a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 环境定义 -->
      <a-tab-pane key="envs" tab="环境定义">
        <a-card :bordered="false">
          <template #extra><a-button type="primary" @click="showEnvModal()">新增环境</a-button></template>
          <a-table :columns="envColumns" :data-source="envDefs" :loading="loadingEnvs" row-key="id" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'color'">
                <a-tag :color="record.color">{{ record.display_name || record.name }}</a-tag>
              </template>
              <template v-if="column.key === 'action'">
                <a-space>
                  <a @click="showEnvModal(record)">编辑</a>
                  <a-popconfirm title="确定删除？" @confirm="handleDeleteEnv(record.id)"><a style="color:#ff4d4f">删除</a></a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>

    <!-- 组织弹窗 -->
    <a-modal v-model:open="orgModalVisible" :title="editingOrg?.id ? '编辑组织' : '新增组织'" @ok="handleSaveOrg" :confirmLoading="savingOrg">
      <a-form :label-col="{span:5}" :wrapper-col="{span:17}">
        <a-form-item label="标识" required><a-input v-model:value="orgForm.name" :disabled="!!editingOrg?.id" /></a-form-item>
        <a-form-item label="名称"><a-input v-model:value="orgForm.display_name" /></a-form-item>
        <a-form-item label="负责人"><a-input v-model:value="orgForm.owner" /></a-form-item>
        <a-form-item label="描述"><a-textarea v-model:value="orgForm.description" :rows="2" /></a-form-item>
      </a-form>
    </a-modal>

    <!-- 项目弹窗 -->
    <a-modal v-model:open="projModalVisible" :title="editingProj?.id ? '编辑项目' : '新增项目'" @ok="handleSaveProject" :confirmLoading="savingProj">
      <a-form :label-col="{span:5}" :wrapper-col="{span:17}">
        <a-form-item label="组织" required>
          <a-select v-model:value="projForm.organization_id" placeholder="选择组织">
            <a-select-option v-for="o in orgs" :key="o.id" :value="o.id">{{ o.display_name || o.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="标识" required><a-input v-model:value="projForm.name" :disabled="!!editingProj?.id" /></a-form-item>
        <a-form-item label="名称"><a-input v-model:value="projForm.display_name" /></a-form-item>
        <a-form-item label="负责人"><a-input v-model:value="projForm.owner" /></a-form-item>
        <a-form-item label="描述"><a-textarea v-model:value="projForm.description" :rows="2" /></a-form-item>
      </a-form>
    </a-modal>

    <!-- 环境弹窗 -->
    <a-modal v-model:open="envModalVisible" :title="editingEnv?.id ? '编辑环境' : '新增环境'" @ok="handleSaveEnv" :confirmLoading="savingEnv">
      <a-form :label-col="{span:5}" :wrapper-col="{span:17}">
        <a-form-item label="标识" required><a-input v-model:value="envForm.name" :disabled="!!editingEnv?.id" /></a-form-item>
        <a-form-item label="名称"><a-input v-model:value="envForm.display_name" /></a-form-item>
        <a-form-item label="排序"><a-input-number v-model:value="envForm.sort_order" :min="0" /></a-form-item>
        <a-form-item label="颜色">
          <a-select v-model:value="envForm.color">
            <a-select-option v-for="c in ['blue','cyan','green','orange','purple','red','default']" :key="c" :value="c"><a-tag :color="c">{{ c }}</a-tag></a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { catalogApi, type Organization, type Project, type EnvDefinition } from '@/services/catalog'

const router = useRouter()
const activeTab = ref('orgs')

// --- Orgs ---
const orgs = ref<Organization[]>([])
const loadingOrgs = ref(false)
const orgModalVisible = ref(false)
const savingOrg = ref(false)
const editingOrg = ref<Organization | null>(null)
const orgForm = reactive<Organization>({ name: '', display_name: '', description: '', owner: '', status: 'active' })
const orgColumns = [
  { title: '标识', dataIndex: 'name', key: 'name', width: 140 },
  { title: '名称', dataIndex: 'display_name', key: 'display_name', width: 180 },
  { title: '负责人', dataIndex: 'owner', key: 'owner', width: 120 },
  { title: '状态', key: 'status', width: 80 },
  { title: '操作', key: 'action', width: 120 },
]
const loadOrgs = async () => { loadingOrgs.value = true; try { orgs.value = (await catalogApi.listOrgs()).data || [] } catch {} finally { loadingOrgs.value = false } }
const showOrgModal = (r?: Organization) => { editingOrg.value = r || null; Object.assign(orgForm, r || { name: '', display_name: '', description: '', owner: '', status: 'active' }); orgModalVisible.value = true }
const handleSaveOrg = async () => {
  if (!orgForm.name) { message.warning('请填写标识'); return }
  savingOrg.value = true
  try { editingOrg.value?.id ? await catalogApi.updateOrg(editingOrg.value.id, { ...orgForm }) : await catalogApi.createOrg({ ...orgForm }); message.success('保存成功'); orgModalVisible.value = false; loadOrgs() } catch (e: any) { message.error(e?.response?.data?.message || '保存失败') } finally { savingOrg.value = false }
}
const handleDeleteOrg = async (id: number) => { try { await catalogApi.deleteOrg(id); message.success('已删除'); loadOrgs() } catch (e: any) { message.error(e?.response?.data?.message || '删除失败') } }

// --- Projects ---
const projects = ref<Project[]>([])
const loadingProjects = ref(false)
const projOrgFilter = ref<number | undefined>(undefined)
const projModalVisible = ref(false)
const savingProj = ref(false)
const editingProj = ref<Project | null>(null)
const projForm = reactive<Project>({ organization_id: 0, name: '', display_name: '', description: '', owner: '', status: 'active' })
const projColumns = [
  { title: '标识', dataIndex: 'name', key: 'name', width: 140 },
  { title: '名称', dataIndex: 'display_name', key: 'display_name', width: 180 },
  { title: '所属组织', dataIndex: 'org_name', key: 'org_name', width: 140 },
  { title: '负责人', dataIndex: 'owner', key: 'owner', width: 120 },
  { title: '操作', key: 'action', width: 120 },
]
const loadProjects = async () => { loadingProjects.value = true; try { projects.value = (await catalogApi.listProjects(projOrgFilter.value)).data || [] } catch {} finally { loadingProjects.value = false } }
const showProjModal = (r?: Project) => { editingProj.value = r || null; Object.assign(projForm, r || { organization_id: 0, name: '', display_name: '', description: '', owner: '', status: 'active' }); projModalVisible.value = true }
const handleSaveProject = async () => {
  if (!projForm.name || !projForm.organization_id) { message.warning('请填写标识和选择组织'); return }
  savingProj.value = true
  try { editingProj.value?.id ? await catalogApi.updateProject(editingProj.value.id, { ...projForm }) : await catalogApi.createProject({ ...projForm }); message.success('保存成功'); projModalVisible.value = false; loadProjects() } catch (e: any) { message.error(e?.response?.data?.message || '保存失败') } finally { savingProj.value = false }
}
const handleDeleteProject = async (id: number) => { try { await catalogApi.deleteProject(id); message.success('已删除'); loadProjects() } catch (e: any) { message.error(e?.response?.data?.message || '删除失败') } }
const goProjectDetail = (project: Project) => {
  if (!project.id) return
  router.push(`/catalog/projects/${project.id}`)
}
const goProjectApplications = (project: Project) => {
  router.push({
    path: '/applications',
    query: {
      organization_id: project.organization_id ? String(project.organization_id) : undefined,
      project_id: project.id ? String(project.id) : undefined,
    },
  })
}

// --- Env Definitions ---
const envDefs = ref<EnvDefinition[]>([])
const loadingEnvs = ref(false)
const envModalVisible = ref(false)
const savingEnv = ref(false)
const editingEnv = ref<EnvDefinition | null>(null)
const envForm = reactive<EnvDefinition>({ name: '', display_name: '', sort_order: 0, color: 'blue' })
const envColumns = [
  { title: '标识', dataIndex: 'name', key: 'name', width: 120 },
  { title: '显示', key: 'color', width: 120 },
  { title: '排序', dataIndex: 'sort_order', key: 'sort_order', width: 80 },
  { title: '操作', key: 'action', width: 120 },
]
const loadEnvs = async () => { loadingEnvs.value = true; try { envDefs.value = (await catalogApi.listEnvs()).data || [] } catch {} finally { loadingEnvs.value = false } }
const showEnvModal = (r?: EnvDefinition) => { editingEnv.value = r || null; Object.assign(envForm, r || { name: '', display_name: '', sort_order: 0, color: 'blue' }); envModalVisible.value = true }
const handleSaveEnv = async () => {
  if (!envForm.name) { message.warning('请填写标识'); return }
  savingEnv.value = true
  try { editingEnv.value?.id ? await catalogApi.updateEnv(editingEnv.value.id, { ...envForm }) : await catalogApi.createEnv({ ...envForm }); message.success('保存成功'); envModalVisible.value = false; loadEnvs() } catch (e: any) { message.error(e?.response?.data?.message || '保存失败') } finally { savingEnv.value = false }
}
const handleDeleteEnv = async (id: number) => { try { await catalogApi.deleteEnv(id); message.success('已删除'); loadEnvs() } catch (e: any) { message.error(e?.response?.data?.message || '删除失败') } }

onMounted(() => { loadOrgs(); loadProjects(); loadEnvs() })
</script>
