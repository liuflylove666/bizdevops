<template>
  <span>
    <a-button
      type="primary"
      :loading="loading"
      :disabled="!canTrigger"
      @click="onClickDryRun"
    >
      <template #icon><BranchesOutlined /></template>
      {{ t('release.gitopsPR.dryRunButton') }}
    </a-button>

    <a-modal
      v-model:open="visible"
      :title="t('release.gitopsPR.previewTitle')"
      :width="640"
      :ok-text="t('release.gitopsPR.confirmOpen')"
      :cancel-text="t('common.cancel')"
      :confirm-loading="opening"
      :ok-button-props="{ disabled: !preview || !!preview?.dry_run === false }"
      @ok="onConfirmOpen"
    >
      <a-spin :spinning="loading">
        <a-descriptions
          v-if="preview"
          :column="1"
          size="small"
          bordered
        >
          <a-descriptions-item :label="t('release.gitopsPR.branch')">
            <code>{{ preview.branch_name }}</code>
          </a-descriptions-item>
          <a-descriptions-item :label="t('release.gitopsPR.filesChanged')">
            <ul class="files-list">
              <li v-for="f in preview.files_changed" :key="f">{{ f }}</li>
            </ul>
            <a-empty
              v-if="!preview.files_changed?.length"
              :description="t('release.gitopsPR.noFiles')"
              :image="emptyImage"
            />
          </a-descriptions-item>
          <a-descriptions-item :label="t('release.gitopsPR.message')">
            {{ preview.message || '-' }}
          </a-descriptions-item>
        </a-descriptions>

        <div class="commit-msg-input">
          <a-form-item :label="t('release.gitopsPR.commitMessage')">
            <a-textarea
              v-model:value="commitMessage"
              :rows="3"
              :placeholder="t('release.gitopsPR.commitMessagePlaceholder')"
            />
          </a-form-item>
        </div>
      </a-spin>
    </a-modal>
  </span>
</template>

<script setup lang="ts">
/**
 * ReleaseGitOpsPRButton
 *
 * Release 详情/列表页通用的 "Generate GitOps PR" 触发组件。
 *
 * 工作流：
 *   1. 用户点击按钮 → 调 dry-run API 拿到 branch/files 预览
 *   2. 弹窗展示预览结果，用户确认后调真实 API
 *   3. 预览确认后触发真实创建
 *
 * 触发条件：Release 状态必须为 approved，否则按钮 disabled
 */
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { message, Empty } from 'ant-design-vue'
import { BranchesOutlined } from '@ant-design/icons-vue'
import { releaseApi, type GitOpsPRResponse } from '@/services/release'

const props = defineProps<{
  releaseId: number
  releaseStatus: string
  /** 可选：传入 PR 目标分支，默认由后端推导 */
  targetBranch?: string
}>()

const emit = defineEmits<{
  (e: 'opened', resp: GitOpsPRResponse): void
}>()

const { t } = useI18n()
const visible = ref(false)
const loading = ref(false)
const opening = ref(false)
const preview = ref<GitOpsPRResponse | null>(null)
const commitMessage = ref('')
const emptyImage = Empty.PRESENTED_IMAGE_SIMPLE

// 仅 approved 状态可触发
const canTrigger = computed(() => props.releaseStatus === 'approved')

async function onClickDryRun() {
  if (!canTrigger.value) {
    message.warning(t('release.gitopsPR.requireApproved'))
    return
  }
  loading.value = true
  visible.value = true
  preview.value = null
  try {
    const res = await releaseApi.dryRunGitOpsPR(props.releaseId, {
      target_branch: props.targetBranch,
      dry_run: true,
    })
    preview.value = res?.data || null
  } catch (e: any) {
    message.error(e?.response?.data?.message || e?.message || 'dry-run 失败')
    visible.value = false
  } finally {
    loading.value = false
  }
}

async function onConfirmOpen() {
  opening.value = true
  try {
    const res = await releaseApi.openGitOpsPR(props.releaseId, {
      target_branch: props.targetBranch,
      commit_message: commitMessage.value || undefined,
      dry_run: false,
    })
    if (res?.data) {
      message.success(res.data.message || t('release.gitopsPR.openedSuccess'))
      emit('opened', res.data)
      visible.value = false
      commitMessage.value = ''
    }
  } catch (e: any) {
    message.error(e?.response?.data?.message || e?.message || '创建 GitOps PR 失败')
  } finally {
    opening.value = false
  }
}
</script>

<style scoped>
.files-list {
  margin: 0;
  padding-left: 20px;
  max-height: 200px;
  overflow-y: auto;
}
.files-list li {
  font-family: 'SF Mono', Monaco, Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
}
.commit-msg-input {
  margin-top: 12px;
}
</style>
