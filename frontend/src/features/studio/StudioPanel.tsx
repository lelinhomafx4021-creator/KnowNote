import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  BookOpenCheck, FileBarChart2, MessageCircleQuestion, CalendarClock, Network, Loader2, Plus, StickyNote, ArrowLeft,
  Trash2, Eye, PencilLine, PanelRightClose, Maximize2, Pin, Sparkles, MessageSquareText,
} from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { confirmDialog, toast } from '@/store/useUIStore'
import { Button, IconButton } from '@/components/ui/Button'
import { EmptyState, Segmented } from '@/components/ui/misc'
import { Modal } from '@/components/ui/Modal'
import { PanelHeader } from '@/features/sources/SourcePanel'
import { AudioOverviewCard } from './AudioOverview'
import { MindMap } from './MindMap'
import type { Note, StudioArtifact, StudioKind } from '@/types'
import { cn, formatRelative } from '@/lib/utils'

const TOOLS: { kind: StudioKind; label: string; icon: typeof Network; tint: string }[] = [
  { kind: 'mindmap', label: '思维导图', icon: Network, tint: 'text-violet-500 bg-violet-500/10' },
  { kind: 'study-guide', label: '学习指南', icon: BookOpenCheck, tint: 'text-sky-500 bg-sky-500/10' },
  { kind: 'briefing', label: '简报文档', icon: FileBarChart2, tint: 'text-emerald-500 bg-emerald-500/10' },
  { kind: 'faq', label: '常见问题', icon: MessageCircleQuestion, tint: 'text-amber-500 bg-amber-500/10' },
  { kind: 'timeline', label: '时间线', icon: CalendarClock, tint: 'text-rose-500 bg-rose-500/10' },
]
const toolOf = (k: StudioKind) => TOOLS.find((t) => t.kind === k)!

export function StudioPanel({ onCollapse }: { onCollapse?: () => void }) {
  const { studioView, setStudioView, notes, artifacts } = useWorkspace()
  const note = studioView.type === 'note' ? notes.find((n) => n.id === studioView.id) : undefined
  const art = studioView.type === 'artifact' ? artifacts.find((a) => a.id === studioView.id) : undefined

  return (
    <div className="flex h-full flex-col">
      <PanelHeader title="工作室" onCollapse={onCollapse} collapseIcon={<PanelRightClose />} />
      {note ? (
        <NoteEditor key={note.id} note={note} onBack={() => setStudioView({ type: 'list' })} />
      ) : art ? (
        <ArtifactViewer art={art} onBack={() => setStudioView({ type: 'list' })} />
      ) : (
        <StudioHome />
      )}
    </div>
  )
}

function StudioHome() {
  const { generate, generating, notes, artifacts, createNote, setStudioView } = useWorkspace()

  type Item = { kind: 'note'; v: Note } | { kind: 'art'; v: StudioArtifact }
  const items: Item[] = [
    ...artifacts.map((v) => ({ kind: 'art' as const, v })),
    ...notes.map((v) => ({ kind: 'note' as const, v })),
  ].sort((a, b) => ('updatedAt' in b.v ? b.v.updatedAt : b.v.createdAt).localeCompare('updatedAt' in a.v ? a.v.updatedAt : a.v.createdAt))

  return (
    <div className="scroll-thin flex-1 overflow-y-auto">
      <div className="space-y-5 p-4">
        <AudioOverviewCard />

        <div>
          <h3 className="mb-2.5 px-1 text-sm font-medium">一键生成</h3>
          <div className="grid grid-cols-2 gap-2">
            {TOOLS.map(({ kind, label, icon: Icon, tint }) => {
              const busy = generating.includes(kind)
              return (
                <button
                  key={kind}
                  onClick={() => generate(kind)}
                  disabled={busy}
                  className={cn(
                    'focus-ring group flex items-center gap-2.5 rounded-2xl border border-border/70 bg-surface p-2.5 text-left text-[13px] font-medium transition-all hover:border-primary/30 hover:shadow-card disabled:cursor-wait',
                    kind === 'mindmap' && 'col-span-2',
                  )}
                >
                  <span className={cn('flex h-8 w-8 shrink-0 items-center justify-center rounded-xl', tint)}>
                    {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Icon className="h-4 w-4" />}
                  </span>
                  <span className="flex-1">{busy ? `正在生成${label}…` : label}</span>
                </button>
              )
            })}
          </div>
        </div>

        <div>
          <div className="mb-2 flex items-center px-1">
            <h3 className="flex-1 text-sm font-medium">笔记与生成内容</h3>
            <span className="text-xs text-subtle">{items.length}</span>
          </div>
          <Button variant="outline" size="sm" className="mb-2 w-full" icon={<Plus className="h-4 w-4" />} onClick={() => createNote()}>
            添加笔记
          </Button>
          {items.length === 0 ? (
            <EmptyState icon={<StickyNote />} title="还没有笔记" description="在对话中点击「保存到笔记」，或使用上方工具一键生成学习资料。" className="py-6" />
          ) : (
            <ul className="space-y-1.5">
              {items.map((it) =>
                it.kind === 'art' ? (
                  <ListRow
                    key={it.v.id}
                    icon={(() => {
                      const T = toolOf(it.v.kind)
                      return <span className={cn('flex h-8 w-8 items-center justify-center rounded-xl', T.tint)}><T.icon className="h-4 w-4" /></span>
                    })()}
                    title={it.v.title}
                    meta={`AI 生成 · ${formatRelative(it.v.createdAt)}`}
                    onClick={() => setStudioView({ type: 'artifact', id: it.v.id })}
                  />
                ) : (
                  <ListRow
                    key={it.v.id}
                    icon={
                      <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-surface-3 text-muted">
                        {it.v.origin === 'chat' ? <MessageSquareText className="h-4 w-4" /> : <StickyNote className="h-4 w-4" />}
                      </span>
                    }
                    title={it.v.title}
                    meta={`${it.v.origin === 'chat' ? '来自对话' : it.v.origin === 'studio' ? '保存的回答' : '手写笔记'} · ${formatRelative(it.v.updatedAt)}`}
                    preview={it.v.content.replace(/[#*>|-]/g, '').slice(0, 60)}
                    onClick={() => setStudioView({ type: 'note', id: it.v.id })}
                  />
                ),
              )}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}

function ListRow({ icon, title, meta, preview, onClick }: { icon: React.ReactNode; title: string; meta: string; preview?: string; onClick: () => void }) {
  return (
    <li>
      <button onClick={onClick} className="focus-ring flex w-full items-start gap-3 rounded-2xl p-2.5 text-left transition-colors hover:bg-surface-3/70">
        {icon}
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium">{title}</div>
          {preview && <div className="mt-0.5 line-clamp-2 text-xs leading-5 text-muted">{preview}</div>}
          <div className="mt-0.5 text-[11px] text-subtle">{meta}</div>
        </div>
      </button>
    </li>
  )
}

function SubHeader({ onBack, title, actions }: { onBack: () => void; title: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <div className="flex items-center gap-1 border-b border-border/50 px-2 py-1.5">
      <IconButton label="返回" size="sm" onClick={onBack}><ArrowLeft /></IconButton>
      <div className="min-w-0 flex-1 truncate px-1 text-xs text-muted">{title}</div>
      {actions}
    </div>
  )
}

function NoteEditor({ note, onBack }: { note: Note; onBack: () => void }) {
  const { updateNote, removeNote, addSource } = useWorkspace()
  const [title, setTitle] = useState(note.title)
  const [content, setContent] = useState(note.content)
  const [mode, setMode] = useState<'edit' | 'preview'>(note.content ? 'preview' : 'edit')
  const [saved, setSaved] = useState(true)

  // 自动保存（防抖 600ms）
  useEffect(() => {
    if (title === note.title && content === note.content) return
    setSaved(false)
    const t = setTimeout(() => updateNote(note.id, { title: title.trim() || '未命名笔记', content }).then(() => setSaved(true)), 600)
    return () => clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [title, content])

  return (
    <div className="flex min-h-0 flex-1 flex-col animate-fade-in">
      <SubHeader
        onBack={onBack}
        title={<>工作室 › 笔记 <span className="ml-2 text-subtle">{saved ? '已保存' : '保存中…'}</span></>}
        actions={
          <>
            <IconButton
              label="转换为来源"
              size="sm"
              onClick={() => {
                addSource({ type: 'text', title: `笔记：${title}`, text: content })
                toast('笔记正在转换为来源')
              }}
            >
              <Sparkles />
            </IconButton>
            <IconButton
              label="删除笔记"
              size="sm"
              onClick={async () => (await confirmDialog({ title: '删除这条笔记？', confirmText: '删除', danger: true })) && removeNote(note.id)}
            >
              <Trash2 />
            </IconButton>
          </>
        }
      />
      <div className="flex min-h-0 flex-1 flex-col p-4">
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="笔记标题"
          className="mb-3 w-full bg-transparent text-lg font-semibold outline-none placeholder:text-subtle"
        />
        <Segmented
          className="mb-3 self-start"
          value={mode}
          onChange={setMode}
          options={[
            { value: 'edit', label: '编辑', icon: <PencilLine /> },
            { value: 'preview', label: '预览', icon: <Eye /> },
          ]}
        />
        {mode === 'edit' ? (
          <textarea
            autoFocus
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="写点什么…支持 Markdown 语法"
            className="scroll-thin min-h-0 flex-1 resize-none rounded-2xl border border-border bg-surface-2 p-3.5 font-mono text-[13px] leading-6 outline-none focus:border-primary/50"
          />
        ) : (
          <div className="scroll-thin prose-ai min-h-0 flex-1 overflow-y-auto rounded-2xl bg-surface-2 p-4 text-sm" onDoubleClick={() => setMode('edit')}>
            {content ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown> : <p className="text-subtle">空笔记，双击开始编辑</p>}
          </div>
        )}
      </div>
    </div>
  )
}

function ArtifactViewer({ art, onBack }: { art: StudioArtifact; onBack: () => void }) {
  const { removeArtifact, createNote } = useWorkspace()
  const [full, setFull] = useState(false)
  const T = toolOf(art.kind)

  const body = art.mindmap ? (
    <MindMap data={art.mindmap} className="h-full min-h-[360px]" />
  ) : (
    <div className="prose-ai text-sm">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{art.content ?? ''}</ReactMarkdown>
    </div>
  )

  return (
    <div className="flex min-h-0 flex-1 flex-col animate-fade-in">
      <SubHeader
        onBack={onBack}
        title={`工作室 › ${art.title}`}
        actions={
          <>
            {!art.mindmap && (
              <IconButton
                label="保存到笔记"
                size="sm"
                onClick={async () => {
                  await createNote({ title: art.title, content: art.content ?? '', origin: 'studio' }, false)
                  toast('已保存到笔记', 'success')
                }}
              >
                <Pin />
              </IconButton>
            )}
            <IconButton label="全屏查看" size="sm" onClick={() => setFull(true)}><Maximize2 /></IconButton>
            <IconButton label="删除" size="sm" onClick={async () => (await confirmDialog({ title: `删除${art.title}？`, confirmText: '删除', danger: true })) && removeArtifact(art.id)}>
              <Trash2 />
            </IconButton>
          </>
        }
      />
      <div className="scroll-thin flex min-h-0 flex-1 flex-col overflow-y-auto p-4">
        <div className="mb-3 flex items-center gap-2.5">
          <span className={cn('flex h-9 w-9 items-center justify-center rounded-xl', T.tint)}><T.icon className="h-[18px] w-[18px]" /></span>
          <div>
            <div className="font-semibold">{art.title}</div>
            <div className="text-xs text-muted">基于所有来源生成 · {formatRelative(art.createdAt)}</div>
          </div>
        </div>
        <div className={art.mindmap ? 'min-h-0 flex-1' : ''}>{body}</div>
      </div>
      <Modal open={full} onClose={() => setFull(false)} title={art.title} size="xl" className="h-[88vh]">
        <div className={art.mindmap ? 'h-full pb-4' : 'pb-4'}>{art.mindmap ? <MindMap data={art.mindmap} className="h-full min-h-[60vh]" /> : body}</div>
      </Modal>
    </div>
  )
}
