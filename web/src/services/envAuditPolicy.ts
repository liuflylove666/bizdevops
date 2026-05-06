import request from './api'
import type { ApiResponse } from '../types'

export interface EnvAuditPolicy {
  id?: number
  env_name: string
  display_name: string
  risk_level: string
  require_approval: boolean
  min_approvers: number
  require_chain: boolean
  default_chain_id?: number | null
  require_deploy_window: boolean
  auto_reject_outside_window: boolean
  require_code_review: boolean
  require_test_pass: boolean
  allow_emergency: boolean
  allow_rollback: boolean
  max_deploys_per_day: number
  enabled: boolean
  created_by?: number
  created_at?: string
  updated_at?: string
}

export const envAuditPolicyApi = {
  list: (): Promise<ApiResponse<EnvAuditPolicy[]>> =>
    request.get('/approval/env-policies'),
  get: (id: number): Promise<ApiResponse<EnvAuditPolicy>> =>
    request.get(`/approval/env-policies/${id}`),
  create: (data: Partial<EnvAuditPolicy>): Promise<ApiResponse<EnvAuditPolicy>> =>
    request.post('/approval/env-policies', data),
  update: (id: number, data: Partial<EnvAuditPolicy>): Promise<ApiResponse<EnvAuditPolicy>> =>
    request.put(`/approval/env-policies/${id}`, data),
  delete: (id: number): Promise<ApiResponse<void>> =>
    request.delete(`/approval/env-policies/${id}`),
  applyPreset: (id: number, preset: string): Promise<ApiResponse<EnvAuditPolicy>> =>
    request.post(`/approval/env-policies/${id}/preset`, { preset }),
}
