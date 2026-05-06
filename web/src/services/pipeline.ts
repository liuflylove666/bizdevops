import request from '@/services/api'

export const pipelineApi = {
  // 流水线管理
  list(params?: { name?: string; project_id?: number; status?: string; application_id?: number; application_name?: string; env?: string; page?: number; page_size?: number }) {
    return request.get('/pipelines', { params })
  },

  get(id: number) {
    return request.get(`/pipelines/${id}`)
  },

  create(data: {
    name: string
    description?: string
    project_id?: number
    application_id?: number
    application_name?: string
    env?: string
    source_template_id?: number
    git_repo_id?: number
    git_branch?: string
    gitlab_ci_yaml?: string
    gitlab_ci_yaml_custom?: boolean
    dockerfile_content?: string
    stages?: any[]
    variables?: any[]
    trigger_config?: any
  }) {
    return request.post('/pipelines', data)
  },

  update(id: number, data: {
    name: string
    description?: string
    project_id?: number
    application_id?: number
    application_name?: string
    env?: string
    source_template_id?: number
    git_repo_id?: number
    git_branch?: string
    gitlab_ci_yaml?: string
    gitlab_ci_yaml_custom?: boolean
    dockerfile_content?: string
    stages?: any[]
    variables?: any[]
    trigger_config?: any
  }) {
    return request.put(`/pipelines/${id}`, data)
  },

  delete(id: number) {
    return request.delete(`/pipelines/${id}`)
  },

  toggle(id: number) {
    return request.post(`/pipelines/${id}/toggle`)
  },

  // 流水线执行
  run(id: number, data?: { parameters?: Record<string, string>; branch?: string }) {
    return request.post(`/pipelines/${id}/run`, data)
  },

  // 智能默认（FE-03 / BE-06）：取该 pipeline 上次运行的 ref + parameters
  getLastRunConfig(id: number) {
    return request.get(`/pipelines/${id}/last-run-config`)
  },

  // YAML 导出（FE-11 / BE-12）：DB → IR → YAML 文本
  // 返回原始 YAML 字符串，不包 {code, message, data} 信封。
  exportYAML(id: number, includeLayout = false) {
    return request.get<string>(`/pipeline/${id}/yaml`, {
      params: { include_layout: includeLayout },
      responseType: 'text',
      transformResponse: [(data: any) => data], // 阻止 axios 默认 JSON 解析
    })
  },

  // 末尾日志（FE-08 / BE-13）：列表 hover 预览专用
  getRunLogTail(runId: number, n = 50) {
    return request.get(`/pipeline/runs/${runId}/log/tail`, { params: { n } })
  },

  cancelRun(id: number) {
    return request.post(`/pipelines/runs/${id}/cancel`)
  },

  retryRun(id: number, fromStage?: string) {
    return request.post(`/pipelines/runs/${id}/retry`, null, { params: { from_stage: fromStage } })
  },

  // 执行历史
  listRuns(params?: { pipeline_id?: number; application_id?: number; application_name?: string; status?: string; page?: number; page_size?: number }) {
    return request.get('/pipelines/runs', { params })
  },

  getRun(id: number) {
    return request.get(`/pipelines/runs/${id}`)
  },

  getStepLogs(stepRunId: number) {
    return request.get(`/pipelines/steps/${stepRunId}/logs`)
  },

  // 模板市场
  listTemplates(params?: { category?: string; language?: string; keyword?: string; page?: number; page_size?: number }) {
    return request.get('/pipeline/templates', { params })
  },

  // 凭证
  getCredentials() {
    return request.get('/pipelines/credentials')
  },

  createCredential(data: { name: string; type: string; description?: string; data: string }) {
    return request.post('/pipelines/credentials', data)
  },

  updateCredential(id: number, data: { name: string; type: string; description?: string; data?: string }) {
    return request.put(`/pipelines/credentials/${id}`, data)
  },

  deleteCredential(id: number) {
    return request.delete(`/pipelines/credentials/${id}`)
  },

  // 变量
  getVariables(params?: { scope?: string; pipeline_id?: number }) {
    return request.get('/pipelines/variables', { params })
  },

  createVariable(data: { name: string; value: string; is_secret?: boolean; scope?: string; pipeline_id?: number }) {
    return request.post('/pipelines/variables', data)
  },

  updateVariable(id: number, data: { name: string; value: string; is_secret?: boolean; scope?: string; pipeline_id?: number }) {
    return request.put(`/pipelines/variables/${id}`, data)
  },

  deleteVariable(id: number) {
    return request.delete(`/pipelines/variables/${id}`)
  }
}

// Git 仓库 API
export const gitRepoApi = {
  // 仓库管理
  list(params?: { name?: string; provider?: string; page?: number; page_size?: number }) {
    return request.get('/git/repos', { params })
  },

  get(id: number) {
    return request.get(`/git/repos/${id}`)
  },

  create(data: {
    name: string
    url: string
    provider?: string
    default_branch?: string
    credential_id?: number
    description?: string
  }) {
    return request.post('/git/repos', data)
  },

  update(id: number, data: {
    name: string
    url: string
    provider?: string
    default_branch?: string
    credential_id?: number
    description?: string
  }) {
    return request.put(`/git/repos/${id}`, data)
  },

  delete(id: number) {
    return request.delete(`/git/repos/${id}`)
  },

  // 仓库操作
  testConnection(data: { url: string; credential_id?: number }) {
    return request.post('/git/repos/test', data)
  },

  getBranches(id: number) {
    return request.get(`/git/repos/${id}/branches`)
  },

  getTags(id: number) {
    return request.get(`/git/repos/${id}/tags`)
  },

  regenerateSecret(id: number) {
    return request.post(`/git/repos/${id}/regenerate-secret`)
  }
}
