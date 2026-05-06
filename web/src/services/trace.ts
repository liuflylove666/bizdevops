import request from './api'
import type { ApiResponse } from '../types'

export interface TraceRecord {
  trace_id: string
  span_id: string
  parent_id?: string
  operation: string
  service: string
  kind: string
  start_time: string
  end_time: string
  duration_ms: number
  status: string
  error_msg?: string
  attributes?: Record<string, any>
  events?: TraceEvent[]
}

export interface TraceEvent {
  timestamp: string
  name: string
  attributes?: Record<string, any>
}

export interface TraceListResponse {
  total: number
  traces: TraceRecord[]
  limit: number
  offset: number
}

export interface TraceTreeResponse {
  root: TraceRecord
  spans: TraceRecord[]
}

export interface TraceQueryParams {
  service?: string
  operation?: string
  start?: string
  end?: string
  limit?: number
  offset?: number
}

export const traceApi = {
  getStatus: () =>
    request.get<ApiResponse<{ enabled: boolean; endpoint: string }>>('/tracing/status'),

  listServices: () =>
    request.get<ApiResponse<{ services: string[] }>>('/tracing/services'),

  getServiceOperations: (service: string) =>
    request.get<ApiResponse<{ service: string; operations: string[] }>>(`/tracing/services/${service}/operations`),

  queryTraces: (params: TraceQueryParams) =>
    request.get<ApiResponse<TraceListResponse>>('/tracing/traces', { params }),

  getTrace: (traceId: string) =>
    request.get<ApiResponse<TraceRecord>>(`/tracing/traces/${traceId}`),

  getTraceTree: (traceId: string) =>
    request.get<ApiResponse<TraceTreeResponse>>(`/tracing/traces/${traceId}/tree`),
}
