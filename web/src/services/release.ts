import request from './api'
import type { ApiResponse, PageResponse } from '../types'

type RequestPromise<T> = Promise<T>

export interface Release {
  id?: number
  created_at?: string
  updated_at?: string
  title: string
  application_id?: number
  application_name: string
  project_id?: number
  project_name?: string
  env: string
  version: string
  description: string
  status: string
  risk_level: string
  created_by: number
  created_by_name: string
  approved_by?: number
  approved_by_name: string
  approved_at?: string
  published_at?: string
  published_by?: number
  published_by_name: string
  rollback_at?: string
  reject_reason: string
  pipeline_runs?: any[]
  nacos_releases?: any[]
  biz_version_id?: number
  biz_version_name?: string
  biz_goal_id?: number
  biz_goal_name?: string
  // ---------- v2.0 字段 ----------
  rollout_strategy?: 'direct' | 'canary' | 'blue_green'
  rollout_config?: Record<string, any>
  risk_score?: number
  risk_factors?: {
    score: number
    level: string
    hits: Array<{ key: string; name: string; weight: number; detail?: string }>
    calculated: string
    version: string
  }
  approval_instance_id?: number
  gitops_change_request_id?: number
  argo_app_name?: string
  argo_sync_status?: string
  jira_issue_keys?: string
}

// v2.0: GitOps PR 请求 / 响应
export interface GitOpsPRRequest {
  target_branch?: string
  commit_message?: string
  dry_run?: boolean
}

export interface GitOpsPRResponse {
  change_request_id: number
  pr_url?: string
  branch_name: string
  files_changed: string[]
  dry_run: boolean
  message?: string
}

export interface ReleaseItem {
  id?: number
  created_at?: string
  release_id: number
  item_type: string
  item_id: number
  item_title: string
  item_status: string
  sort_order: number
}

export interface ReleaseFilter {
  env?: string
  status?: string
  application_id?: number
  project_id?: number
  title?: string
  page?: number
  page_size?: number
}

export interface CreateReleaseFromPipelineRunRequest {
  pipeline_run_id: number
  existing_release_id?: number
  title?: string
  env?: string
  version?: string
  description?: string
  risk_level?: string
  rollout_strategy?: 'direct' | 'canary' | 'blue_green'
  rollout_config?: Record<string, any>
}

export interface ReleaseOverviewStage {
  key: string
  label: string
  status: 'wait' | 'process' | 'finish' | 'error' | string
  message?: string
}

export interface ReleaseOverview {
  release_id: number
  status: string
  current_stage: string
  blocked: boolean
  blocked_reason?: string
  next_action: string
  approval: {
    instance_id?: number
    status: string
    chain_name?: string
    current_node_order?: number
    started_at?: string
    finished_at?: string
  }
  gitops: {
    change_request_id?: number
    status: string
    mr_url?: string
    approval_status?: string
    auto_merge_status?: string
    error_message?: string
    updated_at?: string
  }
  argocd: {
    application_id?: number
    app_name?: string
    sync_status?: string
    health_status?: string
    drift_detected: boolean
    last_sync_at?: string
  }
  stages: ReleaseOverviewStage[]
}

export interface ReleaseGateResult {
  key: string
  name: string
  category: string
  status: 'pass' | 'warn' | 'block' | 'skip' | string
  severity: string
  policy: string
  blocker: boolean
  message: string
  detail?: Record<string, any>
  evaluated_at: string
}

export interface ReleaseGateSummary {
  release_id: number
  status: 'pass' | 'warn' | 'block' | string
  blocked: boolean
  can_publish: boolean
  block_reasons: string[]
  warn_reasons: string[]
  next_action: string
  evaluated_at: string
  items: ReleaseGateResult[]
}

export const releaseApi = {
  list(params: ReleaseFilter) {
    return request.get('/releases', { params }) as RequestPromise<ApiResponse<PageResponse<Release>>>
  },
  getById(id: number) {
    return request.get(`/releases/${id}`) as RequestPromise<ApiResponse<Release>>
  },
  getOverview(id: number) {
    return request.get(`/releases/${id}/overview`) as RequestPromise<ApiResponse<ReleaseOverview>>
  },
  getGates(id: number) {
    return request.get(`/releases/${id}/gates`) as RequestPromise<ApiResponse<ReleaseGateSummary>>
  },
  refreshGates(id: number) {
    return request.post(`/releases/${id}/gates/refresh`) as RequestPromise<ApiResponse<ReleaseGateSummary>>
  },
  create(data: Partial<Release>) {
    return request.post('/releases', data) as RequestPromise<ApiResponse<Release>>
  },
  createFromPipelineRun(data: CreateReleaseFromPipelineRunRequest) {
    return request.post(`/delivery/pipeline-runs/${data.pipeline_run_id}/release`, data) as RequestPromise<ApiResponse<Release>>
  },
  update(id: number, data: Partial<Release>) {
    return request.put(`/releases/${id}`, data) as RequestPromise<ApiResponse<Release>>
  },
  delete(id: number) {
    return request.delete(`/releases/${id}`) as RequestPromise<ApiResponse<void>>
  },
  listItems(id: number) {
    return request.get(`/releases/${id}/items`) as RequestPromise<ApiResponse<ReleaseItem[]>>
  },
  addItem(id: number, data: { item_type: string; item_id: number; item_title?: string }) {
    return request.post(`/releases/${id}/items`, data) as RequestPromise<ApiResponse<void>>
  },
  removeItem(releaseId: number, itemId: number) {
    return request.delete(`/releases/${releaseId}/items/${itemId}`) as RequestPromise<ApiResponse<void>>
  },
  submit(id: number) {
    return request.post(`/releases/${id}/submit`) as RequestPromise<ApiResponse<Release>>
  },
  approve(id: number) {
    return request.post(`/releases/${id}/approve`) as RequestPromise<ApiResponse<Release>>
  },
  reject(id: number, reason: string) {
    return request.post(`/releases/${id}/reject`, { reason }) as RequestPromise<ApiResponse<Release>>
  },
  publish(id: number) {
    return request.post(`/releases/${id}/publish`) as RequestPromise<ApiResponse<Release>>
  },

  // ---------- v2.0 ----------
  /** 真实触发 GitOps PR 生成 */
  openGitOpsPR(id: number, body: GitOpsPRRequest = {}) {
    return request.post(`/releases/${id}/gitops-pr`, body) as RequestPromise<ApiResponse<GitOpsPRResponse>>
  },
  /** dry-run 预览 GitOps PR 将影响的文件列表（不调用 ArgoCD/Git） */
  dryRunGitOpsPR(id: number, body: GitOpsPRRequest = {}) {
    return request.post(`/releases/${id}/gitops-pr/dry-run`, body) as RequestPromise<ApiResponse<GitOpsPRResponse>>
  },
}
