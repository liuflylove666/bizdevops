<template>
  <div class="alert-center-v2">
    <a-page-header title="告警中心" sub-title="配置、模板、静默与升级规则（E4-02）">
      <template #extra>
        <a-space wrap>
          <a-button @click="router.push('/alert/overview')">运行保障工作台</a-button>
          <a-button type="primary" @click="router.push('/alert/history')">告警历史</a-button>
          <a-button @click="router.push('/alert/gateway')">接入指南</a-button>
        </a-space>
      </template>
    </a-page-header>

    <a-tabs v-model:activeKey="activeTab" type="card">
      <a-tab-pane key="config" tab="告警配置">
        <AlertConfigView />
      </a-tab-pane>
      <a-tab-pane key="templates" tab="消息模板">
        <MessageTemplateView />
      </a-tab-pane>
      <a-tab-pane key="silence" tab="静默规则">
        <AlertSilenceView />
      </a-tab-pane>
      <a-tab-pane key="escalation" tab="升级规则">
        <AlertEscalationView />
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, defineAsyncComponent, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const AlertConfigView = defineAsyncComponent(() => import('./AlertConfig.vue'))
const MessageTemplateView = defineAsyncComponent(() => import('./MessageTemplate.vue'))
const AlertSilenceView = defineAsyncComponent(() => import('./AlertSilence.vue'))
const AlertEscalationView = defineAsyncComponent(() => import('./AlertEscalation.vue'))

const route = useRoute()
const router = useRouter()

const allowed = new Set(['config', 'templates', 'silence', 'escalation'])
const activeTab = ref('config')

const syncFromRoute = () => {
  const q = route.query.tab
  if (typeof q === 'string' && allowed.has(q) && q !== activeTab.value) {
    activeTab.value = q
  }
}

watch(
  () => route.query.tab,
  () => {
    syncFromRoute()
  },
  { immediate: true },
)

watch(activeTab, (k) => {
  if (route.query.tab === k) return
  void router.replace({ path: '/alert/center', query: { tab: k } })
})

onMounted(() => {
  if (route.query.tab == null || route.query.tab === '') {
    void router.replace({ path: '/alert/center', query: { tab: activeTab.value } })
  }
})
</script>

<style scoped>
.alert-center-v2 {
  padding: 0 8px 24px;
}
</style>
