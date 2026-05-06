<template>
  <div class="oncall-management">
    <a-tabs v-model:activeKey="activeTab">
      <!-- 排班表 -->
      <a-tab-pane key="schedules" tab="排班表">
        <div style="margin-bottom: 16px; display: flex; justify-content: space-between">
          <a-space>
            <a-button type="primary" @click="showScheduleModal()">新建排班表</a-button>
          </a-space>
        </div>
        <a-row :gutter="16">
          <a-col :span="8" v-for="sch in schedules" :key="sch.id">
            <a-card hoverable style="margin-bottom: 16px" @click="selectSchedule(sch)">
              <template #title>
                <div style="display: flex; align-items: center; justify-content: space-between">
                  <span>{{ sch.name }}</span>
                  <a-tag :color="sch.enabled ? 'green' : 'default'">{{ sch.enabled ? '启用' : '停用' }}</a-tag>
                </div>
              </template>
              <template #extra>
                <a-dropdown>
                  <a-button type="text" size="small"><MoreOutlined /></a-button>
                  <template #overlay>
                    <a-menu @click="onScheduleMenuClick($event, sch)">
                      <a-menu-item key="edit">编辑</a-menu-item>
                      <a-menu-item key="generate">生成排班</a-menu-item>
                      <a-menu-item key="delete" danger>删除</a-menu-item>
                    </a-menu>
                  </template>
                </a-dropdown>
              </template>
              <p style="color: #999; margin-bottom: 8px">{{ sch.description || '暂无描述' }}</p>
              <a-space>
                <a-tag>{{ sch.rotation_type === 'daily' ? '日轮' : sch.rotation_type === 'weekly' ? '周轮' : '自定义' }}</a-tag>
                <a-tag>{{ sch.timezone }}</a-tag>
              </a-space>
              <div style="margin-top: 12px" v-if="currentOnCall[sch.id]">
                <a-tag color="blue">
                  当前值班: {{ currentOnCall[sch.id]?.user_name || '无' }}
                </a-tag>
              </div>
            </a-card>
          </a-col>
        </a-row>
      </a-tab-pane>

      <!-- 班次日历 -->
      <a-tab-pane key="shifts" tab="班次管理" :disabled="!selectedSchedule">
        <div v-if="selectedSchedule">
          <div style="margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center">
            <a-space>
              <a-button @click="shiftWeekOffset--"><LeftOutlined /></a-button>
              <span style="font-weight: 600; font-size: 15px">{{ shiftWeekLabel }}</span>
              <a-button @click="shiftWeekOffset++"><RightOutlined /></a-button>
              <a-button @click="shiftWeekOffset = 0">本周</a-button>
            </a-space>
            <a-space>
              <a-button type="primary" @click="showGenerateModal = true">自动生成</a-button>
            </a-space>
          </div>

          <a-table :dataSource="shifts" :columns="shiftColumns" rowKey="id" :pagination="false" size="small">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'time'">
                {{ formatDate(record.start_time) }} ~ {{ formatDate(record.end_time) }}
              </template>
              <template v-if="column.key === 'type'">
                <a-tag :color="record.shift_type === 'primary' ? 'blue' : 'orange'">
                  {{ record.shift_type === 'primary' ? '主班' : '副班' }}
                </a-tag>
              </template>
              <template v-if="column.key === 'action'">
                <a-popconfirm title="确认删除?" @confirm="deleteShift(record.id)">
                  <a-button type="link" danger size="small">删除</a-button>
                </a-popconfirm>
              </template>
            </template>
          </a-table>
        </div>
        <a-empty v-else description="请先选择排班表" />
      </a-tab-pane>

      <!-- 临时替换 -->
      <a-tab-pane key="overrides" tab="临时替换" :disabled="!selectedSchedule">
        <div v-if="selectedSchedule">
          <div style="margin-bottom: 16px">
            <a-button type="primary" @click="showOverrideModal = true">新建替换</a-button>
          </div>
          <a-table :dataSource="overrides" :columns="overrideColumns" rowKey="id" :pagination="false" size="small">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'time'">
                {{ formatDate(record.start_time) }} ~ {{ formatDate(record.end_time) }}
              </template>
              <template v-if="column.key === 'action'">
                <a-popconfirm title="确认删除?" @confirm="deleteOverride(record.id)">
                  <a-button type="link" danger size="small">删除</a-button>
                </a-popconfirm>
              </template>
            </template>
          </a-table>
        </div>
        <a-empty v-else description="请先选择排班表" />
      </a-tab-pane>

      <!-- 我的告警 -->
      <a-tab-pane key="assignments" tab="我的告警">
        <div style="margin-bottom: 16px">
          <a-radio-group v-model:value="assignmentStatus" button-style="solid" @change="loadMyAssignments">
            <a-radio-button value="">全部</a-radio-button>
            <a-radio-button value="pending">待处理</a-radio-button>
            <a-radio-button value="claimed">已认领</a-radio-button>
            <a-radio-button value="resolved">已解决</a-radio-button>
          </a-radio-group>
        </div>
        <a-table :dataSource="myAssignments" :columns="assignmentColumns" rowKey="id" :pagination="false" size="small">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
            </template>
            <template v-if="column.key === 'action'">
              <a-space>
                <a-button v-if="record.status === 'pending'" type="link" size="small" @click="claimAlert(record.alert_history_id)">认领</a-button>
                <a-button v-if="record.status === 'claimed'" type="link" size="small" @click="showResolveModal(record)">解决</a-button>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>

    <!-- 排班表表单弹窗 -->
    <a-modal v-model:open="scheduleModalVisible" :title="editingSchedule ? '编辑排班表' : '新建排班表'" @ok="saveSchedule" :confirmLoading="saving">
      <a-form :label-col="{ span: 5 }">
        <a-form-item label="名称" required>
          <a-input v-model:value="scheduleForm.name" placeholder="排班表名称" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="scheduleForm.description" :rows="2" />
        </a-form-item>
        <a-form-item label="轮转类型">
          <a-select v-model:value="scheduleForm.rotation_type">
            <a-select-option value="daily">日轮</a-select-option>
            <a-select-option value="weekly">周轮</a-select-option>
            <a-select-option value="custom">自定义</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="时区">
          <a-select v-model:value="scheduleForm.timezone" show-search>
            <a-select-option value="Asia/Shanghai">Asia/Shanghai</a-select-option>
            <a-select-option value="Asia/Tokyo">Asia/Tokyo</a-select-option>
            <a-select-option value="UTC">UTC</a-select-option>
            <a-select-option value="America/New_York">America/New_York</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="启用">
          <a-switch v-model:checked="scheduleForm.enabled" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 自动生成排班弹窗 -->
    <a-modal v-model:open="showGenerateModal" title="自动生成排班" @ok="generateShifts" :confirmLoading="generating">
      <a-form :label-col="{ span: 6 }">
        <a-form-item label="轮转类型">
          <a-select v-model:value="generateForm.type">
            <a-select-option value="weekly">周轮</a-select-option>
            <a-select-option value="daily">日轮</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="开始日期" required>
          <a-date-picker v-model:value="generateForm.startDate" style="width: 100%" />
        </a-form-item>
        <a-form-item label="轮次数">
          <a-input-number v-model:value="generateForm.count" :min="1" :max="52" style="width: 100%" />
        </a-form-item>
        <a-form-item label="值班人员" required>
          <div v-for="(_, idx) in generateForm.users" :key="idx" style="display: flex; gap: 8px; margin-bottom: 8px">
            <a-input v-model:value="generateForm.users[idx].name" placeholder="姓名" style="flex: 1" />
            <a-input-number v-model:value="generateForm.users[idx].id" placeholder="用户ID" style="width: 100px" />
            <a-button type="text" danger @click="generateForm.users.splice(idx, 1)" :disabled="generateForm.users.length <= 1">
              <DeleteOutlined />
            </a-button>
          </div>
          <a-button type="dashed" block @click="generateForm.users.push({ id: 0, name: '' })">
            <PlusOutlined /> 添加人员
          </a-button>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 临时替换弹窗 -->
    <a-modal v-model:open="showOverrideModal" title="新建临时替换" @ok="createOverride" :confirmLoading="saving">
      <a-form :label-col="{ span: 6 }">
        <a-form-item label="原值班人ID">
          <a-input-number v-model:value="overrideForm.original_user_id" style="width: 100%" />
        </a-form-item>
        <a-form-item label="原值班人">
          <a-input v-model:value="overrideForm.original_user_name" />
        </a-form-item>
        <a-form-item label="替换人ID">
          <a-input-number v-model:value="overrideForm.override_user_id" style="width: 100%" />
        </a-form-item>
        <a-form-item label="替换人">
          <a-input v-model:value="overrideForm.override_user_name" />
        </a-form-item>
        <a-form-item label="开始时间">
          <a-date-picker v-model:value="overrideForm.start_time" show-time style="width: 100%" />
        </a-form-item>
        <a-form-item label="结束时间">
          <a-date-picker v-model:value="overrideForm.end_time" show-time style="width: 100%" />
        </a-form-item>
        <a-form-item label="原因">
          <a-textarea v-model:value="overrideForm.reason" :rows="2" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 解决弹窗 -->
    <a-modal v-model:open="resolveModalVisible" title="解决告警" @ok="resolveAlert" :confirmLoading="saving">
      <a-form-item label="备注">
        <a-textarea v-model:value="resolveComment" :rows="3" placeholder="处理备注" />
      </a-form-item>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { message } from 'ant-design-vue'
import { MoreOutlined, LeftOutlined, RightOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons-vue'
import type { MenuInfo } from 'ant-design-vue/es/menu/src/interface'
import { oncallApi } from '@/services/oncall'
import type { OncallSchedule, OncallShift, OncallOverride, AlertAssignment } from '@/services/oncall'
import dayjs from 'dayjs'

const activeTab = ref('schedules')
const schedules = ref<OncallSchedule[]>([])
const selectedSchedule = ref<OncallSchedule | null>(null)
const shifts = ref<OncallShift[]>([])
const overrides = ref<OncallOverride[]>([])
const myAssignments = ref<AlertAssignment[]>([])
const currentOnCall = ref<Record<number, OncallShift | null>>({})
const assignmentStatus = ref('')
const saving = ref(false)
const generating = ref(false)
const shiftWeekOffset = ref(0)

const scheduleModalVisible = ref(false)
const editingSchedule = ref(false)
const showGenerateModal = ref(false)
const showOverrideModal = ref(false)
const resolveModalVisible = ref(false)
const resolveAlertId = ref(0)
const resolveComment = ref('')

const onScheduleMenuClick = (info: MenuInfo, sch: OncallSchedule) => {
  handleScheduleAction(String(info.key), sch)
}

const scheduleForm = reactive({
  id: 0,
  name: '',
  description: '',
  rotation_type: 'weekly',
  timezone: 'Asia/Shanghai',
  enabled: true,
})

const generateForm = reactive({
  type: 'weekly',
  startDate: dayjs() as any,
  count: 4,
  users: [{ id: 0, name: '' }] as { id: number; name: string }[],
})

const overrideForm = reactive({
  original_user_id: 0,
  original_user_name: '',
  override_user_id: 0,
  override_user_name: '',
  start_time: null as any,
  end_time: null as any,
  reason: '',
})

const shiftWeekLabel = computed(() => {
  const start = dayjs().startOf('week').add(shiftWeekOffset.value, 'week')
  const end = start.add(6, 'day')
  return `${start.format('MM/DD')} - ${end.format('MM/DD')}`
})

const shiftColumns = [
  { title: '值班人', dataIndex: 'user_name', key: 'user_name' },
  { title: '时间范围', key: 'time' },
  { title: '类型', key: 'type' },
  { title: '操作', key: 'action', width: 80 },
]

const overrideColumns = [
  { title: '原值班人', dataIndex: 'original_user_name', key: 'original_user_name' },
  { title: '替换人', dataIndex: 'override_user_name', key: 'override_user_name' },
  { title: '时间范围', key: 'time' },
  { title: '原因', dataIndex: 'reason', key: 'reason', ellipsis: true },
  { title: '操作', key: 'action', width: 80 },
]

const assignmentColumns = [
  { title: '告警ID', dataIndex: 'alert_history_id', key: 'alert_history_id' },
  { title: '处理人', dataIndex: 'assignee_name', key: 'assignee_name' },
  { title: '状态', key: 'status' },
  { title: '认领时间', dataIndex: 'claimed_at', key: 'claimed_at' },
  { title: '解决时间', dataIndex: 'resolved_at', key: 'resolved_at' },
  { title: '备注', dataIndex: 'comment', key: 'comment', ellipsis: true },
  { title: '操作', key: 'action', width: 120 },
]

const formatDate = (d: string) => d ? dayjs(d).format('YYYY-MM-DD HH:mm') : '-'
const statusColor = (s: string) => ({ pending: 'orange', claimed: 'blue', resolved: 'green', escalated: 'red' }[s] || 'default')
const statusText = (s: string) => ({ pending: '待处理', claimed: '已认领', resolved: '已解决', escalated: '已升级' }[s] || s)

async function loadSchedules() {
  try {
    const res = await oncallApi.listSchedules()
    schedules.value = res.data?.data || []
    for (const sch of schedules.value) {
      try {
        const r = await oncallApi.getCurrentOnCall(sch.id)
        const list = r.data?.data || []
        currentOnCall.value[sch.id] = list.length > 0 ? list[0] : null
      } catch { /* ignore */ }
    }
  } catch { message.error('加载排班表失败') }
}

function selectSchedule(sch: OncallSchedule) {
  selectedSchedule.value = sch
  activeTab.value = 'shifts'
  loadShifts()
  loadOverrides()
}

async function loadShifts() {
  if (!selectedSchedule.value) return
  const start = dayjs().startOf('week').add(shiftWeekOffset.value, 'week').format('YYYY-MM-DD')
  const end = dayjs().startOf('week').add(shiftWeekOffset.value + 1, 'week').format('YYYY-MM-DD')
  try {
    const res = await oncallApi.listShifts(selectedSchedule.value.id, start, end)
    shifts.value = res.data?.data || []
  } catch { message.error('加载班次失败') }
}

watch(shiftWeekOffset, () => { if (selectedSchedule.value) loadShifts() })

async function loadOverrides() {
  if (!selectedSchedule.value) return
  try {
    const res = await oncallApi.listOverrides(selectedSchedule.value.id)
    overrides.value = res.data?.data || []
  } catch { /* ignore */ }
}

async function loadMyAssignments() {
  try {
    const res = await oncallApi.listMyAssignments(assignmentStatus.value || undefined)
    myAssignments.value = res.data?.data || []
  } catch { message.error('加载告警分配失败') }
}

function showScheduleModal(sch?: OncallSchedule) {
  if (sch) {
    editingSchedule.value = true
    Object.assign(scheduleForm, { id: sch.id, name: sch.name, description: sch.description, rotation_type: sch.rotation_type, timezone: sch.timezone, enabled: sch.enabled })
  } else {
    editingSchedule.value = false
    Object.assign(scheduleForm, { id: 0, name: '', description: '', rotation_type: 'weekly', timezone: 'Asia/Shanghai', enabled: true })
  }
  scheduleModalVisible.value = true
}

async function saveSchedule() {
  if (!scheduleForm.name) { message.warning('请输入名称'); return }
  saving.value = true
  try {
    if (editingSchedule.value) {
      await oncallApi.updateSchedule(scheduleForm.id, scheduleForm)
      message.success('更新成功')
    } else {
      await oncallApi.createSchedule(scheduleForm)
      message.success('创建成功')
    }
    scheduleModalVisible.value = false
    loadSchedules()
  } catch { message.error('保存失败') } finally { saving.value = false }
}

function handleScheduleAction(key: string, sch: OncallSchedule) {
  if (key === 'edit') showScheduleModal(sch)
  else if (key === 'generate') { selectedSchedule.value = sch; showGenerateModal.value = true }
  else if (key === 'delete') deleteSchedule(sch.id)
}

async function deleteSchedule(id: number) {
  try {
    await oncallApi.deleteSchedule(id)
    message.success('已删除')
    loadSchedules()
  } catch { message.error('删除失败') }
}

async function generateShifts() {
  const users = generateForm.users.filter(u => u.id > 0 && u.name)
  if (users.length === 0) { message.warning('请添加值班人员'); return }
  if (!generateForm.startDate) { message.warning('请选择开始日期'); return }
  generating.value = true
  try {
    await oncallApi.generateShifts(selectedSchedule.value!.id, {
      user_ids: users.map(u => u.id),
      user_names: users.map(u => u.name),
      start_date: dayjs(generateForm.startDate).format('YYYY-MM-DD'),
      count: generateForm.count,
      type: generateForm.type,
    })
    message.success('排班生成成功')
    showGenerateModal.value = false
    loadShifts()
  } catch { message.error('生成失败') } finally { generating.value = false }
}

async function deleteShift(id: number) {
  try {
    await oncallApi.deleteShift(id)
    message.success('已删除')
    loadShifts()
  } catch { message.error('删除失败') }
}

async function createOverride() {
  if (!overrideForm.start_time || !overrideForm.end_time) { message.warning('请选择时间'); return }
  saving.value = true
  try {
    await oncallApi.createOverride(selectedSchedule.value!.id, {
      original_user_id: overrideForm.original_user_id,
      original_user_name: overrideForm.original_user_name,
      override_user_id: overrideForm.override_user_id,
      override_user_name: overrideForm.override_user_name,
      start_time: dayjs(overrideForm.start_time).format('YYYY-MM-DDTHH:mm:ssZ'),
      end_time: dayjs(overrideForm.end_time).format('YYYY-MM-DDTHH:mm:ssZ'),
      reason: overrideForm.reason,
    })
    message.success('创建成功')
    showOverrideModal.value = false
    loadOverrides()
  } catch { message.error('创建失败') } finally { saving.value = false }
}

async function deleteOverride(id: number) {
  try {
    await oncallApi.deleteOverride(id)
    message.success('已删除')
    loadOverrides()
  } catch { message.error('删除失败') }
}

async function claimAlert(alertId: number) {
  try {
    await oncallApi.claimAlert(alertId)
    message.success('已认领')
    loadMyAssignments()
  } catch { message.error('认领失败') }
}

function showResolveModal(record: AlertAssignment) {
  resolveAlertId.value = record.alert_history_id
  resolveComment.value = ''
  resolveModalVisible.value = true
}

async function resolveAlert() {
  saving.value = true
  try {
    await oncallApi.resolveAlert(resolveAlertId.value, resolveComment.value)
    message.success('已解决')
    resolveModalVisible.value = false
    loadMyAssignments()
  } catch { message.error('操作失败') } finally { saving.value = false }
}

onMounted(() => {
  loadSchedules()
  loadMyAssignments()
})
</script>

<style scoped>
.oncall-management {
  padding: 0;
}
</style>
