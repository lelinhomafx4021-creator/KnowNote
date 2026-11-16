import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  Loader2, Plus, StickyNote, ArrowLeft, Download, ChevronDown, Sparkles,
  Trash2, Eye, PencilLine, PanelRightClose, Maximize2, Pin, MessageSquareText, Wand2, FileDown,
  SlidersHorizontal, Check, Columns2,
} from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { confirmDialog, toast } from '@/store/useUIStore'
import { studioApi, notesApi } from '@/api'
import { USE_MOCK, triggerBlobDownload } from '@/api/client'
import { Button, IconButton } from '@/components/ui/Button'
import { Dropdown } from '@/components/ui/Dropdown'
import { EmptyState, Segmented } from '@/components/ui/misc'
import { Modal } from '@/components/ui/Modal'
import { StudioToolIcon } from '@/components/ui/FeatureIcons'
import { SourceIcon } from '@/components/ui/SourceIcon'
import { PanelHeader } from '@/features/sources/SourcePanel'
import { MindMap } from './MindMap'
import { SlideDeckViewer } from './SlideDeckViewer'
import type { ExportFormat, Note, StudioArtifact, StudioKind } from '@/types'
import { cn, formatRelative } from '@/lib/utils'

type GenerateKind = StudioKind

// 拖拽产物时使用的自定义数据类型，只携带产物 id，避免与普通文字拖拽混淆
const ARTIFACT_DRAG_TYPE = 'application/x-knownote-artifact'

interface GeneratorPreset {
  id: string
  label: string
}

interface PrimaryGenerator {
  kind: 'ppt' | 'word' | 'pdf'
  label: string
  ext: ExportFormat
  extLabel: string
  hint: string
  tint: string
  badge: string
}

const TOOL_PRESETS: Record<GenerateKind, GeneratorPreset[]> = {
  ppt: [
    { id: 'classroom', label: '课堂课件' },
    { id: 'review', label: '复习串讲' },
    { id: 'seminar', label: '教研汇报' },
  ],
  word: [
    { id: 'lesson-plan', label: '教案讲义' },
    { id: 'exam-bank', label: '随堂试题' },
    { id: 'summary-doc', label: '专题报告' },
  ],
  pdf: [
    { id: 'handout', label: '学生讲义' },
    { id: 'study-sheet', label: '知识清单' },
    { id: 'reading-digest', label: '精读笔记' },
  ],
  mindmap: [
    { id: 'overview', label: '全局知识树' },
    { id: 'compare', label: '概念对比图' },
    { id: 'chapter', label: '章节脉络' },
  ],
  'study-guide': [
    { id: 'exam', label: '考前冲刺' },
    { id: 'concept', label: '概念精讲' },
    { id: 'practice', label: '习题自测' },
  ],
  briefing: [
    { id: 'executive', label: '执行摘要' },
    { id: 'synthesis', label: '多源对比' },
    { id: 'research', label: '科研速览' },
  ],
  faq: [
    { id: 'common', label: '高频答疑' },
    { id: 'deep', label: '原理深究' },
  ],
  timeline: [
    { id: 'chronology', label: '时间顺序' },
    { id: 'evolution', label: '演进脉络' },
  ],
}

const PRIMARY_GENERATORS: PrimaryGenerator[] = [
  {
    kind: 'ppt',
    label: 'PPT 课件',
    ext: 'pptx',
    extLabel: '.pptx',
    hint: '分页幻灯片 + 教师讲稿备注',
    tint: 'text-orange-600 bg-orange-500/12 dark:text-orange-300',
    badge: 'bg-orange-500/10 text-orange-700 dark:text-orange-300',
  },
  {
    kind: 'word',
    label: 'Word 文档',
    ext: 'docx',
    extLabel: '.docx',
    hint: '结构化标题、表格与随堂练习',
    tint: 'text-blue-600 bg-blue-500/12 dark:text-blue-300',
    badge: 'bg-blue-500/10 text-blue-700 dark:text-blue-300',
  },
  {
    kind: 'pdf',
    label: 'PDF 讲义',
    ext: 'pdf',
    extLabel: '.pdf',
    hint: '中文排版成稿，即下即打发学生',
    tint: 'text-rose-600 bg-rose-500/12 dark:text-rose-300',
    badge: 'bg-rose-500/10 text-rose-700 dark:text-rose-300',
  },
]

const SECONDARY_TOOLS: { kind: StudioKind; label: string; tint: string }[] = [
  { kind: 'mindmap', label: '思维导图', tint: 'text-violet-600 bg-violet-500/12 dark:text-violet-300' },
  { kind: 'study-guide', label: '学习指南', tint: 'text-sky-600 bg-sky-500/12 dark:text-sky-300' },
  { kind: 'briefing', label: '简报文档', tint: 'text-emerald-600 bg-emerald-500/12 dark:text-emerald-300' },
  { kind: 'faq', label: '常见问题', tint: 'text-amber-600 bg-amber-500/12 dark:text-amber-300' },
  { kind: 'timeline', label: '时间线', tint: 'text-rose-600 bg-rose-500/12 dark:text-rose-300' },
]

const ALL_TOOLS: { kind: GenerateKind; label: string; tint: string }[] = [
  ...PRIMARY_GENERATORS.map(({ kind, label, tint }) => ({ kind, label, tint })),
  ...SECONDARY_TOOLS,
]
const toolOf = (k: GenerateKind) => ALL_TOOLS.find((t) => t.kind === k) ?? SECONDARY_TOOLS[0]

const EXPORT_CHOICES: { format: ExportFormat; label: string }[] = [
  { format: 'pptx', label: '导出 PPT (.pptx)' },
  { format: 'docx', label: '导出 Word (.docx)' },
  { format: 'pdf', label: '导出 PDF (.pdf)' },
]

function nativeExportFormat(art: StudioArtifact): ExportFormat {
  if (art.format) return art.format
  if (art.kind === 'ppt') return 'pptx'
  if (art.kind === 'pdf') return 'pdf'
  return 'docx'
}

export function StudioPanel({
  onCollapse,
  wide = false,
  onToggleWide,
}: {
  onCollapse?: () => void
  wide?: boolean
  onToggleWide?: () => void
}) {
  const { studioView, setStudioView, notes, artifacts } = useWorkspace()
  const note = studioView.type === 'note' ? notes.find((n) => n.id === studioView.id) : undefined
  const art = studioView.type === 'artifact' ? artifacts.find((a) => a.id === studioView.id) : undefined

  return (
    <div className="flex h-full flex-col">
      <PanelHeader title="工作室" icon={<Wand2 />} onCollapse={onCollapse} collapseIcon={<PanelRightClose />} />
      {note ? (
        <NoteEditor key={note.id} note={note} onBack={() => setStudioView({ type: 'list' })} wide={wide} onToggleWide={onToggleWide} />
      ) : art ? (
        <ArtifactViewer key={art.id} art={art} onBack={() => setStudioView({ type: 'list' })} wide={wide} onToggleWide={onToggleWide} />
      ) : (
        <StudioHome />
      )}
    </div>
  )
}

function StudioHome() {
  const { generating, notes, artifacts, sources, createNote, updateNote, setStudioView } = useWorkspace()
  const [configKind, setConfigKind] = useState<GenerateKind | null>(null)
  // 拖拽产物：当前悬停的落点（'section' 表示整个“产物与笔记”区域，否则为笔记 id）
  const [dropTarget, setDropTarget] = useState<string | null>(null)
  const [downloadingId, setDownloadingId] = useState<string | null>(null)

  const readySources = sources.filter((s) => s.status === 'ready')

  // 只有带文字正文的产物才能拖到笔记（思维导图是树状结构，没有文字正文；PPT 以讲稿作为正文）
  const canDragArtifact = (art: StudioArtifact) => !art.mindmap && !!art.content?.trim()

  const saveArtifactAsNote = async (art: StudioArtifact) => {
    if (!canDragArtifact(art)) {
      toast('思维导图是树状结构，暂不能保存为笔记，请使用导出', 'error')
      return
    }
    const note = await createNote({ title: art.title, content: art.content ?? '', origin: 'studio' }, false)
    if (note) toast('已保存到笔记，可继续编辑并导出', 'success')
  }

  const appendArtifactToNote = async (note: Note, art: StudioArtifact) => {
    if (!canDragArtifact(art)) {
      toast('思维导图是树状结构，暂不能追加到笔记，请使用导出', 'error')
      return
    }
    const existing = note.content.trimEnd()
    const separator = existing ? '\n\n---\n\n' : ''
    const heading = `## 来自产物：${art.title}\n\n`
    await updateNote(note.id, { content: `${existing}${separator}${heading}${art.content!.trim()}` })
    toast(`已将「${art.title}」追加到「${note.title}」`, 'success')
  }

  const handleQuickDownload = async (art: StudioArtifact) => {
    if (downloadingId) return
    setDownloadingId(art.id)
    try {
      const { blob, filename } = await studioApi.exportFile(art)
      triggerBlobDownload(blob, filename)
      toast(`正在下载 ${filename}`, 'success')
    } catch { /* reportError 已在 API 层提示 */ } finally {
      setDownloadingId(null)
    }
  }

  type Item = { kind: 'note'; v: Note } | { kind: 'art'; v: StudioArtifact }
  const items: Item[] = [
    ...artifacts.map((v) => ({ kind: 'art' as const, v })),
    ...notes.map((v) => ({ kind: 'note' as const, v })),
  ].sort((a, b) => ('updatedAt' in b.v ? b.v.updatedAt : b.v.createdAt).localeCompare('updatedAt' in a.v ? a.v.updatedAt : a.v.createdAt))

  return (
    <div className="scroll-thin flex-1 overflow-y-auto">
      <div className="space-y-5 p-4">
        {/* 备课文件生成：保持紧凑小块并完整显示标签与扩展名，点击可自定义来源、用途、篇幅与具体要求 */}
        <section>
          <div className="mb-2 flex items-center justify-between gap-2 px-1">
            <h3 className="text-xs font-medium text-muted">备课文件生成</h3>
            <span className="rounded-full bg-surface-3/80 px-2 py-0.5 text-[10px] font-medium text-muted">
              {readySources.length} 份来源可用
            </span>
          </div>
          <div className="grid grid-cols-2 gap-2">
            {PRIMARY_GENERATORS.map((item) => {
              const busy = generating.includes(item.kind)
              const fullRow = item.kind === 'ppt'
              return (
                <button
                  key={item.kind}
                  type="button"
                  onClick={() => setConfigKind(item.kind)}
                  disabled={busy}
                  title={`自定义生成${item.label}（可选来源、篇幅与具体要求）`}
                  className={cn(
                    'focus-ring group flex items-center gap-2 rounded-xl border border-border/75 bg-surface px-2.5 py-2.5 text-left text-xs font-medium shadow-xs transition-all hover:-translate-y-px hover:border-primary/40 hover:bg-surface-2/60 hover:shadow-card active:translate-y-0 disabled:cursor-wait disabled:hover:translate-y-0',
                    fullRow && 'col-span-2',
                  )}
                >
                  <span className={cn('flex h-7 w-7 shrink-0 items-center justify-center rounded-lg transition-transform group-hover:scale-105', item.tint)}>
                    {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <StudioToolIcon kind={item.kind} className="h-4 w-4" />}
                  </span>
                  {fullRow ? (
                    <>
                      <span className="min-w-0 flex-1 whitespace-nowrap font-medium text-fg">
                        {busy ? `正在生成${item.label}…` : item.label}
                      </span>
                      <span className={cn('shrink-0 rounded-md px-1.5 py-0.5 font-mono text-[10px] font-semibold', item.badge)}>
                        {item.extLabel}
                      </span>
                    </>
                  ) : (
                    <span className="min-w-0 flex-1">
                      <span className="block whitespace-nowrap text-xs font-medium leading-tight text-fg">
                        {busy ? '生成中…' : item.label}
                      </span>
                      <span className={cn('mt-0.5 inline-block rounded px-1 py-px font-mono text-[10px] font-semibold leading-none', item.badge)}>
                        {item.extLabel}
                      </span>
                    </span>
                  )}
                  <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-lg text-subtle transition-colors group-hover:bg-primary-soft group-hover:text-primary">
                    <SlidersHorizontal className="h-3.5 w-3.5" />
                  </span>
                </button>
              )
            })}
          </div>
        </section>

        {/* 思维导图与专题工具：统一参照备课文件交互，先自选来源、风格与要求再生成 */}
        <section>
          <h3 className="mb-2 px-1 text-xs font-medium text-muted">思维导图与专题工具</h3>
          <div className="grid grid-cols-2 gap-2">
            {SECONDARY_TOOLS.map(({ kind, label, tint }) => {
              const busy = generating.includes(kind)
              return (
                <button
                  key={kind}
                  type="button"
                  onClick={() => setConfigKind(kind)}
                  disabled={busy}
                  title={`自定义生成${label}（可选来源、侧重点与具体要求）`}
                  className={cn(
                    'focus-ring group flex items-center gap-2 rounded-xl border border-border/75 bg-surface px-2.5 py-2.5 text-left text-xs font-medium shadow-xs transition-all hover:-translate-y-px hover:border-primary/40 hover:bg-surface-2/60 hover:shadow-card active:translate-y-0 disabled:cursor-wait disabled:hover:translate-y-0',
                    kind === 'mindmap' && 'col-span-2',
                  )}
                >
                  <span className={cn('flex h-7 w-7 shrink-0 items-center justify-center rounded-lg transition-transform group-hover:scale-105', tint)}>
                    {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <StudioToolIcon kind={kind} className="h-4 w-4" />}
                  </span>
                  <span className="min-w-0 flex-1 whitespace-nowrap text-xs font-medium text-fg">
                    {busy ? (kind === 'mindmap' ? `正在生成${label}…` : '生成中…') : label}
                  </span>
                  <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-lg text-subtle transition-colors group-hover:bg-primary-soft group-hover:text-primary">
                    <SlidersHorizontal className="h-3.5 w-3.5" />
                  </span>
                </button>
              )
            })}
          </div>
        </section>

        {/* 生成历史与学习笔记：可把文字类产物拖到笔记上追加，或拖到此区域新建笔记 */}
        <section
          onDragOver={(e) => {
            if (!e.dataTransfer.types.includes(ARTIFACT_DRAG_TYPE)) return
            e.preventDefault()
            e.dataTransfer.dropEffect = 'copy'
            if (dropTarget !== 'section') setDropTarget('section')
          }}
          onDragLeave={(e) => {
            if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDropTarget(null)
          }}
          onDrop={(e) => {
            if (!e.dataTransfer.types.includes(ARTIFACT_DRAG_TYPE)) return
            e.preventDefault()
            setDropTarget(null)
            const art = artifacts.find((a) => a.id === e.dataTransfer.getData(ARTIFACT_DRAG_TYPE))
            if (art) void saveArtifactAsNote(art)
          }}
          className={cn('rounded-2xl transition-colors', dropTarget === 'section' && 'bg-primary-soft/40 ring-2 ring-primary/40')}
        >
          <div className="mb-2 flex items-center px-1">
            <h3 className="flex-1 text-sm font-semibold">产物与笔记</h3>
            <span className="text-xs tabular-nums text-subtle">{items.length}</span>
          </div>
          <Button variant="outline" size="sm" className="mb-2.5 w-full" icon={<Plus className="h-4 w-4" />} onClick={() => createNote()}>
            添加手写笔记
          </Button>
          {items.length === 0 ? (
            <EmptyState icon={<StickyNote />} title="还没有产出或笔记" description="在上方选择任意文件或专题工具自定义来源与要求生成，或把对话回答保存到笔记。" className="py-6" />
          ) : (
            <ul className="space-y-1.5">
              {items.map((it) =>
                it.kind === 'art' ? (
                  <li
                    key={it.v.id}
                    draggable={canDragArtifact(it.v)}
                    onDragStart={(e) => {
                      if (!canDragArtifact(it.v)) {
                        e.preventDefault()
                        toast('思维导图是树状结构，暂不能拖到笔记，请使用导出', 'error')
                        return
                      }
                      e.dataTransfer.setData(ARTIFACT_DRAG_TYPE, it.v.id)
                      e.dataTransfer.setData('text/plain', it.v.title)
                      e.dataTransfer.effectAllowed = 'copy'
                    }}
                    onDragEnd={() => setDropTarget(null)}
                    title={canDragArtifact(it.v) ? '拖到某条笔记上可追加内容，拖到此区域可新建笔记' : undefined}
                    className={cn(
                      'group flex items-center gap-1 rounded-2xl border border-transparent pr-1.5 transition-all hover:border-border/75 hover:bg-surface-2/80',
                      canDragArtifact(it.v) && 'cursor-grab active:cursor-grabbing',
                    )}
                  >
                    <button
                      onClick={() => setStudioView({ type: 'artifact', id: it.v.id })}
                      className="focus-ring flex min-w-0 flex-1 items-start gap-2.5 rounded-2xl p-2.5 text-left"
                    >
                      <span className={cn('flex h-8 w-8 shrink-0 items-center justify-center rounded-xl', toolOf(it.v.kind).tint)}>
                        <StudioToolIcon kind={it.v.kind} className="h-4 w-4" />
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-medium text-fg">{it.v.title}</div>
                        <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] text-subtle">
                          <span className="rounded bg-surface-3 px-1.5 py-0.5 font-mono text-[10px] font-medium uppercase text-muted">
                            {nativeExportFormat(it.v)}
                          </span>
                          {it.v.retrievalMode === 'rag' && (
                            <span className="rounded bg-violet-500/10 px-1.5 py-0.5 text-[10px] font-medium text-violet-700 dark:text-violet-300">
                              定向检索
                            </span>
                          )}
                          <span>{formatRelative(it.v.createdAt)}</span>
                          <span className="hidden text-[10px] text-primary group-hover:inline">· 展开预览</span>
                        </div>
                      </div>
                    </button>
                    <IconButton
                      label={`下载 .${nativeExportFormat(it.v)}`}
                      size="sm"
                      onClick={() => handleQuickDownload(it.v)}
                      disabled={downloadingId === it.v.id}
                      className="shrink-0 opacity-75 group-hover:opacity-100"
                    >
                      {downloadingId === it.v.id ? <Loader2 className="animate-spin" /> : <Download />}
                    </IconButton>
                  </li>
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
                    dropActive={dropTarget === it.v.id}
                    onDragOver={(e) => {
                      if (!e.dataTransfer.types.includes(ARTIFACT_DRAG_TYPE)) return
                      e.preventDefault()
                      e.stopPropagation()
                      e.dataTransfer.dropEffect = 'copy'
                      if (dropTarget !== it.v.id) setDropTarget(it.v.id)
                    }}
                    onDragLeave={(e) => {
                      if (!e.currentTarget.contains(e.relatedTarget as Node | null) && dropTarget === it.v.id) setDropTarget(null)
                    }}
                    onDrop={(e) => {
                      if (!e.dataTransfer.types.includes(ARTIFACT_DRAG_TYPE)) return
                      e.preventDefault()
                      e.stopPropagation()
                      setDropTarget(null)
                      const art = artifacts.find((a) => a.id === e.dataTransfer.getData(ARTIFACT_DRAG_TYPE))
                      if (art) void appendArtifactToNote(it.v, art)
                    }}
                  />
                ),
              )}
            </ul>
          )}
        </section>
      </div>
      {configKind && (
        <StudioGenerateModal
          kind={configKind}
          onClose={() => setConfigKind(null)}
        />
      )}
    </div>
  )
}

function ListRow({
  icon, title, meta, preview, onClick, dropActive = false, onDragOver, onDragLeave, onDrop,
}: {
  icon: React.ReactNode; title: string; meta: string; preview?: string; onClick: () => void
  dropActive?: boolean
  onDragOver?: React.DragEventHandler<HTMLLIElement>
  onDragLeave?: React.DragEventHandler<HTMLLIElement>
  onDrop?: React.DragEventHandler<HTMLLIElement>
}) {
  return (
    <li onDragOver={onDragOver} onDragLeave={onDragLeave} onDrop={onDrop}>
      <button
        onClick={onClick}
        className={cn(
          'focus-ring flex w-full items-start gap-2.5 rounded-2xl border border-transparent p-2.5 text-left transition-all hover:border-border/75 hover:bg-surface-2/80',
          dropActive && 'border-primary/60 bg-primary-soft/50 ring-2 ring-primary/30',
        )}
      >
        {icon}
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium text-fg">{title}</div>
          {preview && <div className="mt-0.5 line-clamp-2 text-xs leading-5 text-muted">{preview}</div>}
          <div className="mt-0.5 text-[11px] text-subtle">{meta}</div>
        </div>
      </button>
    </li>
  )
}

function SubHeader({ onBack, title, actions }: { onBack: () => void; title: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <div className="flex items-center gap-1 border-b border-border/60 bg-surface/60 px-2.5 py-2">
      <IconButton label="返回" size="sm" onClick={onBack}><ArrowLeft /></IconButton>
      <div className="min-w-0 flex-1 truncate px-1 text-xs font-medium text-muted">{title}</div>
      {actions}
    </div>
  )
}

function NoteEditor({ note, onBack, wide = false, onToggleWide }: { note: Note; onBack: () => void; wide?: boolean; onToggleWide?: () => void }) {
  const { updateNote, removeNote, addSource } = useWorkspace()
  const [title, setTitle] = useState(note.title)
  const [content, setContent] = useState(note.content)
  const [mode, setMode] = useState<'edit' | 'preview'>(note.content ? 'preview' : 'edit')
  const [saved, setSaved] = useState(true)
  const [exporting, setExporting] = useState(false)

  // 自动保存（防抖 600ms）
  useEffect(() => {
    if (title === note.title && content === note.content) return
    setSaved(false)
    const t = setTimeout(() => updateNote(note.id, { title: title.trim() || '未命名笔记', content }).then(() => setSaved(true)).catch(() => setSaved(false)), 600)
    return () => clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [title, content])

  const handleExport = async (format: ExportFormat) => {
    if (exporting) return
    setExporting(true)
    try {
      if (!saved) {
        await updateNote(note.id, { title: title.trim() || '未命名笔记', content })
        setSaved(true)
      }
      const { blob, filename } = await notesApi.exportFile({ ...note, title: title.trim() || note.title, content }, format)
      triggerBlobDownload(blob, filename)
      toast(`正在下载 ${filename}`, 'success')
    } catch { /* reportError 已提示 */ } finally {
      setExporting(false)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col animate-fade-in">
      <SubHeader
        onBack={onBack}
        title={<>工作室 › 笔记 <span className="ml-2 text-subtle">{saved ? '已保存' : '尚未保存'}</span></>}
        actions={
          <>
            {onToggleWide && (
              <IconButton
                label={wide ? '收窄工作室面板' : '宽屏预览'}
                size="sm"
                onClick={onToggleWide}
                className={cn('hidden lg:inline-flex', wide && 'bg-primary-soft text-primary hover:bg-primary-soft')}
              >
                <Columns2 />
              </IconButton>
            )}
            <Dropdown
              trigger={({ toggle }) => (
                <IconButton label="导出为 Word / PDF / PPT" size="sm" onClick={toggle} disabled={exporting}>
                  {exporting ? <Loader2 className="animate-spin" /> : <FileDown />}
                </IconButton>
              )}
              items={EXPORT_CHOICES.map((c) => ({
                label: c.label,
                icon: <Download />,
                onClick: () => handleExport(c.format),
              }))}
            />
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

function ArtifactViewer({
  art,
  onBack,
  wide = false,
  onToggleWide,
}: {
  art: StudioArtifact
  onBack: () => void
  wide?: boolean
  onToggleWide?: () => void
}) {
  const { removeArtifact, createNote } = useWorkspace()
  const [full, setFull] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [viewMode, setViewMode] = useState<'read' | 'pdf'>('read')
  const [pdfUrl, setPdfUrl] = useState<string | null>(null)
  const [pdfLoading, setPdfLoading] = useState(false)
  const T = toolOf(art.kind)
  const primaryExt = nativeExportFormat(art)

  useEffect(() => {
    return () => {
      if (pdfUrl) URL.revokeObjectURL(pdfUrl)
    }
  }, [pdfUrl])

  const loadPdfPreview = async () => {
    setViewMode('pdf')
    if (pdfUrl || pdfLoading || USE_MOCK) return
    setPdfLoading(true)
    try {
      const { blob } = await studioApi.exportFile(art, 'pdf', true)
      const url = URL.createObjectURL(blob)
      setPdfUrl(url)
    } catch {
      setViewMode('read')
    } finally {
      setPdfLoading(false)
    }
  }

  useEffect(() => {
    if (art.kind === 'pdf' && !USE_MOCK) {
      void loadPdfPreview()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [art.id, art.kind])

  const handleExport = async (format: ExportFormat) => {
    if (exporting) return
    setExporting(true)
    try {
      const { blob, filename } = await studioApi.exportFile(art, format)
      triggerBlobDownload(blob, filename)
      toast(`正在下载 ${filename}`, 'success')
    } catch { /* reportError 已在 API 层提示 */ } finally {
      setExporting(false)
    }
  }

  const renderContent = (expanded = false) => {
    const isExpandedView = expanded || wide
    if (viewMode === 'pdf' && !USE_MOCK && art.kind !== 'mindmap') {
      if (pdfLoading) {
        return (
          <div className="flex h-80 flex-col items-center justify-center gap-2 rounded-2xl border border-border/70 bg-surface-2 text-xs text-muted">
            <Loader2 className="h-5 w-5 animate-spin text-primary" />
            <span>正在生成中文排版 PDF 预览…</span>
          </div>
        )
      }
      if (pdfUrl) {
        return (
          <div className="overflow-hidden rounded-2xl border border-border/75 bg-surface-2 shadow-card">
            <iframe
              title={art.title}
              src={pdfUrl}
              className={cn('w-full bg-white', expanded ? 'h-[74vh]' : wide ? 'h-[68vh] min-h-[520px]' : 'h-[480px]')}
            />
          </div>
        )
      }
    }
    if (art.mindmap) {
      return <MindMap data={art.mindmap} className={expanded ? 'h-full min-h-[62vh]' : wide ? 'h-full min-h-[480px]' : 'h-full min-h-[360px]'} />
    }
    if (art.kind === 'ppt' && art.slides) {
      return <SlideDeckViewer deck={art.slides} expanded={isExpandedView} />
    }
    return (
      <div className={cn('prose-ai text-sm', !expanded && 'rounded-2xl border border-border/65 bg-surface-2/45 p-5 sm:p-6')}>
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{art.content ?? ''}</ReactMarkdown>
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col animate-fade-in">
      <SubHeader
        onBack={onBack}
        title={`工作室 › ${art.title}`}
        actions={
          <>
            {onToggleWide && (
              <IconButton
                label={wide ? '收窄工作室面板' : '宽屏预览'}
                size="sm"
                onClick={onToggleWide}
                className={cn('hidden lg:inline-flex', wide && 'bg-primary-soft text-primary hover:bg-primary-soft')}
              >
                <Columns2 />
              </IconButton>
            )}
            {!art.mindmap && (
              <IconButton
                label="保存到笔记"
                size="sm"
                onClick={async () => {
                  const note = await createNote({ title: art.title, content: art.content ?? '', origin: 'studio' }, false)
                  if (note) toast('已保存到笔记，可继续编辑并导出', 'success')
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
      <div className="scroll-thin flex min-h-0 flex-1 flex-col overflow-y-auto p-4 sm:p-5">
        <div className="mb-4 rounded-2xl border border-border/65 bg-surface-2/50 p-3.5">
          <div className="flex items-start gap-2.5">
            <span className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-xl', T.tint)}>
              <StudioToolIcon kind={art.kind} className="h-[18px] w-[18px]" />
            </span>
            <div className="min-w-0 flex-1">
              <div className="font-semibold leading-snug text-fg">{art.title}</div>
              <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-muted">
                <span>{art.retrievalMode === 'rag' ? '定向检索生成' : '基于选定来源生成'} · {formatRelative(art.createdAt)}</span>
              </div>
            </div>
          </div>

          {/* 快捷下载与跨格式导出工具栏 */}
          <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-border/50 pt-3">
            <Button
              size="sm"
              variant="primary"
              loading={exporting}
              icon={<Download className="h-3.5 w-3.5" />}
              onClick={() => handleExport(primaryExt)}
            >
              下载 .{primaryExt}
            </Button>

            <Dropdown
              align="left"
              trigger={({ toggle }) => (
                <Button size="sm" variant="outline" disabled={exporting} onClick={toggle}>
                  更多格式 <ChevronDown className="ml-1 h-3.5 w-3.5" />
                </Button>
              )}
              items={EXPORT_CHOICES.map((c) => ({
                label: c.label,
                icon: <FileDown />,
                onClick: () => handleExport(c.format),
              }))}
            />

            {!USE_MOCK && art.kind !== 'mindmap' && (
              <div className="ml-auto">
                <Segmented<'read' | 'pdf'>
                  value={viewMode}
                  onChange={(v) => (v === 'pdf' ? void loadPdfPreview() : setViewMode('read'))}
                  options={[
                    { value: 'read', label: '内容' },
                    { value: 'pdf', label: 'PDF 预览' },
                  ]}
                />
              </div>
            )}
          </div>
        </div>

        <div className={art.mindmap ? 'min-h-0 flex-1' : ''}>{renderContent(false)}</div>
      </div>

      <Modal open={full} onClose={() => setFull(false)} title={art.title} size="xl" className="h-[88vh]">
        <div className={cn('pb-4', (art.mindmap || (art.kind === 'ppt' && viewMode === 'read')) && 'flex h-full flex-col')}>
          {renderContent(true)}
        </div>
      </Modal>
    </div>
  )
}

const LENGTH_OPTIONS: Record<'ppt' | 'mindmap' | 'doc', { id: string; label: string }[]> = {
  ppt: [
    { id: '简明精炼（约 6-8 页）', label: '简明 (6-8页)' },
    { id: '', label: '标准 (10-12页)' },
    { id: '详尽深入（15 页以上）', label: '详尽 (15+页)' },
  ],
  mindmap: [
    { id: '精简核心脉络（2-3 层）', label: '精简脉络' },
    { id: '', label: '标准层级' },
    { id: '详尽细致分支（4 层展开）', label: '详尽分支' },
  ],
  doc: [
    { id: '精简提纲版', label: '精简提纲' },
    { id: '', label: '标准详实' },
    { id: '深度展开版', label: '深度展开' },
  ],
}

const AUDIENCE_OPTIONS = ['通用', '零基础入门', '课堂教学', '高阶备考', '学术严谨']
const FOCUS_OPTIONS = ['概念梳理', '案例与例题', '对比表格', '随堂思考题']
const PROMPT_HINTS = ['突出重难点与考点', '条理清晰分点阐述', '附带对比归纳与总结']

function StudioGenerateModal({ kind, onClose }: { kind: GenerateKind; onClose: () => void }) {
  const { sources, generate, generating } = useWorkspace()
  const readySources = sources.filter((s) => s.status === 'ready')
  const primary = PRIMARY_GENERATORS.find((g) => g.kind === kind)
  const presets = TOOL_PRESETS[kind]
  const tool = toolOf(kind)

  // 默认选中所有已就绪来源，用户可按需减少
  const [checkedIds, setCheckedIds] = useState<string[]>(() => readySources.map((s) => s.id))
  const [preset, setPreset] = useState<string>(presets[0]?.id ?? '')
  const [lengthOpt, setLengthOpt] = useState<string>('')
  const [audience, setAudience] = useState<string>('通用')
  const [focuses, setFocuses] = useState<string[]>([])
  const [prompt, setPrompt] = useState('')

  const busy = generating.includes(kind)
  const allChecked = readySources.length > 0 && checkedIds.length === readySources.length

  const toggleSourceId = (id: string) => {
    setCheckedIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
  }

  const toggleFocus = (tag: string) => {
    setFocuses((prev) => (prev.includes(tag) ? prev.filter((x) => x !== tag) : [...prev, tag]))
  }

  const appendHint = (hint: string) => {
    setPrompt((prev) => {
      const trimmed = prev.trim()
      if (trimmed.includes(hint)) return prev
      return trimmed ? `${trimmed}；${hint}` : hint
    })
  }

  const handleSubmit = () => {
    if (!checkedIds.length || busy) return
    const parts: string[] = []
    if (lengthOpt) parts.push(`篇幅要求：${lengthOpt}`)
    if (audience && audience !== '通用') parts.push(`受众风格：${audience}`)
    if (focuses.length) parts.push(`内容侧重：${focuses.join('、')}`)
    if (prompt.trim()) parts.push(prompt.trim())
    const composedTopic = parts.join('；')
    onClose()
    void generate(kind, {
      topic: composedTopic || undefined,
      preset: preset || undefined,
      sourceIds: checkedIds,
    })
  }

  const lengthChoices = kind === 'ppt' ? LENGTH_OPTIONS.ppt : kind === 'mindmap' ? LENGTH_OPTIONS.mindmap : LENGTH_OPTIONS.doc

  return (
    <Modal
      open
      onClose={onClose}
      size="lg"
      title={
        <div className="flex items-center gap-2.5">
          <span className={cn('flex h-8 w-8 items-center justify-center rounded-xl', tool.tint)}>
            <StudioToolIcon kind={kind} className="h-4 w-4" />
          </span>
          <span>自定义生成{tool.label}</span>
          {primary && (
            <span className={cn('rounded-md px-1.5 py-0.5 font-mono text-[11px] font-semibold', primary.badge)}>
              {primary.extLabel}
            </span>
          )}
        </div>
      }
      description={`已默认纳入全部 ${readySources.length} 份已就绪来源，可按需删减来源、选择用途风格、篇幅受众或补充具体要求。`}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button
            variant="primary"
            loading={busy}
            disabled={!checkedIds.length || busy}
            onClick={handleSubmit}
          >
            开始生成{tool.label}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="space-y-3.5 rounded-2xl border border-border/65 bg-surface-2/45 p-3.5">
          {presets.length > 0 && (
            <div>
              <div className="mb-1.5 text-xs font-medium text-muted">用途风格</div>
              <div className="flex flex-wrap gap-1.5">
                {presets.map((p) => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => setPreset(p.id)}
                    className={cn(
                      'focus-ring rounded-full border px-3 py-1 text-xs transition',
                      preset === p.id
                        ? 'border-primary/45 bg-primary-soft font-medium text-primary shadow-xs'
                        : 'border-border/75 bg-surface text-muted hover:text-fg',
                    )}
                  >
                    {p.label}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="grid gap-3.5 sm:grid-cols-2">
            <div>
              <div className="mb-1.5 text-xs font-medium text-muted">
                {kind === 'ppt' ? '页数 / 篇幅' : kind === 'mindmap' ? '层级深度' : '篇幅详略'}
              </div>
              <div className="flex flex-wrap gap-1.5">
                {lengthChoices.map((opt) => (
                  <button
                    key={opt.id}
                    type="button"
                    onClick={() => setLengthOpt(opt.id)}
                    className={cn(
                      'focus-ring rounded-full border px-2.5 py-1 text-xs transition',
                      lengthOpt === opt.id
                        ? 'border-primary/45 bg-primary-soft font-medium text-primary shadow-xs'
                        : 'border-border/75 bg-surface text-muted hover:text-fg',
                    )}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <div className="mb-1.5 text-xs font-medium text-muted">目标受众 / 语气</div>
              <div className="flex flex-wrap gap-1.5">
                {AUDIENCE_OPTIONS.map((opt) => (
                  <button
                    key={opt}
                    type="button"
                    onClick={() => setAudience(opt)}
                    className={cn(
                      'focus-ring rounded-full border px-2.5 py-1 text-xs transition',
                      audience === opt
                        ? 'border-primary/45 bg-primary-soft font-medium text-primary shadow-xs'
                        : 'border-border/75 bg-surface text-muted hover:text-fg',
                    )}
                  >
                    {opt}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <div>
            <div className="mb-1.5 text-xs font-medium text-muted">内容侧重（可多选）</div>
            <div className="flex flex-wrap gap-1.5">
              {FOCUS_OPTIONS.map((tag) => {
                const active = focuses.includes(tag)
                return (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => toggleFocus(tag)}
                    className={cn(
                      'focus-ring rounded-full border px-2.5 py-1 text-xs transition',
                      active
                        ? 'border-primary/45 bg-primary-soft font-medium text-primary shadow-xs'
                        : 'border-border/75 bg-surface text-muted hover:text-fg',
                    )}
                  >
                    {tag}
                  </button>
                )
              })}
            </div>
          </div>
        </div>

        <div>
          <div className="mb-1.5 flex flex-wrap items-center justify-between gap-2">
            <span className="text-xs font-medium text-muted">具体要求 / 聚焦主题（可选）</span>
            <div className="flex flex-wrap gap-1">
              {PROMPT_HINTS.map((hint) => (
                <button
                  key={hint}
                  type="button"
                  onClick={() => appendHint(hint)}
                  className="rounded-full bg-surface-3/80 px-2 py-0.5 text-[11px] text-muted transition hover:bg-primary-soft hover:text-primary"
                >
                  + {hint}
                </button>
              ))}
            </div>
          </div>
          <textarea
            rows={3}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="例如：重点讲解核心原理与实验对比、加入 2 道随堂例题、每页控制要点数量…（留空则综合所选来源自动生成）"
            className="scroll-thin w-full resize-none rounded-2xl border border-border/80 bg-surface-2/80 p-3 text-xs leading-5 text-fg outline-none transition focus:border-primary/50 focus:bg-surface placeholder:text-subtle"
          />
        </div>

        <div>
          <div className="mb-1.5 flex items-center justify-between gap-2">
            <span className="flex items-center gap-2 text-xs font-medium text-muted">
              参考来源
              <span className="rounded-full bg-surface-3 px-2 py-0.5 text-[11px] tabular-nums text-fg">
                {checkedIds.length} / {readySources.length}
              </span>
              {readySources.length > 0 && checkedIds.length === 0 && (
                <span className="text-[11px] text-warning">请至少保留 1 份来源</span>
              )}
            </span>
            {readySources.length > 0 && (
              <button
                type="button"
                onClick={() => setCheckedIds(allChecked ? [] : readySources.map((s) => s.id))}
                className="text-xs font-medium text-primary hover:underline"
              >
                {allChecked ? '清空' : '全选'}
              </button>
            )}
          </div>
          {readySources.length === 0 ? (
            <div className="rounded-2xl border border-border/70 bg-surface-2 px-3 py-4 text-center text-xs text-muted">
              当前没有已就绪的来源，请先在左侧添加来源。
            </div>
          ) : (
            <ul className="scroll-thin max-h-44 space-y-1 overflow-y-auto rounded-2xl border border-border/75 bg-surface-2/60 p-2">
              {readySources.map((s) => {
                const checked = checkedIds.includes(s.id)
                return (
                  <li key={s.id}>
                    <button
                      type="button"
                      onClick={() => toggleSourceId(s.id)}
                      className={cn(
                        'focus-ring flex w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-left text-xs transition',
                        checked ? 'bg-surface text-fg shadow-xs' : 'text-muted hover:bg-surface/60',
                      )}
                    >
                      <span
                        className={cn(
                          'flex h-4 w-4 shrink-0 items-center justify-center rounded border transition',
                          checked ? 'border-primary bg-primary text-white' : 'border-border bg-surface',
                        )}
                      >
                        {checked && <Check className="h-3 w-3" />}
                      </span>
                      <SourceIcon type={s.type} size="sm" />
                      <span className="min-w-0 flex-1 truncate">{s.title}</span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      </div>
    </Modal>
  )
}
