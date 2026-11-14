import type { Source } from '@/types'
import { USE_MOCK, BASE_URL, http } from './client'
import * as mock from './mock/handlers'
import type { AddSourceInput } from './mock/handlers'

export type { AddSourceInput }

/** 真实后端：上传文件（XHR 以获取上传进度），随后轮询解析状态 */
async function uploadReal(notebookId: string, input: AddSourceInput & { file?: File }, onProgress?: (s: Source) => void) {
  let created: Source
  if (input.file) {
    created = await new Promise<Source>((resolve, reject) => {
      const fd = new FormData()
      fd.append('file', input.file!)
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${BASE_URL}/notebooks/${notebookId}/sources/upload`)
      xhr.upload.onprogress = (e) =>
        onProgress?.({ id: 'pending', notebookId, type: input.type, title: input.title, status: 'uploading', progress: Math.round((e.loaded / e.total) * 100), createdAt: new Date().toISOString() })
      xhr.onload = () => (xhr.status < 300 ? resolve(JSON.parse(xhr.responseText).data) : reject(new Error('上传失败')))
      xhr.onerror = () => reject(new Error('网络错误'))
      xhr.send(fd)
    })
  } else {
    created = await http.post<Source>(`/notebooks/${notebookId}/sources`, { type: input.type, title: input.title, url: input.url, text: input.text })
  }
  onProgress?.(created)
  while (created.status === 'processing' || created.status === 'uploading') {
    await new Promise((r) => setTimeout(r, 1500))
    created = await http.get<Source>(`/sources/${created.id}`)
    onProgress?.(created)
  }
  return created
}

export const sourcesApi = {
  list: (notebookId: string): Promise<Source[]> =>
    USE_MOCK ? mock.listSources(notebookId) : http.get(`/notebooks/${notebookId}/sources`),
  add: (notebookId: string, input: AddSourceInput & { file?: File }, onProgress?: (s: Source) => void): Promise<Source> =>
    USE_MOCK ? mock.addSource(notebookId, input, onProgress) : uploadReal(notebookId, input, onProgress),
  rename: (id: string, title: string): Promise<void> =>
    USE_MOCK ? mock.renameSource(id, title) : http.patch(`/sources/${id}`, { title }),
  remove: (id: string): Promise<void> => (USE_MOCK ? mock.deleteSource(id) : http.del(`/sources/${id}`)),
}
