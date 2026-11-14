import type { AudioOverview, StudioArtifact, StudioKind, UserSettings } from '@/types'
import { USE_MOCK, http } from './client'
import * as mock from './mock/handlers'

export const STUDIO_TITLES = mock.STUDIO_TITLES

export const studioApi = {
  list: (notebookId: string): Promise<StudioArtifact[]> =>
    USE_MOCK ? mock.listArtifacts(notebookId) : http.get(`/notebooks/${notebookId}/artifacts`),
  generate: (notebookId: string, kind: StudioKind): Promise<StudioArtifact> =>
    USE_MOCK ? mock.generateArtifact(notebookId, kind) : http.post(`/notebooks/${notebookId}/artifacts`, { kind }),
  remove: (id: string): Promise<void> => (USE_MOCK ? mock.deleteArtifact(id) : http.del(`/artifacts/${id}`)),
  getAudio: (notebookId: string): Promise<AudioOverview> =>
    USE_MOCK ? mock.getAudio(notebookId) : http.get(`/notebooks/${notebookId}/audio`),
  generateAudio: (notebookId: string): Promise<AudioOverview> =>
    USE_MOCK ? mock.generateAudio(notebookId) : http.post(`/notebooks/${notebookId}/audio`),
}

export const settingsApi = {
  get: (): Promise<UserSettings> => (USE_MOCK ? mock.getSettings() : http.get('/me/settings')),
  update: (patch: Partial<UserSettings>): Promise<UserSettings> =>
    USE_MOCK ? mock.updateSettings(patch) : http.patch('/me/settings', patch),
}
