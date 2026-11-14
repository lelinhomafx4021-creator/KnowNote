import type { Notebook, NotebookGuide } from '@/types'
import { USE_MOCK, http } from './client'
import * as mock from './mock/handlers'

export const notebooksApi = {
  list: (tag?: string): Promise<Notebook[]> => (USE_MOCK ? mock.listNotebooks(tag) : http.get(`/notebooks${tag ? `?tag=${encodeURIComponent(tag)}` : ''}`)),
  get: (id: string): Promise<Notebook> => (USE_MOCK ? mock.getNotebook(id) : http.get(`/notebooks/${id}`)),
  create: (title?: string): Promise<Notebook> => (USE_MOCK ? mock.createNotebook(title) : http.post('/notebooks', { title })),
  update: (id: string, patch: Partial<Pick<Notebook, 'title' | 'emoji' | 'color' | 'description'>>): Promise<Notebook> =>
    USE_MOCK ? mock.updateNotebook(id, patch) : http.patch(`/notebooks/${id}`, patch),
  remove: (id: string): Promise<void> => (USE_MOCK ? mock.deleteNotebook(id) : http.del(`/notebooks/${id}`)),
  setTags: (id: string, tagIds: string[]): Promise<Notebook> => USE_MOCK ? mock.setNotebookTags(id, tagIds) : http.put(`/notebooks/${id}/tags`, { tagIds }),
  guide: (id: string): Promise<NotebookGuide> => (USE_MOCK ? mock.getGuide(id) : http.get(`/notebooks/${id}/guide`)),
}
