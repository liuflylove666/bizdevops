import request from './api'

export interface BizGoal {
  id: number
  name: string
  code?: string
  owner?: string
  status: string
  priority: string
  description?: string
  value_metric?: string
  start_date?: string
  end_date?: string
  created_at: string
  updated_at: string
}

export interface BizGoalDetailSummary {
  requirement_total: number
  requirement_backlog: number
  requirement_in_progress: number
  requirement_done: number
  version_total: number
  version_planning: number
  version_in_progress: number
  version_released: number
}

export interface BizRequirement {
  id: number
  external_key?: string
  jira_epic_key?: string
  jira_labels?: string
  jira_components?: string
  goal_id?: number
  version_id?: number
  application_id?: number
  pipeline_id?: number
  title: string
  source: string
  owner?: string
  priority: string
  status: string
  description?: string
  value_score: number
  due_date?: string
  goal_name?: string
  version_name?: string
  application_name?: string
  pipeline_name?: string
  created_at: string
  updated_at: string
}

export interface BizRequirementDetail {
  requirement: BizRequirement
  goal?: BizGoal
  version?: BizVersion
  application?: LinkedApplication
  pipeline?: LinkedPipeline
}

export interface BizVersion {
  id: number
  name: string
  code?: string
  goal_id?: number
  application_id?: number
  pipeline_id?: number
  release_id?: number
  owner?: string
  status: string
  description?: string
  start_date?: string
  release_date?: string
  window_start?: string
  window_end?: string
  goal_name?: string
  application_name?: string
  pipeline_name?: string
  release_title?: string
  created_at: string
  updated_at: string
}

export interface LinkedApplication {
  id: number
  name: string
  display_name?: string
  owner?: string
  team?: string
  status?: string
}

export interface LinkedPipeline {
  id: number
  name: string
  description?: string
  git_branch?: string
  status?: string
}

export interface LinkedRelease {
  id: number
  title: string
  env: string
  version: string
  status: string
  risk_level: string
  application_id?: number
  application_name: string
}

export interface BizVersionDetailSummary {
  requirement_total: number
  requirement_backlog: number
  requirement_in_progress: number
  requirement_done: number
}

export interface BizGoalDetail {
  goal: BizGoal
  requirements: BizRequirement[]
  versions: BizVersion[]
  summary: BizGoalDetailSummary
}

export interface BizVersionDetail {
  version: BizVersion
  goal?: BizGoal
  application?: LinkedApplication
  pipeline?: LinkedPipeline
  release?: LinkedRelease
  requirements: BizRequirement[]
  summary: BizVersionDetailSummary
}

type PageParams = {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

export const bizApi = {
  getPlanningSource() {
    return request({ url: '/biz/planning/source', method: 'get' })
  },

  getGoals(params?: PageParams) {
    return request({ url: '/biz/goals', method: 'get', params })
  },
  getGoal(id: number) {
    return request({ url: `/biz/goals/${id}`, method: 'get' })
  },
  createGoal(data: Partial<BizGoal>) {
    return request({ url: '/biz/goals', method: 'post', data })
  },
  updateGoal(id: number, data: Partial<BizGoal>) {
    return request({ url: `/biz/goals/${id}`, method: 'put', data })
  },
  deleteGoal(id: number) {
    return request({ url: `/biz/goals/${id}`, method: 'delete' })
  },

  getRequirements(params?: PageParams & {
    priority?: string
    source?: string
    goal_id?: number
    version_id?: number
    external_key?: string
    jira_epic_key?: string
    jira_label?: string
    jira_component?: string
  }) {
    return request({ url: '/biz/requirements', method: 'get', params })
  },
  getRequirement(id: number) {
    return request({ url: `/biz/requirements/${id}`, method: 'get' })
  },
  createRequirement(data: Partial<BizRequirement>) {
    return request({ url: '/biz/requirements', method: 'post', data })
  },
  updateRequirement(id: number, data: Partial<BizRequirement>) {
    return request({ url: `/biz/requirements/${id}`, method: 'put', data })
  },
  deleteRequirement(id: number) {
    return request({ url: `/biz/requirements/${id}`, method: 'delete' })
  },

  getVersions(params?: PageParams & { goal_id?: number }) {
    return request({ url: '/biz/versions', method: 'get', params })
  },
  getVersion(id: number) {
    return request({ url: `/biz/versions/${id}`, method: 'get' })
  },
  createVersion(data: Partial<BizVersion>) {
    return request({ url: '/biz/versions', method: 'post', data })
  },
  updateVersion(id: number, data: Partial<BizVersion>) {
    return request({ url: `/biz/versions/${id}`, method: 'put', data })
  },
  deleteVersion(id: number) {
    return request({ url: `/biz/versions/${id}`, method: 'delete' })
  }
}
