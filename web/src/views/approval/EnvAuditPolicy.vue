<template>
  <div class="env-audit-policy">
    <a-card title="环境审核策略" :bordered="false">
      <template #extra>
        <a-button type="primary" @click="showCreateModal">新增策略</a-button>
      </template>
      <a-spin :spinning="loading">
        <a-row :gutter="16">
          <a-col :span="8" v-for="p in policies" :key="p.id" style="margin-bottom:16px">
            <a-card :bordered="true" size="small" :class="['policy-card', `risk-${p.risk_level}`]">
              <template #title>
                <div style="display:flex;justify-content:space-between;align-items:center">
                  <span>
                    <a-tag :color="riskColor(p.risk_level)">{{ p.risk_level.toUpperCase() }}</a-tag>
                    {{ p.display_name || p.env_name }}
                  </span>
                  <a-switch size="small" :checked="p.enabled" @change="toggleEnabled(p)" />
                </div>
              </template>
              <template #extra>
                <a-dropdown>
                  <a @click.prevent><MoreOutlined /></a>
                  <template #overlay>
                    <a-menu @click="({ key }: any) => handleAction(key, p)">
                      <a-menu-item key="edit">编辑</a-menu-item>
                      <a-menu-item key="loose">应用预设: 宽松</a-menu-item>
                      <a-menu-item key="moderate">应用预设: 中等</a-menu-item>
                      <a-menu-item key="strict">应用预设: 严格</a-menu-item>
                      <a-menu-item key="critical">应用预设: 关键</a-menu-item>
                      <a-menu-divider />
                      <a-menu-item key="delete" danger>删除</a-menu-item>
                    </a-menu>
                  </template>
                </a-dropdown>
              </template>

              <a-descriptions :column="2" size="small">
                <a-descriptions-item label="审批">
                  <a-tag :color="p.require_approval ? 'orange' : 'green'">{{ p.require_approval ? '需要' : '免审' }}</a-tag>
                </a-descriptions-item>
                <a-descriptions-item label="最少审批人">{{ p.min_approvers }}</a-descriptions-item>
                <a-descriptions-item label="多级审批链">
                  <CheckCircleTwoTone v-if="p.require_chain" two-tone-color="#52c41a" />
                  <CloseCircleTwoTone v-else two-tone-color="#ccc" />
                </a-descriptions-item>
                <a-descriptions-item label="发布窗口">
                  <CheckCircleTwoTone v-if="p.require_deploy_window" two-tone-color="#52c41a" />
                  <CloseCircleTwoTone v-else two-tone-color="#ccc" />
                </a-descriptions-item>
                <a-descriptions-item label="代码审查">
                  <CheckCircleTwoTone v-if="p.require_code_review" two-tone-color="#52c41a" />
                  <CloseCircleTwoTone v-else two-tone-color="#ccc" />
                </a-descriptions-item>
                <a-descriptions-item label="测试通过">
                  <CheckCircleTwoTone v-if="p.require_test_pass" two-tone-color="#52c41a" />
                  <CloseCircleTwoTone v-else two-tone-color="#ccc" />
                </a-descriptions-item>
                <a-descriptions-item label="窗口外拒绝">
                  <CheckCircleTwoTone v-if="p.auto_reject_outside_window" two-tone-color="#ff4d4f" />
                  <CloseCircleTwoTone v-else two-tone-color="#ccc" />
                </a-descriptions-item>
                <a-descriptions-item label="紧急发布">
                  <CheckCircleTwoTone v-if="p.allow_emergency" two-tone-color="#52c41a" />
                  <CloseCircleTwoTone v-else two-tone-color="#ff4d4f" />
                </a-descriptions-item>
                <a-descriptions-item label="允许回滚">
                  <CheckCircleTwoTone v-if="p.allow_rollback" two-tone-color="#52c41a" />
                  <CloseCircleTwoTone v-else two-tone-color="#ff4d4f" />
                </a-descriptions-item>
                <a-descriptions-item label="日部署上限">{{ p.max_deploys_per_day || '不限' }}</a-descriptions-item>
              </a-descriptions>
            </a-card>
          </a-col>
        </a-row>
        <a-empty v-if="!loading && policies.length === 0" description="暂无环境策略" />
      </a-spin>
    </a-card>

    <!-- 编辑/新增弹窗 -->
    <a-modal v-model:open="modalVisible" :title="editingPolicy?.id ? '编辑策略' : '新增策略'" @ok="handleSave" :confirmLoading="saving" width="640px">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="环境标识" required>
          <a-input v-model:value="form.env_name" :disabled="!!editingPolicy?.id" placeholder="如 dev / staging / prod" />
        </a-form-item>
        <a-form-item label="显示名称">
          <a-input v-model:value="form.display_name" />
        </a-form-item>
        <a-form-item label="风险等级">
          <a-select v-model:value="form.risk_level">
            <a-select-option value="low"><a-tag color="green">Low</a-tag></a-select-option>
            <a-select-option value="medium"><a-tag color="orange">Medium</a-tag></a-select-option>
            <a-select-option value="high"><a-tag color="red">High</a-tag></a-select-option>
            <a-select-option value="critical"><a-tag color="#722ed1">Critical</a-tag></a-select-option>
          </a-select>
        </a-form-item>
        <a-divider orientation="left" plain>审批控制</a-divider>
        <a-form-item label="需要审批"><a-switch v-model:checked="form.require_approval" /></a-form-item>
        <a-form-item label="最少审批人"><a-input-number v-model:value="form.min_approvers" :min="0" :max="10" /></a-form-item>
        <a-form-item label="多级审批链"><a-switch v-model:checked="form.require_chain" /></a-form-item>
        <a-divider orientation="left" plain>发布管控</a-divider>
        <a-form-item label="发布窗口"><a-switch v-model:checked="form.require_deploy_window" /></a-form-item>
        <a-form-item label="窗口外自动拒绝"><a-switch v-model:checked="form.auto_reject_outside_window" /></a-form-item>
        <a-form-item label="允许紧急发布"><a-switch v-model:checked="form.allow_emergency" /></a-form-item>
        <a-form-item label="允许回滚"><a-switch v-model:checked="form.allow_rollback" /></a-form-item>
        <a-form-item label="日部署上限"><a-input-number v-model:value="form.max_deploys_per_day" :min="0" /> <span style="margin-left:8px;color:#999">0=不限</span></a-form-item>
        <a-divider orientation="left" plain>质量门禁</a-divider>
        <a-form-item label="代码审查"><a-switch v-model:checked="form.require_code_review" /></a-form-item>
        <a-form-item label="测试通过"><a-switch v-model:checked="form.require_test_pass" /></a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { MoreOutlined, CheckCircleTwoTone, CloseCircleTwoTone } from '@ant-design/icons-vue'
import { envAuditPolicyApi, type EnvAuditPolicy } from '@/services/envAuditPolicy'

const loading = ref(false)
const saving = ref(false)
const modalVisible = ref(false)
const policies = ref<EnvAuditPolicy[]>([])
const editingPolicy = ref<EnvAuditPolicy | null>(null)

const defaultForm: EnvAuditPolicy = {
  env_name: '', display_name: '', risk_level: 'low',
  require_approval: false, min_approvers: 1, require_chain: false,
  require_deploy_window: false, auto_reject_outside_window: false,
  require_code_review: false, require_test_pass: false,
  allow_emergency: true, allow_rollback: true, max_deploys_per_day: 0, enabled: true,
}
const form = reactive<EnvAuditPolicy>({ ...defaultForm })

const riskColor = (level: string) => {
  const m: Record<string, string> = { low: 'green', medium: 'orange', high: 'red', critical: '#722ed1' }
  return m[level] || 'default'
}

const loadPolicies = async () => {
  loading.value = true
  try { policies.value = (await envAuditPolicyApi.list()).data || [] }
  catch { policies.value = [] }
  finally { loading.value = false }
}

const showCreateModal = () => {
  editingPolicy.value = null
  Object.assign(form, { ...defaultForm })
  modalVisible.value = true
}

const showEditModal = (p: EnvAuditPolicy) => {
  editingPolicy.value = p
  Object.assign(form, { ...p })
  modalVisible.value = true
}

const handleSave = async () => {
  if (!form.env_name) { message.warning('请填写环境标识'); return }
  saving.value = true
  try {
    if (editingPolicy.value?.id) {
      await envAuditPolicyApi.update(editingPolicy.value.id, { ...form })
    } else {
      await envAuditPolicyApi.create({ ...form })
    }
    message.success('保存成功')
    modalVisible.value = false
    loadPolicies()
  } catch (e: any) { message.error(e?.response?.data?.message || '保存失败') }
  finally { saving.value = false }
}

const toggleEnabled = async (p: EnvAuditPolicy) => {
  if (!p.id) return
  try {
    await envAuditPolicyApi.update(p.id, { ...p, enabled: !p.enabled })
    message.success('更新成功')
    loadPolicies()
  } catch {}
}

const handleAction = (key: string, p: EnvAuditPolicy) => {
  if (key === 'edit') { showEditModal(p); return }
  if (key === 'delete') {
    Modal.confirm({
      title: '确定删除？',
      content: `将删除环境 "${p.display_name || p.env_name}" 的审核策略`,
      onOk: async () => {
        if (!p.id) return
        await envAuditPolicyApi.delete(p.id)
        message.success('已删除')
        loadPolicies()
      },
    })
    return
  }
  // presets
  if (['loose', 'moderate', 'strict', 'critical'].includes(key) && p.id) {
    const labels: Record<string, string> = { loose: '宽松', moderate: '中等', strict: '严格', critical: '关键' }
    Modal.confirm({
      title: `应用预设: ${labels[key]}`,
      content: `将把 "${p.display_name || p.env_name}" 策略重置为 "${labels[key]}" 模板`,
      onOk: async () => {
        await envAuditPolicyApi.applyPreset(p.id!, key)
        message.success('预设已应用')
        loadPolicies()
      },
    })
  }
}

onMounted(loadPolicies)
</script>

<style scoped>
.policy-card.risk-low { border-left: 3px solid #52c41a; }
.policy-card.risk-medium { border-left: 3px solid #faad14; }
.policy-card.risk-high { border-left: 3px solid #ff4d4f; }
.policy-card.risk-critical { border-left: 3px solid #722ed1; }
</style>
