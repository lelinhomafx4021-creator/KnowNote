import type { ExportFormat, Note } from '@/types'
import { USE_MOCK, fetchBinary, http } from './client'
import * as mock from './mock/handlers'

export const notesApi = {
  list: (notebookId: string): Promise<Note[]> => (USE_MOCK ? mock.listNotes(notebookId) : http.get(`/notebooks/${notebookId}/notes`)),
  create: (notebookId: string, data: Pick<Note, 'title' | 'content' | 'origin'>): Promise<Note> =>
    USE_MOCK ? mock.createNote(notebookId, data) : http.post(`/notebooks/${notebookId}/notes`, data),
  update: (id: string, patch: Partial<Pick<Note, 'title' | 'content'>>): Promise<Note> =>
    USE_MOCK ? mock.updateNote(id, patch) : http.patch(`/notes/${id}`, patch),
  exportFile: async (note: Note, format: ExportFormat = 'docx'): Promise<{ blob: Blob; filename: string }> => {
    if (USE_MOCK) return mock.exportMarkdownFallback(note.title, note.content, format)
    return fetchBinary(`/notes/${note.id}/export?format=${format}`, `${note.title || '学习笔记'}.${format}`)
  },
  remove: (id: string): Promise<void> => (USE_MOCK ? mock.deleteNote(id) : http.del(`/notes/${id}`)),
}
