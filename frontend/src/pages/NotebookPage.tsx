import { useEffect, useState } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import { Settings, BookText, MessageSquare, Wand2, PanelLeftOpen, PanelRightOpen, Plus, ChevronLeft } from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { useUIStore, type MobileTab } from '@/store/useUIStore'
import { Logo } from '@/components/layout/Logo'
import { ThemeToggle, UserMenu } from '@/components/layout/UserMenu'
import { Button, IconButton } from '@/components/ui/Button'
import { Skeleton, EmptyState } from '@/components/ui/misc'
import { SourceIcon } from '@/components/ui/SourceIcon'
import { SourcePanel } from '@/features/sources/SourcePanel'
import { AddSourceModal } from '@/features/sources/AddSourceModal'
import { SourcePreview } from '@/features/sources/SourcePreview'
import { ChatPanel } from '@/features/chat/ChatPanel'
import { StudioPanel } from '@/features/studio/StudioPanel'
import { cn } from '@/lib/utils'

function useIsDesktop() {
  const q = '(min-width: 1024px)'
  const [v, setV] = useState(() => matchMedia(q).matches)
  useEffect(() => {
    const m = matchMedia(q)
    const fn = () => setV(m.matches)
    m.addEventListener('change', fn)
    return () => m.removeEventListener('change', fn)
  }, [])
  return v
}

function readPanelPref(key: string, fallback: boolean) {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? fallback : raw === 'true'
  } catch {
    return fallback
  }
}

export default function NotebookPage() {
  const { id = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const { load, notebook, loading, error, renameNotebook, sources, reset, moduleErrors, openSourceId, studioView } = useWorkspace()
  const { setSettingsOpen, mobileTab, setMobileTab } = useUIStore()
  const isDesktop = useIsDesktop()
  const [leftOpen, setLeftOpen] = useState(() => readPanelPref('kn-left-open', true))
  const [rightOpen, setRightOpen] = useState(() => readPanelPref('kn-right-open', window.innerWidth >= 1280))
  const [studioWide, setStudioWide] = useState(true)
  const [adding, setAdding] = useState(false)
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleVal, setTitleVal] = useState('')
  const previewActive = studioView.type !== 'list'
  const previewWide = previewActive && studioWide

  useEffect(() => {
    load(id)
    setMobileTab('chat')
    return reset
  }, [id, load, reset, setMobileTab])

  useEffect(() => { if (openSourceId) setLeftOpen(true) }, [openSourceId])
  useEffect(() => {
    if (studioView.type !== 'list') {
      setRightOpen(true)
      setStudioWide(true)
    }
  }, [studioView.type])

  useEffect(() => {
    try { localStorage.setItem('kn-left-open', String(leftOpen)) } catch { /* ignore */ }
  }, [leftOpen])

  useEffect(() => {
    try { localStorage.setItem('kn-right-open', String(rightOpen)) } catch { /* ignore */ }
  }, [rightOpen])

  // 新建笔记本后自动弹出“添加来源”
  useEffect(() => {
    if (params.get('new') && !loading) {
      setAdding(true)
      params.delete('new')
      setParams(params, { replace: true })
    }
  }, [params, loading, setParams])

  useEffect(() => {
    if (notebook) document.title = `${notebook.title} · 知页 KnowNote`
    return () => {
      document.title = '知页 KnowNote · AI 个人资料工作台'
    }
  }, [notebook])

  if (error) {
    return (
      <div className="flex h-full items-center justify-center">
        <EmptyState icon={<BookText />} title="无法打开笔记本" description={error} action={<Link to="/"><Button>返回首页</Button></Link>} />
      </div>
    )
  }

  const commitTitle = () => {
    setEditingTitle(false)
    if (titleVal.trim() && titleVal.trim() !== notebook?.title) renameNotebook(titleVal)
  }

  const panelCls = 'panel flex min-h-0 flex-col overflow-hidden'

  return (
    <div className="flex h-full flex-col bg-bg">
      {/* 顶栏 */}
      <header className="flex h-14 shrink-0 items-center gap-2 px-3 sm:h-16 sm:px-5">
        <Link to="/" className="focus-ring -ml-1 flex items-center rounded-full p-1 text-muted hover:text-fg lg:hidden" aria-label="返回首页">
          <ChevronLeft className="h-5 w-5" />
        </Link>
        <div className="hidden lg:block"><Logo compact /></div>
        <div className="min-w-0 flex-1 lg:ml-2">
          {loading || !notebook ? (
            <Skeleton className="h-6 w-56" />
          ) : editingTitle ? (
            <input
              autoFocus
              value={titleVal}
              onChange={(e) => setTitleVal(e.target.value)}
              onBlur={commitTitle}
              onKeyDown={(e) => (e.key === 'Enter' ? commitTitle() : e.key === 'Escape' && setEditingTitle(false))}
              className="w-full max-w-xl rounded-lg bg-surface px-2 py-1 text-[17px] font-medium outline-none ring-2 ring-primary/40"
            />
          ) : (
            <button
              onClick={() => (setTitleVal(notebook.title), setEditingTitle(true))}
              className="flex max-w-full items-center gap-2 truncate rounded-lg px-2 py-1 text-left text-base font-semibold tracking-tight hover:bg-surface-3/70"
              title="点击重命名"
            >
              <span className="text-lg leading-none">{notebook.emoji}</span>
              <span className="truncate">{notebook.title}</span>
            </button>
          )}
        </div>
        <IconButton label="设置" onClick={() => setSettingsOpen(true)} className="hidden sm:inline-flex"><Settings /></IconButton>
        <ThemeToggle />
        <UserMenu />
      </header>

      {!!moduleErrors.length && <div role="status" className="mx-4 mb-2 flex items-center gap-2 rounded-xl bg-warning/10 px-3 py-2 text-xs text-warning"><span className="flex-1">部分内容加载失败：{moduleErrors.join('、')}，可稍后重试。</span><button className="shrink-0 underline" onClick={() => void load(id)}>重新加载</button></div>}
      {/* 主体 */}
      {isDesktop ? (
        <main className="flex min-h-0 flex-1 gap-3 px-3 pb-3 sm:px-5 sm:pb-5">
          {leftOpen ? (
            <section className={cn(panelCls, 'w-[300px] shrink-0 xl:w-[320px]')}>
              <SourcePanel onCollapse={() => setLeftOpen(false)} />
            </section>
          ) : (
            <Rail side="left" label="展开来源" icon={<PanelLeftOpen />} onExpand={() => setLeftOpen(true)}>
              <IconButton label="添加来源" size="sm" onClick={() => setAdding(true)}><Plus /></IconButton>
              {sources.slice(0, 8).map((s) => (
                <button key={s.id} title={s.title} onClick={() => setLeftOpen(true)} className="focus-ring rounded-lg transition-transform hover:scale-105">
                  <SourceIcon type={s.type} size="sm" />
                </button>
              ))}
            </Rail>
          )}
          <div className="relative flex min-w-0 flex-1 gap-3">
            <section className={cn(panelCls, 'min-w-0 flex-1')}>
              <ChatPanel onAddSource={() => setAdding(true)} />
            </section>
            {rightOpen ? (
              <section
                className={cn(
                  panelCls,
                  'shrink-0 transition-[width] duration-300 ease-out',
                  previewWide ? 'w-[58%] min-w-[460px] max-w-[820px]' : 'w-[340px] xl:w-[360px]',
                )}
              >
                <StudioPanel
                  onCollapse={() => setRightOpen(false)}
                  wide={previewWide}
                  onToggleWide={previewActive ? () => setStudioWide((v) => !v) : undefined}
                />
              </section>
            ) : (
              <Rail side="right" label="展开工作室" icon={<PanelRightOpen />} onExpand={() => setRightOpen(true)}>
                <button onClick={() => setRightOpen(true)} title="展开工作室" className="focus-ring flex h-8 w-8 items-center justify-center rounded-xl bg-primary-soft text-primary transition-transform hover:scale-105">
                  <Wand2 className="h-4 w-4" />
                </button>
              </Rail>
            )}
            <SourcePreview />
          </div>
        </main>
      ) : (
        <>
          <main className="min-h-0 flex-1 px-2 pb-2">
            <section className={cn(panelCls, 'h-full rounded-2xl')}>
              {mobileTab === 'sources' && <SourcePanel />}
              {mobileTab === 'chat' && <ChatPanel onAddSource={() => setAdding(true)} />}
              {mobileTab === 'studio' && <StudioPanel />}
            </section>
          </main>
          <MobileTabBar value={mobileTab} onChange={setMobileTab} />
          <SourcePreview />
        </>
      )}
      <AddSourceModal open={adding} onClose={() => setAdding(false)} />
    </div>
  )
}

function Rail({ side, label, icon, onExpand, children }: { side: 'left' | 'right'; label: string; icon: React.ReactNode; onExpand: () => void; children?: React.ReactNode }) {
  return (
    <aside className={cn('panel flex w-14 shrink-0 flex-col items-center gap-2 py-3', side === 'right' && 'order-last')}>
      <IconButton label={label} size="sm" onClick={onExpand}>{icon}</IconButton>
      <div className="my-1 h-px w-6 bg-border" />
      {children}
    </aside>
  )
}

function MobileTabBar({ value, onChange }: { value: MobileTab; onChange: (t: MobileTab) => void }) {
  const tabs: { v: MobileTab; label: string; icon: typeof BookText }[] = [
    { v: 'sources', label: '来源', icon: BookText },
    { v: 'chat', label: '对话', icon: MessageSquare },
    { v: 'studio', label: '工作室', icon: Wand2 },
  ]
  return (
    <nav className="flex shrink-0 border-t border-border/60 bg-surface pb-[env(safe-area-inset-bottom)]">
      {tabs.map(({ v, label, icon: Icon }) => (
        <button key={v} onClick={() => onChange(v)} className={cn('flex flex-1 flex-col items-center gap-0.5 py-2 text-[11px] font-medium transition-colors', value === v ? 'text-primary' : 'text-muted')}>
          <span className={cn('flex h-7 w-14 items-center justify-center rounded-full transition-colors', value === v && 'bg-primary-soft')}>
            <Icon className="h-5 w-5" />
          </span>
          {label}
        </button>
      ))}
    </nav>
  )
}
