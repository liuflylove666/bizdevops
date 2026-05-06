<template>
  <div class="change-timeline">
    <!-- 统计卡片 -->
    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="4" v-for="stat in statsWithAll" :key="stat.event_type">
        <a-card size="small" :bordered="false" hoverable
          :class="{ 'stat-active': filter.event_type === stat.event_type }"
          @click="filterByType(stat.event_type)">
          <a-statistic :title="eventTypeText(stat.event_type)" :value="stat.count"
            :value-style="{ color: eventTypeColor(stat.event_type), fontSize: '20px' }" />
        </a-card>
      </a-col>
    </a-row>

    <a-card :bordered="false">
      <template #title>
        <a-space>
          <span>变更时间线</span>
          <a-tag color="blue">{{ total }} 条</a-tag>
        </a-space>
      </template>
      <template #extra>
        <a-space>
          <a-select v-model:value="filter.env" placeholder="环境" allow-clear style="width: 110px" @change="loadList">
            <a-select-option value="dev">dev</a-select-option>
            <a-select-option value="test">test</a-select-option>
            <a-select-option value="uat">uat</a-select-option>
            <a-select-option value="gray">gray</a-select-option>
            <a-select-option value="prod">prod</a-select-option>
          </a-select>
          <a-input-search v-model:value="filter.operator" placeholder="操作人" style="width: 150px" @search="loadList" allow-clear />
          <a-range-picker v-model:value="dateRange" style="width: 260px" @change="handleDateChange" />
        </a-space>
      </template>

      <!-- 时间线视图 -->
      <a-timeline mode="left" v-if="list.length > 0">
        <a-timeline-item v-for="event in list" :key="event.id" :color="timelineColor(event.event_type)">
          <template #dot>
            <div class="timeline-dot" :style="{ background: timelineColor(event.event_type) }">
              {{ eventTypeIcon(event.event_type) }}
            </div>
          </template>
          <a-card size="small" :bordered="true" style="margin-bottom: 4px">
            <div style="display: flex; justify-content: space-between; align-items: flex-start">
              <div>
                <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px">
                  <a-tag :color="eventTypeBadgeColor(event.event_type)" size="small">{{ eventTypeText(event.event_type) }}</a-tag>
                  <span style="font-weight: 600">{{ event.title }}</span>
                  <a-tag v-if="event.env" :color="envColor(event.env)" size="small">{{ event.env }}</a-tag>
                  <a-tag v-if="event.status" size="small">{{ event.status }}</a-tag>
                  <a-tag v-if="event.risk_level && event.risk_level !== 'low'" :color="riskColor(event.risk_level)" size="small">
                    {{ riskText(event.risk_level) }}
                  </a-tag>
                </div>
                <div v-if="event.description" style="color: #666; font-size: 13px; margin-bottom: 4px">{{ event.description }}</div>
                <div style="color: #999; font-size: 12px">
                  <span v-if="event.application_name">{{ event.application_name }} · </span>
                  <span>{{ event.operator }}</span>
                  <span> · #{{ event.event_id }}</span>
                </div>
              </div>
              <div style="color: #999; font-size: 12px; white-space: nowrap; margin-left: 16px">
                {{ formatTime(event.created_at) }}
              </div>
            </div>
          </a-card>
        </a-timeline-item>
      </a-timeline>

      <a-empty v-else description="暂无变更事件" />

      <div style="text-align: center; margin-top: 16px">
        <a-pagination v-model:current="filter.page" v-model:pageSize="filter.page_size"
          :total="total" show-size-changer :show-total="(t: number) => `共 ${t} 条`"
          @change="loadList" @showSizeChange="loadList" />
      </div>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { message } from 'ant-design-vue'
import { changeEventApi } from '@/services/changeEvent'
import type { ChangeEvent, EventTypeStat } from '@/services/changeEvent'
import type { Dayjs } from 'dayjs'

const loading = ref(false)
const list = ref<ChangeEvent[]>([])
const total = ref(0)
const stats = ref<EventTypeStat[]>([])
const dateRange = ref<[Dayjs, Dayjs] | null>(null)

const filter = reactive({
  event_type: undefined as string | undefined,
  env: undefined as string | undefined,
  operator: undefined as string | undefined,
  start_time: undefined as string | undefined,
  end_time: undefined as string | undefined,
  page: 1,
  page_size: 20,
})

const statsWithAll = computed(() => {
  const totalCount = stats.value.reduce((sum, s) => sum + s.count, 0)
  return [{ event_type: '', count: totalCount }, ...stats.value]
})

const eventTypeText = (t: string) => {
  const map: Record<string, string> = {
    '': '全部', deploy: '部署', nacos_release: 'Nacos 配置',
    sql_ticket: 'SQL 工单', pipeline_run: '流水线', promotion: '环境晋级', release: '统一发布',
  }
  return map[t] || t
}
const eventTypeColor = (t: string) => {
  const map: Record<string, string> = {
    '': '#1890ff', deploy: '#52c41a', nacos_release: '#722ed1',
    sql_ticket: '#fa8c16', pipeline_run: '#1890ff', promotion: '#13c2c2', release: '#eb2f96',
  }
  return map[t] || '#666'
}
const eventTypeBadgeColor = (t: string) => {
  const map: Record<string, string> = {
    deploy: 'green', nacos_release: 'purple', sql_ticket: 'orange',
    pipeline_run: 'blue', promotion: 'cyan', release: 'magenta',
  }
  return map[t] || 'default'
}
const eventTypeIcon = (t: string) => {
  const map: Record<string, string> = {
    deploy: '🚀', nacos_release: '⚙', sql_ticket: '🗄', pipeline_run: '▶', promotion: '⬆', release: '📦',
  }
  return map[t] || '•'
}
const timelineColor = (t: string) => eventTypeColor(t)
const envColor = (e: string) => ({ dev: 'blue', test: 'green', uat: 'orange', gray: 'purple', prod: 'red' }[e] || 'default')
const riskColor = (r: string) => ({ low: 'green', medium: 'orange', high: 'red' }[r] || 'default')
const riskText = (r: string) => ({ low: '低', medium: '中', high: '高' }[r] || r)
const formatTime = (t: string) => t ? new Date(t).toLocaleString('zh-CN') : '-'

function filterByType(type: string) {
  filter.event_type = type || undefined
  filter.page = 1
  loadList()
}

function handleDateChange() {
  if (dateRange.value && dateRange.value.length === 2) {
    filter.start_time = dateRange.value[0].format('YYYY-MM-DD 00:00:00')
    filter.end_time = dateRange.value[1].format('YYYY-MM-DD 23:59:59')
  } else {
    filter.start_time = undefined
    filter.end_time = undefined
  }
  loadList()
}

async function loadList() {
  loading.value = true
  try {
    const res = await changeEventApi.list(filter)
    list.value = res.list || res.items || []
    total.value = res.total || 0
  } catch {
    message.error('加载变更事件失败')
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const res = await changeEventApi.stats()
    stats.value = res || []
  } catch { /* ignore */ }
}

onMounted(() => {
  loadList()
  loadStats()
})
</script>

<style scoped>
.change-timeline {
  padding: 0;
}
.stat-active {
  border: 2px solid #1890ff;
}
.timeline-dot {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: white;
}
</style>
