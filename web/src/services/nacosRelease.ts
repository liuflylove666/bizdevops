import request from './api'
import type { ApiResponse, PageResponse } from '../types'

export interface NacosRelease {
  id?: number
  created_at?: string
  updated_at?: string
  title: string
  nacos_instance_id: number
  nacos_instance_name: string
  tenant: string
  group: string
  data_id: string
  env: string
  config_type: string
  content_before: string
  content_after: string
  content_hash: string
  service_id?: number
  service_name: string
  release_id?: number
  status: string
  risk_level: string
  description: string
  created_by: number
  created_by_name: string
  approved_by?: number
  approved_by_name: string
  approved_at?: string
  approval_instance_id?: number
  approval_chain_id?: number
  approval_chain_name?: string
  published_at?: string
  published_by?: number
  published_by_name: string
  rollback_from_id?: number
  reject_reason: string
}

export interface NacosReleaseFilter {
  env?: string
  status?: string
  data_id?: string
  service_id?: number
  page?: number
  page_size?: number
}

export const nacosReleaseApi = {
  list(params: NacosReleaseFilter) {
    return request.get<PageResponse<NacosRelease>>('/nacos-releases', { params })
  },
  getById(id: number) {
    return request.get<ApiResponse<NacosRelease>>(`/nacos-releases/${id}`)
  },
  getByApprovalInstance(approvalInstanceId: number) {
    return request.get<ApiResponse<NacosRelease>>(`/nacos-releases/by-approval/${approvalInstanceId}`)
  },
  create(data: Partial<NacosRelease>) {
    return request.post<ApiResponse<NacosRelease>>('/nacos-releases', data)
  },
  update(id: number, data: Partial<NacosRelease>) {
    return request.put<ApiResponse<NacosRelease>>(`/nacos-releases/${id}`, data)
  },
  delete(id: number) {
    return request.delete<ApiResponse<void>>(`/nacos-releases/${id}`)
  },
  submit(id: number) {
    return request.post<ApiResponse<NacosRelease>>(`/nacos-releases/${id}/submit`)
  },
  approve(id: number) {
    return request.post<ApiResponse<NacosRelease>>(`/nacos-releases/${id}/approve`)
  },
  reject(id: number, reason: string) {
    return request.post<ApiResponse<NacosRelease>>(`/nacos-releases/${id}/reject`, { reason })
  },
  publish(id: number) {
    return request.post<ApiResponse<NacosRelease>>(`/nacos-releases/${id}/publish`)
  },
  rollback(id: number) {
    return request.post<ApiResponse<NacosRelease>>(`/nacos-releases/${id}/rollback`)
  },
  fetchContent(params: { instance_id: number; tenant: string; group: string; data_id: string }) {
    return request.get<ApiResponse<string>>('/nacos-releases/fetch-content', { params })
  },
  listByService(serviceId: number, limit?: number) {
    return request.get<ApiResponse<NacosRelease[]>>(`/nacos-releases/by-service/${serviceId}`, { params: { limit } })
  },
}
