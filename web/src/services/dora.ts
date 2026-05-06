import request from './api'
import type { ApiResponse } from '../types'

export interface DORASeriesPoint {
  date: string
  value: number
}

export interface DORAMetric {
  key: 'deploy_freq' | 'lead_time' | 'change_fail_rate' | 'mttr'
  title: string
  value: number
  unit: string
  trend: 'up' | 'down' | 'flat'
  delta: number
  delta_text: string
  benchmark: 'elite' | 'high' | 'medium' | 'low'
  description: string
  sample: number
  series?: DORASeriesPoint[]
  prev_series?: DORASeriesPoint[]
  /** v2.2: 应用下钻对标（未开启下钻时为 0/空） */
  fleet_value?: number
  fleet_benchmark?: 'elite' | 'high' | 'medium' | 'low' | ''
  app_vs_fleet?: 'better' | 'worse' | 'equal' | ''
  app_vs_fleet_text?: string
}

export interface DORASnapshot {
  from: string
  to: string
  env: string
  metrics: DORAMetric[]
}

export interface DORAResponse {
  enabled: boolean
  message?: string
  snapshot?: DORASnapshot
}

export interface DORAQuery {
  env?: string
  days?: number
  from?: string
  to?: string
  /** v2.1: 按 application_id 下钻 */
  application_id?: number
  /** v2.1: 按 application_name 下钻（与 id 二选一） */
  application_name?: string
}

export const doraApi = {
  /** 获取 DORA 四指标快照 */
  get(params: DORAQuery = {}) {
    return request.get<ApiResponse<DORAResponse>>('/metrics/dora', { params })
  },
}
