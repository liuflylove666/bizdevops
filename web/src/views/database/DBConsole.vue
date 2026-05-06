<template>
  <div class="db-console">
    <div class="page-header">
      <h1>SQL 控制台</h1>
    </div>

    <a-card :bordered="false">
      <a-row :gutter="[16, 16]">
        <a-col :xs="24" :lg="7">
          <div class="sidebar-pane">
            <div class="sider-head">
              <a-select
                v-model:value="currentInstance"
                placeholder="选择实例"
                style="width: 100%"
                :options="instanceOptions"
                @change="onInstanceChange"
                show-search
                :filter-option="filterOption"
              />
              <a-select
                v-model:value="currentSchema"
                placeholder="选择数据库"
                style="width: 100%; margin-top: 8px"
                :options="schemaOptions"
                @change="onSchemaChange"
                :disabled="!currentInstance"
              />
            </div>
            <div class="tree-wrap">
              <a-spin :spinning="treeLoading">
                <a-tree
                  v-if="tables.length"
                  :tree-data="tableTree"
                  :load-data="loadTableDetail"
                  show-icon
                  block-node
                >
                  <template #title="{ title, isTable, isColumn, column }">
                    <span v-if="isTable" class="tbl-title" @dblclick="insertSQL('SELECT * FROM ' + title + ' LIMIT 100;')">{{ title }}</span>
                    <span v-else-if="isColumn" class="col-title">
                      {{ column.name }}
                      <span class="col-type">{{ column.column_type }}</span>
                      <a-tag v-if="column.column_key === 'PRI'" color="red" size="small">PK</a-tag>
                    </span>
                    <span v-else>{{ title }}</span>
                  </template>
                </a-tree>
                <a-empty v-else-if="currentSchema" description="该库下没有表" />
              </a-spin>
            </div>
          </div>
        </a-col>

        <a-col :xs="24" :lg="17">
          <div class="editor-toolbar">
            <a-space>
              <a-button type="primary" :loading="executing" :disabled="!currentInstance" @click="execute">
                <template #icon><CaretRightOutlined /></template>
                执行
              </a-button>
              <a-input-number v-model:value="limit" :min="1" :max="10000" addon-before="LIMIT" style="width: 160px" />
              <span class="hint">仅支持 SELECT / SHOW / DESC / EXPLAIN</span>
            </a-space>
          </div>
          <a-textarea
            v-model:value="sqlText"
            :rows="10"
            placeholder="-- 输入只读 SQL"
            class="sql-editor"
            @keydown.ctrl.enter="execute"
            @keydown.meta.enter="execute"
          />

          <a-divider orientation="left" style="margin: 16px 0 12px">查询结果</a-divider>
          <div v-if="queryResult" class="result-meta-line">
            {{ queryResult.rows.length }} 行 · {{ queryResult.exec_ms }} ms
            <a-button size="small" class="export-btn" :disabled="!queryResult.rows.length" @click="exportCsv">导出 CSV</a-button>
          </div>
          <a-alert v-if="errorMsg" type="error" :message="errorMsg" banner closable style="margin-bottom: 12px" @close="errorMsg = ''" />
          <a-table
            v-if="queryResult"
            :columns="resultColumns"
            :data-source="queryResult.rows"
            :pagination="{ pageSize: 50 }"
            size="small"
            :scroll="{ x: 'max-content', y: 400 }"
            row-key="__idx"
          />
          <a-empty v-else-if="!errorMsg" description="尚未执行查询" />
        </a-col>
      </a-row>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { CaretRightOutlined } from '@ant-design/icons-vue'
import { dbInstanceApi, dbConsoleApi, type DBInstance, type ColumnInfo, type QueryResult } from '@/services/database'

type TreeNode = { key: string; title: string; isTable?: boolean; isColumn?: boolean; column?: ColumnInfo; children?: TreeNode[]; isLeaf?: boolean }

const currentInstance = ref<number>()
const currentSchema = ref<string>()
const instances = ref<DBInstance[]>([])
const schemas = ref<string[]>([])
const tables = ref<string[]>([])
const columnCache = ref<Record<string, ColumnInfo[]>>({})
const treeLoading = ref(false)

const sqlText = ref<string>('')
const limit = ref(1000)
const executing = ref(false)
const queryResult = ref<QueryResult | null>(null)
const errorMsg = ref<string>('')

const instanceOptions = computed(() => instances.value.map(i => ({ value: i.id, label: `${i.env}/${i.name} (${i.db_type})` })))
const schemaOptions = computed(() => schemas.value.map(s => ({ value: s, label: s })))

const tableTree = computed<TreeNode[]>(() =>
  tables.value.map(t => ({
    key: `tbl:${t}`,
    title: t,
    isTable: true,
    children: columnCache.value[t]?.map(c => ({
      key: `col:${t}:${c.name}`,
      title: c.name,
      isColumn: true,
      column: c,
      isLeaf: true
    }))
  }))
)

const resultColumns = computed(() => {
  if (!queryResult.value) return []
  return queryResult.value.columns.map(c => ({ title: c, dataIndex: c, key: c, ellipsis: true }))
})

function filterOption(input: string, option: any) {
  return (option.label || '').toLowerCase().includes(input.toLowerCase())
}

async function loadInstances() {
  const res = await dbInstanceApi.listAll()
  instances.value = res.data || []
}

async function onInstanceChange() {
  schemas.value = []
  currentSchema.value = undefined
  tables.value = []
  columnCache.value = {}
  if (!currentInstance.value) return
  try {
    const res = await dbInstanceApi.databases(currentInstance.value)
    schemas.value = res.data || []
  } catch (e: any) {
    message.error(e?.message || '加载数据库列表失败')
  }
}

async function onSchemaChange() {
  tables.value = []
  columnCache.value = {}
  if (!currentInstance.value || !currentSchema.value) return
  treeLoading.value = true
  try {
    const res = await dbInstanceApi.tables(currentInstance.value, currentSchema.value)
    tables.value = res.data || []
  } catch (e: any) {
    message.error(e?.message || '加载表列表失败')
  } finally {
    treeLoading.value = false
  }
}

async function loadTableDetail(node: any): Promise<void> {
  const name = node.dataRef?.title as string
  if (!node.dataRef?.isTable || columnCache.value[name]) return
  const res = await dbInstanceApi.columns(currentInstance.value!, currentSchema.value!, name)
  columnCache.value = { ...columnCache.value, [name]: res.data || [] }
}

function insertSQL(snippet: string) {
  sqlText.value = sqlText.value ? sqlText.value + '\n' + snippet : snippet
}

async function execute() {
  if (!currentInstance.value) {
    message.warning('请先选择实例')
    return
  }
  if (!sqlText.value.trim()) {
    message.warning('请输入 SQL')
    return
  }
  executing.value = true
  errorMsg.value = ''
  queryResult.value = null
  try {
    const res = await dbConsoleApi.execute({
      instance_id: currentInstance.value,
      schema: currentSchema.value,
      sql: sqlText.value,
      limit: limit.value
    })
    if (res.code === 0 && res.data) {
      res.data.rows = (res.data.rows || []).map((r: any, idx: number) => ({ ...r, __idx: idx }))
      queryResult.value = res.data
    } else {
      errorMsg.value = res.message || '执行失败'
    }
  } catch (e: any) {
    errorMsg.value = e?.message || '执行失败'
  } finally {
    executing.value = false
  }
}

function exportCsv() {
  if (!queryResult.value) return
  const { columns, rows } = queryResult.value
  const escape = (v: unknown) => {
    if (v == null) return ''
    const s = String(v).replace(/"/g, '""')
    return /[",\n]/.test(s) ? `"${s}"` : s
  }
  const csv = [columns.join(',')]
    .concat(rows.map(r => columns.map(c => escape((r as any)[c])).join(',')))
    .join('\n')
  const blob = new Blob(['\ufeff' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `query-${Date.now()}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(loadInstances)
</script>

<style scoped>
.page-header { margin-bottom: 16px; }
.page-header h1 { margin: 0; font-size: 20px; }
.sidebar-pane {
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  overflow: hidden;
}
.sider-head { padding: 12px; border-bottom: 1px solid #f0f0f0; background: #fff; }
.tree-wrap { padding: 8px 4px; max-height: calc(100vh - 280px); overflow: auto; background: #fff; }
.editor-toolbar { margin-bottom: 8px; }
.hint { color: #8c8c8c; font-size: 12px; }
.sql-editor { font-family: Menlo, Consolas, monospace; font-size: 13px; }
.result-meta-line { margin-bottom: 8px; color: #8c8c8c; font-size: 13px; display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px; }
.export-btn { margin-left: auto; }
.tbl-title { cursor: pointer; }
.col-type { color: #8c8c8c; margin-left: 6px; font-size: 12px; }
</style>
