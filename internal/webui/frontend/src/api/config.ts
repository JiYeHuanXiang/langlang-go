import { api } from './client'

interface ConfigResponse {
  code: number
  config: Record<string, any>
}

export function getConfig() {
  return api.request<ConfigResponse>('GET', '/api/config')
}

export function saveConfig(cfg: Record<string, any>) {
  return api.request('POST', '/api/config', cfg)
}

export function getTestMode() {
  return api.request<{ code: number; test_mode: string }>('GET', '/api/testmode')
}

export function setTestMode(enabled: boolean) {
  return api.request('POST', '/api/testmode', { enabled })
}
