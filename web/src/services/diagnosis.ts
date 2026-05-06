// Pipeline run diagnosis API client (Sprint 1 BE-05).
// Contract source of truth: docs/api/diagnosis_v1.md.
//
// The HTTP envelope is {code, message, data}; the response interceptor in
// services/api.ts returns the envelope verbatim, so callers access .data.
//
// All fields below are intentionally permissive (nullable / optional) to
// honor the contract's "缺数据填 null 或空数组，禁填猜测值" rule. Components
// must defensively handle missing fields.

import request from '@/services/api'

export type FlakyReason = 'same_commit_retry_succeeded' | 'cross_commit_recurrence' | null

export interface LastSuccessInfo {
  run_id: number
  commit: string
  happened_at: string
  diff_url?: string
}

export interface ChangedFile {
  path: string
  additions: number
  deletions: number
}

export interface SimilarRun {
  run_id: number
  happened_at: string
  fixed_by_commit?: string
  fix_diff_url?: string
}

export type FixReferenceKind = 'jira_issue' | 'postmortem' | 'improvement_item'

export interface FixReference {
  kind: FixReferenceKind
  key?: string
  id?: string
  title: string
  url?: string
}

export interface LogTailLine {
  ts?: string
  stream?: string
  line: string
}

export interface FailureDiagnosis {
  run_id: number
  pipeline_id: number
  status: string

  failure_signature: string | null
  signature_first_seen_at?: string
  signature_occurrences?: number
  signature_distinct_commits?: number

  is_flaky: boolean
  flaky_reason: FlakyReason

  last_success: LastSuccessInfo | null
  changed_files: ChangedFile[]
  similar_runs: SimilarRun[]
  fix_references: FixReference[]
  log_tail: LogTailLine[]
}

export interface DiagnosisEnvelope {
  code: number
  message: string
  data: FailureDiagnosis
}

export const diagnosisApi = {
  /**
   * Fetch the structured diagnosis for a failed run. Returns the full
   * envelope; consumers use .data for the typed payload.
   *
   * Backend endpoint: GET /pipeline/runs/:id/diagnosis
   * - 404 if run does not exist
   * - 409 if run status is not failed/cancelled
   * - 200 with degraded payload (failure_signature=null, log_tail only)
   *   when signature computation failed
   */
  getRunDiagnosis(runId: number): Promise<DiagnosisEnvelope> {
    return request.get(`/pipeline/runs/${runId}/diagnosis`)
  }
}
