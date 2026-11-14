import type { Notebook, NotebookGuide } from '@/types'
import { USE_MOCK, http } from './client'
import * as mock from './mock/handlers'

export const notebooksApi = {
  list: (): Promise<Notebook[]> => (USE_MOCK ? mock.listNotebooks() : http.get('/notebooks')),
  get: (id: string): Promise<Notebook> => (USE_MOCK ? mock.getNotebook(id) : http.get(`/notebooks/${id}`)),
  create: (title?: string): Promise<Notebook> => (USE_MOCK ? mock.createNotebook(title) : http.post('/notebooks', { title })),
  update: (id: string, patch: Partial<Notebook>): Promise<Notebook> =>
    USE_MOCK ? mock.updateNotebook(id, patch) : http.patch(`/notebooks/${id}`, patch),
  remove: (id: string): Promise<void> => (USE_MOCK ? mock.deleteNotebook(id) : http.del(`/notebooks/${id}`)),
  guide: (id: string): Promise<NotebookGuide> => (USE_MOCK ? mock.getGuide(id) : http.get(`/notebooks/${id}/guide`)),
}
