<!--
  YamlExportModal (Sprint 2 FE-11).

  Pipeline 详情页头按钮触发：调用 GET /pipeline/:id/yaml，渲染 IR → YAML
  文本到 <pre> 框，提供复制 + "包含画布坐标"（include_layout）切换。

  Endpoint 返回的是 raw YAML（Content-Type: text/yaml），不走 {code,
  message, data} 信封；axios 拦截器将原始字符串透传。
-->
<template>
  <a-modal
    :open="open"
    :title="`导出 YAML · ${pipelineName || '#' + pipelineId}`"
    width="780px"
    :footer="null"
    @cancel="emit('update:open', false)"
  >
    <div class="yaml-export-toolbar">
      <a-checkbox v-model:checked="includeLayout" @change="reload">
        包含画布坐标 <span class="hint">（__layout，仅设计器还原需要）</span>
      </a-checkbox>
      <a-space>
        <a-button size="small" :disabled="!yamlText" @click="copy">
          <template #icon><CopyOutlined /></template>
          复制
        </a-button>
        <a-button size="small" :disabled="!yamlText" @click="download">
          <template #icon><DownloadOutlined /></template>
          下载
        </a-button>
      </a-space>
    </div>

    <a-spin :spinning="loading">
      <a-alert
        v-if="errorMessage"
        type="error"
        show-icon
        :message="errorMessage"
        style="margin-bottom: 8px"
      />
      <pre v-if="yamlText" class="yaml-pane">{{ yamlText }}</pre>
      <a-empty v-else-if="!loading && !errorMessage" description="无内容" />
    </a-spin>

    <div v-if="yamlText" class="yaml-meta">
      共 {{ lineCount }} 行 · {{ byteCount }} 字节
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons-vue'
import { pipelineApi } from '@/services/pipeline'

interface Props {
  open: boolean
  pipelineId: number | null
  pipelineName?: string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:open', v: boolean): void
}>()

const includeLayout = ref(false)
const loading = ref(false)
const errorMessage = ref('')
const yamlText = ref('')

const lineCount = computed(() => yamlText.value ? yamlText.value.split('\n').length : 0)
const byteCount = computed(() => new Blob([yamlText.value]).size)

const reload = async () => {
  if (!props.pipelineId) return
  loading.value = true
  errorMessage.value = ''
  yamlText.value = ''
  try {
    const text: any = await pipelineApi.exportYAML(props.pipelineId, includeLayout.value)
    // pkgresponse Error path returns JSON envelope despite responseType=text;
    // detect & surface readable message instead of dumping JSON to <pre>.
    if (typeof text === 'string' && text.startsWith('{') && text.includes('"code"')) {
      try {
        const parsed = JSON.parse(text)
        if (parsed.code !== 0) {
          errorMessage.value = parsed.message || '导出失败'
          return
        }
      } catch {
        /* fallthrough — treat as YAML */
      }
    }
    yamlText.value = typeof text === 'string' ? text : String(text)
  } catch (e: any) {
    errorMessage.value = e?.message || '导出失败'
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.open, props.pipelineId],
  ([isOpen]) => {
    if (isOpen && props.pipelineId) {
      reload()
    } else if (!isOpen) {
      // 关闭时清空，避免下次打开闪现旧内容
      yamlText.value = ''
      errorMessage.value = ''
    }
  },
  { immediate: true },
)

const copy = async () => {
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(yamlText.value)
    } else {
      const ta = document.createElement('textarea')
      ta.value = yamlText.value
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    message.success('已复制到剪贴板')
  } catch {
    message.error('复制失败，请手动选中复制')
  }
}

const download = () => {
  const blob = new Blob([yamlText.value], { type: 'text/yaml; charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${(props.pipelineName || 'pipeline').replace(/[^\w.-]+/g, '_')}.yaml`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.yaml-export-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.hint {
  color: #999;
  font-size: 12px;
}
.yaml-pane {
  background: #1f1f1f;
  color: #e8e8e8;
  padding: 12px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 1.5;
  max-height: 480px;
  overflow: auto;
  white-space: pre;
  margin: 0;
}
.yaml-meta {
  margin-top: 8px;
  font-size: 12px;
  color: #888;
  text-align: right;
}
</style>
