import request from './api'
import type { ApiResponse, PageResponse } from '../types'

export interface ChangeEvent {
  id: number
  created_at: string
  event_type: string
  event_id: number
  title: string
  description: string
  application_id?: number
  application_name: string
  env: string
  status: string
  risk_level: string
  operator: string
  operator_id: number
  metadata: string
}

export interface ChangeEventFilter {
  event_type?: string
  application_id?: number
  env?: string
  status?: string
  operator?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface EventTypeStat {
  event_type: string
  count: number
}

export const changeEventApi = {
  list(params: ChangeEventFilter) {
    return request.get<any, PageResponse<ChangeEvent>>('/change-events', { params })
  },
  stats() {
    return request.get<any, EventTypeStat[]>('/change-events/stats')
  },
  listByApp(appId: number, limit?: number) {
    return request.get<any, ChangeEvent[]>(`/change-events/by-app/${appId}`, { params: { limit } })
  },
}
