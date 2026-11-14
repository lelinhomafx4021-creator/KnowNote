import type { Source } from '@/types'
import { USE_MOCK, BASE_URL, http, authHeaders, unwrap, reportError, waitForPoll } from './client'
import * as mock from './mock/handlers'
import type { AddSourceInput } from './mock/handlers'
export type { AddSourceInput }

/** 文件地址统一在接口层生成，预览和下载使用同一 API 服务及查询参数鉴权。 */
export function sourceFileUrl(source: Source, action: 'preview' | 'file', token: string | null, page = 1): string {
  const target = Math.max(1, Math.min(source.pageCount || Infinity, Math.floor(page) || 1))
  if (USE_MOCK) {
    if (action === 'file') return mock.sourceDownloadUrl(source)
    return `/demo/${source.type === 'pptx' ? 'slide' : 'page'}-${target}.svg#page=${target}`
  }
  const url = new URL(`${BASE_URL}/sources/${encodeURIComponent(source.id)}/${action}`, window.location.origin)
  if (token) url.searchParams.set('token', token)
  if (action === 'preview') url.hash = `page=${target}`
  return url.toString()
}

export const sourceDownloadName = (source: Source) => USE_MOCK ? mock.sourceDownloadName(source) : source.title

async function poll(source: Source, onProgress?: (s: Source) => void, signal?: AbortSignal): Promise<Source> {
  let current = source
  while (current.status === 'processing' || current.status === 'uploading') {
    await waitForPoll(signal)
    current = USE_MOCK ? await mock.getSource(current.id) : await http.get<Source>(`/sources/${current.id}`, signal)
    signal?.throwIfAborted()
    onProgress?.(current)
  }
  return current
}

async function uploadReal(notebookId: string, input: AddSourceInput & { file?: File }, onProgress?: (s: Source) => void, signal?: AbortSignal) {
  try {
    let created: Source
    if (input.file) {
      created = await new Promise<Source>((resolve, reject) => {
        const path = `/notebooks/${notebookId}/sources/upload`
        const xhr = new XMLHttpRequest()
        const abort = () => xhr.abort()
        const cleanup = () => signal?.removeEventListener('abort', abort)
        xhr.open('POST', BASE_URL + path)
        Object.entries(authHeaders()).forEach(([key, value]) => xhr.setRequestHeader(key, value))
        xhr.timeout = 10 * 60 * 1000
        xhr.upload.onprogress = (event) => {
          if (event.lengthComputable && event.total > 0) onProgress?.({ id: 'pending', notebookId, type: input.type, title: input.title, size: input.file!.size, status: 'uploading', progress: Math.round(event.loaded / event.total * 100), createdAt: new Date().toISOString() })
        }
        xhr.onload = () => { cleanup(); try { resolve(unwrap<Source>(xhr.status, xhr.responseText, path)) } catch (e) { reject(e) } }
        xhr.onerror = () => { cleanup(); reject(new Error('上传失败，请检查网络连接')) }
        xhr.ontimeout = () => { cleanup(); reject(new Error('上传超时，请重试')) }
        xhr.onabort = () => { cleanup(); reject(new DOMException('上传已取消', 'AbortError')) }
        if (signal?.aborted) { reject(new DOMException('上传已取消', 'AbortError')); return }
        signal?.addEventListener('abort', abort, { once: true })
        const form = new FormData()
        form.append('file', input.file!)
        xhr.send(form)
      })
    } else {
      created = await http.post<Source>(`/notebooks/${notebookId}/sources`, { type: input.type, title: input.title, url: input.url, text: input.text }, signal)
    }
    signal?.throwIfAborted()
    onProgress?.(created)
    return await poll(created, onProgress, signal)
  } catch (e) { throw reportError(e) }
}

export const sourcesApi = {
  list: (id: string, signal?: AbortSignal): Promise<Source[]> => USE_MOCK ? mock.listSources(id) : http.get(`/notebooks/${id}/sources`, signal),
  poll,
  add: (id: string, input: AddSourceInput & { file?: File }, onProgress?: (s: Source) => void, signal?: AbortSignal): Promise<Source> =>
    USE_MOCK ? mock.addSource(id, input, onProgress, signal) : uploadReal(id, input, onProgress, signal),
  rename: (id: string, title: string): Promise<void> => USE_MOCK ? mock.renameSource(id, title) : http.patch(`/sources/${id}`, { title }),
  remove: (id: string): Promise<void> => USE_MOCK ? mock.deleteSource(id) : http.del(`/sources/${id}`),
}
