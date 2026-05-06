import request from './api'
import type { ApiResponse } from '../types'

export interface JiraInstance {
  id?: number
  name: string
  base_url: string
  username: string
  token: string
  auth_type: string
  is_default: boolean
  status: string
  created_at?: string
}

export interface JiraProjectMapping {
  id?: number
  jira_instance_id: number
  jira_project_key: string
  jira_project_name: string
  devops_project_id?: number | null
  devops_app_id?: number | null
}

export const pickDefaultJiraInstance = (instances: JiraInstance[]): JiraInstance | null => {
  const active = (instances || []).filter(item => item.status === 'active' || !item.status)
  return active.find(item => item.is_default) || active[0] || null
}

export const resolveDefaultJiraBaseURL = async (): Promise<string> => {
  const res = await jiraApi.listInstances()
  const instance = pickDefaultJiraInstance(res.data || [])
  return instance?.base_url || ''
}

export const jiraApi = {
  // 实例管理
  listInstances: (): Promise<ApiResponse<JiraInstance[]>> =>
    request.get('/jira/instances'),
  createInstance: (data: Partial<JiraInstance>): Promise<ApiResponse<JiraInstance>> =>
    request.post('/jira/instances', data),
  updateInstance: (id: number, data: Partial<JiraInstance>): Promise<ApiResponse<JiraInstance>> =>
    request.put(`/jira/instances/${id}`, data),
  deleteInstance: (id: number): Promise<ApiResponse<void>> =>
    request.delete(`/jira/instances/${id}`),
  testConnection: (id: number): Promise<ApiResponse> =>
    request.post(`/jira/instances/${id}/test`),

  // 项目映射
  listMappings: (instanceId: number): Promise<ApiResponse<JiraProjectMapping[]>> =>
    request.get(`/jira/instances/${instanceId}/mappings`),
  createMapping: (instanceId: number, data: Partial<JiraProjectMapping>): Promise<ApiResponse<JiraProjectMapping>> =>
    request.post(`/jira/instances/${instanceId}/mappings`, data),
  updateMapping: (id: number, data: Partial<JiraProjectMapping>): Promise<ApiResponse<JiraProjectMapping>> =>
    request.put(`/jira/mappings/${id}`, data),
  deleteMapping: (id: number): Promise<ApiResponse<void>> =>
    request.delete(`/jira/mappings/${id}`),

  // Jira API 代理
  listProjects: (instanceId: number): Promise<ApiResponse<any[]>> =>
    request.get(`/jira/instances/${instanceId}/projects`),
  searchIssues: (instanceId: number, params: { jql: string; start_at?: number; max_results?: number }): Promise<ApiResponse> =>
    request.get(`/jira/instances/${instanceId}/issues`, { params }),
  getIssue: (instanceId: number, key: string): Promise<ApiResponse> =>
    request.get(`/jira/instances/${instanceId}/issues/${key}`),
  getBoards: (instanceId: number, projectKey?: string): Promise<ApiResponse> =>
    request.get(`/jira/instances/${instanceId}/boards`, { params: { project_key: projectKey } }),
  getSprints: (instanceId: number, boardId: number, state?: string): Promise<ApiResponse> =>
    request.get(`/jira/instances/${instanceId}/boards/${boardId}/sprints`, { params: { state } }),
  getSprintIssues: (instanceId: number, sprintId: number, params?: { start_at?: number; max_results?: number }): Promise<ApiResponse> =>
    request.get(`/jira/instances/${instanceId}/sprints/${sprintId}/issues`, { params }),
  addComment: (instanceId: number, key: string, comment: string): Promise<ApiResponse> =>
    request.post(`/jira/instances/${instanceId}/issues/${key}/comment`, { comment }),
  transitionIssue: (instanceId: number, key: string, transitionId: string): Promise<ApiResponse> =>
    request.post(`/jira/instances/${instanceId}/issues/${key}/transition`, { transition_id: transitionId }),
  getTransitions: (instanceId: number, key: string): Promise<ApiResponse> =>
    request.get(`/jira/instances/${instanceId}/issues/${key}/transitions`),
}
