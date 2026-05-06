import request from '@/services/api'

export interface OncallSchedule {
  id: number
  created_at?: string
  updated_at?: string
  name: string
  description: string
  timezone: string
  rotation_type: string
  enabled: boolean
  created_by: number
}

export interface OncallShift {
  id: number
  schedule_id: number
  user_id: number
  user_name: string
  start_time: string
  end_time: string
  shift_type: string
}

export interface OncallOverride {
  id: number
  schedule_id: number
  original_user_id: number
  original_user_name: string
  override_user_id: number
  override_user_name: string
  start_time: string
  end_time: string
  reason: string
  created_by: number
}

export interface AlertAssignment {
  id: number
  alert_history_id: number
  assignee_id: number
  assignee_name: string
  schedule_id?: number
  status: string
  claimed_at?: string
  resolved_at?: string
  comment?: string
  created_at?: string
}

export const oncallApi = {
  // 排班表
  listSchedules: () => request.get('/oncall/schedules'),
  getSchedule: (id: number) => request.get(`/oncall/schedules/${id}`),
  createSchedule: (data: Partial<OncallSchedule>) => request.post('/oncall/schedules', data),
  updateSchedule: (id: number, data: Partial<OncallSchedule>) => request.put(`/oncall/schedules/${id}`, data),
  deleteSchedule: (id: number) => request.delete(`/oncall/schedules/${id}`),

  // 班次
  listShifts: (scheduleId: number, start?: string, end?: string) =>
    request.get(`/oncall/schedules/${scheduleId}/shifts`, { params: { start, end } }),
  createShift: (scheduleId: number, data: Partial<OncallShift>) =>
    request.post(`/oncall/schedules/${scheduleId}/shifts`, data),
  deleteShift: (shiftId: number) => request.delete(`/oncall/shifts/${shiftId}`),
  generateShifts: (scheduleId: number, data: { user_ids: number[]; user_names: string[]; start_date: string; count: number; type: string }) =>
    request.post(`/oncall/schedules/${scheduleId}/generate`, data),

  // 当前值班
  getCurrentOnCall: (scheduleId: number) => request.get(`/oncall/schedules/${scheduleId}/current`),

  // 临时替换
  listOverrides: (scheduleId: number) => request.get(`/oncall/schedules/${scheduleId}/overrides`),
  createOverride: (scheduleId: number, data: Partial<OncallOverride>) =>
    request.post(`/oncall/schedules/${scheduleId}/overrides`, data),
  deleteOverride: (overrideId: number) => request.delete(`/oncall/overrides/${overrideId}`),

  // 告警分配
  assignAlert: (alertId: number, scheduleId: number) =>
    request.post(`/oncall/alerts/${alertId}/assign`, { schedule_id: scheduleId }),
  claimAlert: (alertId: number) => request.post(`/oncall/alerts/${alertId}/claim`),
  resolveAlert: (alertId: number, comment: string) =>
    request.post(`/oncall/alerts/${alertId}/resolve`, { comment }),
  getAssignment: (alertId: number) => request.get(`/oncall/alerts/${alertId}/assignment`),
  listMyAssignments: (status?: string) => request.get('/oncall/my-assignments', { params: { status } }),
}
