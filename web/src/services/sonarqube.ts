import request from '@/services/api'

export interface SonarQubeInstance {
  id: number
  created_at?: string
  updated_at?: string
  name: string
  base_url: string
  token: string
  is_default: boolean
  status: string
}

export interface SonarQubeProjectBinding {
  id: number
  sonarqube_id: number
  sonar_project_key: string
  sonar_project_name: string
  devops_app_id?: number
  devops_app_name: string
  quality_gate_status: string
}

export interface SonarProject {
  key: string
  name: string
  qualifier: string
  visibility: string
}

export interface QualityGate {
  status: string
  conditions: { status: string; metricKey: string; comparator: string; errorThreshold: string; actualValue: string }[]
}

export interface SonarMeasure {
  metric: string
  value: string
}

export interface SonarIssue {
  key: string
  rule: string
  severity: string
  component: string
  line: number
  message: string
  status: string
  type: string
}

export const sonarqubeApi = {
  // 实例
  listInstances: () => request.get('/sonarqube/instances'),
  getInstance: (id: number) => request.get(`/sonarqube/instances/${id}`),
  createInstance: (data: Partial<SonarQubeInstance>) => request.post('/sonarqube/instances', data),
  updateInstance: (id: number, data: Partial<SonarQubeInstance>) => request.put(`/sonarqube/instances/${id}`, data),
  deleteInstance: (id: number) => request.delete(`/sonarqube/instances/${id}`),
  testConnection: (id: number) => request.post(`/sonarqube/instances/${id}/test`),

  // 绑定
  listBindings: (instanceId: number) => request.get(`/sonarqube/instances/${instanceId}/bindings`),
  createBinding: (instanceId: number, data: Partial<SonarQubeProjectBinding>) =>
    request.post(`/sonarqube/instances/${instanceId}/bindings`, data),
  updateBinding: (bindingId: number, data: Partial<SonarQubeProjectBinding>) =>
    request.put(`/sonarqube/bindings/${bindingId}`, data),
  deleteBinding: (bindingId: number) => request.delete(`/sonarqube/bindings/${bindingId}`),

  // SonarQube API 代理
  listProjects: (instanceId: number, page?: number, pageSize?: number) =>
    request.get(`/sonarqube/instances/${instanceId}/projects`, { params: { page, pageSize } }),
  getQualityGate: (instanceId: number, projectKey: string) =>
    request.get(`/sonarqube/instances/${instanceId}/quality-gate`, { params: { projectKey } }),
  getMeasures: (instanceId: number, projectKey: string) =>
    request.get(`/sonarqube/instances/${instanceId}/measures`, { params: { projectKey } }),
  getIssues: (instanceId: number, projectKey: string, page?: number, pageSize?: number, severities?: string) =>
    request.get(`/sonarqube/instances/${instanceId}/issues`, { params: { projectKey, page, pageSize, severities } }),
}
