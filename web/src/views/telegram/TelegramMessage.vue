<template>
  <div class="telegram-message">
    <div class="page-header">
      <h1>Telegram 管理</h1>
      <a-space>
        <a-button v-if="activeTab === 'bots'" type="primary" size="small" @click="showBotModal()">
          <template #icon><PlusOutlined /></template>
          添加机器人
        </a-button>
      </a-space>
    </div>

    <a-tabs v-model:activeKey="activeTab">
      <!-- 机器人管理 -->
      <a-tab-pane key="bots" tab="机器人管理">
        <a-card :bordered="false">
          <a-table :columns="botColumns" :data-source="botList" :loading="loadingBots" row-key="id" :pagination="{ pageSize: 10 }">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'token'">
                <a-typography-text :copyable="{ text: record.token }">{{ maskToken(record.token) }}</a-typography-text>
              </template>
              <template v-if="column.key === 'default_chat_id'">
                <span>{{ record.default_chat_id || '-' }}</span>
              </template>
              <template v-if="column.key === 'status'">
                <a-badge :status="record.status === 'active' ? 'success' : 'default'" :text="record.status === 'active' ? '启用' : '禁用'" />
              </template>
              <template v-if="column.key === 'is_default'">
                <a-tag v-if="record.is_default" color="blue">默认</a-tag>
                <a-button v-else type="link" size="small" @click="setDefaultBot(record.id)">设为默认</a-button>
              </template>
              <template v-if="column.key === 'action'">
                <a-space>
                  <a-button type="link" size="small" @click="testBot(record.id)">测试</a-button>
                  <a-button type="link" size="small" @click="showBotModal(record)">编辑</a-button>
                  <a-popconfirm title="确定删除？" @confirm="() => deleteBot(record.id)" :disabled="!record.id">
                    <a-button type="link" size="small" danger :disabled="record.is_default">删除</a-button>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 发送消息 -->
      <a-tab-pane key="message" tab="发送消息">
        <a-row :gutter="24">
          <a-col :xs="24" :lg="14">
            <a-card title="发送 Telegram 消息" :bordered="false">
              <a-form :model="messageForm" layout="vertical">
                <a-form-item label="选择机器人">
                  <a-select v-model:value="messageForm.bot_id" placeholder="使用默认机器人" style="width: 100%" allow-clear>
                    <a-select-option v-for="bot in botList" :key="bot.id" :value="bot.id">
                      {{ bot.name }}<template v-if="bot.is_default"> (默认)</template>
                    </a-select-option>
                  </a-select>
                </a-form-item>
                <a-form-item label="Chat ID">
                  <a-input v-model:value="messageForm.chat_id" placeholder="留空则使用机器人默认 Chat ID" />
                </a-form-item>
                <a-form-item label="解析模式">
                  <a-radio-group v-model:value="messageForm.parse_mode" button-style="solid">
                    <a-radio-button value="text">纯文本</a-radio-button>
                    <a-radio-button value="MarkdownV2">MarkdownV2</a-radio-button>
                    <a-radio-button value="HTML">HTML</a-radio-button>
                  </a-radio-group>
                </a-form-item>
                <a-form-item label="消息内容" required>
                  <a-textarea v-model:value="messageForm.content" :placeholder="messageContentPlaceholder" :rows="6" />
                </a-form-item>
                <a-form-item>
                  <a-space>
                    <a-checkbox v-model:checked="messageForm.disable_web_page_preview">禁用网页预览</a-checkbox>
                    <a-checkbox v-model:checked="messageForm.disable_notification">静默发送</a-checkbox>
                  </a-space>
                </a-form-item>
                <a-form-item>
                  <a-button type="primary" @click="sendMessage" :loading="sendingMessage" block>
                    <template #icon><SendOutlined /></template>发送消息
                  </a-button>
                </a-form-item>
              </a-form>
            </a-card>
          </a-col>
          <a-col :xs="24" :lg="10">
            <a-card title="使用说明" :bordered="false">
              <a-typography-paragraph>
                <ul>
                  <li>在 Telegram 中与 <b>@BotFather</b> 对话创建 Bot，得到 Token</li>
                  <li>将 Bot 加入群组或频道，或与用户开启私聊</li>
                  <li>通过 <code>getUpdates</code> 或 @userinfobot 获取 Chat ID</li>
                  <li>若国内环境访问受限，可在机器人配置中填写代理 API 地址</li>
                </ul>
              </a-typography-paragraph>
            </a-card>
          </a-col>
        </a-row>
      </a-tab-pane>

      <!-- 发送记录 -->
      <a-tab-pane key="logs" tab="发送记录">
        <a-card :bordered="false">
          <template #extra>
            <a-space>
              <a-select v-model:value="logFilter.source" placeholder="来源" style="width: 120px" allow-clear @change="fetchLogs">
                <a-select-option value="manual">手动发送</a-select-option>
              </a-select>
              <a-button @click="fetchLogs"><template #icon><ReloadOutlined /></template></a-button>
            </a-space>
          </template>
          <a-table :columns="logColumns" :data-source="logList" :loading="loadingLogs" row-key="id" :pagination="logPagination" @change="onLogTableChange">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'parse_mode'">
                <a-tag v-if="record.parse_mode">{{ record.parse_mode }}</a-tag>
                <span v-else>text</span>
              </template>
              <template v-if="column.key === 'status'">
                <a-badge :status="record.status === 'success' ? 'success' : 'error'" :text="record.status === 'success' ? '成功' : '失败'" />
              </template>
              <template v-if="column.key === 'created_at'">{{ formatTime(record.created_at) }}</template>
              <template v-if="column.key === 'action'">
                <a-button type="link" size="small" @click="viewLogDetail(record)">详情</a-button>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 消息模板 -->
      <a-tab-pane key="templates" tab="消息模板">
        <a-card :bordered="false">
          <div class="toolbar">
            <a-input-search
              v-model:value="templateKeyword"
              placeholder="搜索模板名称"
              style="width: 240px"
              @search="loadTemplates"
            />
            <a-button type="primary" @click="openTemplateModal()">
              <template #icon><PlusOutlined /></template>
              新增模板
            </a-button>
            <a-button @click="loadTemplates">
              <template #icon><ReloadOutlined /></template>
              刷新
            </a-button>
          </div>

          <a-table :dataSource="templates" :loading="templateLoading" rowKey="id" :pagination="false" size="middle">
            <a-table-column title="名称" dataIndex="name" :width="180" />
            <a-table-column title="类型" dataIndex="type" :width="120">
              <template #default="{ record }">
                <a-tag color="geekblue">{{ record.type }}</a-tag>
              </template>
            </a-table-column>
            <a-table-column title="描述" dataIndex="description" ellipsis />
            <a-table-column title="启用" dataIndex="is_active" :width="80">
              <template #default="{ record }">
                <a-tag :color="record.is_active ? 'green' : 'default'">{{ record.is_active ? '启用' : '停用' }}</a-tag>
              </template>
            </a-table-column>
            <a-table-column title="操作" :width="200">
              <template #default="{ record }">
                <a-space>
                  <a-button type="link" size="small" @click="previewTemplate(record)">预览</a-button>
                  <a-button type="link" size="small" @click="openTemplateModal(record)">编辑</a-button>
                  <a-popconfirm title="确认删除此模板?" @confirm="deleteTemplate(record.id)">
                    <a-button type="link" size="small" danger>删除</a-button>
                  </a-popconfirm>
                </a-space>
              </template>
            </a-table-column>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>

    <!-- 机器人编辑弹窗 -->
    <a-modal v-model:open="botModalVisible" :title="editingBotId ? '编辑机器人' : '添加机器人'" @ok="saveBot" :confirm-loading="savingBot" width="600px">
      <a-form :model="editingBot" layout="vertical">
        <a-form-item label="机器人名称" required>
          <a-input v-model:value="editingBot.name" placeholder="我的 Telegram 机器人" />
        </a-form-item>
        <a-form-item label="Bot Token" required>
          <a-input-password v-model:value="editingBot.token" placeholder="123456:ABC-DEF..." />
        </a-form-item>
        <a-form-item label="默认 Chat ID">
          <a-input v-model:value="editingBot.default_chat_id" placeholder="可选，如 -1001234567890" />
        </a-form-item>
        <a-form-item label="API 代理地址">
          <a-input v-model:value="editingBot.api_base_url" placeholder="可选，默认 https://api.telegram.org" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="editingBot.description" :rows="2" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="状态">
              <a-select v-model:value="editingBot.status" style="width: 100%">
                <a-select-option value="active">启用</a-select-option>
                <a-select-option value="inactive">禁用</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label=" ">
              <a-checkbox v-model:checked="editingBot.is_default">设为默认机器人</a-checkbox>
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-modal>

    <!-- 日志详情抽屉 -->
    <a-drawer v-model:open="logDetailVisible" title="发送详情" placement="right" :width="500">
      <a-descriptions :column="1" bordered size="small">
        <a-descriptions-item label="发送时间">{{ formatTime(currentLog?.created_at) }}</a-descriptions-item>
        <a-descriptions-item label="机器人 ID">{{ currentLog?.bot_id }}</a-descriptions-item>
        <a-descriptions-item label="Chat ID">{{ currentLog?.chat_id }}</a-descriptions-item>
        <a-descriptions-item label="解析模式">{{ currentLog?.parse_mode || 'text' }}</a-descriptions-item>
        <a-descriptions-item label="状态">
          <a-badge :status="currentLog?.status === 'success' ? 'success' : 'error'" :text="currentLog?.status === 'success' ? '成功' : '失败'" />
        </a-descriptions-item>
        <a-descriptions-item v-if="currentLog?.error_msg" label="错误信息">
          <a-typography-text type="danger">{{ currentLog?.error_msg }}</a-typography-text>
        </a-descriptions-item>
      </a-descriptions>
      <a-divider orientation="left">消息内容</a-divider>
      <pre class="content-preview">{{ currentLog?.content }}</pre>
    </a-drawer>

    <!-- 模板编辑 Modal -->
    <a-modal
      v-model:open="templateModalVisible"
      :title="templateForm.id ? '编辑模板' : '新增模板'"
      :confirm-loading="templateSaving"
      width="720px"
      @ok="saveTemplate"
    >
      <a-form :model="templateForm" :label-col="{ span: 4 }" :wrapper-col="{ span: 20 }">
        <a-form-item label="名称" required>
          <a-input v-model:value="templateForm.name" placeholder="如: pipeline-success" />
        </a-form-item>
        <a-form-item label="类型" required>
          <a-select v-model:value="templateForm.type">
            <a-select-option value="telegram">Telegram</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="描述">
          <a-input v-model:value="templateForm.description" />
        </a-form-item>
        <a-form-item label="启用">
          <a-switch v-model:checked="templateForm.is_active" />
        </a-form-item>
        <a-form-item label="内容" required>
          <a-textarea v-model:value="templateForm.content" :rows="10" placeholder="支持 Go text/template 语法，变量示例: {{.PipelineName}} {{.Status}}" />
          <div class="template-vars">
            <span class="label">常用变量：</span>
            <a-tag v-for="v in availableVars" :key="v" @click="insertVar(v)">{{ v }}</a-tag>
          </div>
        </a-form-item>
        <a-form-item :wrapper-col="{ offset: 4 }">
          <a-button @click="previewTemplateContent">预览效果</a-button>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 模板预览 Modal -->
    <a-modal v-model:open="previewVisible" title="模板预览" width="640px" :footer="null">
      <pre class="preview-content">{{ previewContent }}</pre>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { SendOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import {
  telegramBotApi,
  telegramApi,
  telegramLogApi,
  type TelegramBot,
  type TelegramMessageLog
} from '@/services/telegram'
import { templateApi, type MessageTemplate } from '@/services/template'

const activeTab = ref('bots')
const sendingMessage = ref(false)
const loadingBots = ref(false)
const loadingLogs = ref(false)
const savingBot = ref(false)
const botModalVisible = ref(false)
const logDetailVisible = ref(false)
const botList = ref<TelegramBot[]>([])
const logList = ref<TelegramMessageLog[]>([])
const currentLog = ref<TelegramMessageLog | null>(null)
const editingBotId = ref<number | undefined>(undefined)

const editingBot = reactive<Partial<TelegramBot>>({
  name: '',
  token: '',
  default_chat_id: '',
  api_base_url: '',
  description: '',
  status: 'active',
  is_default: false
})

const messageForm = reactive({
  bot_id: undefined as number | undefined,
  chat_id: '',
  parse_mode: 'text' as 'text' | 'MarkdownV2' | 'HTML',
  content: '',
  disable_web_page_preview: false,
  disable_notification: false
})

const logFilter = reactive({ source: '' })
const logPagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`
})

const botColumns = [
  { title: '名称', dataIndex: 'name', key: 'name', width: 150 },
  { title: 'Token', dataIndex: 'token', key: 'token', width: 200 },
  { title: '默认 Chat ID', dataIndex: 'default_chat_id', key: 'default_chat_id', width: 180 },
  { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
  { title: '状态', dataIndex: 'status', key: 'status', width: 80 },
  { title: '默认', dataIndex: 'is_default', key: 'is_default', width: 100 },
  { title: '操作', key: 'action', width: 200 }
]

const logColumns = [
  { title: '时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: 'Chat ID', dataIndex: 'chat_id', key: 'chat_id', width: 160 },
  { title: '解析模式', dataIndex: 'parse_mode', key: 'parse_mode', width: 120 },
  { title: '来源', dataIndex: 'source', key: 'source', width: 100 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 80 },
  { title: '操作', key: 'action', width: 80 }
]

const formatTime = (time?: string) => (time ? new Date(time).toLocaleString('zh-CN') : '-')

const maskToken = (token: string) => {
  if (!token) return ''
  if (token.length <= 10) return token
  return token.slice(0, 6) + '****' + token.slice(-4)
}

const fetchBots = async () => {
  loadingBots.value = true
  try {
    const res = await telegramBotApi.list()
    if (res.code === 0) {
      const raw = res.data as { list?: TelegramBot[] } | TelegramBot[] | undefined
      botList.value = Array.isArray(raw) ? raw : raw?.list ?? []
    }
  } catch {
    message.error('获取机器人列表失败')
  } finally {
    loadingBots.value = false
  }
}

const fetchLogs = async () => {
  loadingLogs.value = true
  try {
    const res = await telegramLogApi.list(logPagination.current, logPagination.pageSize, logFilter.source)
    if (res.code === 0) {
      const raw = res.data as { list?: TelegramMessageLog[]; total?: number } | TelegramMessageLog[] | undefined
      logList.value = Array.isArray(raw) ? raw : raw?.list ?? []
      logPagination.total = Array.isArray(raw) ? raw.length : raw?.total ?? 0
    }
  } catch {
    message.error('获取日志失败')
  } finally {
    loadingLogs.value = false
  }
}

const onLogTableChange = (pagination: any) => {
  logPagination.current = pagination.current
  logPagination.pageSize = pagination.pageSize
  fetchLogs()
}

const showBotModal = (bot?: TelegramBot) => {
  if (bot) {
    editingBotId.value = bot.id
    Object.assign(editingBot, bot)
  } else {
    editingBotId.value = undefined
    Object.assign(editingBot, {
      name: '',
      token: '',
      default_chat_id: '',
      api_base_url: '',
      description: '',
      status: 'active',
      is_default: false
    })
  }
  botModalVisible.value = true
}

const saveBot = async () => {
  if (!editingBot.name || !editingBot.token) {
    message.warning('请填写名称和 Token')
    return
  }
  savingBot.value = true
  try {
    const res = editingBotId.value
      ? await telegramBotApi.update(editingBotId.value, editingBot)
      : await telegramBotApi.create(editingBot)
    if (res.code === 0) {
      message.success('保存成功')
      botModalVisible.value = false
      fetchBots()
    } else {
      message.error(res.message || '保存失败')
    }
  } catch {
    message.error('保存失败')
  } finally {
    savingBot.value = false
  }
}

const deleteBot = async (id: number) => {
  try {
    const res = await telegramBotApi.delete(id)
    if (res.code === 0) {
      message.success('删除成功')
      fetchBots()
    } else {
      message.error(res.message || '删除失败')
    }
  } catch {
    message.error('删除失败')
  }
}

const setDefaultBot = async (id: number) => {
  try {
    const res = await telegramBotApi.setDefault(id)
    if (res.code === 0) {
      message.success('设置成功')
      fetchBots()
    } else {
      message.error(res.message || '设置失败')
    }
  } catch {
    message.error('设置失败')
  }
}

const testBot = async (id: number) => {
  try {
    const res = await telegramBotApi.test(id)
    if (res.code === 0) {
      const info = res.data as any
      message.success(`连接成功: ${info?.username || info?.first_name || 'OK'}`)
    } else {
      message.error(res.message || '测试失败')
    }
  } catch {
    message.error('测试失败')
  }
}

const sendMessage = async () => {
  if (!messageForm.content) {
    message.warning('请填写消息内容')
    return
  }
  sendingMessage.value = true
  try {
    const res = await telegramApi.sendMessage({
      bot_id: messageForm.bot_id,
      chat_id: messageForm.chat_id || undefined,
      content: messageForm.content,
      parse_mode: messageForm.parse_mode === 'text' ? '' : messageForm.parse_mode,
      disable_web_page_preview: messageForm.disable_web_page_preview,
      disable_notification: messageForm.disable_notification
    })
    if (res.code === 0) {
      message.success('发送成功')
      fetchLogs()
    } else {
      message.error(res.message || '发送失败')
    }
  } catch {
    message.error('发送失败')
  } finally {
    sendingMessage.value = false
  }
}

const viewLogDetail = (log: TelegramMessageLog) => {
  currentLog.value = log
  logDetailVisible.value = true
}

const messageContentPlaceholder = computed(() => {
  if (messageForm.parse_mode === 'MarkdownV2') return '示例：*加粗* _斜体_ `代码`'
  if (messageForm.parse_mode === 'HTML') return '示例：<b>加粗</b> <i>斜体</i> <code>代码</code>'
  return '示例：Hello from DevOps 平台'
})

// ==================== 消息模板 ====================
const templates = ref<MessageTemplate[]>([])
const templateLoading = ref(false)
const templateKeyword = ref('')
const templateModalVisible = ref(false)
const templateSaving = ref(false)

const availableVars = [
  '{{.PipelineName}}', '{{.RunID}}', '{{.Status}}',
  '{{.TriggerBy}}', '{{.GitBranch}}', '{{.GitCommit}}',
  '{{.Duration}}', '{{.URL}}'
]

const defaultTemplateForm = (): Partial<MessageTemplate> => ({
  name: '',
  type: 'telegram',
  content: '',
  description: '',
  is_active: true
})

const templateForm = ref<Partial<MessageTemplate>>(defaultTemplateForm())

const loadTemplates = async () => {
  templateLoading.value = true
  try {
    const res = await templateApi.list({ keyword: templateKeyword.value })
    templates.value = res.data?.list || []
  } finally {
    templateLoading.value = false
  }
}

const openTemplateModal = (record?: MessageTemplate) => {
  templateForm.value = record ? { ...record } : defaultTemplateForm()
  templateModalVisible.value = true
}

const saveTemplate = async () => {
  if (!templateForm.value.name || !templateForm.value.content) {
    message.warning('请填写模板名称和内容')
    return
  }
  templateSaving.value = true
  try {
    if (templateForm.value.id) {
      await templateApi.update(templateForm.value.id, templateForm.value)
      message.success('更新成功')
    } else {
      await templateApi.create(templateForm.value)
      message.success('创建成功')
    }
    templateModalVisible.value = false
    await loadTemplates()
  } finally {
    templateSaving.value = false
  }
}

const deleteTemplate = async (id: number) => {
  await templateApi.delete(id)
  message.success('删除成功')
  await loadTemplates()
}

const insertVar = (v: string) => {
  templateForm.value.content = (templateForm.value.content || '') + v
}

const previewVisible = ref(false)
const previewContent = ref('')

const sampleData = {
  PipelineName: 'frontend-build',
  RunID: 123,
  Status: 'success',
  TriggerBy: 'admin',
  GitBranch: 'main',
  GitCommit: 'abc123def',
  Duration: 120,
  URL: 'https://devops.example.com/pipeline/123'
}

const localRender = (tpl: string) => {
  let result = tpl
  for (const [key, value] of Object.entries(sampleData)) {
    result = result.replace(new RegExp(`\\{\\{\\.${key}\\}\\}`, 'g'), String(value))
  }
  return result
}

const previewTemplate = async (record: MessageTemplate) => {
  try {
    const res = await templateApi.preview({ template_id: record.id, data: sampleData })
    previewContent.value = typeof res.data === 'string' ? res.data : JSON.stringify(res.data, null, 2)
  } catch {
    previewContent.value = localRender(record.content)
  }
  previewVisible.value = true
}

const previewTemplateContent = async () => {
  if (!templateForm.value.content) {
    message.warning('请先填写模板内容')
    return
  }
  try {
    const res = await templateApi.preview({ content: templateForm.value.content, data: sampleData })
    previewContent.value = typeof res.data === 'string' ? res.data : JSON.stringify(res.data, null, 2)
  } catch {
    previewContent.value = localRender(templateForm.value.content!)
  }
  previewVisible.value = true
}

onMounted(() => {
  fetchBots()
  fetchLogs()
  loadTemplates()
})
</script>

<style scoped>
.telegram-message { padding: 0; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header h1 { margin: 0; font-size: 20px; }
.content-preview { background: #f5f5f5; padding: 12px; border-radius: 4px; white-space: pre-wrap; word-break: break-all; max-height: 300px; overflow: auto; }
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
.template-vars { margin-top: 8px; }
.template-vars .label { color: #666; margin-right: 8px; }
.template-vars :deep(.ant-tag) { cursor: pointer; margin: 2px; }
.template-vars :deep(.ant-tag:hover) { background: #e6f7ff; }
.preview-content { background: #f5f5f5; padding: 16px; border-radius: 4px; font-family: Menlo, Consolas, monospace; white-space: pre-wrap; word-break: break-all; max-height: 400px; overflow: auto; margin: 0; }
</style>
