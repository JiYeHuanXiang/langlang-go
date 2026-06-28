export class ApiError extends Error {
  code: number
  constructor(code: number, msg: string) {
    super(msg)
    this.code = code
    this.name = 'ApiError'
  }
}

const BASE = ''

async function request<T = any>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) {
    opts.body = JSON.stringify(body)
  }
  const resp = await fetch(BASE + path, opts)
  const data = await resp.json()
  if (data.code !== 0) {
    throw new ApiError(data.code ?? -1, data.msg || '请求失败')
  }
  return data as T
}

export const api = { request, BASE }
