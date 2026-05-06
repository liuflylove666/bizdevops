<template>
  <div class="logs-unified-console">
    <a-page-header title="日志中心" sub-title="实时查看、检索、导出、统计、对比、书签与日志侧告警（E4-01）" />
    <div class="global-filter-bar">
      <a-select
        v-model:value="globalClusterId"
        placeholder="全局集群"
        style="width: 220px"
        allow-clear
        @change="onGlobalClusterChange"
      >
        <a-select-option v-for="cluster in clusters" :key="cluster.id" :value="cluster.id">
          {{ cluster.name }}
        </a-select-option>
      </a-select>
      <a-select
        v-model:value="globalNamespace"
        placeholder="全局命名空间"
        style="width: 220px"
        allow-clear
        :disabled="!globalClusterId"
        @change="onGlobalNamespaceChange"
      >
        <a-select-option v-for="ns in namespaces" :key="ns" :value="ns">
          {{ ns }}
        </a-select-option>
      </a-select>
      <a-button @click="resetGlobalFilters">清空全局过滤</a-button>
    </div>

    <a-tabs v-model:activeKey="activeTab" type="card">
      <a-tab-pane key="center" tab="实时查看">
        <LogCenterView />
      </a-tab-pane>
      <a-tab-pane key="search" tab="检索">
        <LogSearchView />
      </a-tab-pane>
      <a-tab-pane key="export" tab="导出">
        <LogExportView />
      </a-tab-pane>
      <a-tab-pane key="stats" tab="统计">
        <LogStatsView />
      </a-tab-pane>
      <a-tab-pane key="compare" tab="对比">
        <LogCompareView />
      </a-tab-pane>
      <a-tab-pane key="bookmarks" tab="书签">
        <LogBookmarksView />
      </a-tab-pane>
      <a-tab-pane key="alerts" tab="日志告警">
        <LogAlertsView />
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, defineAsyncComponent, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { k8sApi } from '@/services/k8s'

const LogCenterView = defineAsyncComponent(() => import('./LogCenter.vue'))
const LogSearchView = defineAsyncComponent(() => import('./LogSearch.vue'))
const LogExportView = defineAsyncComponent(() => import('./LogExportPage.vue'))
const LogStatsView = defineAsyncComponent(() => import('./LogStats.vue'))
const LogCompareView = defineAsyncComponent(() => import('./LogCompare.vue'))
const LogBookmarksView = defineAsyncComponent(() => import('./LogBookmarks.vue'))
const LogAlertsView = defineAsyncComponent(() => import('./LogAlertConfig.vue'))

const route = useRoute()
const router = useRouter()

const allowed = new Set(['center', 'search', 'export', 'stats', 'compare', 'bookmarks', 'alerts'])
const activeTab = ref('center')
const clusters = ref<Array<{ id: number; name: string }>>([])
const namespaces = ref<string[]>([])
const globalClusterId = ref<number | null>(null)
const globalNamespace = ref('')

function parseQueryClusterId(): number | null {
  const q = route.query.cluster_id
  if (typeof q !== 'string') return null
  const v = Number(q)
  return Number.isFinite(v) && v > 0 ? v : null
}

async function loadClusters() {
  try {
    const res = await k8sApi.getClusters()
    clusters.value = res.data || []
  } catch (e) {
    console.warn('[logs-unified] load clusters failed', e)
  }
}

async function loadNamespaces() {
  if (!globalClusterId.value) {
    namespaces.value = []
    return
  }
  try {
    const res = await k8sApi.getNamespaces(globalClusterId.value)
    namespaces.value = res.data || []
  } catch (e) {
    namespaces.value = []
    console.warn('[logs-unified] load namespaces failed', e)
  }
}

async function applyFiltersFromQuery() {
  globalClusterId.value = parseQueryClusterId()
  globalNamespace.value = typeof route.query.namespace === 'string' ? route.query.namespace : ''
  await loadNamespaces()
  if (globalNamespace.value && !namespaces.value.includes(globalNamespace.value)) {
    globalNamespace.value = ''
  }
}

function replaceQuery(patch: Record<string, string | undefined>) {
  const query = { ...route.query } as Record<string, any>
  Object.entries(patch).forEach(([k, v]) => {
    if (!v) delete query[k]
    else query[k] = v
  })
  void router.replace({ path: '/logs/unified', query })
}

async function onGlobalClusterChange() {
  if (!globalClusterId.value) {
    globalNamespace.value = ''
    namespaces.value = []
    replaceQuery({ cluster_id: undefined, namespace: undefined })
    return
  }
  await loadNamespaces()
  if (globalNamespace.value && !namespaces.value.includes(globalNamespace.value)) {
    globalNamespace.value = ''
  }
  replaceQuery({
    cluster_id: String(globalClusterId.value),
    namespace: globalNamespace.value || undefined,
  })
}

function onGlobalNamespaceChange() {
  replaceQuery({
    cluster_id: globalClusterId.value ? String(globalClusterId.value) : undefined,
    namespace: globalNamespace.value || undefined,
  })
}

function resetGlobalFilters() {
  globalClusterId.value = null
  globalNamespace.value = ''
  namespaces.value = []
  replaceQuery({ cluster_id: undefined, namespace: undefined })
}

watch(
  () => route.query.tab,
  () => {
    const q = route.query.tab
    if (typeof q === 'string' && allowed.has(q) && q !== activeTab.value) {
      activeTab.value = q
    }
  },
  { immediate: true },
)

watch(activeTab, (k) => {
  if (route.path !== '/logs/unified') return
  if (route.query.tab === k) return
  void router.replace({ path: '/logs/unified', query: { ...route.query, tab: k } })
})

onMounted(() => {
  void loadClusters().then(() => applyFiltersFromQuery())
  if (route.path !== '/logs/unified') return
  const q = route.query.tab
  if (typeof q !== 'string' || !allowed.has(q)) {
    void router.replace({ path: '/logs/unified', query: { ...route.query, tab: activeTab.value } })
  }
})

watch(
  () => [route.query.cluster_id, route.query.namespace],
  () => {
    void applyFiltersFromQuery()
  },
)
</script>

<style scoped>
.logs-unified-console {
  padding: 0 8px 24px;
}

.global-filter-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 12px 0;
}
</style>
