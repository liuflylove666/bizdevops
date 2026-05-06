<template>
  <div class="db-ticket-detail" v-if="detail">
    <div class="page-header">
      <div>
        <h1>{{ detail.ticket.title }}</h1>
        <a-space>
          <a-tag :color="statusColor(detail.ticket.status)">{{ statusText(detail.ticket.status) }}</a-tag>
          <span class="muted">工单号 {{ detail.ticket.work_id }}</span>
          <span class="muted">申请人 {{ detail.ticket.applicant }}</span>
          <a v-if="detail.ticket.approval_instance_id" @click="goToApprovalInstance(detail.ticket.approval_instance_id)">审批详情</a>
        </a-space>
      </div>
      <a-space>
        <a-button v-if="canAgree" type="primary" @click="act('agree')">审批通过</a-button>
        <a-button v-if="canAgree" danger @click="act('reject')">驳回</a-button>
        <a-button v-if="canExecute" type="primary" @click="act('execute')">执行</a-button>
        <a-button v-if="canCancel" @click="act('cancel')">撤回</a-button>
        <a-button @click="openRollback">查看回滚脚本</a-button>
      </a-space>
    </div>

    <a-row :gutter="16">
      <a-col :span="16">
        <a-card title="审核报告" :bordered="false" style="margin-bottom: 16px">
          <a-alert
            :message="detail.ticket.audit_report?.summary || '无报告'"
            :type="reportType"
            show-icon
            style="margin-bottom: 12px"
          />
          <a-collapse v-if="detail.ticket.audit_report?.statements?.length">
            <a-collapse-panel
              v-for="s in detail.ticket.audit_report.statements"
              :key="s.seq"
              :header="`#${s.seq} [${s.kind}] ${truncate(s.sql)}`"
            >
              <pre class="sql-block">{{ s.sql }}</pre>
              <a-list v-if="s.findings?.length" size="small" :data-source="s.findings">
                <template #renderItem="{ item }">
                  <a-list-item>
                    <a-tag :color="findingColor(item.level)">{{ item.level }}</a-tag>
                    <span style="margin-left: 8px">[{{ item.rule }}] {{ item.message }}</span>
                  </a-list-item>
                </template>
              </a-list>
              <a-typography-text v-else type="secondary">无风险提示</a-typography-text>
            </a-collapse-panel>
          </a-collapse>
        </a-card>

        <a-card title="执行明细" :bordered="false">
          <a-table
            :columns="stmtColumns"
            :data-source="detail.statements"
            :pagination="false"
            row-key="id"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'state'">
                <a-tag :color="stateColor(record.state)">{{ record.state }}</a-tag>
              </template>
              <template v-if="column.key === 'sql_text'">
                <a-typography-paragraph :ellipsis="{ rows: 2, expandable: true }" :content="record.sql_text" />
              </template>
              <template v-if="column.key === 'error_msg'">
                <a-typography-text type="danger" v-if="record.error_msg">{{ record.error_msg }}</a-typography-text>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-col>

      <a-col :span="8">
        <a-card title="工单信息" :bordered="false" style="margin-bottom: 16px">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="实例">{{ detail.ticket.instance_id }}</a-descriptions-item>
            <a-descriptions-item label="库">{{ detail.ticket.schema_name }}</a-descriptions-item>
            <a-descriptions-item label="类型">{{ detail.ticket.change_type === 0 ? 'DDL' : 'DML' }}</a-descriptions-item>
            <a-descriptions-item label="需要备份">{{ detail.ticket.need_backup ? '是' : '否' }}</a-descriptions-item>
            <a-descriptions-item label="创建时间">{{ detail.ticket.created_at }}</a-descriptions-item>
            <a-descriptions-item label="当前处理人">{{ detail.ticket.assigned || '—' }}</a-descriptions-item>
          </a-descriptions>
          <a-typography-paragraph v-if="detail.ticket.description" style="margin-top: 12px">
            {{ detail.ticket.description }}
          </a-typography-paragraph>
        </a-card>

        <a-card title="审批流" :bordered="false" style="margin-bottom: 16px">
          <a-steps direction="vertical" size="small" :current="detail.ticket.current_step">
            <a-step
              v-for="(s, i) in detail.audit_steps"
              :key="i"
              :title="s.step_name"
              :description="s.approvers.join(', ')"
            />
          </a-steps>
        </a-card>

        <a-card title="流转记录" :bordered="false">
          <a-timeline>
            <a-timeline-item v-for="w in detail.workflow" :key="w.id">
              <div><strong>{{ w.action }}</strong> · {{ w.username }}</div>
              <div class="muted">{{ w.created_at }}</div>
              <div v-if="w.comment">{{ w.comment }}</div>
            </a-timeline-item>
          </a-timeline>
        </a-card>
      </a-col>
    </a-row>

    <a-drawer
      v-model:open="rollbackOpen"
      title="回滚脚本"
      width="720"
    >
      <a-empty v-if="!rollbacks.length" :description="rollbackHint" />
      <div v-for="r in rollbacks" :key="r.id" style="margin-bottom: 16px">
        <div class="muted" style="margin-bottom: 6px">#{{ r.statement_id }} · {{ r.created_at }}</div>
        <pre class="sql-block">{{ r.rollback_sql }}</pre>
      </div>
      <a-button v-if="rollbacks.length" type="primary" block @click="downloadRollback">下载全部</a-button>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { useRoute, useRouter } from 'vue-router'
import { dbTicketApi, type TicketDetail, type SQLRollbackScript } from '@/services/database'

const route = useRoute()
const router = useRouter()
const detail = ref<TicketDetail | null>(null)
const id = Number(route.params.id)
const rollbackOpen = ref(false)
const rollbacks = ref<SQLRollbackScript[]>([])

const stmtColumns = [
  { title: '#', dataIndex: 'seq', key: 'seq', width: 60 },
  { title: 'SQL', dataIndex: 'sql_text', key: 'sql_text', ellipsis: true },
  { title: '影响行', dataIndex: 'affect_rows', key: 'affect_rows', width: 90 },
  { title: '耗时(ms)', dataIndex: 'exec_ms', key: 'exec_ms', width: 100 },
  { title: '状态', dataIndex: 'state', key: 'state', width: 100 },
  { title: '错误', dataIndex: 'error_msg', key: 'error_msg', ellipsis: true }
]

const reportType = computed<'error' | 'warning' | 'success'>(() => {
  if (!detail.value?.ticket.audit_report) return 'success'
  if (detail.value.ticket.audit_report.has_error) return 'error'
  if (detail.value.ticket.audit_report.has_warning) return 'warning'
  return 'success'
})

const hasUnifiedApproval = computed(() => Boolean(detail.value?.ticket.approval_instance_id))
const canAgree = computed(() => detail.value?.ticket.status === 0 && !hasUnifiedApproval.value)
const canExecute = computed(() => detail.value?.ticket.status === 2)
const canCancel = computed(() => {
  const s = detail.value?.ticket.status
  return s === 0 || s === 2
})
const rollbackHint = computed(() => {
  const ticket = detail.value?.ticket
  if (!ticket) return '该工单暂无回滚脚本'
  if (!ticket.need_backup) return '该工单未开启备份，执行后不会生成回滚脚本'
  if (ticket.status === 0 || ticket.status === 2 || ticket.status === 3) {
    return '该工单已开启备份，执行工单时会生成回滚脚本'
  }
  if (ticket.status === 1 || ticket.status === 6) {
    return '该工单未执行完成，因此不会生成回滚脚本'
  }
  return '该工单暂无回滚脚本，请检查 SQL 类型与执行结果'
})

function goToApprovalInstance(approvalInstanceId: number) {
  router.push(`/approval/instances/${approvalInstanceId}`)
}

function statusColor(s: number) {
  return ['orange', 'red', 'blue', 'processing', 'green', 'red', 'default'][s] || 'default'
}
function statusText(s: number) {
  return ['审批中', '已驳回', '待执行', '执行中', '已成功', '已失败', '已撤回'][s] || '未知'
}
function stateColor(s: string) {
  return { success: 'green', failed: 'red', running: 'blue', pending: 'default' }[s] || 'default'
}
function findingColor(l: string) {
  return { error: 'red', warning: 'orange', info: 'blue' }[l] || 'default'
}
function truncate(s: string) {
  s = s.replace(/\s+/g, ' ')
  return s.length > 80 ? s.slice(0, 80) + '…' : s
}

async function load() {
  const res = await dbTicketApi.get(id)
  detail.value = res.data || null
}

async function act(kind: 'agree' | 'reject' | 'cancel' | 'execute') {
  const label = { agree: '审批通过', reject: '驳回', cancel: '撤回', execute: '执行' }[kind]
  let comment = ''
  if (kind === 'agree' || kind === 'reject') {
    comment = window.prompt(`请输入${label}备注`, '') || ''
    if (kind === 'reject' && !comment) {
      message.warning('驳回必须填写原因')
      return
    }
  } else {
    const ok = await new Promise<boolean>(res => {
      Modal.confirm({ title: `确认${label}该工单？`, onOk: () => res(true), onCancel: () => res(false) })
    })
    if (!ok) return
  }
  try {
    const fn = {
      agree: () => dbTicketApi.agree(id, comment),
      reject: () => dbTicketApi.reject(id, comment),
      cancel: () => dbTicketApi.cancel(id),
      execute: () => dbTicketApi.execute(id)
    }[kind]
    const res = await fn()
    if (res.code === 0) {
      message.success(`${label}成功`)
      load()
    } else {
      message.error(res.message || `${label}失败`)
    }
  } catch (e: any) {
    message.error(e?.message || `${label}失败`)
  }
}

async function openRollback() {
  try {
    const res = await dbTicketApi.rollback(id)
    rollbacks.value = res.data || []
    rollbackOpen.value = true
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  }
}

function downloadRollback() {
  const content = rollbacks.value.map(r => `-- statement #${r.statement_id}\n${r.rollback_sql}`).join('\n')
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `rollback-${detail.value?.ticket.work_id || id}.sql`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-header h1 { margin: 0 0 8px 0; font-size: 20px; }
.muted { color: rgba(0, 0, 0, 0.45); }
.sql-block { background: #f5f5f5; padding: 8px; border-radius: 4px; white-space: pre-wrap; word-break: break-all; }
</style>
