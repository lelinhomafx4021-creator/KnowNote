import { useEffect, useRef, useState } from 'react'
import { AudioLines, Play, Pause, RotateCcw, Info, AlertCircle, ChevronDown } from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { useAuthStore } from '@/store/useAuthStore'
import { Button } from '@/components/ui/Button'
import { audioFileUrl } from '@/api/client'
import { toast } from '@/store/useUIStore'
import { formatDuration } from '@/lib/utils'

const SPEEDS = [1, 1.25, 1.5, 2, 0.75]
export function AudioOverviewCard() {
  const { audio, generateAudio } = useWorkspace()
  const token = useAuthStore((s) => s.token)
  const player = useRef<HTMLAudioElement>(null)
  const [playing, setPlaying] = useState(false)
  const [pos, setPos] = useState(0)
  const [duration, setDuration] = useState(0)
  const [speed, setSpeed] = useState(1)
  const [expanded, setExpanded] = useState(false)
  const [mediaError, setMediaError] = useState(false)
  const src = audio.url ? audioFileUrl(audio.url, token) : ''
  const dur = duration || audio.durationSec || 0

  useEffect(() => {
    setPlaying(false); setPos(0); setDuration(0); setSpeed(1); setExpanded(false); setMediaError(false)
  }, [audio.notebookId, audio.status, src])

  const toggle = async () => {
    const element = player.current
    if (!element) return
    if (!element.paused) element.pause()
    else {
      if (element.ended) element.currentTime = 0
      try { await element.play() } catch { toast('音频暂时无法播放，请稍后重试', 'error') }
    }
  }
  return (
    <div className="rounded-3xl border border-border/70 bg-gradient-to-br from-primary-soft/80 via-surface to-surface p-4">
      <div className="flex items-center gap-2"><h3 className="flex-1 text-sm font-medium">音频概览</h3><span title="AI 主持人以播客对谈形式讲解你的资料"><Info className="h-4 w-4 text-subtle" /></span></div>
      {audio.status === 'idle' && <>
        <div className="mt-3 flex items-center gap-3"><span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary text-primary-fg"><AudioLines className="h-5 w-5" /></span><p className="text-[13px] leading-5 text-muted">两位 AI 主持人以对谈形式，深入浅出地讲解你的资料。</p></div>
        <Button size="sm" className="mt-3 w-full" onClick={generateAudio}>生成对谈</Button>
      </>}
      {audio.status === 'generating' && <div className="mt-3 flex items-center gap-3"><div className="flex h-11 w-11 shrink-0 items-center justify-center gap-[3px] rounded-2xl bg-primary">{[0, 1, 2, 3].map((i) => <span key={i} className="h-5 w-[3px] animate-wave rounded-full bg-primary-fg" style={{ animationDelay: `${i * 0.15}s` }} />)}</div><div><p className="text-sm font-medium">正在生成音频对谈…</p><p className="text-xs text-muted">可以先继续对话，完成后会自动更新</p></div></div>}
      {audio.status === 'error' && <div className="mt-3"><p role="alert" className="flex items-center gap-2 text-sm text-danger"><AlertCircle className="h-4 w-4" />音频概览暂不可用，请重试</p><Button size="sm" variant="outline" className="mt-3" onClick={generateAudio}>重新生成</Button></div>}
      {audio.status === 'ready' && <div className="mt-3">
        {src ? <>
          <audio key={src} ref={player} src={src} preload="metadata"
            onPlay={() => setPlaying(true)} onPause={() => setPlaying(false)} onEnded={() => setPlaying(false)}
            onTimeUpdate={(e) => setPos(e.currentTarget.currentTime)}
            onLoadedMetadata={(e) => { if (Number.isFinite(e.currentTarget.duration)) setDuration(e.currentTarget.duration); e.currentTarget.playbackRate = speed }}
            onError={() => { setPlaying(false); setMediaError(true) }} />
          <div className="flex items-center gap-3"><button onClick={() => void toggle()} aria-label={playing ? '暂停' : '播放'} disabled={mediaError} className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-primary text-primary-fg shadow-sm disabled:opacity-50">{playing ? <Pause className="h-5 w-5 fill-current" /> : <Play className="ml-0.5 h-5 w-5 fill-current" />}</button><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{audio.title || '资料深度对谈'}</p><p className="text-xs text-muted">{formatDuration(pos)} / {formatDuration(dur)}</p></div></div>
          <input type="range" min={0} max={dur} step={0.1} value={Math.min(pos, dur)} disabled={!dur || mediaError} onChange={(e) => { if (player.current) { player.current.currentTime = Number(e.target.value); setPos(Number(e.target.value)) } }} className="mt-3 h-1 w-full cursor-pointer accent-[rgb(var(--primary))]" aria-label="播放进度" />
          <div className="mt-1 flex justify-end"><button aria-label="切换播放速度" onClick={() => { const next = SPEEDS[(SPEEDS.indexOf(speed) + 1) % SPEEDS.length]; setSpeed(next); if (player.current) player.current.playbackRate = next }} className="rounded-full px-2 py-0.5 text-xs font-medium text-muted hover:bg-surface-3">{speed}x</button></div>
          {mediaError && <p role="alert" className="mt-2 text-xs text-danger">音频加载失败，请重新生成或查看文稿。</p>}
        </> : <p className="text-sm font-medium">{audio.title || '资料深度对谈'}</p>}
        {!src && !audio.transcript && <p className="mt-2 text-xs text-muted">暂未提供音频或文稿，请重新生成。</p>}
        {!!audio.transcript && <><button aria-expanded={expanded} onClick={() => setExpanded(!expanded)} className="focus-ring mt-3 flex w-full items-center justify-between rounded-xl bg-surface-2 px-3 py-2.5 text-xs font-medium text-primary">{expanded ? '收起对谈文稿' : '查看对谈文稿'}<ChevronDown className={`h-4 w-4 transition-transform ${expanded ? 'rotate-180' : ''}`} /></button>{expanded && <div className="scroll-thin mt-3 max-h-80 overflow-y-auto whitespace-pre-wrap rounded-2xl bg-surface/80 p-3 text-[13px] leading-6">{audio.transcript}</div>}</>}
        <Button size="sm" variant="ghost" className="mt-2" icon={<RotateCcw className="h-3.5 w-3.5" />} onClick={generateAudio}>重新生成</Button>
      </div>}
    </div>
  )
}
