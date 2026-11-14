import type { Tag } from '@/types'
import { USE_MOCK, http } from './client'
import * as mock from './mock/handlers'

export const tagsApi = {
  list: (): Promise<Tag[]> => USE_MOCK ? mock.listTags() : http.get('/tags'),
  create: (name: string, color: string): Promise<Tag> => USE_MOCK ? mock.createTag(name, color) : http.post('/tags', { name, color }),
  update: (id: string, patch: Partial<Pick<Tag, 'name' | 'color'>>): Promise<Tag> => USE_MOCK ? mock.updateTag(id, patch) : http.patch(`/tags/${id}`, patch),
  remove: (id: string): Promise<void> => USE_MOCK ? mock.deleteTag(id) : http.del(`/tags/${id}`),
}
