import { api } from './client'

export interface PluginInfo {
  name: string
  code: string
  lang: string
  enabled: boolean
  created_at: string
  updated_at: string
}

interface PluginListResponse {
  code: number
  plugins: PluginInfo[]
}

interface PluginDetailResponse {
  code: number
  data: PluginInfo
}

export function getPlugins() {
  return api.request<PluginListResponse>('GET', '/api/plugins')
}

export function getPlugin(name: string) {
  return api.request<PluginDetailResponse>('GET', `/api/plugin/${encodeURIComponent(name)}`)
}

export function savePlugin(name: string, code: string, lang = 'redlang') {
  return api.request('POST', `/api/plugin/${encodeURIComponent(name)}`, { code, lang })
}

export function deletePlugin(name: string) {
  return api.request('DELETE', `/api/plugin/${encodeURIComponent(name)}`)
}

export function reloadPlugins() {
  return api.request('POST', '/api/reload')
}

export function validateCode(code: string, lang = 'redlang') {
  return api.request('POST', '/api/validate', { code, lang })
}
