<template>
  <div class="db-audit-rule">
    <div class="page-header">
      <h1>SQL 审核规则集</h1>
      <a-button type="primary" @click="openCreate">新建规则集</a-button>
    </div>

    <a-card :bordered="false">
      <a-table :columns="columns" :data-source="list" :loading="loading" row-key="id" :pagination="false">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'is_default'">
            <a-tag v-if="record.is_default" color="green">默认</a-tag>
          </template>
          <template v-if="column.key === 'action'">
            <a-space>
              <a-button size="small" type="link" @click="openEdit(record)">编辑</a-button>
              <a-button size="small" type="link" :disabled="record.is_default" @click="setDefault(record.id)">设为默认</a-button>
              <a-popconfirm title="确认删除？" @confirm="del(record.id)">
                <a-button size="small" type="link" danger>删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal
      v-model:open="editorOpen"
      :title="editing ? '编辑规则集' : '新建规则集'"
      width="680"
      @ok="submit"
      :confirm-loading="submitting"
    >
      <a-form layout="vertical" :model="form">
        <a-form-item label="名称" required>
          <a-input v-model:value="form.name" placeholder="例如: 生产库严格策略" />
        </a-form-item>
        <a-form-item label="说明">
          <a-textarea v-model:value="form.description" :rows="2" />
        </a-form-item>
        <a-form-item label="设为默认">
          <a-switch v-model:checked="form.is_default" />
        </a-form-item>
        <a-divider>规则开关</a-divider>
        <a-row :gutter="[8, 8]">
          <a-col :span="12" v-for="r in ruleList" :key="r.key">
            <a-checkbox v-model:checked="(form.config as any)[r.key]">{{ r.label }}</a-checkbox>
          </a-col>
        </a-row>
        <a-divider>阈值</a-divider>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="单工单最大语句数 (0 不限)">
              <a-input-number v-model:value="form.config.max_statement_per_ticket" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="单条最大字节 (0 用 100KB 默认)">
              <a-input-number v-model:value="form.config.max_statement_bytes" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { dbRuleApi, type SQLAuditRuleSet, type AuditRuleConfig, type RuleInput } from '@/services/database'

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '说明', dataIndex: 'description', key: 'description', ellipsis: true },
  { title: '默认', key: 'is_default', width: 80 },
  { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180 },
  { title: '操作', key: 'action', width: 240 }
]

const ruleList: { key: keyof AuditRuleConfig; label: string }[] = [
  { key: 'require_where', label: 'UPDATE/DELETE 必须 WHERE' },
  { key: 'suggest_dml_limit', label: 'UPDATE/DELETE 建议 LIMIT' },
  { key: 'tautology_where', label: '检测 1=1 恒真条件' },
  { key: 'select_star', label: '禁止 SELECT *' },
  { key: 'select_limit', label: 'SELECT 建议 LIMIT' },
  { key: 'no_drop', label: '禁 DROP TABLE/DATABASE' },
  { key: 'no_truncate', label: '禁 TRUNCATE' },
  { key: 'rename_table', label: 'RENAME TABLE 告警' },
  { key: 'alter_drop', label: 'ALTER DROP 子句告警' },
  { key: 'create_engine', label: 'CREATE TABLE 需 ENGINE=' },
  { key: 'create_charset', label: 'CREATE TABLE 需字符集' },
  { key: 'create_primary_key', label: 'CREATE TABLE 需主键' },
  { key: 'no_lock_tables', label: '禁 LOCK/UNLOCK TABLES' },
  { key: 'no_set_global', label: '禁 SET GLOBAL' },
  { key: 'no_grant', label: '禁 GRANT/REVOKE' },
  { key: 'insert_columns', label: 'INSERT 需指定列名' }
]

const loading = ref(false)
const list = ref<SQLAuditRuleSet[]>([])
const editorOpen = ref(false)
const editing = ref<SQLAuditRuleSet | null>(null)
const submitting = ref(false)

const defaultConfig: AuditRuleConfig = {
  max_statement_per_ticket: 0,
  max_statement_bytes: 0
}

const form = reactive<RuleInput>({
  name: '',
  description: '',
  is_default: false,
  config: { ...defaultConfig }
})

async function load() {
  loading.value = true
  try {
    const res = await dbRuleApi.list()
    list.value = res.data || []
  } finally {
    loading.value = false
  }
}

function normalizeConfig(raw: unknown): AuditRuleConfig {
  let obj: Record<string, unknown> = {}
  if (raw == null) {
    return { ...defaultConfig }
  }
  if (typeof raw === 'string') {
    try {
      obj = JSON.parse(raw) as Record<string, unknown>
    } catch {
      return { ...defaultConfig }
    }
  } else if (typeof raw === 'object') {
    obj = { ...(raw as Record<string, unknown>) }
  }
  return { ...defaultConfig, ...obj } as AuditRuleConfig
}

async function openCreate() {
  const res = await dbRuleApi.defaultConfig()
  editing.value = null
  form.name = ''
  form.description = ''
  form.is_default = false
  form.config = normalizeConfig(res.data)
  editorOpen.value = true
}

function openEdit(r: SQLAuditRuleSet) {
  editing.value = r
  form.name = r.name
  form.description = r.description
  form.is_default = r.is_default
  form.config = normalizeConfig(r.config)
  editorOpen.value = true
}

async function submit() {
  if (!form.name) {
    message.warning('名称必填')
    return
  }
  submitting.value = true
  try {
    const res = editing.value
      ? await dbRuleApi.update(editing.value.id, form)
      : await dbRuleApi.create(form)
    if (res.code === 0) {
      message.success('保存成功')
      editorOpen.value = false
      load()
    } else {
      message.error(res.message || '保存失败')
    }
  } finally {
    submitting.value = false
  }
}

async function setDefault(id: number) {
  await dbRuleApi.setDefault(id)
  message.success('已设为默认')
  load()
}

async function del(id: number) {
  await dbRuleApi.delete(id)
  message.success('已删除')
  load()
}

onMounted(load)
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header h1 { margin: 0; font-size: 20px; }
</style>
