import request from './api'
import type { ApiResponse } from '../types'

// 对应后端 internal/models/system/oa.go:MessageTemplate
export interface MessageTemplate {
  id: number
  name: string
  type: string
  content: string
  description?: string
  is_active: boolean
  created_by?: number | null
  created_at?: string
  updated_at?: string
}

export interface PreviewRequest {
  template_name?: string
  template_id?: number
  content?: string
  data: Record<string, any>
}

export const templateApi = {
  list: (params?: { keyword?: string; page?: number; page_size?: number }): Promise<ApiResponse<{ list: MessageTemplate[]; total: number }>> => {
    return request.get('/notification/templates', { params })
  },

  get: (id: number): Promise<ApiResponse<MessageTemplate>> => {
    return request.get(`/notification/templates/${id}`)
  },

  create: (data: Partial<MessageTemplate>): Promise<ApiResponse<MessageTemplate>> => {
    return request.post('/notification/templates', data)
  },

  update: (id: number, data: Partial<MessageTemplate>): Promise<ApiResponse<MessageTemplate>> => {
    return request.put(`/notification/templates/${id}`, data)
  },

  delete: (id: number): Promise<ApiResponse> => {
    return request.delete(`/notification/templates/${id}`)
  },

  preview: (data: PreviewRequest): Promise<ApiResponse<string>> => {
    return request.post('/notification/templates/preview', data)
  }
}
