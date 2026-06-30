import { api } from './client'

export interface ScriptRunResult {
  code: number
  output: string
  error?: string
  duration_ms: number
  cancelled?: boolean
  timeout?: boolean
}

export function runScript(code: string, lang: string, timeout = 10) {
  return api.request<ScriptRunResult>('POST', '/api/script/run', {
    code,
    lang,
    timeout,
  })
}

export function stopScript() {
  return api.request('POST', '/api/script/stop')
}
