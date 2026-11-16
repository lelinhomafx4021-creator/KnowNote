import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import { ArrowUp, Square, Trash2, Pin, Upload, Sparkles, ArrowDown, MessagesSquare, Mic, X, Loader2 } from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { confirmDialog, toast } from '@/store/useUIStore'
import { IconButton, Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/misc'
import { UploadIllustration } from '@/components/ui/FeatureIcons'
import { MessageBubble } from './MessageBubble'
import { useVoiceInput } from './useVoiceInput'
import { PanelHeader } from '@/features/sources/SourcePanel'
import { cn, formatDuration } from '@/lib/utils'

export function ChatPanel({ onAddSource }: { onAddSource: () => void }) {
  const { messages, streaming, clearChat, loading } = useWorkspace()
  const scrollRef = useRef<HTMLDivElement>(null)
  const [atBottom, setAtBottom] = useState(true)

  // 新内容到达时，若用户停留在底部则自动跟随滚动
  useLayoutEffect(() => {
    const el = scrollRef.current
    if (el && atBottom) el.scrollTop = el.scrollHeight
  }, [messages, atBottom])

  const onScroll = () => {
    const el = scrollRef.current
    if (el) setAtBottom(el.scrollHeight - el.scrollTop - el.clientHeight < 80)
  }

  return (
    <div className="flex h-full flex-col">
      <PanelHeader
        title="对话"
        icon={<MessagesSquare />}
        extra={
          messages.length > 0 && (
            <IconButton
              label="清空对话"
              size="sm"
              disabled={streaming}
              onClick={async () => (await confirmDialog({ title: '清空对话记录？', description: '已保存到笔记的内容不受影响。', confirmText: '清空', danger: true })) && clearChat()}
            >
              <Trash2 />
            </IconButton>
          )
        }
      />
      <div ref={scrollRef} onScroll={onScroll} className="scroll-thin relative flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-3xl px-4 py-6 sm:px-6">
          {loading ? (
            <div className="space-y-3">
              <Skeleton className="h-10 w-10 rounded-2xl" />
              <Skeleton className="h-7 w-2/3" />
              <Skeleton className="h-24" />
            </div>
          ) : messages.length === 0 ? (
            <Welcome onAddSource={onAddSource} />
          ) : (
            <div className="space-y-7">
              {messages.map((m) => <MessageBubble key={m.id} m={m} />)}
            </div>
          )}
        </div>
      </div>
      {!atBottom && (
        <div className="relative">
          <button
            onClick={() => scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' })}
            className="absolute -top-12 left-1/2 flex h-9 w-9 -translate-x-1/2 items-center justify-center rounded-full border border-border bg-surface shadow-pop"
            aria-label="滚动到底部"
          >
            <ArrowDown className="h-4 w-4" />
          </button>
        </div>
      )}
      <ChatInput />
    </div>
  )
}

function Welcome({ onAddSource }: { onAddSource: () => void }) {
  const { notebook, sources, guide, guideLoading, ask, createNote } = useWorkspace()
  const ready = sources.filter((s) => s.status === 'ready')

  if (sources.length === 0) {
    return (
      <div className="flex flex-col items-center py-16 text-center">
        <div className="mb-4 text-primary">
          <UploadIllustration className="h-20 w-20" />
        </div>
        <h2 className="text-xl font-semibold">添加来源以开始使用</h2>
        <p className="mt-2 max-w-sm text-sm leading-6 text-muted">上传论文、讲义、网页或会议录音，AI 将基于这些资料与你对话、整理笔记。</p>
        <Button className="mt-6" icon={<Upload className="h-4 w-4" />} onClick={onAddSource}>上传来源</Button>
      </div>
    )
  }

  return (
    <div className="animate-fade-in">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-surface-2 text-3xl ring-1 ring-inset ring-border/80">{notebook?.emoji}</div>
      <h1 className="mt-5 text-2xl font-semibold leading-snug tracking-tight sm:text-[26px]">{notebook?.title}</h1>
      <p className="mt-1.5 text-sm text-muted">{ready.length} 个来源</p>
      <div className="mt-6">
        {guideLoading ? (
          <div className="space-y-2.5">
            <Skeleton className="h-4" />
            <Skeleton className="h-4 w-11/12" />
            <Skeleton className="h-4 w-4/5" />
          </div>
        ) : !guide ? <p className="text-sm text-muted">笔记本概要暂不可用，请稍后重新加载。</p> : (
          <>
            <div className="prose-ai">
              <ReactMarkdown>{guide.summary}</ReactMarkdown>
            </div>
            <div className="mt-4 flex gap-2">
              <Button
                variant="outline"
                size="sm"
                icon={<Pin className="h-3.5 w-3.5" />}
                onClick={async () => {
                  const note = await createNote({ title: '笔记本概要', content: guide.summary, origin: 'studio' }, false)
                  if (note) toast('已保存到笔记', 'success')
                }}
              >
                保存到笔记
              </Button>
            </div>
            {guide.suggestedQuestions.length > 0 && (
              <div className="mt-8">
                <div className="mb-3 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-subtle">
                  <Sparkles className="h-3.5 w-3.5" /> 你可以这样问
                </div>
                <div className="flex flex-col items-start gap-2">
                  {guide.suggestedQuestions.map((q) => (
                    <button key={q} onClick={() => ask(q)} className="focus-ring rounded-2xl border border-border bg-surface px-4 py-2.5 text-left text-sm transition-colors hover:border-primary/40 hover:bg-primary-soft/50">
                      {q}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function ChatInput() {
  const { ask, stop, streaming, selectedIds, sources, guide, messages, notebookId } = useWorkspace()
  const [val, setVal] = useState('')
  const ta = useRef<HTMLTextAreaElement>(null)
  const voiceGroupRef = useRef<HTMLDivElement>(null)
  const selectionRef = useRef<{ start: number; end: number } | null>(null)
  const pendingCursorRef = useRef<number | null>(null)

  const ready = sources.filter((s) => s.status === 'ready').length
  const disabled = ready === 0

  const insertVoiceText = (raw: string) => {
    const inserted = raw.trim()
    if (!inserted) return
    setVal((prev) => {
      const sel = selectionRef.current
      const start = sel ? Math.min(sel.start, prev.length) : prev.length
      const end = sel ? Math.min(sel.end, prev.length) : prev.length
      const before = prev.slice(0, start)
      const after = prev.slice(end)
      const needLeadingSpace = before.length > 0 && !/[\s，。！？、；：“‘（《【,.!?;:(\[{]$/.test(before)
      const needTrailingSpace =
        after.length > 0 &&
        !/^[\s，。！？、；：”’）》】,.!?;:)\]}]/.test(after) &&
        !/[，。！？、；：,.!?]$/.test(inserted)
      const prefix = needLeadingSpace ? ' ' : ''
      const suffix = needTrailingSpace ? ' ' : ''
      const next = `${before}${prefix}${inserted}${suffix}${after}`
      const cursor = before.length + prefix.length + inserted.length
      pendingCursorRef.current = cursor
      selectionRef.current = { start: cursor, end: cursor }
      return next
    })
  }

  const voice = useVoiceInput({
    disabled,
    streaming,
    notebookId,
    onResult: insertVoiceText,
  })

  const isRecording = voice.status === 'recording'
  const isTranscribing = voice.status === 'transcribing'
  const inputDisabled = disabled || isTranscribing

  useEffect(() => {
    const el = ta.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 180) + 'px'
  }, [val])

  // 识别结果填入并解除输入框禁用后，重新聚焦并恢复光标位置与高度
  useEffect(() => {
    if (voice.status !== 'idle' || pendingCursorRef.current === null) return
    const el = ta.current
    if (!el) return
    const cursor = pendingCursorRef.current
    pendingCursorRef.current = null
    el.focus()
    el.setSelectionRange(cursor, cursor)
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 180) + 'px'
  }, [val, voice.status])

  const syncSelection = () => {
    const el = ta.current
    if (!el) return
    selectionRef.current = {
      start: el.selectionStart ?? el.value.length,
      end: el.selectionEnd ?? el.value.length,
    }
  }

  const send = () => {
    if (!val.trim() || streaming || disabled || isRecording || isTranscribing) return
    selectionRef.current = null
    ask(val)
    setVal('')
  }

  return (
    <div className="shrink-0 px-3 pb-3 sm:px-6 sm:pb-4">
      <div className="mx-auto w-full max-w-3xl">
        {messages.length > 0 && !streaming && guide?.suggestedQuestions.length ? (
          <div className="scroll-thin -mx-1 mb-2 flex gap-2 overflow-x-auto px-1 pb-1">
            {guide.suggestedQuestions.map((q) => (
              <button key={q} onClick={() => ask(q)} className="shrink-0 rounded-full border border-border bg-surface px-3 py-1.5 text-[13px] text-muted transition-colors hover:border-primary/40 hover:text-fg">
                {q}
              </button>
            ))}
          </div>
        ) : null}
        <div
          className={cn(
            'flex items-end gap-2 rounded-[28px] border bg-surface p-2 pl-5 shadow-card transition-colors focus-within:border-primary/50 focus-within:shadow-pop',
            disabled ? 'border-border opacity-70' : 'border-border',
          )}
        >
          <textarea
            ref={ta}
            rows={1}
            value={val}
            disabled={inputDisabled}
            onChange={(e) => {
              setVal(e.target.value)
              selectionRef.current = {
                start: e.target.selectionStart ?? e.target.value.length,
                end: e.target.selectionEnd ?? e.target.value.length,
              }
            }}
            onSelect={syncSelection}
            onClick={syncSelection}
            onKeyUp={syncSelection}
            onBlur={(e) => {
              const next = e.relatedTarget as Node | null
              if (next && voiceGroupRef.current?.contains(next)) {
                syncSelection()
                return
              }
              selectionRef.current = null
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                e.preventDefault()
                send()
              }
            }}
            placeholder={disabled ? '上传来源后即可开始提问' : isTranscribing ? '正在识别…' : '基于你的资料提问…'}
            className="scroll-thin max-h-[180px] min-w-0 flex-1 resize-none bg-transparent py-2 text-[15px] leading-6 outline-none placeholder:text-subtle"
          />
          <span className="mb-2 hidden shrink-0 rounded-full bg-surface-3 px-2.5 py-1 text-xs text-muted sm:inline">
            {selectedIds.length} 个来源
          </span>
          {voice.visible && (
            <div ref={voiceGroupRef} className="flex shrink-0 items-center">
              {!voice.supported ? (
                <button
                  type="button"
                  disabled={disabled || streaming}
                  aria-disabled="true"
                  aria-label="语音输入需要 HTTPS 或 localhost 访问"
                  title="语音输入需要 HTTPS 或 localhost 访问"
                  onClick={() => {
                    if (!disabled && !streaming) toast('语音输入需要 HTTPS 或 localhost 访问')
                  }}
                  className="focus-ring flex h-10 w-10 shrink-0 cursor-not-allowed items-center justify-center rounded-full text-subtle opacity-40 transition-colors"
                >
                  <Mic className="h-5 w-5" />
                </button>
              ) : isRecording ? (
                <div className="flex items-center gap-1 rounded-full bg-danger/10 pl-1">
                  <button
                    type="button"
                    onClick={voice.cancelRecording}
                    aria-label="取消录音"
                    title="取消录音 (Esc)"
                    className="focus-ring flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-muted transition-colors hover:bg-surface-3 hover:text-fg"
                  >
                    <X className="h-4 w-4" />
                  </button>
                  <span className="min-w-[2.4rem] text-center font-mono text-xs font-medium tabular-nums text-danger" aria-live="polite">
                    {formatDuration(voice.elapsedSeconds)}
                  </span>
                  <button
                    type="button"
                    onClick={() => void voice.stopRecording()}
                    aria-label="停止录音并识别"
                    title="停止录音并识别"
                    className="focus-ring flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-danger text-danger-fg shadow-sm transition-transform hover:bg-danger/90 active:scale-95"
                  >
                    <Square className="h-3.5 w-3.5 fill-current" />
                  </button>
                </div>
              ) : isTranscribing ? (
                <button
                  type="button"
                  disabled
                  aria-label="正在识别"
                  title="正在识别…"
                  className="focus-ring flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-soft text-primary"
                >
                  <Loader2 className="h-5 w-5 animate-spin" />
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => void voice.startRecording()}
                  disabled={disabled || streaming}
                  aria-label="开始语音输入"
                  title="开始语音输入"
                  className="focus-ring flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-muted transition-colors hover:bg-surface-3 hover:text-fg disabled:pointer-events-none disabled:opacity-40"
                >
                  <Mic className="h-5 w-5" />
                </button>
              )}
            </div>
          )}
          {streaming ? (
            <button onClick={stop} aria-label="停止生成" className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-fg text-bg transition-transform active:scale-95">
              <Square className="h-3.5 w-3.5 fill-current" />
            </button>
          ) : (
            <button
              onClick={send}
              disabled={!val.trim() || disabled || isRecording || isTranscribing}
              aria-label="发送"
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary text-primary-fg transition-all active:scale-95 disabled:bg-surface-3 disabled:text-subtle"
            >
              <ArrowUp className="h-5 w-5" />
            </button>
          )}
        </div>
        <p className="mt-2 text-center text-[11px] text-subtle">AI 的回答仅基于所选来源生成，仍可能存在不准确之处，请核对引用内容。</p>
      </div>
    </div>
  )
}
