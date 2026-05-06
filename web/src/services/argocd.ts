import request from './api'
import type { ApiResponse } from '../types'

interface PageResponse<T> {
  code: number
  message: string
  data: {
    list: T[]
    total: number
    page: number
    page_size: number
  }
}

// --- Argo CD Instance ---
export interface ArgoCDInstance {
  id?: number
  created_at?: string
  updated_at?: string
  name: string
  server_url: string
  auth_token: string
  insecure: boolean
  is_default: boolean
  status: string
  created_by?: number
}

// --- Argo CD Application ---
export interface ArgoCDApplication {
  id?: number
  created_at?: string
  updated_at?: string
  argocd_instance_id: number
  name: string
  project: string
  repo_url: string
  repo_path: string
  target_revision: string
  dest_server: string
  dest_namespace: string
  sync_status: string
  health_status: string
  sync_policy: string
  last_sync_at?: string
  drift_detected: boolean
  application_id?: number
  application_name: string
  env: string
}

// --- GitOps Repo ---
export interface GitOpsRepo {
  id?: number
  created_at?: string
  updated_at?: string
  name: string
  repo_url: string
  branch: string
  path: string
  auth_type: string
  auth_credential: string
  application_id?: number
  application_name: string
  env: string
  sync_enabled: boolean
  last_commit_hash: string
  last_commit_msg: string
  created_by?: number
}

export interface GitOpsDashboardSummary {
  instance_total: number
  instance_active: number
  app_total: number
  app_synced: number
  app_out_of_sync: number
  app_healthy: number
  app_degraded: number
  app_drifted: number
  app_auto_sync: number
  repo_total: number
  repo_sync_enabled: number
  change_request_open: number
  change_request_draft: number
  change_request_failed: number
}

export interface GitOpsChangeRequest {
  id?: number
  created_at?: string
  updated_at?: string
  gitops_repo_id: number
  argocd_application_id?: number
  application_id?: number
  application_name: string
  env: string
  pipeline_id?: number
  pipeline_run_id?: number
  title: string
  description: string
  file_path: string
  image_repository: string
  image_tag: string
  helm_chart_path?: string
  helm_values_path?: string
  helm_release_name?: string
  replicas?: number
  cpu_request?: string
  cpu_limit?: string
  memory_request?: string
  memory_limit?: string
  source_branch?: string
  target_branch: string
  status?: string
  provider?: string
  merge_request_iid?: string
  merge_request_url?: string
  last_commit_sha?: string
  approval_instance_id?: number
  approval_chain_id?: number
  approval_chain_name?: string
  approval_status?: string
  approval_finished_at?: string
  auto_merge_status?: string
  auto_merged_at?: string
  error_message?: string
  created_by?: number
}

export interface ChangeRequestPrecheckItem {
  key: string
  name: string
  required: boolean
  passed: boolean
  message: string
  detail?: string
}

export interface ChangeRequestPolicySummary {
  env_name: string
  require_approval: boolean
  require_chain: boolean
  require_code_review: boolean
  require_test_pass: boolean
  require_deploy_window: boolean
}

export interface ChangeRequestPrecheck {
  can_create: boolean
  policy?: ChangeRequestPolicySummary
  checks: ChangeRequestPrecheckItem[]
}

// --- Resource Node ---
export interface ResourceNode {
  group: string
  kind: string
  namespace: string
  name: string
  status: string
  health?: { status: string }
}

export const argocdApi = {
  getDashboardSummary: (params?: { project_id?: number }) =>
    request.get<ApiResponse<GitOpsDashboardSummary>>('/argocd/dashboard', { params }),

  // Instance
  listInstances: () =>
    request.get<ApiResponse<ArgoCDInstance[]>>('/argocd/instances'),
  getInstance: (id: number) =>
    request.get<ApiResponse<ArgoCDInstance>>(`/argocd/instances/${id}`),
  createInstance: (data: Partial<ArgoCDInstance>) =>
    request.post<ApiResponse<ArgoCDInstance>>('/argocd/instances', data),
  updateInstance: (id: number, data: Partial<ArgoCDInstance>) =>
    request.put<ApiResponse<ArgoCDInstance>>(`/argocd/instances/${id}`, data),
  deleteInstance: (id: number) =>
    request.delete<ApiResponse<null>>(`/argocd/instances/${id}`),
  testConnection: (id: number) =>
    request.post<ApiResponse<null>>(`/argocd/instances/${id}/test`),
  syncApps: (id: number) =>
    request.post<ApiResponse<null>>(`/argocd/instances/${id}/sync-apps`),

  // Application
  listApps: (params: {
    page?: number
    page_size?: number
    instance_id?: number
    project_id?: number
    sync_status?: string
    health_status?: string
    env?: string
    drift_only?: string
  }) =>
    request.get<PageResponse<ArgoCDApplication>>('/argocd/apps', { params }),
  getApp: (id: number) =>
    request.get<ApiResponse<ArgoCDApplication>>(`/argocd/apps/${id}`),
  triggerSync: (id: number) =>
    request.post<ApiResponse<null>>(`/argocd/apps/${id}/sync`),
  getResources: (id: number) =>
    request.get<ApiResponse<ResourceNode[]>>(`/argocd/apps/${id}/resources`),

  // GitOps Repo
  listRepos: (params: { page?: number; page_size?: number; project_id?: number }) =>
    request.get<PageResponse<GitOpsRepo>>('/argocd/repos', { params }),
  getRepo: (id: number) =>
    request.get<ApiResponse<GitOpsRepo>>(`/argocd/repos/${id}`),
  createRepo: (data: Partial<GitOpsRepo>) =>
    request.post<ApiResponse<GitOpsRepo>>('/argocd/repos', data),
  updateRepo: (id: number, data: Partial<GitOpsRepo>) =>
    request.put<ApiResponse<GitOpsRepo>>(`/argocd/repos/${id}`, data),
  deleteRepo: (id: number) =>
    request.delete<ApiResponse<null>>(`/argocd/repos/${id}`),

  // Change request
  listChangeRequests: (params: { page?: number; page_size?: number; project_id?: number }) =>
    request.get<PageResponse<GitOpsChangeRequest>>('/argocd/change-requests', { params }),
  getChangeRequest: (id: number) =>
    request.get<ApiResponse<GitOpsChangeRequest>>(`/argocd/change-requests/${id}`),
  getChangeRequestByApprovalInstance: (approvalInstanceId: number) =>
    request.get<ApiResponse<GitOpsChangeRequest>>(`/argocd/change-requests/by-approval/${approvalInstanceId}`),
  precheckChangeRequest: (data: Partial<GitOpsChangeRequest>) =>
    request.post<ApiResponse<ChangeRequestPrecheck>>('/argocd/change-requests/precheck', data),
  createChangeRequest: (data: Partial<GitOpsChangeRequest>) =>
    request.post<ApiResponse<GitOpsChangeRequest>>('/argocd/change-requests', data),
}
