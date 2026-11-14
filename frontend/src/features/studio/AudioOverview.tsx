import { useEffect, useRef, useState } from 'react'
import { AudioLines, Play, Pause, Download, RotateCcw, MoreVertical, Info } from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { Button, IconButton } from '@/components/ui/Button'
import { Dropdown } from '@/components/ui/Dropdown'
import { toast } from '@/store/useUIStore'
import { formatDuration } from '@/lib/utils'

const SPEEDS = [1, 1.25, 1.5, 2, 0.75]

export function AudioOverviewCard() {
  const { audio, generateAudio } = useWorkspace()
  const [playing, setPlaying] = useState(false)
  const [pos, setPos] = useState(0)
  const [speed, setSpeed] = useState(1)
  const timer = useRef<number>()
  const dur = audio.durationSec ?? 0

  // 演示用：用计时器模拟播放进度；接入后端后替换为 <audio src={audio.url}>
  useEffect(() => {
    if (!playing) return
    timer.current = window.setInterval(() => setPos((p) => (p + speed >= dur ? (setPlaying(false), dur) : p + speed)), 1000)
    return () => window.clearInterval(timer.current)
  }, [playing, speed, dur])

  useEffect(() => {
    setPlaying(false)
    setPos(0)
  }, [audio.notebookId, audio.status])

  return (
    <div className="rounded-3xl border border-border/70 bg-gradient-to-br from-primary-soft/80 via-surface to-surface p-4">
      <div className="flex items-center gap-2">
        <h3 className="flex-1 text-sm font-medium">音频概览</h3>
        <span title="AI 主持人以播客对谈形式讲解你的资料">
          <Info className="h-4 w-4 text-subtle" />
        </span>
      </div>

      {audio.status === 'idle' && (
        <>
          <div className="mt-3 flex items-center gap-3">
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary text-primary-fg">
              <AudioLines className="h-5 w-5" />
            </span>
            <p className="text-[13px] leading-5 text-muted">两位 AI 主持人以对谈形式，深入浅出地讲解你的资料。</p>
          </div>
          <div className="mt-3 flex gap-2">
            <Button size="sm" variant="outline" className="flex-1" onClick={() => toast('可在此输入关注重点（演示）')}>自定义</Button>
            <Button size="sm" className="flex-1" onClick={generateAudio}>生成</Button>
          </div>
        </>
      )}

      {audio.status === 'generating' && (
        <div className="mt-3 flex items-center gap-3">
          <div className="flex h-11 w-11 shrink-0 items-center justify-center gap-[3px] rounded-2xl bg-primary">
            {[0, 1, 2, 3].map((i) => (
              <span key={i} className="h-5 w-[3px] origin-center animate-wave rounded-full bg-white" style={{ animationDelay: `${i * 0.15}s` }} />
            ))}
          </div>
          <div className="min-w-0">
            <p className="text-sm font-medium">正在生成音频对谈…</p>
            <p className="text-xs text-muted">通常需要几分钟，可以先继续对话</p>
          </div>
        </div>
      )}

      {audio.status === 'ready' && (
        <div className="mt-3">
          <div className="flex items-center gap-3">
            <button
              onClick={() => (pos >= dur && setPos(0), setPlaying((p) => !p))}
              aria-label={playing ? '暂停' : '播放'}
              className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-primary text-primary-fg shadow-sm transition-transform active:scale-95"
            >
              {playing ? <Pause className="h-5 w-5 fill-current" /> : <Play className="ml-0.5 h-5 w-5 fill-current" />}
            </button>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{audio.title}</p>
              <p className="text-xs text-muted">{formatDuration(pos)} / {formatDuration(dur)}</p>
            </div>
            <Dropdown
              trigger={({ toggle }) => <IconButton label="更多" size="sm" onClick={toggle}><MoreVertical /></IconButton>}
              items={[
                { label: '下载音频', icon: <Download />, onClick: () => toast('开始下载（演示）') },
                { label: '重新生成', icon: <RotateCcw />, onClick: generateAudio },
              ]}
            />
          </div>
          <input
            type="range"
            min={0}
            max={dur}
            value={pos}
            onChange={(e) => setPos(Number(e.target.value))}
            className="mt-3 h-1 w-full cursor-pointer accent-[rgb(var(--primary))]"
            aria-label="播放进度"
          />
          <div className="mt-1 flex justify-end">
            <button onClick={() => setSpeed(SPEEDS[(SPEEDS.indexOf(speed) + 1) % SPEEDS.length])} className="rounded-full px-2 py-0.5 text-xs font-medium text-muted hover:bg-surface-3">
              {speed}x
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
