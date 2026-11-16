import { useCallback, useEffect, useRef, useState } from 'react'
import { speechApi } from '@/api'
import { ApiError, isAbort } from '@/api/client'
import { encodeWav } from '@/lib/wav'
import { toast } from '@/store/useUIStore'
import type { SpeechConfig } from '@/types'

export type VoiceInputStatus = 'idle' | 'recording' | 'transcribing'

export interface UseVoiceInputOptions {
  disabled?: boolean
  streaming?: boolean
  notebookId?: string | null
  onResult: (text: string) => void
}

export interface UseVoiceInputReturn {
  /** 后端启用语音输入后才在界面展示麦克风 */
  visible: boolean
  /** 浏览器是否处于安全上下文并具备麦克风采集能力 */
  supported: boolean
  status: VoiceInputStatus
  elapsedSeconds: number
  maxSeconds: number
  startRecording: () => Promise<void>
  stopRecording: () => Promise<void>
  cancelRecording: () => void
}

const MIN_RECORD_MS = 500
const DEFAULT_MAX_SECONDS = 60
const WORKLET_PROCESSOR_NAME = 'knownote-pcm-capture'
const WORKLET_SOURCE = `
class KnowNotePcmCaptureProcessor extends AudioWorkletProcessor {
  process(inputs) {
    const channel = inputs[0]?.[0]
    if (channel && channel.length > 0) {
      this.port.postMessage(new Float32Array(channel))
    }
    return true
  }
}
registerProcessor('${WORKLET_PROCESSOR_NAME}', KnowNotePcmCaptureProcessor)
`

function hasMediaSupport(): boolean {
  return typeof navigator !== 'undefined' && typeof navigator.mediaDevices?.getUserMedia === 'function'
}

function mapMediaErrorMessage(err: unknown): string {
  const name = err instanceof DOMException || err instanceof Error ? err.name : ''
  if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
    return '无法使用麦克风，请在浏览器地址栏中允许麦克风权限'
  }
  if (name === 'NotFoundError' || name === 'DevicesNotFoundError') {
    return '没有检测到麦克风'
  }
  if (err instanceof Error && err.message) {
    return err.message
  }
  return '无法启动麦克风，请重试'
}

export function useVoiceInput({
  disabled = false,
  streaming = false,
  notebookId,
  onResult,
}: UseVoiceInputOptions): UseVoiceInputReturn {
  const [config, setConfig] = useState<SpeechConfig | null>(null)
  const [status, setStatus] = useState<VoiceInputStatus>('idle')
  const [elapsedSeconds, setElapsedSeconds] = useState(0)

  const supported = hasMediaSupport()
  const maxSeconds = config?.maxSeconds && config.maxSeconds > 0 ? config.maxSeconds : DEFAULT_MAX_SECONDS
  const visible = Boolean(config?.enabled)

  const onResultRef = useRef(onResult)
  onResultRef.current = onResult

  const statusRef = useRef<VoiceInputStatus>('idle')
  statusRef.current = status

  const sessionIdRef = useRef(0)
  const startedAtRef = useRef(0)
  const sampleRateRef = useRef(16000)
  const chunksRef = useRef<Float32Array[]>([])
  const totalSamplesRef = useRef(0)

  const streamRef = useRef<MediaStream | null>(null)
  const audioCtxRef = useRef<AudioContext | null>(null)
  const sourceNodeRef = useRef<MediaStreamAudioSourceNode | null>(null)
  const captureNodeRef = useRef<AudioNode | null>(null)
  const silentGainRef = useRef<GainNode | null>(null)
  const tickTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const maxTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const transcribeAbortRef = useRef<AbortController | null>(null)

  const releaseAudio = useCallback(() => {
    if (tickTimerRef.current !== null) {
      clearInterval(tickTimerRef.current)
      tickTimerRef.current = null
    }
    if (maxTimerRef.current !== null) {
      clearTimeout(maxTimerRef.current)
      maxTimerRef.current = null
    }

    const captureNode = captureNodeRef.current
    if (captureNode) {
      if ('onaudioprocess' in captureNode) {
        ;(captureNode as ScriptProcessorNode).onaudioprocess = null
      }
      if ('port' in captureNode) {
        ;(captureNode as AudioWorkletNode).port.onmessage = null
      }
      try { captureNode.disconnect() } catch { /* 忽略已断开节点 */ }
      captureNodeRef.current = null
    }

    if (sourceNodeRef.current) {
      try { sourceNodeRef.current.disconnect() } catch { /* 忽略已断开节点 */ }
      sourceNodeRef.current = null
    }

    if (silentGainRef.current) {
      try { silentGainRef.current.disconnect() } catch { /* 忽略已断开节点 */ }
      silentGainRef.current = null
    }

    if (streamRef.current) {
      for (const track of streamRef.current.getTracks()) {
        track.onended = null
        track.stop()
      }
      streamRef.current = null
    }

    if (audioCtxRef.current) {
      const ctx = audioCtxRef.current
      audioCtxRef.current = null
      void ctx.close().catch(() => {})
    }
  }, [])

  const resetBuffers = useCallback(() => {
    chunksRef.current = []
    totalSamplesRef.current = 0
    startedAtRef.current = 0
  }, [])

  const cancelRecording = useCallback(() => {
    sessionIdRef.current += 1
    transcribeAbortRef.current?.abort()
    transcribeAbortRef.current = null
    releaseAudio()
    resetBuffers()
    setStatus('idle')
    setElapsedSeconds(0)
  }, [releaseAudio, resetBuffers])

  const stopRecording = useCallback(async () => {
    if (statusRef.current !== 'recording') return

    const activeSessionId = sessionIdRef.current
    const durationMs = Date.now() - startedAtRef.current
    const chunks = chunksRef.current
    const totalSamples = totalSamplesRef.current
    const sampleRate = sampleRateRef.current

    releaseAudio()
    resetBuffers()

    if (durationMs < MIN_RECORD_MS || totalSamples === 0) {
      setStatus('idle')
      setElapsedSeconds(0)
      toast('录音太短，请再试一次', 'error')
      return
    }

    setStatus('transcribing')
    const controller = new AbortController()
    transcribeAbortRef.current = controller

    try {
      const wav = encodeWav(chunks, sampleRate)
      const result = await speechApi.transcribe(wav, undefined, controller.signal)
      if (sessionIdRef.current !== activeSessionId || controller.signal.aborted) return

      const text = result.text.trim()
      if (text) {
        onResultRef.current(text)
      } else {
        toast('未识别到有效语音，请再试一次')
      }
    } catch (err) {
      if (isAbort(err) || sessionIdRef.current !== activeSessionId) return
      if (!(err instanceof ApiError && err.reported)) {
        toast(err instanceof Error && err.message ? err.message : '语音识别失败，请重试', 'error')
      }
    } finally {
      if (transcribeAbortRef.current === controller) {
        transcribeAbortRef.current = null
      }
      if (sessionIdRef.current === activeSessionId) {
        setStatus('idle')
        setElapsedSeconds(0)
      }
    }
  }, [releaseAudio, resetBuffers])

  const stopRecordingRef = useRef(stopRecording)
  stopRecordingRef.current = stopRecording

  const startRecording = useCallback(async () => {
    if (statusRef.current !== 'idle' || disabled || streaming || !visible) return
    if (!supported) {
      toast('语音输入需要 HTTPS 或 localhost 访问')
      return
    }

    const activeSessionId = sessionIdRef.current + 1
    sessionIdRef.current = activeSessionId
    resetBuffers()

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      })

      if (sessionIdRef.current !== activeSessionId) {
        stream.getTracks().forEach((t) => t.stop())
        return
      }

      const AudioCtx = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
      if (!AudioCtx) {
        stream.getTracks().forEach((t) => t.stop())
        toast('当前浏览器不支持音频采集', 'error')
        return
      }

      const ctx = new AudioCtx()
      if (ctx.state === 'suspended') {
        await ctx.resume()
      }

      if (sessionIdRef.current !== activeSessionId) {
        stream.getTracks().forEach((t) => t.stop())
        void ctx.close().catch(() => {})
        return
      }

      streamRef.current = stream
      audioCtxRef.current = ctx
      sampleRateRef.current = ctx.sampleRate

      const source = ctx.createMediaStreamSource(stream)
      const silentGain = ctx.createGain()
      silentGain.gain.value = 0
      sourceNodeRef.current = source
      silentGainRef.current = silentGain

      const pushChunk = (data: Float32Array) => {
        if (sessionIdRef.current !== activeSessionId || statusRef.current !== 'recording') return
        chunksRef.current.push(data)
        totalSamplesRef.current += data.length
      }

      // 优先使用 AudioWorklet；若受 CSP 或浏览器兼容性限制加载失败，则退回 ScriptProcessorNode
      let captureNode: AudioNode | null = null
      if (ctx.audioWorklet) {
        const blobUrl = URL.createObjectURL(new Blob([WORKLET_SOURCE], { type: 'application/javascript' }))
        try {
          await ctx.audioWorklet.addModule(blobUrl)
          if (sessionIdRef.current !== activeSessionId) return
          const workletNode = new AudioWorkletNode(ctx, WORKLET_PROCESSOR_NAME, {
            numberOfInputs: 1,
            numberOfOutputs: 1,
            channelCount: 1,
          })
          workletNode.port.onmessage = (event: MessageEvent<Float32Array>) => {
            if (event.data && event.data.length > 0) pushChunk(event.data)
          }
          captureNode = workletNode
        } catch {
          captureNode = null
        } finally {
          URL.revokeObjectURL(blobUrl)
        }
      }

      if (!captureNode) {
        // 退化方案：部分环境禁用 blob AudioWorklet 或缺少 audioWorklet 支持时使用 ScriptProcessorNode
        const processor = ctx.createScriptProcessor(4096, 1, 1)
        processor.onaudioprocess = (event: AudioProcessingEvent) => {
          const input = event.inputBuffer.getChannelData(0)
          if (input.length > 0) pushChunk(new Float32Array(input))
        }
        captureNode = processor
      }

      if (sessionIdRef.current !== activeSessionId) {
        releaseAudio()
        return
      }

      captureNodeRef.current = captureNode
      source.connect(captureNode)
      captureNode.connect(silentGain)
      silentGain.connect(ctx.destination)

      for (const track of stream.getTracks()) {
        track.onended = () => {
          if (sessionIdRef.current === activeSessionId && statusRef.current === 'recording') {
            cancelRecording()
            toast('麦克风连接已中断', 'error')
          }
        }
      }

      startedAtRef.current = Date.now()
      statusRef.current = 'recording'
      setStatus('recording')
      setElapsedSeconds(0)

      tickTimerRef.current = setInterval(() => {
        if (sessionIdRef.current !== activeSessionId) return
        const sec = Math.min(maxSeconds, Math.floor((Date.now() - startedAtRef.current) / 1000))
        setElapsedSeconds(sec)
      }, 200)

      // 提前 0.5 秒自动停止：定时器与音频回调都有抖动，恰好卡在上限会让后端判定“超过 60 秒”而拒绝。
      maxTimerRef.current = setTimeout(() => {
        if (sessionIdRef.current === activeSessionId && statusRef.current === 'recording') {
          void stopRecordingRef.current()
        }
      }, Math.max(1000, maxSeconds * 1000 - 500))
    } catch (err) {
      if (sessionIdRef.current !== activeSessionId) return
      releaseAudio()
      resetBuffers()
      setStatus('idle')
      setElapsedSeconds(0)
      toast(mapMediaErrorMessage(err), 'error')
    }
  }, [cancelRecording, disabled, maxSeconds, releaseAudio, resetBuffers, streaming, supported, visible])

  // 加载语音输入配置
  useEffect(() => {
    const controller = new AbortController()
    speechApi
      .getConfig(controller.signal)
      .then((res) => {
        if (!controller.signal.aborted) setConfig(res)
      })
      .catch((err) => {
        if (!isAbort(err) && !controller.signal.aborted) {
          setConfig({ enabled: false, maxSeconds: DEFAULT_MAX_SECONDS })
        }
      })
    return () => controller.abort()
  }, [])

  // 录音中按 Esc 丢弃录音
  useEffect(() => {
    if (status !== 'recording') return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        cancelRecording()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [status, cancelRecording])

  // 切换笔记本、开始 AI 回答或来源不可用时中断并清理录音
  useEffect(() => {
    if (streaming || disabled) {
      if (statusRef.current !== 'idle') cancelRecording()
    }
  }, [streaming, disabled, cancelRecording])

  const prevNotebookIdRef = useRef(notebookId)
  useEffect(() => {
    if (prevNotebookIdRef.current !== notebookId) {
      prevNotebookIdRef.current = notebookId
      if (statusRef.current !== 'idle') cancelRecording()
    }
  }, [notebookId, cancelRecording])

  // 组件卸载时释放麦克风与音频上下文
  useEffect(() => () => { cancelRecording() }, [cancelRecording])

  return {
    visible,
    supported,
    status,
    elapsedSeconds,
    maxSeconds,
    startRecording,
    stopRecording,
    cancelRecording,
  }
}
