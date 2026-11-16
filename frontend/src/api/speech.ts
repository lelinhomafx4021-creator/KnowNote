import type { SpeechConfig, SpeechResult } from '@/types'
import { USE_MOCK, http } from './client'
import * as mock from './mock/handlers'

export interface TranscribeOptions {
  language?: string
}

export const speechApi = {
  /** 获取语音输入配置（是否启用及最长录音秒数） */
  getConfig: (signal?: AbortSignal): Promise<SpeechConfig> =>
    USE_MOCK ? mock.getSpeechConfig() : http.get<SpeechConfig>('/speech/config', signal),

  /** 上传 16kHz 单声道 16-bit WAV 音频进行语音转文字 */
  transcribe: (wav: Blob, options?: TranscribeOptions, signal?: AbortSignal): Promise<SpeechResult> => {
    if (USE_MOCK) return mock.transcribe(wav, options, signal)
    const form = new FormData()
    form.append('audio', wav, 'recording.wav')
    const language = options?.language?.trim()
    if (language) form.append('language', language)
    return http.post<SpeechResult>('/speech/transcribe', form, signal)
  },
}
