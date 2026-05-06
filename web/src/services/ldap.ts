import request from './api'
import type { ApiResponse } from '../types'

export interface LDAPConfig {
  enabled: boolean
  server: string
  port: number
  use_tls: boolean
  skip_verify: boolean
  bind_dn: string
  bind_password: string
  base_dn: string
  user_filter: string
  attr_username: string
  attr_email: string
  attr_phone: string
  attr_real_name: string
  group_base_dn: string
  group_filter: string
  group_attr_name: string
  group_attr_member: string
}

export interface LDAPGroupMapping {
  id?: number
  group_dn: string
  group_name: string
  role_id: number
  role_name?: string
}

export const ldapApi = {
  getConfig: (): Promise<ApiResponse<LDAPConfig>> =>
    request.get('/system/ldap/config'),

  saveConfig: (data: LDAPConfig): Promise<ApiResponse> =>
    request.post('/system/ldap/config', data),

  testConnection: (data: LDAPConfig): Promise<ApiResponse> =>
    request.post('/system/ldap/test-connection', data),

  listGroupMappings: (): Promise<ApiResponse<LDAPGroupMapping[]>> =>
    request.get('/system/ldap/group-mappings'),

  createGroupMapping: (data: LDAPGroupMapping): Promise<ApiResponse<LDAPGroupMapping>> =>
    request.post('/system/ldap/group-mappings', data),

  updateGroupMapping: (id: number, data: LDAPGroupMapping): Promise<ApiResponse<LDAPGroupMapping>> =>
    request.put(`/system/ldap/group-mappings/${id}`, data),

  deleteGroupMapping: (id: number): Promise<ApiResponse> =>
    request.delete(`/system/ldap/group-mappings/${id}`),
}
