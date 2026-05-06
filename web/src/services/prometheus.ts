import request from './api'
import type { ApiResponse } from '../types'

export interface PrometheusInstance {
  id?: number
  created_at?: string
  updated_at?: string
  name: string
  url: string
  auth_type: string
  username: string
  password: string
  is_default: boolean
  status: string
  created_by?: number
}

export const prometheusApi = {
  // Instance CRUD
  listInstances: () =>
    request.get<ApiResponse<PrometheusInstance[]>>('/prometheus/instances'),
  getInstance: (id: number) =>
    request.get<ApiResponse<PrometheusInstance>>(`/prometheus/instances/${id}`),
  createInstance: (data: Partial<PrometheusInstance>) =>
    request.post<ApiResponse<PrometheusInstance>>('/prometheus/instances', data),
  updateInstance: (id: number, data: Partial<PrometheusInstance>) =>
    request.put<ApiResponse<PrometheusInstance>>(`/prometheus/instances/${id}`, data),
  deleteInstance: (id: number) =>
    request.delete<ApiResponse<null>>(`/prometheus/instances/${id}`),
  testConnection: (id: number) =>
    request.post<ApiResponse<null>>(`/prometheus/instances/${id}/test`),

  // Query Proxy BFF
  query: (params: { query: string; time?: string; instance_id?: number }) =>
    request.get<ApiResponse<any>>('/prometheus/query', { params }),
  queryRange: (params: { query: string; start: string; end: string; step?: string; instance_id?: number }) =>
    request.get<ApiResponse<any>>('/prometheus/query_range', { params }),
  labels: (params?: { instance_id?: number }) =>
    request.get<ApiResponse<string[]>>('/prometheus/labels', { params }),
  labelValues: (name: string, params?: { instance_id?: number }) =>
    request.get<ApiResponse<string[]>>(`/prometheus/label/${name}/values`, { params }),
  series: (params: { match: string; start?: string; end?: string; instance_id?: number }) =>
    request.get<ApiResponse<any>>('/prometheus/series', { params }),
  targets: (params?: { instance_id?: number }) =>
    request.get<ApiResponse<any>>('/prometheus/targets', { params }),
}
