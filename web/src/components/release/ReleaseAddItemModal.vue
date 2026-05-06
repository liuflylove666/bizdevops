<template>
  <a-modal
    v-model:open="visible"
    title="新增变更项"
    :confirm-loading="submitting"
    width="720px"
    @ok="onSubmit"
    @cancel="close"
  >
    <a-tabs v-model:activeKey="activeTab" size="small">
      <!-- 流水线 -->
      <a-tab-pane key="pipeline_run" tab="关联流水线运行">
        <a-input-search
          v-model:value="pipelineKeyword"
          placeholder="按流水线名 / 分支 / 构建号过滤"
          style="margin-bottom: 8px"
          @search="fetchPipelineRuns"
        />
        <a-table
          :columns="pipelineCols"
          :data-source="pipelineRuns"
          :pagination="{ pageSize: 10, total: pipelineTotal, current: pipelinePage }"
          :row-selection="{
            type: 'radio',
            selectedRowKeys: pipelineSelected,
            onChange: (keys: number[]) => (pipelineSelected = keys),
          }"
          :loading="pipelineLoading"
          size="small"
          row-key="id"
          @change="onPipelineTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="runStatusColor(record.status)">{{ record.status }}</a-tag>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <!-- Nacos 发布 -->
      <a-tab-pane key="nacos_release" tab="关联 Nacos 发布">
        <a-input-search
          v-model:value="nacosKeyword"
          placeholder="按 dataId 过滤"
          style="margin-bottom: 8px"
          @search="fetchNacos"
        />
        <a-table
          :columns="nacosCols"
          :data-source="nacosList"
          :pagination="{ pageSize: 10, total: nacosTotal, current: nacosPage }"
          :row-selection="{
            type: 'radio',
            selectedRowKeys: nacosSelected,
            onChange: (keys: number[]) => (nacosSelected = keys),
          }"
          :loading="nacosLoading"
          size="small"
          row-key="id"
          @change="onNacosTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag>{{ record.status }}</a-tag>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <!-- 手工 -->
      <a-tab-pane key="manual" tab="手工登记">
        <a-form layout="vertical">
          <a-form-item label="标题" required>
            <a-input v-model:value="manualForm.item_title" placeholder="例：DBA-SQL-2026-0421-001" />
          </a-form-item>
          <a-form-item label="外部 ID">
            <a-input-number v-model:value="manualForm.item_id" :min="0" style="width: 100%" placeholder="关联外部系统 ID（可选）" />
          </a-form-item>
          <a-form-item label="说明">
            <a-textarea v-model:value="manualForm.note" :rows="3" placeholder="变更描述" />
          </a-form-item>
        </a-form>
      </a-tab-pane>
    </a-tabs>
  </a-modal>
</template>

<script setup lang="ts">
/**
 * Release 新增变更项弹窗（v2.1）。
 *
 * 支持三种来源：
 *   - pipeline_run：从最近流水线运行中挑选一条
 *   - nacos_release：从 Nacos 发布单中挑选一条
 *   - manual：人工登记（如 DBA 手工变更、外部 SaaS 变更）
 */
import { ref, reactive, computed, watch } from 'vue'
import { message } from 'ant-design-vue'
import { releaseApi } from '@/services/release'
import { pipelineApi } from '@/services/pipeline'
import { nacosReleaseApi } from '@/services/nacosRelease'

const props = defineProps<{
  open: boolean
  releaseId: number
  defaultAppId?: number
  defaultEnv?: string
}>()

const emit = defineEmits<{
  (e: 'update:open', v: boolean): void
  (e: 'added'): void
}>()

const visible = computed({
  get: () => props.open,
  set: (v) => emit('update:open', v),
})

const activeTab = ref<'pipeline_run' | 'nacos_release' | 'manual'>('pipeline_run')
const submitting = ref(false)

// ---------- Pipeline ----------
const pipelineKeyword = ref('')
const pipelineRuns = ref<any[]>([])
const pipelineLoading = ref(false)
const pipelineSelected = ref<number[]>([])
const pipelinePage = ref(1)
const pipelineTotal = ref(0)
const pipelineCols = [
  { title: '运行 ID', dataIndex: 'id', key: 'id', width: 80 },
  { title: '流水线', dataIndex: 'pipeline_name', key: 'pipeline_name' },
  { title: '分支', dataIndex: 'branch', key: 'branch', width: 120 },
  { title: '构建号', dataIndex: 'build_number', key: 'build_number', width: 90 },
  { title: '状态', key: 'status', width: 100 },
]

async function fetchPipelineRuns(page = 1) {
  pipelineLoading.value = true
  try {
    const res = await pipelineApi.listRuns({
      page,
      page_size: 10,
      status: 'success',
    })
    const data: any = (res as any)?.data || {}
    const list = (data.data || data.list || []) as any[]
    pipelineRuns.value = pipelineKeyword.value
      ? list.filter(
          (r) =>
            (r.pipeline_name || '').includes(pipelineKeyword.value) ||
            (r.branch || '').includes(pipelineKeyword.value) ||
            String(r.build_number || '').includes(pipelineKeyword.value),
        )
      : list
    pipelineTotal.value = data.total || list.length
    pipelinePage.value = page
  } catch (e) {
    pipelineRuns.value = []
  } finally {
    pipelineLoading.value = false
  }
}
function onPipelineTableChange(pag: any) {
  fetchPipelineRuns(pag.current)
}
function runStatusColor(s: string) {
  return s === 'success' ? 'green' : s === 'failed' ? 'red' : 'blue'
}

// ---------- Nacos ----------
const nacosKeyword = ref('')
const nacosList = ref<any[]>([])
const nacosLoading = ref(false)
const nacosSelected = ref<number[]>([])
const nacosPage = ref(1)
const nacosTotal = ref(0)
const nacosCols = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
  { title: '标题', dataIndex: 'title', key: 'title' },
  { title: 'dataId', dataIndex: 'data_id', key: 'data_id' },
  { title: '环境', dataIndex: 'env', key: 'env', width: 90 },
  { title: '状态', key: 'status', width: 110 },
]

async function fetchNacos(page = 1) {
  nacosLoading.value = true
  try {
    const res = await nacosReleaseApi.list({
      env: props.defaultEnv,
      data_id: nacosKeyword.value || undefined,
      page,
      page_size: 10,
    })
    const data: any = (res as any)?.data || {}
    nacosList.value = (data.data || data.list || []) as any[]
    nacosTotal.value = data.total || nacosList.value.length
    nacosPage.value = page
  } catch (e) {
    nacosList.value = []
  } finally {
    nacosLoading.value = false
  }
}
function onNacosTableChange(pag: any) {
  fetchNacos(pag.current)
}

// ---------- Manual ----------
const manualForm = reactive({
  item_title: '',
  item_id: 0,
  note: '',
})

// ---------- Submit ----------
async function onSubmit() {
  if (activeTab.value === 'pipeline_run') {
    if (pipelineSelected.value.length === 0) {
      message.warning('请选择一条流水线运行')
      return
    }
    const id = pipelineSelected.value[0]
    const row = pipelineRuns.value.find((r) => r.id === id)
    await doAdd('pipeline_run', id, `${row?.pipeline_name || ''}#${row?.build_number || ''}`)
    return
  }
  if (activeTab.value === 'nacos_release') {
    if (nacosSelected.value.length === 0) {
      message.warning('请选择一条 Nacos 发布')
      return
    }
    const id = nacosSelected.value[0]
    const row = nacosList.value.find((r) => r.id === id)
    await doAdd('nacos_release', id, row?.title || row?.data_id || '')
    return
  }
  if (activeTab.value === 'manual') {
    if (!manualForm.item_title.trim()) {
      message.warning('请填写标题')
      return
    }
    await doAdd('manual', Number(manualForm.item_id) || 0, manualForm.item_title.trim())
  }
}

async function doAdd(type: string, itemId: number, title: string) {
  submitting.value = true
  try {
    await releaseApi.addItem(props.releaseId, {
      item_type: type,
      item_id: itemId,
      item_title: title,
    })
    message.success('已添加')
    emit('added')
    close()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '添加失败')
  } finally {
    submitting.value = false
  }
}

function close() {
  visible.value = false
  pipelineSelected.value = []
  nacosSelected.value = []
  manualForm.item_title = ''
  manualForm.item_id = 0
  manualForm.note = ''
}

// 首次打开时按当前 Tab 拉列表
watch(
  () => props.open,
  (v) => {
    if (!v) return
    if (activeTab.value === 'pipeline_run' && pipelineRuns.value.length === 0) fetchPipelineRuns(1)
    if (activeTab.value === 'nacos_release' && nacosList.value.length === 0) fetchNacos(1)
  },
)
watch(activeTab, (t) => {
  if (!props.open) return
  if (t === 'pipeline_run' && pipelineRuns.value.length === 0) fetchPipelineRuns(1)
  if (t === 'nacos_release' && nacosList.value.length === 0) fetchNacos(1)
})
</script>
