import request from './api'
import type { ApiResponse } from '../types'

export interface TimelineItem {
  kind: 'incident' | 'change_event' | 'release' | 'alert' | 'approval'
  id: number
  at: string
  title: string
  summary?: string
  status?: string
  severity?: string
  env?: string
  ref: string
  meta?: Record<string, unknown>
}

export interface TimelineResponse {
  items: TimelineItem[]
  from: string
  to: string
  truncated: boolean
}

export const observabilityTimelineApi = {
  get(params: {
    application_id?: number
    env?: string
    from?: string
    to?: string
    limit?: number
  }): Promise<ApiResponse<TimelineResponse>> {
    return request.get('/observability/timeline', { params })
  },
}
