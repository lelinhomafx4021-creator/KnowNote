import type { AudioOverview, ExportFormat, StudioArtifact, StudioKind, UserSettings } from '@/types'
import { USE_MOCK, fetchBinary, http, waitForPoll } from './client'
import * as mock from './mock/handlers'

export const STUDIO_TITLES = mock.STUDIO_TITLES

export interface GenerateArtifactOptions {
  topic?: string
  preset?: string
  sourceIds?: string[]
}

export const studioApi = {
  list: (notebookId: string): Promise<StudioArtifact[]> =>
    USE_MOCK ? mock.listArtifacts(notebookId) : http.get(`/notebooks/${notebookId}/artifacts`),
  generate: (notebookId: string, kind: StudioKind, options?: GenerateArtifactOptions): Promise<StudioArtifact> =>
    USE_MOCK
      ? mock.generateArtifact(notebookId, kind, options)
      : http.post(`/notebooks/${notebookId}/artifacts`, {
          kind,
          ...(options?.topic?.trim() ? { topic: options.topic.trim() } : {}),
          ...(options?.preset ? { preset: options.preset } : {}),
          ...(options?.sourceIds ? { sourceIds: options.sourceIds } : {}),
        }),
  exportFile: async (artifact: StudioArtifact, format?: ExportFormat, inline = false): Promise<{ blob: Blob; filename: string }> => {
    const ext = format || artifact.format || (artifact.kind === 'ppt' ? 'pptx' : artifact.kind === 'pdf' ? 'pdf' : 'docx')
    if (USE_MOCK) return mock.exportArtifactFile(artifact, ext)
    const params = new URLSearchParams({ format: ext })
    if (inline && ext === 'pdf') params.set('inline', '1')
    return fetchBinary(`/artifacts/${artifact.id}/export?${params.toString()}`, `${artifact.title || '知页产物'}.${ext}`)
  },
  remove: (id: string): Promise<void> => (USE_MOCK ? mock.deleteArtifact(id) : http.del(`/artifacts/${id}`)),
  getAudio: (notebookId: string, signal?: AbortSignal): Promise<AudioOverview> =>
    USE_MOCK ? mock.getAudio(notebookId) : http.get(`/notebooks/${notebookId}/audio`, signal),
  pollAudio: async (initial: AudioOverview, onProgress: (audio: AudioOverview) => void, signal?: AbortSignal): Promise<AudioOverview> => {
    let audio = initial
    while (audio.status === 'generating') {
      await waitForPoll(signal)
      audio = await studioApi.getAudio(audio.notebookId, signal)
      signal?.throwIfAborted()
      onProgress(audio)
    }
    return audio
  },
  generateAudio: async (notebookId: string, onProgress: (audio: AudioOverview) => void, signal?: AbortSignal): Promise<AudioOverview> => {
    const audio = USE_MOCK ? await mock.generateAudio(notebookId) : await http.post<AudioOverview>(`/notebooks/${notebookId}/audio`, undefined, signal)
    signal?.throwIfAborted()
    onProgress(audio)
    return studioApi.pollAudio(audio, onProgress, signal)
  },
}

export const settingsApi = {
  get: (): Promise<UserSettings> => (USE_MOCK ? mock.getSettings() : http.get('/me/settings')),
  update: (patch: Partial<Pick<UserSettings, 'model' | 'answerStyle' | 'language'>>): Promise<UserSettings> =>
    USE_MOCK ? mock.updateSettings(patch) : http.patch('/me/settings', patch),
}
