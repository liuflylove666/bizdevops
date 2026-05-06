import request from './api'
import type { ApiResponse } from '../types'

export interface Organization {
  id?: number
  name: string
  display_name: string
  description: string
  owner: string
  status: string
  created_at?: string
}

export interface Project {
  id?: number
  organization_id: number
  name: string
  display_name: string
  description: string
  owner: string
  status: string
  org_name?: string
  created_at?: string
}

export interface ProjectOverview {
  project: Project
  app_count: number
  active_app_count: number
  pipeline_count: number
  release_count: number
  avg_readiness_score: number
  ready_app_count: number
  open_incident_count: number
  failed_pipeline_count: number
  pending_release_count: number
  argocd_app_count: number
  drift_app_count: number
  out_of_sync_app_count: number
  degraded_app_count: number
  apps: Array<{
    id: number
    name: string
    display_name?: string
    owner?: string
    team?: string
    status: string
    readiness_score: number
    readiness_level?: string
    pipeline_count: number
    env_count: number
  }>
  recent_pipelines: Array<{
    id: number
    name: string
    application_name?: string
    env?: string
    status: string
    last_run_status?: string
    last_run_at?: string
  }>
  recent_releases: Array<{
    id: number
    title: string
    application_name: string
    env: string
    status: string
    created_at?: string
    risk_level?: string
    risk_score?: number
  }>
  recent_argocd_apps: Array<{
    id: number
    name: string
    application_name?: string
    env?: string
    sync_status?: string
    health_status?: string
    drift_detected: boolean
    last_sync_at?: string
  }>
  focus_items: Array<{
    key: string
    title: string
    description: string
    severity: string
    path: string
  }>
}

export interface EnvDefinition {
  id?: number
  name: string
  display_name: string
  sort_order: number
  color: string
}

export const catalogApi = {
  listOrgs: (): Promise<ApiResponse<Organization[]>> =>
    request.get('/catalog/orgs'),
  createOrg: (data: Organization): Promise<ApiResponse<Organization>> =>
    request.post('/catalog/orgs', data),
  updateOrg: (id: number, data: Organization): Promise<ApiResponse<Organization>> =>
    request.put(`/catalog/orgs/${id}`, data),
  deleteOrg: (id: number): Promise<ApiResponse> =>
    request.delete(`/catalog/orgs/${id}`),

  listProjects: (orgId?: number): Promise<ApiResponse<Project[]>> =>
    request.get('/catalog/projects', { params: { organization_id: orgId } }),
  getProjectOverview: (id: number): Promise<ApiResponse<ProjectOverview>> =>
    request.get(`/catalog/projects/${id}/overview`),
  createProject: (data: Project): Promise<ApiResponse<Project>> =>
    request.post('/catalog/projects', data),
  updateProject: (id: number, data: Project): Promise<ApiResponse<Project>> =>
    request.put(`/catalog/projects/${id}`, data),
  deleteProject: (id: number): Promise<ApiResponse> =>
    request.delete(`/catalog/projects/${id}`),

  listEnvs: (): Promise<ApiResponse<EnvDefinition[]>> =>
    request.get('/catalog/envs'),
  createEnv: (data: EnvDefinition): Promise<ApiResponse<EnvDefinition>> =>
    request.post('/catalog/envs', data),
  updateEnv: (id: number, data: EnvDefinition): Promise<ApiResponse<EnvDefinition>> =>
    request.put(`/catalog/envs/${id}`, data),
  deleteEnv: (id: number): Promise<ApiResponse> =>
    request.delete(`/catalog/envs/${id}`),
}

export interface ServiceOverview {
  app: any
  envs: any[]
  org_name: string
  project_name: string
  recent_delivery_records: any[]
  delivery_stats: { total: number; success: number; failed: number; success_rate: number; avg_duration: number }
  alert_count: number
  health_status: string
}

export const serviceDetailApi = {
  getOverview: (appId: number): Promise<ApiResponse<ServiceOverview>> =>
    request.get(`/service-detail/${appId}`),
  getAlerts: (appId: number, params?: any): Promise<ApiResponse> =>
    request.get(`/service-detail/${appId}/alerts`, { params }),
  getHealth: (appId: number): Promise<ApiResponse> =>
    request.get(`/service-detail/${appId}/health`),
  getResources: (appId: number): Promise<ApiResponse> =>
    request.get(`/service-detail/${appId}/resources`),
}
