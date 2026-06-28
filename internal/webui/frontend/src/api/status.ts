import { api } from './client'

interface BotInfo {
  platform: string
  self_id: string
  running: boolean
}

interface StatusResponse {
  code: number
  version: string
  uptime: string
  plugins: number
  test_mode: string
  bots: BotInfo[]
  configured_platforms: string[]
}

export function getStatus() {
  return api.request<StatusResponse>('GET', '/api/status')
}
