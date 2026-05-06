import request from './api'
import type { ApiResponse, PageResponse } from '../types'

export interface EnvInstance {
  id?: number
  created_at?: string
  updated_at?: string
  application_id: number
  application_name: string
  env: string
  cluster_id?: number
  cluster_name: string
  namespace: string
  deployment_name: string
  image_url: string
  image_tag: string
  image_digest: string
  replicas: number
  status: string
  last_deploy_at?: string
  last_deploy_by: string
  nacos_instance_id?: number
  nacos_tenant: string
  nacos_group: string
  db_instance_id?: number
  db_instance_name: string
  config_hash: string
  metadata: string
}

export interface EnvInstanceFilter {
  application_id?: number
  env?: string
  cluster_id?: number
  status?: string
  page?: number
  page_size?: number
}

export const envInstanceApi = {
  list(params: EnvInstanceFilter) {
    return request.get<any, PageResponse<EnvInstance>>('/env-instances', { params })
  },
  getById(id: number) {
    return request.get<any, ApiResponse<EnvInstance>>(`/env-instances/${id}`)
  },
  create(data: Partial<EnvInstance>) {
    return request.post<any, ApiResponse<EnvInstance>>('/env-instances', data)
  },
  update(id: number, data: Partial<EnvInstance>) {
    return request.put<any, ApiResponse<EnvInstance>>(`/env-instances/${id}`, data)
  },
  delete(id: number) {
    return request.delete<any, ApiResponse<void>>(`/env-instances/${id}`)
  },
  listByApp(appId: number) {
    return request.get<any, EnvInstance[]>(`/env-instances/by-app/${appId}`)
  },
  matrix(envs?: string) {
    return request.get<any, EnvInstance[]>('/env-instances/matrix', { params: { envs } })
  },
}
