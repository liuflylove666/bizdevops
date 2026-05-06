<template>
  <div class="db-instance">
    <div class="page-header">
      <h1>数据库实例</h1>
      <a-space>
        <a-button type="primary" @click="showModal()">
          <template #icon><PlusOutlined /></template>
          新建实例
        </a-button>
      </a-space>
    </div>

    <a-card :bordered="false">
      <a-form layout="inline" class="filter-bar">
        <a-form-item label="名称">
          <a-input v-model:value="filters.name" allow-clear placeholder="实例名称" @press-enter="loadList" />
        </a-form-item>
        <a-form-item label="环境">
          <a-select v-model:value="filters.env" allow-clear style="width: 120px" placeholder="全部" @change="loadList">
            <a-select-option value="dev">dev</a-select-option>
            <a-select-option value="test">test</a-select-option>
            <a-select-option value="staging">staging</a-select-option>
            <a-select-option value="prod">prod</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="类型">
          <a-select v-model:value="filters.db_type" allow-clear style="width: 120px" placeholder="全部" @change="loadList">
            <a-select-option value="mysql">MySQL</a-select-option>
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
            <a-badge :status="record.status === 'active' ? 'success' : 'default'" :text="record.status === 'active' ? '启用' : '禁用'" />
          </template>
          <template v-if="column.key === 'env'">
            <a-tag :color="envColor(record.env)">{{ record.env }}</a-tag>
          </template>
          <template v-if="column.key === 'action'">
            <a-space>
              <a-button type="link" size="small" @click="testInstance(record)">测试</a-button>
              <a-button type="link" size="small" @click="showModal(record)">编辑</a-button>
              <a-button type="link" size="small" @click="openACL(record)">权限</a-button>
              <a-popconfirm title="确定删除？" @confirm="() => deleteInstance(record.id)">
                <a-button type="link" size="small" danger>删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal v-model:open="modalVisible" :title="form.id ? '编辑实例' : '新建实例'" width="720px" @ok="saveInstance" :confirm-loading="saving">
      <a-form :model="form" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="名称" required>
              <a-input v-model:value="form.name" placeholder="实例唯一名称" />
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item label="类型">
              <a-select v-model:value="form.db_type">
                <a-select-option value="mysql">MySQL</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item label="环境">
              <a-select v-model:value="form.env">
                <a-select-option value="dev">dev</a-select-option>
                <a-select-option value="test">test</a-select-option>
                <a-select-option value="staging">staging</a-select-option>
                <a-select-option value="prod">prod</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="16">
            <a-form-item label="主机" required>
              <a-input v-model:value="form.host" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="端口" required>
              <a-input-number v-model:value="form.port" :min="1" :max="65535" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="账号" required>
              <a-input v-model:value="form.username" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="form.id ? '密码（留空则不修改）' : '密码'" :required="!form.id">
              <a-input-password v-model:value="form.plain_password" placeholder="数据库密码" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="默认库">
              <a-input v-model:value="form.default_db" placeholder="可选" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="屏蔽库 (逗号分隔)">
              <a-input v-model:value="form.exclude_dbs" placeholder="mysql,sys" />
            </a-form-item>
          </a-col>
          <a-col :span="24">
            <a-form-item label="DSN 扩展参数">
              <a-input v-model:value="form.params" placeholder="charset=utf8mb4&parseTime=True&loc=Local" />
            </a-form-item>
          </a-col>
          <a-col :span="24">
            <a-form-item label="描述">
              <a-textarea v-model:value="form.description" :rows="2" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-space>
          <a-button @click="testForm" :loading="testing">测试连接</a-button>
        </a-space>
      </a-form>
    </a-modal>

    <!-- ACL 权限抽屉 -->
    <a-drawer
      v-model:open="aclDrawerVisible"
      :title="`权限管理 - ${aclInstance?.name || ''}`"
      width="640"
      @close="aclDrawerVisible = false"
    >
      <a-card :bordered="false" size="small" style="margin-bottom: 16px">
        <a-form layout="vertical" @finish="bindACL">
          <a-row :gutter="12">
            <a-col :span="6">
              <a-form-item label="类型">
                <a-select v-model:value="aclForm.subject_type" style="width: 100%">
                  <a-select-option value="user">用户</a-select-option>
                  <a-select-option value="role">角色</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item label="对象">
                <a-select
                  v-model:value="aclForm.subject_id"
                  style="width: 100%"
                  show-search
                  option-filter-prop="label"
                  placeholder="请选择"
                >
                  <template v-if="aclForm.subject_type === 'user'">
                    <a-select-option v-for="u in aclUsers" :key="u.id" :value="u.id" :label="u.username">
                      {{ u.username }}
                    </a-select-option>
                  </template>
                  <template v-else>
                    <a-select-option v-for="r in aclRoles" :key="r.id" :value="r.id" :label="r.display_name || r.name">
                      {{ r.display_name || r.name }}
                    </a-select-option>
                  </template>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="6">
              <a-form-item label="级别">
                <a-select v-model:value="aclForm.access_level" style="width: 100%">
                  <a-select-option value="read">只读</a-select-option>
                  <a-select-option value="write">读写</a-select-option>
                  <a-select-option value="owner">管理</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="4" style="display:flex;align-items:flex-end;padding-bottom:24px">
              <a-button type="primary" html-type="submit" :loading="aclBinding" block>添加</a-button>
            </a-col>
          </a-row>
          <a-row>
            <a-col :span="24">
              <a-form-item label="授权库" :extra="aclSchemas.length === 0 ? '加载中...' : '留空表示授权全部库'">
                <a-select
                  v-model:value="aclSelectedSchemas"
                  mode="multiple"
                  style="width: 100%"
                  placeholder="全部库（不选 = 所有库）"
                  :loading="aclSchemasLoading"
                  allow-clear
                  :options="aclSchemas.map(s => ({ label: s, value: s }))"
                />
              </a-form-item>
            </a-col>
          </a-row>
        </a-form>
      </a-card>
      <a-table
        :columns="aclColumns"
        :data-source="aclList"
        :loading="aclLoading"
        row-key="id"
        :pagination="false"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'subject'">
            <a-tag :color="record.subject_type === 'user' ? 'blue' : 'green'">
              {{ record.subject_type === 'user' ? '用户' : '角色' }}
            </a-tag>
            {{ resolveSubjectName(record) }}
          </template>
          <template v-if="column.key === 'access_level'">
            <a-tag :color="levelColor(record.access_level)">{{ levelLabel(record.access_level) }}</a-tag>
          </template>
          <template v-if="column.key === 'schemas'">
            <template v-if="!record.schema_names">
              <a-tag color="purple">全部库</a-tag>
            </template>
            <template v-else>
              <a-tag v-for="s in record.schema_names.split(',')" :key="s" style="margin-bottom:2px">{{ s }}</a-tag>
            </template>
          </template>
          <template v-if="column.key === 'action'">
            <a-popconfirm title="确定移除？" @confirm="() => unbindACL(record.id)">
              <a-button type="link" size="small" danger>移除</a-button>
            </a-popconfirm>
          </template>
        </template>
      </a-table>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { dbInstanceApi, dbACLApi, type DBInstance, type DBInstanceReq, type DBInstanceACL, type ACLBindInput } from '@/services/database'
import { userApi } from '@/services/user'
import { getRoles, type Role } from '@/services/rbac'

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 70 },
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '类型', dataIndex: 'db_type', key: 'db_type', width: 90 },
  { title: '环境', dataIndex: 'env', key: 'env', width: 90 },
  { title: '主机', dataIndex: 'host', key: 'host' },
  { title: '端口', dataIndex: 'port', key: 'port', width: 80 },
  { title: '账号', dataIndex: 'username', key: 'username', width: 120 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 90 },
  { title: '操作', key: 'action', width: 200 }
]

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const list = ref<DBInstance[]>([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const filters = reactive<Record<string, string>>({ name: '', env: '', db_type: '' })
const modalVisible = ref(false)
const form = reactive<DBInstanceReq>({ db_type: 'mysql', env: 'dev', port: 3306, status: 'active', mode: 0 })

function resetForm() {
  Object.assign(form, {
    id: undefined, name: '', db_type: 'mysql', env: 'dev', host: '', port: 3306,
    username: '', plain_password: '', default_db: '', exclude_dbs: '', params: '',
    mode: 0, status: 'active', description: ''
  })
}

function envColor(env: string) {
  return { prod: 'red', staging: 'orange', test: 'blue', dev: 'green' }[env] || 'default'
}

async function loadList() {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      ...filters
    }
    const res = await dbInstanceApi.list(params)
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

function showModal(record?: DBInstance) {
  resetForm()
  if (record) {
    Object.assign(form, { ...record, plain_password: '' })
  }
  modalVisible.value = true
}

async function saveInstance() {
  if (!form.name || !form.host || !form.username) {
    message.warning('请填写必填项')
    return
  }
  if (!form.id && !form.plain_password) {
    message.warning('请填写密码')
    return
  }
  saving.value = true
  try {
    if (form.id) {
      await dbInstanceApi.update(form.id, form)
    } else {
      await dbInstanceApi.create(form)
    }
    message.success('保存成功')
    modalVisible.value = false
    loadList()
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function deleteInstance(id?: number) {
  if (!id) return
  try {
    await dbInstanceApi.delete(id)
    message.success('删除成功')
    loadList()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

async function testInstance(record: DBInstance) {
  if (!record.id) return
  try {
    const res = await dbInstanceApi.testExisting(record.id)
    if (res.code === 0) message.success('连接成功')
    else message.error(res.message || '连接失败')
  } catch (e: any) {
    message.error(e?.message || '连接失败')
  }
}

async function testForm() {
  testing.value = true
  try {
    const res = await dbInstanceApi.test(form)
    if (res.code === 0) message.success('连接成功')
    else message.error(res.message || '连接失败')
  } catch (e: any) {
    message.error(e?.message || '连接失败')
  } finally {
    testing.value = false
  }
}

// ========== ACL 权限管理 ==========

const aclDrawerVisible = ref(false)
const aclInstance = ref<DBInstance | null>(null)
const aclList = ref<DBInstanceACL[]>([])
const aclLoading = ref(false)
const aclBinding = ref(false)
const aclUsers = ref<any[]>([])
const aclRoles = ref<Role[]>([])
const aclForm = reactive<ACLBindInput>({ subject_type: 'user', subject_id: 0, access_level: 'read', schema_names: '' })
const aclSchemas = ref<string[]>([])
const aclSchemasLoading = ref(false)
const aclSelectedSchemas = ref<string[]>([])

const aclColumns = [
  { title: '授权对象', key: 'subject', width: 180 },
  { title: '级别', key: 'access_level', width: 80 },
  { title: '授权库', key: 'schemas' },
  { title: '操作', key: 'action', width: 80 }
]

function levelColor(l: string) {
  return { read: 'blue', write: 'orange', owner: 'red' }[l] || 'default'
}
function levelLabel(l: string) {
  return { read: '只读', write: '读写', owner: '管理' }[l] || l
}

function resolveSubjectName(record: DBInstanceACL) {
  if (record.subject_name) return record.subject_name
  if (record.subject_type === 'user') {
    const u = aclUsers.value.find(u => u.id === record.subject_id)
    return u ? u.username : `User#${record.subject_id}`
  }
  const r = aclRoles.value.find(r => r.id === record.subject_id)
  return r ? (r.display_name || r.name) : `Role#${record.subject_id}`
}

async function loadACLDeps() {
  try {
    const [uRes, rRes] = await Promise.all([
      userApi.getUsers({ page: 1, page_size: 1000 }),
      getRoles({ page: 1, page_size: 1000 })
    ])
    aclUsers.value = uRes.data?.items || []
    aclRoles.value = rRes.data?.list || []
  } catch { /* ignore */ }
}

async function openACL(record: DBInstance) {
  aclInstance.value = record
  aclDrawerVisible.value = true
  aclForm.subject_type = 'user'
  aclForm.subject_id = 0
  aclForm.access_level = 'read'
  aclForm.schema_names = ''
  aclSelectedSchemas.value = []
  aclSchemas.value = []
  loadACLDeps()
  loadACLList()
  loadInstanceSchemas(record.id!)
}

async function loadInstanceSchemas(instanceId: number) {
  aclSchemasLoading.value = true
  try {
    const res = await dbInstanceApi.databases(instanceId)
    aclSchemas.value = res.data || []
  } catch { /* ignore */ }
  finally { aclSchemasLoading.value = false }
}

async function loadACLList() {
  if (!aclInstance.value?.id) return
  aclLoading.value = true
  try {
    const res = await dbACLApi.list(aclInstance.value.id)
    aclList.value = res.data || []
  } catch (e: any) {
    message.error(e?.message || '加载权限失败')
  } finally {
    aclLoading.value = false
  }
}

async function bindACL() {
  if (!aclInstance.value?.id || !aclForm.subject_id) {
    message.warning('请选择授权对象')
    return
  }
  aclBinding.value = true
  try {
    const payload = { ...aclForm, schema_names: aclSelectedSchemas.value.join(',') }
    const res = await dbACLApi.bind(aclInstance.value.id, payload)
    if (res.code === 0) {
      message.success('添加成功')
      loadACLList()
    } else {
      message.error(res.message || '添加失败')
    }
  } catch (e: any) {
    message.error(e?.message || '添加失败')
  } finally {
    aclBinding.value = false
  }
}

async function unbindACL(id: number) {
  try {
    await dbACLApi.unbind(id)
    message.success('已移除')
    loadACLList()
  } catch (e: any) {
    message.error(e?.message || '移除失败')
  }
}

onMounted(loadList)
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header h1 { margin: 0; font-size: 20px; }
.filter-bar { margin-bottom: 16px; }
</style>
