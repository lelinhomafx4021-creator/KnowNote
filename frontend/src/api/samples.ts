import type { Notebook, SampleNotebook } from '@/types'
import { USE_MOCK, http } from './client'
import * as mock from './mock/handlers'

export const samplesApi = {
  list: (): Promise<SampleNotebook[]> => USE_MOCK ? mock.listSamples() : http.get('/samples'),
  use: (slug: string): Promise<Notebook> => USE_MOCK ? mock.useSample(slug) : http.post(`/samples/${encodeURIComponent(slug)}/use`, {}),
}
