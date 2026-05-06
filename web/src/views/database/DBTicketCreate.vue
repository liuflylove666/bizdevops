<template>
  <div class="db-ticket-create">
    <div class="page-header">
      <h1>新建 SQL 工单</h1>
    </div>
    <a-card :bordered="false">
      <a-form layout="vertical" :model="form" ref="formRef">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="标题" required>
              <a-input v-model:value="form.title" placeholder="简要描述本次变更" />
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item label="类型">
              <a-radio-group v-model:value="form.change_type">
                <a-radio-button :value="1">DML</a-radio-button>
                <a-radio-button :value="0">DDL</a-radio-button>
              </a-radio-group>
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item label="需要备份">
              <div>
                <a-switch v-model:checked="form.need_backup" />
                <div class="form-hint">
                  仅在开启备份且工单执行成功后，系统才会生成可下载的回滚脚本。
                </div>
              </div>
            </a-form-item>
          </a-col>
        </a-row>

        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="目标实例" required>
              <a-select
                v-model:value="form.instance_id"
                :options="instanceOptions"
                placeholder="选择实例"
                show-search
                :filter-option="filterOption"
                @change="onInstanceChange"
              />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="目标库" required>
              <a-select
                v-model:value="form.schema_name"
                :options="schemaOptions"
                placeholder="选择库"
                :disabled="!form.instance_id"
                show-search
              />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="审批人 (逗号分隔)">
              <a-input v-model:value="approversRaw" placeholder="user1,user2" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-form-item label="SQL 脚本" required>
          <a-textarea v-model:value="form.sql_text" :rows="12" placeholder="多条语句以 ; 分隔" style="font-family: monospace" />
        </a-form-item>

        <a-row :gutter="16">
          <a-col :span="6">
            <a-form-item label="允许 DROP">
              <a-switch v-model:checked="form.allow_drop" />
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item label="允许 TRUNCATE">
              <a-switch v-model:checked="form.allow_trunc" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-form-item label="备注">
          <a-textarea v-model:value="form.description" :rows="2" />
        </a-form-item>

        <a-form-item>
          <a-space>
            <a-button type="primary" :loading="submitting" @click="submit">提交工单</a-button>
            <a-button @click="$router.push('/database/tickets')">取消</a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useRouter } from 'vue-router'
import { dbInstanceApi, dbTicketApi, type DBInstance, type TicketCreateInput } from '@/services/database'

const router = useRouter()
const submitting = ref(false)
const instances = ref<DBInstance[]>([])
const schemas = ref<string[]>([])
const approversRaw = ref('')

const form = reactive<TicketCreateInput>({
  title: '',
  description: '',
  instance_id: 0,
  schema_name: '',
  change_type: 1,
  need_backup: false,
  sql_text: '',
  allow_drop: false,
  allow_trunc: false
})

const instanceOptions = computed(() =>
  instances.value.map(i => ({ value: i.id, label: `${i.env}/${i.name} (${i.host})` }))
)
const schemaOptions = computed(() => schemas.value.map(s => ({ value: s, label: s })))

function filterOption(input: string, option: any) {
  return (option.label || '').toLowerCase().includes(input.toLowerCase())
}

async function onInstanceChange(id: number) {
  form.schema_name = ''
  schemas.value = []
  if (!id) return
  const res = await dbInstanceApi.databases(id)
  schemas.value = res.data || []
}

async function submit() {
  if (!form.title || !form.instance_id || !form.schema_name || !form.sql_text.trim()) {
    message.warning('请填写完整工单信息')
    return
  }
  const approvers = approversRaw.value.split(',').map(s => s.trim()).filter(Boolean)
  if (approvers.length > 0) {
    form.audit_steps = [{ step_name: 'review', approvers }]
  }
  submitting.value = true
  try {
    const res = await dbTicketApi.create(form)
    if (res.code === 0) {
      message.success('工单提交成功')
      router.push(`/database/tickets/${res.data?.id}`)
    } else {
      message.error(res.message || '提交失败')
    }
  } catch (e: any) {
    message.error(e?.message || '提交失败')
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  const res = await dbInstanceApi.listAll()
  instances.value = res.data || []
})
</script>

<style scoped>
.page-header { margin-bottom: 16px; }
.page-header h1 { margin: 0; font-size: 20px; }
.form-hint { margin-top: 8px; color: rgba(0, 0, 0, 0.45); font-size: 12px; line-height: 1.5; }
</style>
