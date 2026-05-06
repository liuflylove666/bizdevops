import request from './api'
import type { ApiResponse } from '../types'

// ==================== 部署前置检查 ====================

export interface PreCheckItem {
  name: string
  status: 'passed' | 'warning' | 'failed' | 'skipped'
  message: string
  detail?: string
}

export interface DeployPreCheckRequest {
  application_id: number
  env_name: string
  image_tag?: string
}

export interface DeployPreCheckResponse {
  can_deploy: boolean
  checks: PreCheckItem[]
  warnings: string[]
  errors: string[]
}

export const deployCheckApi = {
  // 部署前置检查
  preCheck: (data: DeployPreCheckRequest): Promise<ApiResponse<DeployPreCheckResponse>> => {
    return request.post('/deploy/pre-check', data)
  }
}
