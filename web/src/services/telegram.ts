import request from './api'
import type { ApiResponse } from '../types'

// Telegram 机器人
export interface TelegramBot {
  id?: number
  name: string
  token: string
  default_chat_id?: string
  api_base_url?: string
  description?: string
  status: string
  is_default: boolean
  created_at?: string
  updated_at?: string
}

// Telegram 消息日志
export interface TelegramMessageLog {
  id: number
  created_at: string
  bot_id: number
  chat_id: string
  parse_mode?: string
  content: string
  source: string
  status: string
  error_msg?: string
}

// 发送请求
export interface SendTelegramRequest {
  bot_id?: number
  chat_id?: string
  content: string
  parse_mode?: '' | 'MarkdownV2' | 'HTML'
  disable_web_page_preview?: boolean
  disable_notification?: boolean
}

// 机器人管理 API
export const telegramBotApi = {
  list: (page = 1, pageSize = 100): Promise<ApiResponse<{ list: TelegramBot[]; total: number }>> => {
    return request.get('/telegram/bot', { params: { page, page_size: pageSize } })
  },
  get: (id: number): Promise<ApiResponse<TelegramBot>> => {
    return request.get(`/telegram/bot/${id}`)
  },
  create: (data: Partial<TelegramBot>): Promise<ApiResponse<TelegramBot>> => {
    return request.post('/telegram/bot', data)
  },
  update: (id: number, data: Partial<TelegramBot>): Promise<ApiResponse<TelegramBot>> => {
    return request.put(`/telegram/bot/${id}`, data)
  },
  delete: (id: number): Promise<ApiResponse> => {
    return request.delete(`/telegram/bot/${id}`)
  },
  setDefault: (id: number): Promise<ApiResponse> => {
    return request.post(`/telegram/bot/${id}/default`)
  },
  test: (id: number): Promise<ApiResponse<Record<string, unknown>>> => {
    return request.post(`/telegram/bot/${id}/test`)
  }
}

// 消息 API
export const telegramApi = {
  sendMessage: (data: SendTelegramRequest): Promise<ApiResponse> => {
    return request.post('/telegram/send-message', data)
  }
}

// 消息日志 API
export const telegramLogApi = {
  list: (page = 1, pageSize = 20, source = ''): Promise<ApiResponse<{ list: TelegramMessageLog[]; total: number }>> => {
    return request.get('/telegram/logs', { params: { page, page_size: pageSize, source } })
  }
}
