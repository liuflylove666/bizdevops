import request from './api'
import type { ApiResponse } from '../types'

export interface NacosInstance {
  id?: number
  name: string
  addr: string
  username: string
  password?: string
  env: string
  description: string
  status: string
  is_default: boolean
  created_at?: string
}

export interface NacosNamespace {
  namespace: string
  namespaceShowName: string
  configCount: number
}

export interface NacosConfigItem {
  id: string
  dataId: string
  group: string
  content?: string
  type: string
  tenant: string
  appName: string
  md5: string
}

export interface ConfigListResult {
  totalCount: number
  pageNumber: number
  pagesAvailable: number
  pageItems: NacosConfigItem[]
}

export interface ConfigHistoryItem {
  id: string
  nid: number
  dataId: string
  group: string
  tenant: string
  content?: string
  opType: string
  createdTime: string
  lastModifiedTime: string
}

export interface HistoryListResult {
  totalCount: number
  pageNumber: number
  pagesAvailable: number
  pageItems: ConfigHistoryItem[]
}

export interface ConfigCompareItem {
  data_id: string
  group: string
  source_content: string
  target_content: string
  same: boolean
}

export const nacosApi = {
  // Instance CRUD
  listInstances: (env?: string): Promise<ApiResponse<NacosInstance[]>> =>
    request.get('/nacos/instances', { params: { env } }),

  getInstance: (id: number): Promise<ApiResponse<NacosInstance>> =>
    request.get(`/nacos/instances/${id}`),

  createInstance: (data: NacosInstance): Promise<ApiResponse<NacosInstance>> =>
    request.post('/nacos/instances', data),

  updateInstance: (id: number, data: NacosInstance): Promise<ApiResponse<NacosInstance>> =>
    request.put(`/nacos/instances/${id}`, data),

  deleteInstance: (id: number): Promise<ApiResponse> =>
    request.delete(`/nacos/instances/${id}`),

  testConnection: (id: number): Promise<ApiResponse> =>
    request.post(`/nacos/instances/${id}/test-connection`),

  // Namespaces
  listNamespaces: (instanceId: number): Promise<ApiResponse<NacosNamespace[]>> =>
    request.get(`/nacos/instances/${instanceId}/namespaces`),

  // Configs
  listConfigs: (instanceId: number, params: { tenant?: string; group?: string; data_id?: string; page?: number; page_size?: number }): Promise<ApiResponse<ConfigListResult>> =>
    request.get(`/nacos/instances/${instanceId}/configs`, { params }),

  getConfig: (instanceId: number, params: { tenant: string; group: string; data_id: string }): Promise<ApiResponse<string>> =>
    request.get(`/nacos/instances/${instanceId}/config`, { params }),

  publishConfig: (instanceId: number, data: { tenant?: string; group: string; data_id: string; content: string; config_type?: string }): Promise<ApiResponse> =>
    request.post(`/nacos/instances/${instanceId}/config`, data),

  deleteConfig: (instanceId: number, params: { tenant: string; group: string; data_id: string }): Promise<ApiResponse> =>
    request.delete(`/nacos/instances/${instanceId}/config`, { params }),

  // History
  listConfigHistory: (instanceId: number, params: { tenant: string; group: string; data_id: string; page?: number; page_size?: number }): Promise<ApiResponse<HistoryListResult>> =>
    request.get(`/nacos/instances/${instanceId}/config/history`, { params }),

  getConfigHistoryDetail: (instanceId: number, params: { tenant: string; group: string; data_id: string; nid: number }): Promise<ApiResponse<ConfigHistoryItem>> =>
    request.get(`/nacos/instances/${instanceId}/config/history/detail`, { params }),

  // Cross-env
  compareConfigs: (data: { source_instance_id: number; target_instance_id: number; source_tenant?: string; target_tenant?: string; group?: string }): Promise<ApiResponse<ConfigCompareItem[]>> =>
    request.post('/nacos/compare', data),

  syncConfig: (data: { source_instance_id: number; target_instance_id: number; source_tenant?: string; target_tenant?: string; group: string; data_id: string }): Promise<ApiResponse> =>
    request.post('/nacos/sync', data),
}
