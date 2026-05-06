import request from './api'
import type { ApiResponse, PageResponse } from '../types'

/**
 * Incident（生产事故）前端 API（v2.1）。
 *
 * 与 alert_event 区别：
 *   - alert_event 是瞬时事件流（Prometheus/Grafana 推送）
 *   - incident 是 OnCall 团队认定的"事故"（持久化、可关联发布、MTTR 数据源）
 */
export type IncidentStatus = 'open' | 'mitigated' | 'resolved'
export type IncidentSeverity = 'P0' | 'P1' | 'P2' | 'P3'
export type IncidentSource = 'manual' | 'alert' | 'release_failure'

export interface Incident {
  id?: number
  title: string
  description?: string
  application_id?: number
  application_name?: string
  env: string
  severity: IncidentSeverity
  status: IncidentStatus
  detected_at: string
  mitigated_at?: string
  resolved_at?: string
  source: IncidentSource
  release_id?: number
  alert_fingerprint?: string
  postmortem_url?: string
  root_cause?: string
  created_by?: number
  created_by_name?: string
  resolved_by?: number
  resolved_by_name?: string
  created_at?: string
  updated_at?: string
}

export interface IncidentFilter {
  env?: string
  status?: IncidentStatus
  severity?: IncidentSeverity
  application_id?: number
  project_id?: number
  release_id?: number
  keyword?: string
  from?: string
  to?: string
  page?: number
  pageSize?: number
}

export const incidentApi = {
  list(params: IncidentFilter = {}) {
    return request.get<PageResponse<Incident>>('/incidents', { params })
  },
  getById(id: number) {
    return request.get<ApiResponse<Incident>>(`/incidents/${id}`)
  },
  create(data: Partial<Incident>) {
    return request.post<ApiResponse<Incident>>('/incidents', data)
  },
  update(id: number, data: Partial<Incident>) {
    return request.put<ApiResponse<Incident>>(`/incidents/${id}`, data)
  },
  delete(id: number) {
    return request.delete<ApiResponse<void>>(`/incidents/${id}`)
  },
  mitigate(id: number) {
    return request.post<ApiResponse<Incident>>(`/incidents/${id}/mitigate`)
  },
  resolve(id: number, body?: { root_cause?: string; postmortem_url?: string }) {
    return request.post<ApiResponse<Incident>>(`/incidents/${id}/resolve`, body || {})
  },
  /**
   * 下载 Markdown 复盘文档（v2.2）。
   * 返回原始 Blob，供前端触发下载；responseType=blob 避开 JSON 拦截器。
   */
  exportPostmortem(id: number) {
    return request.get(`/incidents/${id}/postmortem`, {
      params: { format: 'md' },
      responseType: 'blob',
    })
  },
}
