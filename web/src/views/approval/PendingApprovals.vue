<template>
  <div class="pending-approvals">
    <a-card title="审批链待审批" :bordered="false">
      <a-table
        :columns="chainColumns"
        :data-source="chainRecords"
        :loading="chainLoading"
        row-key="id"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'chain_name'">
            <span>{{ record.chain_name }}</span>
          </template>
          <template v-else-if="column.key === 'current_node'">
            <a-tag color="processing">
              第 {{ record.current_node_order }} 节点
            </a-tag>
            <span v-if="getCurrentNodeName(record)" style="margin-left: 8px; color: #666;">
              {{ getCurrentNodeName(record) }}
            </span>
          </template>
          <template v-else-if="column.key === 'status'">
            <a-tag :color="getChainStatusColor(record.status)">
              {{ getChainStatusText(record.status) }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'started_at'">
            {{ formatTime(record.started_at) }}
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button type="primary" size="small" @click="showChainApproveModal(record)">
                审批
              </a-button>
              <a-button type="link" size="small" @click="showChainDetail(record)">
                详情
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <!-- 审批链审批弹窗 -->
    <a-modal
      v-model:open="chainApproveModalVisible"
      title="审批操作"
      width="600px"
      :footer="null"
    >
      <div v-if="currentChainRecord">
        <a-descriptions :column="2" bordered size="small" style="margin-bottom: 16px;">
          <a-descriptions-item label="审批链">{{ currentChainRecord.chain_name }}</a-descriptions-item>
          <a-descriptions-item label="交付记录">{{ currentChainRecord.record_id }}</a-descriptions-item>
          <a-descriptions-item label="当前节点">
            第 {{ currentChainRecord.current_node_order }} 节点
          </a-descriptions-item>
          <a-descriptions-item label="开始时间">{{ formatTime(currentChainRecord.started_at) }}</a-descriptions-item>
        </a-descriptions>

        <!-- 当前节点信息 -->
        <div v-if="currentNodeInstance" class="current-node-info">
          <h4>当前审批节点：{{ currentNodeInstance.node_name }}</h4>
          <p>审批模式：{{ getModeLabel(currentNodeInstance.approve_mode, currentNodeInstance.approve_count) }}</p>
          <p>审批进度：{{ currentNodeInstance.approved_count }} 通过 / {{ currentNodeInstance.rejected_count }} 拒绝</p>
        </div>

        <a-divider />

        <a-form :label-col="{ span: 4 }">
          <a-form-item label="审批意见">
            <a-textarea v-model:value="chainApproveComment" placeholder="可选，填写审批意见" :rows="3" />
          </a-form-item>
        </a-form>

        <div style="text-align: right; margin-top: 16px;">
          <a-space>
            <a-button @click="chainApproveModalVisible = false">取消</a-button>
            <a-button type="primary" danger @click="handleChainReject" :loading="submitting">
              拒绝
            </a-button>
            <a-button type="primary" @click="handleChainApprove" :loading="submitting">
              通过
            </a-button>
          </a-space>
        </div>
      </div>
    </a-modal>

    <!-- 审批链详情弹窗 -->
    <a-modal
      v-model:open="chainDetailModalVisible"
      title="审批详情"
      width="800px"
      :footer="null"
    >
      <ApprovalInstanceDetail
        v-if="currentChainRecord"
        :instance="currentChainRecord"
        @refresh="loadChainDetail"
      />
    </a-modal>

  </div>
</template>


<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import {
  getPendingApprovals,
  getInstance,
  approveNode,
  rejectNode,
  type ApprovalInstance
} from '@/services/approvalChain'
import ApprovalInstanceDetail from './ApprovalInstanceDetail.vue'
import dayjs from 'dayjs'

// 审批链相关
const chainLoading = ref(false)
const chainRecords = ref<ApprovalInstance[]>([])
const currentChainRecord = ref<ApprovalInstance | null>(null)
const chainApproveModalVisible = ref(false)
const chainDetailModalVisible = ref(false)
const chainApproveComment = ref('')

const submitting = ref(false)

// 审批链列表列
const chainColumns = [
  { title: '审批链', key: 'chain_name', dataIndex: 'chain_name' },
  { title: '交付记录', dataIndex: 'record_id', width: 100 },
  { title: '当前节点', key: 'current_node', width: 200 },
  { title: '状态', key: 'status', width: 100 },
  { title: '开始时间', key: 'started_at', width: 180 },
  { title: '操作', key: 'action', width: 150 }
]

// 当前节点实例
const currentNodeInstance = computed(() => {
  if (!currentChainRecord.value?.node_instances) return null
  return currentChainRecord.value.node_instances.find(
    n => n.node_order === currentChainRecord.value!.current_node_order && n.status === 'active'
  )
})

// 获取当前节点名称
const getCurrentNodeName = (record: ApprovalInstance) => {
  if (!record.node_instances) return ''
  const node = record.node_instances.find(n => n.node_order === record.current_node_order)
  return node?.node_name || ''
}

// 颜色和文本映射
const getChainStatusColor = (status: string) => {
  const colors: Record<string, string> = {
    pending: 'processing', approved: 'success', rejected: 'error', cancelled: 'default'
  }
  return colors[status] || 'default'
}

const getChainStatusText = (status: string) => {
  const texts: Record<string, string> = {
    pending: '审批中', approved: '已通过', rejected: '已拒绝', cancelled: '已取消'
  }
  return texts[status] || status
}

const getModeLabel = (mode: string, count: number) => {
  const map: Record<string, string> = {
    any: '任一人通过',
    all: '所有人通过',
    count: `${count}人通过`
  }
  return map[mode] || mode
}

const formatTime = (time: string) => {
  return time ? dayjs(time).format('YYYY-MM-DD HH:mm:ss') : '-'
}

// 加载审批链待审批列表
const loadChainRecords = async () => {
  chainLoading.value = true
  try {
    const res = await getPendingApprovals()
    chainRecords.value = res.data || []
  } catch (error) {
    console.error('加载审批链待审批列表失败:', error)
  } finally {
    chainLoading.value = false
  }
}

// 加载审批链详情
const loadChainDetail = async () => {
  if (!currentChainRecord.value) return
  try {
    const res = await getInstance(currentChainRecord.value.id)
    currentChainRecord.value = res.data
  } catch (error) {
    console.error('加载审批链详情失败:', error)
  }
}

// 审批链操作
const showChainApproveModal = async (record: ApprovalInstance) => {
  try {
    const res = await getInstance(record.id)
    currentChainRecord.value = res.data
    chainApproveComment.value = ''
    chainApproveModalVisible.value = true
  } catch (error: any) {
    message.error(error.message || '加载详情失败')
  }
}

const showChainDetail = async (record: ApprovalInstance) => {
  try {
    const res = await getInstance(record.id)
    currentChainRecord.value = res.data
    chainDetailModalVisible.value = true
  } catch (error: any) {
    message.error(error.message || '加载详情失败')
  }
}

const handleChainApprove = async () => {
  if (!currentNodeInstance.value) {
    message.error('未找到当前审批节点')
    return
  }
  submitting.value = true
  try {
    await approveNode(currentNodeInstance.value.id, chainApproveComment.value)
    message.success('审批通过')
    chainApproveModalVisible.value = false
    loadChainRecords()
  } catch (error: any) {
    message.error(error.message || '审批失败')
  } finally {
    submitting.value = false
  }
}

const handleChainReject = async () => {
  if (!currentNodeInstance.value) {
    message.error('未找到当前审批节点')
    return
  }
  if (!chainApproveComment.value) {
    message.error('请填写拒绝原因')
    return
  }
  submitting.value = true
  try {
    await rejectNode(currentNodeInstance.value.id, chainApproveComment.value)
    message.success('已拒绝')
    chainApproveModalVisible.value = false
    loadChainRecords()
  } catch (error: any) {
    message.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadChainRecords()
})
</script>

<style scoped>
.pending-approvals {
  padding: 16px;
}

.current-node-info {
  background: #f5f5f5;
  padding: 12px 16px;
  border-radius: 4px;
  margin-bottom: 16px;
}

.current-node-info h4 {
  margin: 0 0 8px 0;
  color: #1890ff;
}

.current-node-info p {
  margin: 4px 0;
  color: #666;
}
</style>
