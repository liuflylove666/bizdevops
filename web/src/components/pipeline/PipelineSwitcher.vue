<!--
  PipelineSwitcher (Sprint 1 FE-01).

  Top-bar dropdown for fast pipeline switching: pinned + recent visits +
  fuzzy-search across all pipelines. Sits next to GlobalSearch in the main
  layout header; intentionally narrower in scope (only pipelines) since
  GlobalSearch covers all entities.

  No global keyboard shortcut here — Sprint 3 FE-06 (`/` command palette)
  will own keyboard navigation. V1 is click-only.
-->
<template>
  <a-dropdown :trigger="['click']" v-model:open="open" placement="bottomRight" overlay-class-name="ps-dropdown-overlay">
    <a-tooltip title="切换流水线">
      <a-button type="text" class="ps-trigger">
        <template #icon><RocketOutlined /></template>
        <span class="ps-trigger-label">流水线</span>
      </a-button>
    </a-tooltip>

    <template #overlay>
      <div class="ps-panel" @click.stop>
        <div class="ps-search">
          <SearchOutlined class="ps-search-icon" />
          <input
            ref="searchInputEl"
            v-model="keyword"
            type="text"
            class="ps-search-input"
            placeholder="搜索流水线名称…"
            @keydown="handleKeydown"
          />
          <span v-if="keyword" class="ps-search-clear" @click="keyword = ''">×</span>
        </div>

        <div class="ps-body">
          <!-- 搜索状态 -->
          <template v-if="keyword">
            <div v-if="loading" class="ps-loading">
              <a-spin size="small" /> <span>搜索中…</span>
            </div>
            <div v-else-if="!searchResults.length" class="ps-empty">
              未找到匹配的流水线
            </div>
            <SwitcherSection
              v-else
              label="搜索结果"
              :items="searchResults"
              :pinned-ids="pinnedIds"
              @pick="onPick"
              @toggle-pin="onTogglePin"
            />
          </template>

          <!-- 默认状态：置顶 + 最近 -->
          <template v-else>
            <SwitcherSection
              v-if="recentsStore.pinned.length"
              label="置顶"
              :items="recentsStore.pinned"
              :pinned-ids="pinnedIds"
              @pick="onPick"
              @toggle-pin="onTogglePin"
            />
            <SwitcherSection
              v-if="recentsStore.recents.length"
              label="最近"
              :items="recentsStore.recents"
              :pinned-ids="pinnedIds"
              show-clear
              @pick="onPick"
              @toggle-pin="onTogglePin"
              @clear="recentsStore.clearRecents()"
            />
            <div
              v-if="!recentsStore.pinned.length && !recentsStore.recents.length"
              class="ps-empty"
            >
              输入流水线名称开始搜索
            </div>
          </template>
        </div>

        <div class="ps-footer">
          <a-button type="link" size="small" @click="goToList">查看全部 →</a-button>
        </div>
      </div>
    </template>
  </a-dropdown>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { RocketOutlined, SearchOutlined, PushpinOutlined, PushpinFilled } from '@ant-design/icons-vue'
import { pipelineApi } from '@/services/pipeline'
import { useRecentsStore, type PipelineRef } from '@/stores/recents'

const router = useRouter()
const recentsStore = useRecentsStore()

const open = ref(false)
const keyword = ref('')
const loading = ref(false)
const searchResults = ref<PipelineRef[]>([])
const searchInputEl = ref<HTMLInputElement | null>(null)

const pinnedIds = computed(() => new Set(recentsStore.pinned.map((p) => p.id)))

// Auto-focus the search input when the dropdown opens. nextTick lets the
// element actually mount; without it the ref is still null.
watch(open, async (v) => {
  if (v) {
    await nextTick()
    searchInputEl.value?.focus()
  } else {
    keyword.value = ''
    searchResults.value = []
  }
})

// Debounced fuzzy search. Backend `pipelineApi.list({name})` already does
// fuzzy match on the server.
let searchTimer: number | null = null
watch(keyword, (q) => {
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
  if (!q.trim()) {
    searchResults.value = []
    loading.value = false
    return
  }
  loading.value = true
  searchTimer = window.setTimeout(async () => {
    try {
      const res: any = await pipelineApi.list({ name: q.trim(), page: 1, page_size: 10 })
      const items = (res?.data?.items || []) as Array<{ id: number; name: string; description?: string }>
      searchResults.value = items.map((p) => ({ id: p.id, name: p.name, description: p.description }))
    } catch {
      searchResults.value = []
    } finally {
      loading.value = false
    }
  }, 200)
})

const onPick = (p: PipelineRef) => {
  recentsStore.addRecent(p)
  open.value = false
  router.push({ name: 'PipelineDetail', params: { id: p.id } })
}

const onTogglePin = (p: PipelineRef) => {
  recentsStore.togglePin(p)
}

const goToList = () => {
  open.value = false
  router.push({ name: 'PipelineList' })
}

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape') {
    open.value = false
  } else if (e.key === 'Enter' && searchResults.value.length) {
    onPick(searchResults.value[0])
  }
}

// Internal section renderer. Inline component keeps the parent template
// readable without spinning up a separate file for ~30 lines of markup.
const SwitcherSection = defineComponent({
  name: 'SwitcherSection',
  props: {
    label: { type: String, required: true },
    items: { type: Array as () => PipelineRef[], required: true },
    pinnedIds: { type: Object as () => Set<number>, required: true },
    showClear: { type: Boolean, default: false },
  },
  emits: ['pick', 'toggle-pin', 'clear'],
  setup(props, { emit }) {
    return () =>
      h('div', { class: 'ps-section' }, [
        h('div', { class: 'ps-section-head' }, [
          h('span', { class: 'ps-section-label' }, props.label),
          props.showClear
            ? h(
                'a',
                { class: 'ps-section-clear', onClick: () => emit('clear') },
                '清空',
              )
            : null,
        ]),
        ...props.items.map((it) =>
          h(
            'div',
            { key: it.id, class: 'ps-item', onClick: () => emit('pick', it) },
            [
              h('div', { class: 'ps-item-main' }, [
                h('div', { class: 'ps-item-name' }, it.name),
                it.description
                  ? h('div', { class: 'ps-item-desc' }, it.description)
                  : null,
              ]),
              h(
                'span',
                {
                  class: 'ps-item-pin',
                  onClick: (e: MouseEvent) => {
                    e.stopPropagation()
                    emit('toggle-pin', it)
                  },
                  title: props.pinnedIds.has(it.id) ? '取消置顶' : '置顶',
                },
                [h(props.pinnedIds.has(it.id) ? PushpinFilled : PushpinOutlined)],
              ),
            ],
          ),
        ),
      ])
  },
})
</script>

<style scoped>
.ps-trigger {
  padding: 4px 10px;
}
.ps-trigger-label {
  margin-left: 4px;
  font-size: 13px;
}
.ps-panel {
  width: 360px;
  background: #fff;
  border-radius: 6px;
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.12);
  overflow: hidden;
}
.ps-search {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid #f0f0f0;
}
.ps-search-icon {
  color: #999;
  margin-right: 8px;
}
.ps-search-input {
  flex: 1;
  border: none;
  outline: none;
  font-size: 14px;
  background: transparent;
}
.ps-search-clear {
  cursor: pointer;
  color: #999;
  padding: 0 6px;
}
.ps-search-clear:hover {
  color: #555;
}
.ps-body {
  max-height: 360px;
  overflow-y: auto;
}
.ps-loading,
.ps-empty {
  padding: 20px;
  text-align: center;
  color: #999;
  font-size: 13px;
}
.ps-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
:deep(.ps-section) {
  padding: 6px 0;
}
:deep(.ps-section-head) {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 12px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: #888;
}
:deep(.ps-section-label) {
  font-weight: 500;
}
:deep(.ps-section-clear) {
  font-size: 11px;
  color: #1677ff;
  cursor: pointer;
}
:deep(.ps-item) {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  cursor: pointer;
}
:deep(.ps-item:hover) {
  background: #f5f5f5;
}
:deep(.ps-item-main) {
  flex: 1;
  min-width: 0;
}
:deep(.ps-item-name) {
  font-size: 14px;
  color: #222;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:deep(.ps-item-desc) {
  font-size: 12px;
  color: #888;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
}
:deep(.ps-item-pin) {
  margin-left: 8px;
  padding: 4px;
  color: #aaa;
}
:deep(.ps-item-pin:hover) {
  color: #fa8c16;
}
.ps-footer {
  border-top: 1px solid #f0f0f0;
  padding: 4px 8px;
  text-align: right;
}
</style>
