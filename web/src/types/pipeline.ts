// ==================== 通用类型 ====================

/**
 * 分页参数
 */
export interface PaginationParams {
  page?: number
  page_size?: number
}

/**
 * 分页结果
 */
export interface PaginationResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

/**
 * API 响应
 */
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

// ==================== 制品版本相关类型 ====================

/**
 * 制品版本
 */
export interface ArtifactVersion {
  id: number
  artifact_id: number
  artifact_name: string
  version: string
  size: number
  checksum: string
  metadata: Record<string, any>
  scan_status: 'pending' | 'scanning' | 'completed' | 'failed'
  scan_result?: ArtifactScanResult
  created_at: string
  created_by: string
}

/**
 * 制品扫描结果
 */
export interface ArtifactScanResult {
  version_id: number
  scan_time: string
  vulnerabilities: VulnerabilitySummary
  licenses: LicenseSummary
  quality: QualitySummary
  vulnerability_list: Vulnerability[]
  license_list: License[]
  quality_issues: QualityIssue[]
}

/**
 * 漏洞摘要
 */
export interface VulnerabilitySummary {
  total: number
  critical: number
  high: number
  medium: number
  low: number
}

/**
 * 漏洞详情
 */
export interface Vulnerability {
  id: string
  cve_id: string
  severity: 'critical' | 'high' | 'medium' | 'low'
  package_name: string
  installed_version: string
  fixed_version?: string
  description: string
  cvss_score?: number
  references: string[]
  fix_available: boolean
}

/**
 * 许可证摘要
 */
export interface LicenseSummary {
  total: number
  compliant: number
  non_compliant: number
  unknown: number
}

/**
 * 许可证详情
 */
export interface License {
  package_name: string
  license_type: string
  is_compliant: boolean
  risk_level: 'high' | 'medium' | 'low'
}

/**
 * 质量摘要
 */
export interface QualitySummary {
  score: number
  issues: number
  code_smells: number
  bugs: number
  security_hotspots: number
}

/**
 * 质量问题
 */
export interface QualityIssue {
  type: 'code_smell' | 'bug' | 'security_hotspot'
  severity: 'critical' | 'major' | 'minor'
  message: string
  file: string
  line: number
}

// ==================== 流水线模板相关类型 ====================

/**
 * 流水线模板
 */
export interface PipelineTemplate {
  id: number
  name: string
  slug: string
  description: string
  category: string
  tags: string[]
  version: string
  config_json: PipelineConfig
  is_public: boolean
  is_official: boolean
  usage_count: number
  rating: number
  rating_count: number
  created_at: string
  updated_at: string
  created_by: string
}

/**
 * 流水线配置
 */
export interface PipelineConfig {
  stages: PipelineStage[]
  variables?: Record<string, any>
  triggers?: PipelineTrigger[]
}

/**
 * 流水线阶段
 */
export interface PipelineStage {
  name: string
  description?: string
  steps: PipelineStep[]
  condition?: string
  parallel?: boolean
}

/**
 * 流水线步骤
 */
export interface PipelineStep {
  name: string
  type: string
  config: Record<string, any>
  timeout?: number
  continue_on_error?: boolean
}

/**
 * 流水线触发器
 */
export interface PipelineTrigger {
  type: 'webhook' | 'schedule' | 'manual'
  config: Record<string, any>
}
