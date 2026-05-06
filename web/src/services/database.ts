import request from './api'
import type { ApiResponse } from '../types'

export interface DBInstance {
  id?: number
  name: string
  db_type: 'mysql' | 'postgres'
  env: string
  host: string
  port: number
  username: string
  default_db?: string
  exclude_dbs?: string
  params?: string
  mode?: number
  status?: string
  description?: string
  created_at?: string
  updated_at?: string
}

export interface DBInstanceReq extends Partial<DBInstance> {
  plain_password?: string
}

export interface ColumnInfo {
  name: string
  data_type: string
  column_type: string
  is_nullable: string
  column_key: string
  column_default: string
  extra: string
  comment: string
}

export interface IndexInfo {
  name: string
  column_name: string
  non_unique: number
  seq_in_index: number
}

export interface QueryRequest {
  instance_id: number
  schema?: string
  sql: string
  limit?: number
}

export interface QueryResult {
  columns: string[]
  rows: Record<string, unknown>[]
  affect_rows: number
  exec_ms: number
}

export interface DBQueryLog {
  id: number
  created_at: string
  instance_id: number
  username: string
  schema_name: string
  sql_text: string
  affect_rows: number
  exec_ms: number
  status: string
  error_msg: string
}

export const dbInstanceApi = {
  list: (params: Record<string, unknown> = {}): Promise<ApiResponse<{ list: DBInstance[]; total: number }>> =>
    request.get('/database/instance', { params }),
  listAll: (): Promise<ApiResponse<DBInstance[]>> => request.get('/database/instance/all'),
  get: (id: number): Promise<ApiResponse<DBInstance>> => request.get(`/database/instance/${id}`),
  create: (data: DBInstanceReq): Promise<ApiResponse<DBInstance>> => request.post('/database/instance', data),
  update: (id: number, data: DBInstanceReq): Promise<ApiResponse<DBInstance>> =>
    request.put(`/database/instance/${id}`, data),
  delete: (id: number): Promise<ApiResponse> => request.delete(`/database/instance/${id}`),
  test: (data: DBInstanceReq): Promise<ApiResponse> => request.post('/database/instance/test', data),
  testExisting: (id: number): Promise<ApiResponse> => request.post(`/database/instance/${id}/test`),
  databases: (id: number): Promise<ApiResponse<string[]>> => request.get(`/database/instance/${id}/databases`),
  tables: (id: number, schema: string): Promise<ApiResponse<string[]>> =>
    request.get(`/database/instance/${id}/tables`, { params: { schema } }),
  columns: (id: number, schema: string, table: string): Promise<ApiResponse<ColumnInfo[]>> =>
    request.get(`/database/instance/${id}/columns`, { params: { schema, table } }),
  indexes: (id: number, schema: string, table: string): Promise<ApiResponse<IndexInfo[]>> =>
    request.get(`/database/instance/${id}/indexes`, { params: { schema, table } })
}

export const dbConsoleApi = {
  execute: (data: QueryRequest): Promise<ApiResponse<QueryResult>> => request.post('/database/query/execute', data)
}

export const dbLogApi = {
  list: (params: Record<string, unknown> = {}): Promise<ApiResponse<{ list: DBQueryLog[]; total: number }>> =>
    request.get('/database/logs', { params })
}

export interface AuditStep {
  step_name: string
  approvers: string[]
}

export interface AuditFinding {
  seq: number
  rule: string
  level: 'info' | 'warning' | 'error'
  message: string
}

export interface AuditStatementResult {
  seq: number
  sql: string
  kind: string
  findings: AuditFinding[]
}

export interface AuditReport {
  statements: AuditStatementResult[]
  has_error: boolean
  has_warning: boolean
  summary: string
}

export interface TicketCreateInput {
  title: string
  description?: string
  instance_id: number
  schema_name: string
  change_type: number
  need_backup?: boolean
  sql_text: string
  audit_steps?: AuditStep[]
  delay_mode?: string
  execute_time?: string | null
  allow_drop?: boolean
  allow_trunc?: boolean
}

export interface SQLChangeTicket {
  id: number
  created_at: string
  updated_at: string
  work_id: string
  title: string
  description: string
  applicant: string
  real_name: string
  instance_id: number
  schema_name: string
  change_type: number
  need_backup: boolean
  status: number
  execute_time: string | null
  delay_mode: string
  approval_instance_id?: number
  audit_report: AuditReport | null
  audit_config: AuditStep[] | null
  current_step: number
  assigned: string
}

export interface SQLChangeStatement {
  id: number
  ticket_id: number
  work_id: string
  seq: number
  sql_text: string
  affect_rows: number
  exec_ms: number
  state: string
  error_msg: string
  executed_at: string | null
}

export interface SQLChangeWorkflow {
  id: number
  created_at: string
  ticket_id: number
  work_id: string
  username: string
  action: string
  step: number
  comment: string
}

export interface TicketDetail {
  ticket: SQLChangeTicket
  statements: SQLChangeStatement[]
  workflow: SQLChangeWorkflow[]
  audit_steps: AuditStep[]
}

export const dbTicketApi = {
  create: (data: TicketCreateInput): Promise<ApiResponse<SQLChangeTicket>> =>
    request.post('/database/ticket', data),
  list: (params: Record<string, unknown> = {}): Promise<ApiResponse<{ list: SQLChangeTicket[]; total: number }>> =>
    request.get('/database/ticket', { params }),
  getByApprovalInstance: (approvalInstanceId: number): Promise<ApiResponse<SQLChangeTicket>> =>
    request.get(`/database/ticket/by-approval/${approvalInstanceId}`),
  get: (id: number): Promise<ApiResponse<TicketDetail>> => request.get(`/database/ticket/${id}`),
  agree: (id: number, comment?: string): Promise<ApiResponse> =>
    request.post(`/database/ticket/${id}/agree`, { comment }),
  reject: (id: number, comment?: string): Promise<ApiResponse> =>
    request.post(`/database/ticket/${id}/reject`, { comment }),
  cancel: (id: number): Promise<ApiResponse> => request.post(`/database/ticket/${id}/cancel`),
  execute: (id: number): Promise<ApiResponse> => request.post(`/database/ticket/${id}/execute`),
  rollback: (id: number): Promise<ApiResponse<SQLRollbackScript[]>> =>
    request.get(`/database/ticket/${id}/rollback`)
}

export interface SQLRollbackScript {
  id: number
  created_at: string
  ticket_id: number
  work_id: string
  statement_id: number
  rollback_sql: string
}

export interface AuditRuleConfig {
  require_where?: boolean
  suggest_dml_limit?: boolean
  tautology_where?: boolean
  select_star?: boolean
  select_limit?: boolean
  no_drop?: boolean
  no_truncate?: boolean
  rename_table?: boolean
  alter_drop?: boolean
  create_engine?: boolean
  create_charset?: boolean
  create_primary_key?: boolean
  no_lock_tables?: boolean
  no_set_global?: boolean
  no_grant?: boolean
  insert_columns?: boolean
  max_statement_per_ticket?: number
  max_statement_bytes?: number
}

export interface SQLAuditRuleSet {
  id: number
  created_at: string
  updated_at: string
  name: string
  description: string
  config: AuditRuleConfig
  is_default: boolean
}

export interface RuleInput {
  name: string
  description?: string
  config: AuditRuleConfig
  is_default?: boolean
}

// ========== ACL ==========

export interface DBInstanceACL {
  id: number
  instance_id: number
  subject_type: 'user' | 'role'
  subject_id: number
  access_level: 'read' | 'write' | 'owner'
  schema_names: string
  created_at: string
  created_by: number | null
  subject_name?: string
}

export interface ACLBindInput {
  subject_type: 'user' | 'role'
  subject_id: number
  access_level: 'read' | 'write' | 'owner'
  schema_names: string
}

export const dbACLApi = {
  list: (instanceId: number): Promise<ApiResponse<DBInstanceACL[]>> =>
    request.get(`/database/instance/${instanceId}/acl`),
  bind: (instanceId: number, data: ACLBindInput): Promise<ApiResponse<DBInstanceACL>> =>
    request.post(`/database/instance/${instanceId}/acl`, data),
  unbind: (aclId: number): Promise<ApiResponse> =>
    request.delete(`/database/instance/acl/${aclId}`),
  accessibleSchemas: (instanceId: number): Promise<ApiResponse<{ schemas: string[] | null; is_all: boolean }>> =>
    request.get(`/database/instance/${instanceId}/acl/schemas`)
}

// ========== Statement Records ==========

export interface StatementListItem {
  id: number
  ticket_id: number
  work_id: string
  seq: number
  sql_text: string
  affect_rows: number
  exec_ms: number
  state: string
  error_msg: string
  executed_at: string | null
  ticket_title: string
  ticket_work_id: string
  applicant: string
  instance_id: number
  schema_name: string
  ticket_status: number
}

export const dbStatementApi = {
  list: (params: Record<string, unknown> = {}): Promise<ApiResponse<{ list: StatementListItem[]; total: number }>> =>
    request.get('/database/statements', { params })
}

// ========== Rule ==========

export const dbRuleApi = {
  list: (): Promise<ApiResponse<SQLAuditRuleSet[]>> => request.get('/database/rules'),
  get: (id: number): Promise<ApiResponse<SQLAuditRuleSet>> => request.get(`/database/rules/${id}`),
  defaultConfig: (): Promise<ApiResponse<AuditRuleConfig>> => request.get('/database/rules/default-config'),
  create: (data: RuleInput): Promise<ApiResponse<SQLAuditRuleSet>> => request.post('/database/rules', data),
  update: (id: number, data: RuleInput): Promise<ApiResponse<SQLAuditRuleSet>> =>
    request.put(`/database/rules/${id}`, data),
  delete: (id: number): Promise<ApiResponse> => request.delete(`/database/rules/${id}`),
  setDefault: (id: number): Promise<ApiResponse> => request.post(`/database/rules/${id}/default`)
}
