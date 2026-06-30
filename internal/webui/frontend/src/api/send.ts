import { api } from './client'

export interface SendRequest {
  platform: string
  self_id: string
  target_type: 'private' | 'group'
  target_id: string
  message: string
}

export function sendBotMessage(req: SendRequest) {
  return api.request('POST', '/api/bot/send', req)
}
