<template>
  <div class="instance-page">
    <a-card :bordered="false">
      <template #title>
        <div class="page-header">
          <a-button @click="goBack">
            <template #icon><ArrowLeftOutlined /></template>
            返回
          </a-button>
          <span class="title">审批实例详情</span>
          <a-space v-if="linkedTicket">
            <a-button type="primary" ghost @click="goToSQLTicket">查看 SQL 工单</a-button>
          </a-space>
          <a-space v-if="linkedChangeRequest">
            <a-button type="primary" ghost @click="goToGitOpsChange">查看 GitOps 变更</a-button>
            <a-button v-if="linkedChangeRequest.merge_request_url" @click="openMergeRequest">查看 MR</a-button>
          </a-space>
          <a-space v-if="linkedNacosRelease">
            <a-button type="primary" ghost @click="goToNacosRelease">查看 Nacos 发布单</a-button>
          </a-space>
          <a-button v-if="instance?.status === 'pending'" danger @click="handleCancel">取消审批</a-button>
        </div>
      </template>

      <a-spin :spinning="loading">
        <a-card v-if="linkedChangeRequest" size="small" style="margin-bottom: 16px">
          <a-descriptions :column="2" size="small" bordered>
            <a-descriptions-item label="GitOps 变更">{{ linkedChangeRequest.title || '-' }}</a-descriptions-item>
            <a-descriptions-item label="环境">{{ linkedChangeRequest.env || '-' }}</a-descriptions-item>
            <a-descriptions-item label="状态">{{ linkedChangeRequest.status || '-' }}</a-descriptions-item>
            <a-descriptions-item label="自动合并">{{ linkedChangeRequest.auto_merge_status || '-' }}</a-descriptions-item>
            <a-descriptions-item label="清单文件" :span="2">{{ linkedChangeRequest.file_path || '-' }}</a-descriptions-item>
            <a-descriptions-item label="镜像仓库" :span="2">{{ linkedChangeRequest.image_repository || '-' }}</a-descriptions-item>
            <a-descriptions-item label="镜像标签">{{ linkedChangeRequest.image_tag || '-' }}</a-descriptions-item>
            <a-descriptions-item label="目标分支">{{ linkedChangeRequest.target_branch || '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
        <ApprovalInstanceDetail
          v-if="instance"
          :instance="instance"
          @refresh="loadInstance"
        />
        <a-empty v-else-if="!loading" description="实例不存在" />
      </a-spin>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message, Modal } from 'ant-design-vue'
import { ArrowLeftOutlined } from '@ant-design/icons-vue'
import ApprovalInstanceDetail from './ApprovalInstanceDetail.vue'
import { getInstance, cancelInstance, type ApprovalInstance } from '@/services/approvalChain'
import { argocdApi, type GitOpsChangeRequest } from '@/services/argocd'
import { dbTicketApi, type SQLChangeTicket } from '@/services/database'
import { nacosReleaseApi, type NacosRelease } from '@/services/nacosRelease'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const instance = ref<ApprovalInstance | null>(null)
const linkedChangeRequest = ref<GitOpsChangeRequest | null>(null)
const linkedTicket = ref<SQLChangeTicket | null>(null)
const linkedNacosRelease = ref<NacosRelease | null>(null)

const getNacosReleaseByApprovalInstance = (approvalInstanceId: number) =>
  (nacosReleaseApi as any).getByApprovalInstance(approvalInstanceId)

const loadInstance = async () => {
  const id = Number(route.params.id)
  if (!id) return

  loading.value = true
  try {
    const res = await getInstance(id)
    instance.value = res.data || null
    try {
      const crRes = await argocdApi.getChangeRequestByApprovalInstance(id)
      linkedChangeRequest.value = ((crRes as any)?.data ?? crRes ?? null) as GitOpsChangeRequest | null
    } catch {
      linkedChangeRequest.value = null
    }
    try {
      const ticketRes = await dbTicketApi.getByApprovalInstance(id)
      linkedTicket.value = ticketRes.data || null
    } catch {
      linkedTicket.value = null
    }
    try {
      const nrRes = await getNacosReleaseByApprovalInstance(id)
      linkedNacosRelease.value = nrRes.data || null
    } catch {
      linkedNacosRelease.value = null
    }
  } catch (error: any) {
    message.error(error.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const goBack = () => {
  router.push('/approval/instances')
}

const goToGitOpsChange = () => {
  if (!linkedChangeRequest.value?.id) return
  router.push(`/argocd?tab=changes&changeId=${linkedChangeRequest.value.id}`)
}

const goToSQLTicket = () => {
  if (!linkedTicket.value?.id) return
  router.push(`/database/tickets/${linkedTicket.value.id}`)
}

const goToNacosRelease = () => {
  if (!linkedNacosRelease.value?.id) return
  router.push(`/nacos/releases`)
}

const openMergeRequest = () => {
  if (!linkedChangeRequest.value?.merge_request_url) return
  window.open(linkedChangeRequest.value.merge_request_url, '_blank')
}

const handleCancel = () => {
  Modal.confirm({
    title: '取消审批',
    content: '确定要取消此审批实例吗？',
    okType: 'danger',
    onOk: async () => {
      const reason = window.prompt('请输入取消原因（可选）') || ''
      try {
        await cancelInstance(instance.value!.id, reason)
        message.success('已取消')
        loadInstance()
      } catch (error: any) {
        message.error(error.message || '取消失败')
      }
    }
  })
}

onMounted(() => {
  loadInstance()
})
</script>

<style scoped>
.instance-page {
  padding: 16px;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-header .title {
  flex: 1;
  font-size: 16px;
  font-weight: 500;
}
</style>
