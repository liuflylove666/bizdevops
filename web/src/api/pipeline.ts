import request from '@/services/api'
import type {
  ArtifactVersion,
  ArtifactScanResult,
  PipelineTemplate,
  PaginationParams,
  PaginationResult,
} from '@/types/pipeline'

// ==================== 制品版本 API ====================

/**
 * 获取制品版本列表
 */
export const getArtifactVersions = (artifactId: number, params?: PaginationParams) => {
  return request.get<PaginationResult<ArtifactVersion>>(`/artifacts/${artifactId}/versions`, { params })
}

/**
 * 获取版本详情
 */
export const getArtifactVersionDetail = (versionId: number) => {
  return request.get<ArtifactVersion>(`/artifacts/versions/${versionId}`)
}

/**
 * 对比两个版本
 */
export const compareArtifactVersions = (versionId1: number, versionId2: number) => {
  return request.get(`/artifacts/versions/compare`, {
    params: { version1: versionId1, version2: versionId2 }
  })
}

/**
 * 删除制品版本
 */
export const deleteArtifactVersion = (versionId: number) => {
  return request.delete(`/artifacts/versions/${versionId}`)
}

// ==================== 制品扫描 API ====================

/**
 * 获取扫描结果
 */
export const getArtifactScanResult = (versionId: number) => {
  return request.get<ArtifactScanResult>(`/artifacts/versions/${versionId}/scan`)
}

/**
 * 触发扫描
 */
export const triggerArtifactScan = (versionId: number) => {
  return request.post(`/artifacts/versions/${versionId}/scan`)
}

/**
 * 获取漏洞列表
 */
export const getVulnerabilities = (versionId: number, params?: {
  severity?: string
  page?: number
  page_size?: number
}) => {
  return request.get(`/artifacts/versions/${versionId}/vulnerabilities`, { params })
}

// ==================== 流水线模板 API ====================

/**
 * 获取模板列表
 */
export const getTemplateList = (params?: PaginationParams & {
  category?: string
  keyword?: string
  order_by?: string
  tags?: string
  favorites_only?: boolean
}) => {
  return request.get<PaginationResult<PipelineTemplate>>('/pipeline/templates', { params })
}

/**
 * 获取模板详情
 */
export const getTemplateDetail = (id: number) => {
  return request.get<PipelineTemplate>(`/pipeline/templates/${id}`)
}

/**
 * 创建模板
 */
export const createTemplate = (data: Partial<PipelineTemplate>) => {
  return request.post<PipelineTemplate>('/pipeline/templates', data)
}

/**
 * 更新模板
 */
export const updateTemplate = (id: number, data: Partial<PipelineTemplate>) => {
  return request.put<PipelineTemplate>(`/pipeline/templates/${id}`, data)
}

/**
 * 删除模板
 */
export const deleteTemplate = (id: number) => {
  return request.delete(`/pipeline/templates/${id}`)
}

/**
 * 使用模板
 */
export const useTemplate = (id: number) => {
  return request.post(`/pipeline/templates/${id}/use`)
}

/**
 * 评分模板
 */
export const rateTemplate = (id: number, rating: number) => {
  return request.post(`/pipeline/templates/${id}/rate`, { rating })
}

/**
 * 收藏模板
 */
export const favoriteTemplate = (id: number) => {
  return request.post(`/pipeline/templates/${id}/favorite`)
}

/**
 * 取消收藏模板
 */
export const unfavoriteTemplate = (id: number) => {
  return request.delete(`/pipeline/templates/${id}/favorite`)
}

/**
 * 获取收藏列表
 */
export const getFavoriteTemplates = () => {
  return request.get('/pipeline/templates/favorites')
}

/**
 * 获取模板分类
 */
export const getTemplateCategories = () => {
  return request.get<string[]>('/pipeline/templates/categories')
}

/**
 * 获取模板标签
 */
export const getTemplateTags = () => {
  return request.get<string[]>('/pipeline/templates/tags')
}

// ==================== 流水线列表 API ====================

/**
 * 获取流水线列表
 */
export const getPipelineList = (params?: PaginationParams) => {
  return request.get('/pipeline/list', { params })
}

/**
 * 获取流水线详情
 */
export const getPipelineDetail = (id: number) => {
  return request.get(`/pipeline/${id}`)
}
