/**
 * HTTP 客户端
 * - VITE_USE_MOCK=false 时请求真实后端（默认走 mock，见 .env.example）
 * - 后端统一返回格式：{ code: 0, data: T, message: string }
 */
export const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false'
export const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api'

export class ApiError extends Error {
  constructor(message: string, public status?: number, public code?: number) {
    super(message)
  }
}

function authHeaders(): Record<string, string> {
  try {
    const token = localStorage.getItem('kn-token')
    return token ? { Authorization: `Bearer ${token}` } : {}
  } catch {
    return {}
  }
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const isForm = init.body instanceof FormData
  const res = await fetch(BASE_URL + path, {
    ...init,
    headers: { ...(isForm ? {} : { 'Content-Type': 'application/json' }), ...authHeaders(), ...init.headers },
  })
  if (!res.ok) throw new ApiError(`请求失败（${res.status}）`, res.status)
  if (res.status === 204) return undefined as T
  const body = await res.json()
  if (body && typeof body === 'object' && 'code' in body) {
    if (body.code !== 0) throw new ApiError(body.message ?? '服务异常', res.status, body.code)
    return body.data as T
  }
  return body as T
}

export const http = {
  get: <T>(p: string) => request<T>(p),
  post: <T>(p: string, data?: unknown) =>
    request<T>(p, { method: 'POST', body: data instanceof FormData ? data : JSON.stringify(data ?? {}) }),
  patch: <T>(p: string, data: unknown) => request<T>(p, { method: 'PATCH', body: JSON.stringify(data) }),
  del: <T = void>(p: string) => request<T>(p, { method: 'DELETE' }),
}

/** 解析 SSE：每个事件形如 `data: {...}\n\n` */
export async function* sse<T>(path: string, body: unknown, signal?: AbortSignal): AsyncGenerator<T> {
  const res = await fetch(BASE_URL + path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream', ...authHeaders() },
    body: JSON.stringify(body),
    signal,
  })
  if (!res.ok || !res.body) throw new ApiError(`请求失败（${res.status}）`, res.status)
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    let i: number
    while ((i = buf.indexOf('\n\n')) >= 0) {
      const raw = buf.slice(0, i)
      buf = buf.slice(i + 2)
      const data = raw.split('\n').filter((l) => l.startsWith('data:')).map((l) => l.slice(5).trim()).join('')
      if (data && data !== '[DONE]') yield JSON.parse(data) as T
    }
  }
}
