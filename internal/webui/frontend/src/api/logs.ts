import { api } from './client'

export interface LogEntry {
  level: string
  msg: string
  time: string
}

export function getRecentLogs() {
  return api.request<{ code: number; logs: LogEntry[] }>('GET', '/api/log/recent')
}
