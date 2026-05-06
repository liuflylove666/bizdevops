import request from './api'
import type { ApiResponse } from '../types'

/**
 * Feature Flag API 客户端。
 * 与后端 /app/api/v1/feature-flags 路由对齐（见 internal/modules/system/handler/feature_flag_handler.go）。
 */

export interface FeatureFlag {
  id: number
  name: string
  display_name?: string
  description?: string
  is_enabled: boolean
  rollout_percentage: number
  tenant_whitelist?: { ids?: number[] } | null
  tenant_blacklist?: { ids?: number[] } | null
  created_at: string
  updated_at: string
}

export interface FeatureFlagListResponse {
  items: FeatureFlag[]
  total: number
}

export interface FeatureFlagCheckResponse {
  name: string
  enabled: boolean
}

export interface FeatureFlagStats {
  total: number
  enabled: number
  disabled: number
  rollout: number
}

// 写入请求体（后端将 tenant_*list 数组包装为 {"ids": [...]} JSON 存储）
export interface FeatureFlagPayload {
  name?: string
  display_name?: string
  description?: string
  is_enabled?: boolean
  rollout_percentage?: number
  tenant_whitelist?: number[]
  tenant_blacklist?: number[]
}

export const featureFlagApi = {
  list: (options?: { silent?: boolean }): Promise<ApiResponse<FeatureFlagListResponse>> => {
    return request.get('/feature-flags', { skipErrorToast: Boolean(options?.silent) })
  },

  get: (name: string): Promise<ApiResponse<FeatureFlag>> => {
    return request.get(`/feature-flags/${encodeURIComponent(name)}`)
  },

  create: (data: FeatureFlagPayload): Promise<ApiResponse<FeatureFlag>> => {
    return request.post('/feature-flags', data)
  },

  update: (name: string, data: FeatureFlagPayload): Promise<ApiResponse<FeatureFlag>> => {
    return request.put(`/feature-flags/${encodeURIComponent(name)}`, data)
  },

  delete: (name: string): Promise<ApiResponse> => {
    return request.delete(`/feature-flags/${encodeURIComponent(name)}`)
  },

  check: (name: string, opts?: { tenantId?: number; userId?: number }): Promise<ApiResponse<FeatureFlagCheckResponse>> => {
    return request.get(`/feature-flags/${encodeURIComponent(name)}/check`, {
      params: { tenant_id: opts?.tenantId, user_id: opts?.userId }
    })
  },

  stats: (): Promise<ApiResponse<FeatureFlagStats>> => {
    return request.get('/feature-flags/stats')
  }
}
