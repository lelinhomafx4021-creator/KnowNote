/** 真实请求共用鉴权、中文错误提示与取消机制。 */
import { toast } from '@/store/useUIStore'

export const USE_MOCK = import.meta.env.VITE_USE_MOCK === 'true'
export const BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '/api').replace(/\/$/, '')

export class ApiError extends Error {
  reported = false
  constructor(message: string, public status?: number, public code?: number) {
    super(message)
    this.name = 'ApiError'
  }
}

let readToken = (): string | null => {
  try { return localStorage.getItem('kn-token') } catch { return null }
}
let unauthorized = () => { try { localStorage.removeItem('kn-token') } catch { /* 隐私模式下忽略 */ } }

export function configureAuth(getToken: () => string | null, onUnauthorized: () => void) {
  readToken = getToken
  unauthorized = onUnauthorized
}

export function authHeaders(): Record<string, string> {
  const token = readToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export const isAbort = (error: unknown) => error instanceof Error && error.name === 'AbortError'
export function reportError(error: unknown): Error {
  if (isAbort(error)) return error as Error
  const e = error instanceof ApiError ? error : new ApiError(error instanceof TypeError ? '网络连接失败，请稍后重试' : error instanceof Error ? error.message : '请求失败，请稍后重试')
  if (!e.reported) {
    toast(e.message, 'error')
    e.reported = true
  }
  return e
}

/** fetch 与 XHR 使用相同的响应解包，优先保留后端中文错误。 */
export function unwrap<T>(status: number, text: string, path: string): T {
  let body
  try { body = text ? JSON.parse(text) : null } catch { /* 代理或网关可能返回非 JSON */ }
  if (status === 401 && !path.startsWith('/auth/')) unauthorized()
  if (status < 200 || status >= 300 || (body && body.code !== 0)) {
    throw new ApiError(body?.message || (status === 401 ? '登录已过期，请重新登录' : `请求失败（${status}），请稍后重试`), status, body?.code)
  }
  if (status === 204) return undefined as T
  if (!body || !('data' in body)) throw new ApiError('服务器响应格式异常，请稍后重试', status)
  return body.data as T
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  try {
    const headers = new Headers(init.headers)
    if (!(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
    Object.entries(authHeaders()).forEach(([key, value]) => headers.set(key, value))
    const res = await fetch(BASE_URL + path, { ...init, headers })
    return unwrap<T>(res.status, await res.text(), path)
  } catch (e) { throw reportError(e) }
}

export const http = {
  get: <T>(p: string, signal?: AbortSignal) => request<T>(p, { signal }),
  post: <T>(p: string, data?: unknown, signal?: AbortSignal) =>
    request<T>(p, { method: 'POST', body: data instanceof FormData ? data : JSON.stringify(data ?? {}), signal }),
  put: <T>(p: string, data: unknown) => request<T>(p, { method: 'PUT', body: JSON.stringify(data) }),
  patch: <T>(p: string, data: unknown) => request<T>(p, { method: 'PATCH', body: JSON.stringify(data) }),
  del: <T = void>(p: string) => request<T>(p, { method: 'DELETE' }),
}

export function waitForPoll(signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const abort = () => { clearTimeout(timer); reject(new DOMException('操作已取消', 'AbortError')) }
    const timer = setTimeout(() => { signal?.removeEventListener('abort', abort); resolve() }, 1500)
    if (signal?.aborted) abort()
    else signal?.addEventListener('abort', abort, { once: true })
  })
}

/** 逐行缓存，兼容 UTF-8 分包、CRLF、注释及多行 data，并在退出时关闭流。 */
export async function* sse<T>(path: string, body: unknown, signal?: AbortSignal): AsyncGenerator<T> {
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined
  try {
    const res = await fetch(BASE_URL + path, {
      method: 'POST', headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream', ...authHeaders() },
      body: JSON.stringify(body), signal,
    })
    if (!res.ok) unwrap(res.status, await res.text(), path)
    if (!res.body || !res.headers.get('Content-Type')?.includes('text/event-stream')) throw new ApiError('服务器未返回对话流')
    reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let lines: string[] = []
    let eventName = ''
    const parse = () => {
      const data = lines.join('\n')
      lines = []
      const name = eventName
      eventName = ''
      if (!data) return null
      if (data === '[DONE]') return { type: 'done' }
      let event
      try { event = JSON.parse(data) } catch { throw new ApiError('对话数据格式异常，请重试') }
      if (event.type === 'error' || name === 'error') throw new ApiError(event.message || '回答生成失败，请重试')
      return event
    }
    while (true) {
      const { done, value } = await reader.read()
      buffer += decoder.decode(value, { stream: !done })
      if (done && buffer && !/[\r\n]$/.test(buffer)) buffer += '\n'
      while (true) {
        const match = /\r\n|\r|\n/.exec(buffer)
        if (!match || (!done && match[0] === '\r' && match.index === buffer.length - 1)) break
        const line = buffer.slice(0, match.index)
        buffer = buffer.slice(match.index + match[0].length)
        if (!line) {
          const event = parse()
          if (event) {
            if (event.type === 'done') { if (event.messageId) yield event as T; return }
            yield event as T
          }
        } else if (line.startsWith('data:')) lines.push(line.slice(5).replace(/^ /, ''))
        else if (line.startsWith('event:')) eventName = line.slice(6).trim()
      }
      if (done) {
        const event = parse()
        if (event?.type === 'done') { if (event.messageId) yield event as T; return }
        if (event) yield event as T
        throw new ApiError('对话连接中断，已保留生成内容，请重试')
      }
    }
  } catch (e) { throw reportError(e) }
  finally { if (reader) { await reader.cancel().catch(() => {}); reader.releaseLock() } }
}

function parseContentDispositionFilename(header: string | null): string | undefined {
  if (!header) return undefined
  const utf8Match = /filename\*=UTF-8''([^;]+)/i.exec(header)
  if (utf8Match?.[1]) {
    try { return decodeURIComponent(utf8Match[1].trim().replace(/^"|"$/g, '')) } catch { /* ignore */ }
  }
  const plainMatch = /filename="?([^";]+)"?/i.exec(header)
  return plainMatch?.[1]?.trim()
}

/** 鉴权下载二进制文件并解包后端错误 JSON。 */
export async function fetchBinary(path: string, fallbackName: string): Promise<{ blob: Blob; filename: string }> {
  try {
    const res = await fetch(BASE_URL + path, { headers: { ...authHeaders() } })
    if (!res.ok) unwrap(res.status, await res.text(), path)
    const blob = await res.blob()
    const filename = parseContentDispositionFilename(res.headers.get('Content-Disposition')) || fallbackName
    return { blob, filename }
  } catch (e) { throw reportError(e) }
}

export function triggerBlobDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
