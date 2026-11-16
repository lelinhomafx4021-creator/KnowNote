/**
 * 纯函数 PCM 重采样与 WAV 编码工具（无 DOM 依赖）。
 * 将麦克风采集到的 Float32 PCM 重采样到 16kHz 单声道并编码为 16-bit PCM WAV。
 */

export const WAV_TARGET_SAMPLE_RATE = 16000

/** 合并多段 Float32 PCM 采样块 */
export function mergePcmChunks(chunks: readonly Float32Array[]): Float32Array {
  let total = 0
  for (const chunk of chunks) total += chunk.length
  const merged = new Float32Array(total)
  let offset = 0
  for (const chunk of chunks) {
    merged.set(chunk, offset)
    offset += chunk.length
  }
  return merged
}

/** 将单声道 Float32 PCM 从源采样率重采样到目标采样率（默认 16kHz） */
export function resampleTo16kMono(
  input: Float32Array,
  sourceSampleRate: number,
  targetSampleRate = WAV_TARGET_SAMPLE_RATE,
): Float32Array {
  if (!Number.isFinite(sourceSampleRate) || sourceSampleRate <= 0) {
    throw new RangeError('源采样率必须大于 0')
  }
  if (!Number.isFinite(targetSampleRate) || targetSampleRate <= 0) {
    throw new RangeError('目标采样率必须大于 0')
  }
  if (input.length === 0) return new Float32Array(0)
  if (sourceSampleRate === targetSampleRate) return new Float32Array(input)

  const ratio = sourceSampleRate / targetSampleRate
  const outputLength = Math.max(1, Math.round(input.length / ratio))
  const output = new Float32Array(outputLength)

  if (ratio > 1) {
    // 降采样：对目标采样区间内的样本求均值，兼顾抗混叠与幅值稳定
    for (let i = 0; i < outputLength; i++) {
      const start = Math.floor(i * ratio)
      const end = Math.min(input.length, Math.floor((i + 1) * ratio))
      if (end <= start) {
        const idx = Math.min(input.length - 1, start)
        output[i] = input[idx]
        continue
      }
      let sum = 0
      for (let j = start; j < end; j++) {
        sum += input[j]
      }
      output[i] = sum / (end - start)
    }
    return output
  }

  // 升采样：线性插值
  for (let i = 0; i < outputLength; i++) {
    const pos = i * ratio
    const idx = Math.floor(pos)
    const frac = pos - idx
    const s0 = input[Math.min(idx, input.length - 1)]
    const s1 = input[Math.min(idx + 1, input.length - 1)]
    output[i] = s0 + (s1 - s0) * frac
  }
  return output
}

function writeAscii(view: DataView, offset: number, text: string) {
  for (let i = 0; i < text.length; i++) {
    view.setUint8(offset + i, text.charCodeAt(i))
  }
}

/** 将 Float32 PCM 编码为 16kHz 单声道 16-bit PCM WAV 字节数组（小端序） */
export function encodeWavBytes(
  pcmOrChunks: Float32Array | readonly Float32Array[],
  sourceSampleRate: number,
  targetSampleRate = WAV_TARGET_SAMPLE_RATE,
): Uint8Array {
  const mono = pcmOrChunks instanceof Float32Array ? pcmOrChunks : mergePcmChunks(pcmOrChunks)
  const samples = resampleTo16kMono(mono, sourceSampleRate, targetSampleRate)
  const dataLength = samples.length * 2
  const buffer = new ArrayBuffer(44 + dataLength)
  const view = new DataView(buffer)

  // RIFF 头
  writeAscii(view, 0, 'RIFF')
  view.setUint32(4, 36 + dataLength, true)
  writeAscii(view, 8, 'WAVE')

  // fmt 子块（PCM = 1，单声道 = 1，位深 = 16）
  writeAscii(view, 12, 'fmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true)
  view.setUint16(22, 1, true)
  view.setUint32(24, targetSampleRate, true)
  view.setUint32(28, targetSampleRate * 2, true)
  view.setUint16(32, 2, true)
  view.setUint16(34, 16, true)

  // data 子块
  writeAscii(view, 36, 'data')
  view.setUint32(40, dataLength, true)

  // 样本截断到 [-1, 1] 并转换为有符号 16-bit 小端整数
  let offset = 44
  for (let i = 0; i < samples.length; i++) {
    const raw = samples[i]
    const safe = Number.isFinite(raw) ? raw : 0
    const clamped = Math.max(-1, Math.min(1, safe))
    const int16 = clamped < 0 ? Math.round(clamped * 0x8000) : Math.round(clamped * 0x7fff)
    view.setInt16(offset, int16, true)
    offset += 2
  }

  return new Uint8Array(buffer)
}

/** 将 Float32 PCM 编码为 audio/wav Blob */
export function encodeWav(
  pcmOrChunks: Float32Array | readonly Float32Array[],
  sourceSampleRate: number,
  targetSampleRate = WAV_TARGET_SAMPLE_RATE,
): Blob {
  const bytes = encodeWavBytes(pcmOrChunks, sourceSampleRate, targetSampleRate)
  return new Blob([bytes.buffer as ArrayBuffer], { type: 'audio/wav' })
}
