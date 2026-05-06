<template>
  <div class="ldap-settings">
    <a-tabs v-model:activeKey="activeTab">
      <!-- LDAP 连接配置 -->
      <a-tab-pane key="config" tab="LDAP 配置">
        <a-card :bordered="false">
          <a-form :model="config" :label-col="{ span: 4 }" :wrapper-col="{ span: 16 }">
            <a-form-item label="启用 LDAP">
              <a-switch v-model:checked="config.enabled" />
            </a-form-item>

            <a-divider orientation="left">连接设置</a-divider>

            <a-form-item label="服务器地址">
              <a-input v-model:value="config.server" placeholder="ldap.example.com" />
            </a-form-item>
            <a-form-item label="端口">
              <a-input-number v-model:value="config.port" :min="1" :max="65535" style="width: 120px" />
            </a-form-item>
            <a-form-item label="使用 TLS">
              <a-switch v-model:checked="config.use_tls" />
            </a-form-item>
            <a-form-item v-if="config.use_tls" label="跳过证书验证">
              <a-switch v-model:checked="config.skip_verify" />
            </a-form-item>
            <a-form-item label="Bind DN">
              <a-input v-model:value="config.bind_dn" placeholder="cn=admin,dc=example,dc=com" />
            </a-form-item>
            <a-form-item label="Bind 密码">
              <a-input-password v-model:value="config.bind_password" placeholder="留空则保留原密码" />
            </a-form-item>

            <a-divider orientation="left">用户搜索</a-divider>

            <a-form-item label="Base DN">
              <a-input v-model:value="config.base_dn" placeholder="ou=people,dc=example,dc=com" />
            </a-form-item>
            <a-form-item label="用户过滤器">
              <a-input v-model:value="config.user_filter" placeholder="(uid=%s)" />
              <template #extra>%s 将被替换为用户名</template>
            </a-form-item>
            <a-form-item label="用户名属性">
              <a-input v-model:value="config.attr_username" placeholder="uid" />
            </a-form-item>
            <a-form-item label="邮箱属性">
              <a-input v-model:value="config.attr_email" placeholder="mail" />
            </a-form-item>
            <a-form-item label="手机属性">
              <a-input v-model:value="config.attr_phone" placeholder="telephoneNumber" />
            </a-form-item>
            <a-form-item label="姓名属性">
              <a-input v-model:value="config.attr_real_name" placeholder="cn" />
            </a-form-item>

            <a-divider orientation="left">组搜索（可选）</a-divider>

            <a-form-item label="组 Base DN">
              <a-input v-model:value="config.group_base_dn" placeholder="ou=groups,dc=example,dc=com" />
              <template #extra>留空则不搜索组</template>
            </a-form-item>
            <a-form-item label="组过滤器">
              <a-input v-model:value="config.group_filter" placeholder="(objectClass=groupOfNames)" />
            </a-form-item>
            <a-form-item label="组名属性">
              <a-input v-model:value="config.group_attr_name" placeholder="cn" />
            </a-form-item>
            <a-form-item label="组成员属性">
              <a-input v-model:value="config.group_attr_member" placeholder="member" />
            </a-form-item>

            <a-form-item :wrapper-col="{ offset: 4 }">
              <a-space>
                <a-button type="primary" :loading="saving" @click="handleSave">保存配置</a-button>
                <a-button :loading="testing" @click="handleTest">测试连接</a-button>
              </a-space>
            </a-form-item>
          </a-form>
        </a-card>
      </a-tab-pane>

      <!-- 组→角色映射 -->
      <a-tab-pane key="mappings" tab="组角色映射">
        <a-card :bordered="false">
          <template #extra>
            <a-button type="primary" @click="showMappingModal()">新增映射</a-button>
          </template>

          <a-table :columns="mappingColumns" :data-source="mappings" :loading="loadingMappings" row-key="id" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'action'">
                <a-space>
                  <a @click="showMappingModal(record)">编辑</a>
                  <a-popconfirm title="确定删除？" @confirm="handleDeleteMapping(record.id)">
                    <a style="color: #ff4d4f">删除</a>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>

    <!-- 映射编辑弹窗 -->
    <a-modal v-model:open="mappingModalVisible" :title="editingMapping?.id ? '编辑映射' : '新增映射'" @ok="handleSaveMapping" :confirmLoading="savingMapping">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="组 DN" required>
          <a-input v-model:value="mappingForm.group_dn" placeholder="cn=devops,ou=groups,dc=example,dc=com" />
        </a-form-item>
        <a-form-item label="组名" required>
          <a-input v-model:value="mappingForm.group_name" placeholder="DevOps 团队" />
        </a-form-item>
        <a-form-item label="映射角色" required>
          <a-select v-model:value="mappingForm.role_id" placeholder="选择角色" :loading="loadingRoles">
            <a-select-option v-for="r in roles" :key="r.id" :value="r.id">{{ r.display_name || r.name }}</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { ldapApi, type LDAPConfig, type LDAPGroupMapping } from '@/services/ldap'
import request from '@/services/api'

const activeTab = ref('config')

const config = reactive<LDAPConfig>({
  enabled: false,
  server: '',
  port: 389,
  use_tls: false,
  skip_verify: false,
  bind_dn: '',
  bind_password: '',
  base_dn: '',
  user_filter: '(uid=%s)',
  attr_username: 'uid',
  attr_email: 'mail',
  attr_phone: 'telephoneNumber',
  attr_real_name: 'cn',
  group_base_dn: '',
  group_filter: '(objectClass=groupOfNames)',
  group_attr_name: 'cn',
  group_attr_member: 'member',
})

const saving = ref(false)
const testing = ref(false)

const loadConfig = async () => {
  try {
    const res = await ldapApi.getConfig()
    if (res.data) {
      Object.assign(config, res.data)
      config.bind_password = ''
    }
  } catch {}
}

const handleSave = async () => {
  saving.value = true
  try {
    await ldapApi.saveConfig({ ...config })
    message.success('配置已保存')
  } catch (e: any) {
    message.error(e?.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

const handleTest = async () => {
  testing.value = true
  try {
    await ldapApi.testConnection({ ...config })
    message.success('连接成功')
  } catch (e: any) {
    message.error(e?.response?.data?.message || '连接失败')
  } finally {
    testing.value = false
  }
}

// --- 组映射 ---
const mappings = ref<LDAPGroupMapping[]>([])
const loadingMappings = ref(false)
const mappingModalVisible = ref(false)
const savingMapping = ref(false)
const editingMapping = ref<LDAPGroupMapping | null>(null)
const mappingForm = reactive<LDAPGroupMapping>({ group_dn: '', group_name: '', role_id: 0 })

interface Role { id: number; name: string; display_name?: string }
const roles = ref<Role[]>([])
const loadingRoles = ref(false)

const mappingColumns = [
  { title: '组 DN', dataIndex: 'group_dn', key: 'group_dn', ellipsis: true },
  { title: '组名', dataIndex: 'group_name', key: 'group_name', width: 160 },
  { title: '映射角色', dataIndex: 'role_name', key: 'role_name', width: 140 },
  { title: '操作', key: 'action', width: 120 },
]

const loadMappings = async () => {
  loadingMappings.value = true
  try {
    const res = await ldapApi.listGroupMappings()
    mappings.value = res.data || []
  } catch {} finally {
    loadingMappings.value = false
  }
}

const loadRoles = async () => {
  loadingRoles.value = true
  try {
    const res = await request.get('/rbac/roles', { params: { page: 1, page_size: 500 } })
    const raw = res.data as Role[] | { list?: Role[] } | undefined
    roles.value = Array.isArray(raw) ? raw : raw?.list ?? []
  } catch {} finally {
    loadingRoles.value = false
  }
}

const showMappingModal = (record?: LDAPGroupMapping) => {
  editingMapping.value = record || null
  if (record) {
    Object.assign(mappingForm, { group_dn: record.group_dn, group_name: record.group_name, role_id: record.role_id })
  } else {
    Object.assign(mappingForm, { group_dn: '', group_name: '', role_id: 0 })
  }
  mappingModalVisible.value = true
}

const handleSaveMapping = async () => {
  if (!mappingForm.group_dn || !mappingForm.group_name || !mappingForm.role_id) {
    message.warning('请填写完整')
    return
  }
  savingMapping.value = true
  try {
    if (editingMapping.value?.id) {
      await ldapApi.updateGroupMapping(editingMapping.value.id, { ...mappingForm })
    } else {
      await ldapApi.createGroupMapping({ ...mappingForm })
    }
    message.success('保存成功')
    mappingModalVisible.value = false
    loadMappings()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '保存失败')
  } finally {
    savingMapping.value = false
  }
}

const handleDeleteMapping = async (id: number) => {
  try {
    await ldapApi.deleteGroupMapping(id)
    message.success('已删除')
    loadMappings()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '删除失败')
  }
}

onMounted(() => {
  loadConfig()
  loadMappings()
  loadRoles()
})
</script>

<style scoped>
.ldap-settings {
  padding: 0;
}
</style>
