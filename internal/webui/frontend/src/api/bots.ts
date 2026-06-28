import { api } from './client'

export function botControl(action: string, platform: string, selfId = '') {
  return api.request('POST', '/api/bot/', { action, platform, self_id: selfId })
}
