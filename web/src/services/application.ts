import request from './api'
import type { ApiResponse } from '../types'

export interface Application {
  id?: number
  created_at?: string
  updated_at?: string
  name: string
  display_name?: string
  description?: string
  organization_id?: number
  project_id?: number
  git_repo?: string
  language?: string
  framework?: string
  team?: string
  owner?: string
  org_name?: string
  project_name?: string
  status: string
  notify_platform?: string
  notify_app_id?: number
  notify_receive_id?: string
  notify_receive_type?: string
}

export interface ApplicationEnv {
  id?: number
  application_id: number
  env_name: string
  branch?: string
  gitops_repo_id?: number
  argocd_application_id?: number
  gitops_branch?: string
  gitops_path?: string
  helm_chart_path?: string
  helm_values_path?: string
  helm_release_name?: string
  k8s_cluster_id?: number
  k8s_namespace?: string
  k8s_deployment?: string
  replicas?: number
  cpu_request?: string
  cpu_limit?: string
  memory_request?: string
  memory_limit?: string
  config?: string
}

export interface DeliveryRecord {
  id: number
  created_at: string
  application_id: number
  app_name: string
  env_name: string
  version: string
  branch: string
  commit_id: string
  image_tag?: string
  deploy_type: string
  deploy_method?: string
  status: string
  duration: number
  error_msg?: string
  need_approval?: boolean
  approval_chain_id?: number
  operator: string
  started_at?: string
  finished_at?: string
}

export interface ApplicationRepoBinding {
  id: number
  application_id: number
  git_repo_id: number
  role: string
  is_default: boolean
  created_at?: string
  updated_at?: string
  repo_name?: string
  repo_url?: string
  repo_provider?: string
  default_branch?: string
}

export interface AppStats {
  app_count: number
  team_stats: { name: string; count: number }[]
  lang_stats: { name: string; count: number }[]
  today_deliveries: number
  week_deliveries: number
  success_rate: number
}

export interface ApplicationReadinessCheck {
  key: string
  title: string
  description: string
  status: 'pass' | 'missing' | string
  severity: 'info' | 'low' | 'medium' | 'high' | string
  path?: string
}

export interface ApplicationReadinessAction {
  key: string
  title: string
  path: string
  weight: number
}

export interface ApplicationReadiness {
  application_id: number
  application_name: string
  score: number
  level: string
  completed: number
  total: number
  checks: ApplicationReadinessCheck[]
  next_actions: ApplicationReadinessAction[]
  generated_at: string
}

export interface ApplicationOnboardingRequest {
  application_id?: number
  app: Partial<Application>
  repo?: {
    git_repo_id?: number
    name?: string
    url?: string
    provider?: string
    default_branch?: string
    role?: string
    is_default?: boolean
  }
  env?: Partial<ApplicationEnv>
  pipeline?: {
    pipeline_id?: number
    create?: boolean
    name?: string
    description?: string
    env?: string
    source_template_id?: number
    git_repo_id?: number
    git_branch?: string
  }
}

export interface ApplicationOnboardingResponse {
  application_id: number
  application_name: string
  created: boolean
  repo_binding_id?: number
  git_repo_id?: number
  env_id?: number
  pipeline_id?: number
  updated_sections: string[]
  readiness?: ApplicationReadiness
}

export interface DeployWindowStatus {
  in_window: boolean
  message: string
  next_window?: string
}

export interface DeployLockStatus {
  locked: boolean
  locked_by?: string
  locked_at?: string
}

export interface ApprovalRequiredStatus {
  required: boolean
  approvers?: string[]
}

export const getDeployWindowStatus = (appId: number, envName: string): Promise<ApiResponse<DeployWindowStatus>> => {
  return request.get(`/approval/windows/check`, {
    params: { app_id: appId, env: envName },
    skipErrorToast: true,
  })
}

export const getDeployLockStatus = (appId: number, envName: string): Promise<ApiResponse<DeployLockStatus>> => {
  return request.get(`/deploy/locks/check`, {
    params: { app_id: appId, env: envName },
    skipErrorToast: true,
  })
}

export const checkApprovalRequired = (appId: number, envName: string): Promise<ApiResponse<ApprovalRequiredStatus>> => {
  return request.get(`/approval/check`, {
    params: { app_id: appId, env: envName },
    skipErrorToast: true,
  })
}

export const applicationApi = {
  // 应用管理
  list: (params?: { page?: number; page_size?: number; name?: string; team?: string; status?: string; language?: string; organization_id?: number; project_id?: number }): Promise<ApiResponse<{ list: Application[]; total: number }>> => {
    return request.get('/app', { params })
  },

  get: (id: number): Promise<ApiResponse<{ app: Application; envs: ApplicationEnv[]; repo_bindings?: ApplicationRepoBinding[]; default_repo_binding?: ApplicationRepoBinding }>> => {
    return request.get(`/app/${id}`)
  },

  create: (data: Partial<Application>): Promise<ApiResponse<Application>> => {
    return request.post('/app', data)
  },

  update: (id: number, data: Partial<Application>): Promise<ApiResponse<Application>> => {
    return request.put(`/app/${id}`, data)
  },

  delete: (id: number): Promise<ApiResponse> => {
    return request.delete(`/app/${id}`)
  },

  saveOnboarding: (data: ApplicationOnboardingRequest): Promise<ApiResponse<ApplicationOnboardingResponse>> => {
    return request.post('/app/onboarding', data)
  },

  // 环境管理
  listEnvs: (appId: number): Promise<ApiResponse<ApplicationEnv[]>> => {
    return request.get(`/app/${appId}/envs`)
  },

  getReadiness: (appId: number): Promise<ApiResponse<ApplicationReadiness>> => {
    return request.get(`/app/${appId}/readiness`, { skipErrorToast: true })
  },

  refreshReadiness: (appId: number): Promise<ApiResponse<ApplicationReadiness>> => {
    return request.post(`/app/${appId}/readiness/run`, undefined, { skipErrorToast: true })
  },

  listRepoBindings: (appId: number): Promise<ApiResponse<ApplicationRepoBinding[]>> => {
    return request.get(`/app/${appId}/repo-bindings`)
  },

  bindRepo: (appId: number, data: { git_repo_id: number; role?: string; is_default?: boolean }): Promise<ApiResponse<ApplicationRepoBinding>> => {
    return request.post(`/app/${appId}/repo-bindings`, data)
  },

  setDefaultRepoBinding: (appId: number, bindingId: number): Promise<ApiResponse> => {
    return request.put(`/app/${appId}/repo-bindings/${bindingId}/default`)
  },

  deleteRepoBinding: (appId: number, bindingId: number): Promise<ApiResponse> => {
    return request.delete(`/app/${appId}/repo-bindings/${bindingId}`)
  },

  createEnv: (appId: number, data: Partial<ApplicationEnv>): Promise<ApiResponse<ApplicationEnv>> => {
    return request.post(`/app/${appId}/envs`, data)
  },

  updateEnv: (appId: number, envId: number, data: Partial<ApplicationEnv>): Promise<ApiResponse<ApplicationEnv>> => {
    return request.put(`/app/${appId}/envs/${envId}`, data)
  },

  deleteEnv: (appId: number, envId: number): Promise<ApiResponse> => {
    return request.delete(`/app/${appId}/envs/${envId}`)
  },

  // 交付记录
  listDeliveryRecords: (appId: number, params?: { page?: number; page_size?: number; env_name?: string; status?: string }): Promise<ApiResponse<{ list: DeliveryRecord[]; total: number }>> => {
    return request.get(`/app/${appId}/delivery-records`, { params })
  },

  listAllDeliveryRecords: (params?: { page?: number; page_size?: number; app_name?: string; env?: string; status?: string }): Promise<ApiResponse<{ list: DeliveryRecord[]; total: number }>> => {
    return request.get('/app/delivery-records', { params })
  },

  // 统计
  getStats: (): Promise<ApiResponse<AppStats>> => {
    return request.get('/app/stats')
  },

  getTeams: (): Promise<ApiResponse<string[]>> => {
    return request.get('/app/teams')
  },

  getDeployWindowStatus,
  getDeployLockStatus,
  checkApprovalRequired,
}
