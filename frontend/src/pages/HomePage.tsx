import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LayoutGrid, List, Plus, Search, X, ChevronDown, Settings } from 'lucide-react'
import type { Notebook } from '@/types'
import { reportError } from '@/api/client'
import { notebooksApi } from '@/api'
import { Logo } from '@/components/layout/Logo'
import { ThemeToggle, UserMenu } from '@/components/layout/UserMenu'
import { Button, IconButton } from '@/components/ui/Button'
import { Skeleton, Kbd, EmptyState } from '@/components/ui/misc'
import { Modal } from '@/components/ui/Modal'
import { Dropdown } from '@/components/ui/Dropdown'
import { NotebookCard } from '@/features/home/NotebookCard'
import { confirmDialog, toast, useUIStore } from '@/store/useUIStore'

type Sort = 'recent' | 'name' | 'created'
const SORT_LABEL: Record<Sort, string> = { recent: '最近更新', created: '创建时间', name: '名称' }

export default function HomePage() {
  const navigate = useNavigate()
  const [list, setList] = useState<Notebook[] | null>(null)
  const [view, setView] = useState<'grid' | 'list'>(() => {
    try {
      return (localStorage.getItem('kn-view') as 'grid' | 'list') || 'grid'
    } catch {
      return 'grid'
    }
  })
  const [sort, setSort] = useState<Sort>('recent')
  const [q, setQ] = useState('')
  const [loadError, setLoadError] = useState('')
  const [retry, setRetry] = useState(0)
  const [renamingBusy, setRenamingBusy] = useState(false)
  const [creating, setCreating] = useState(false)
  const [renaming, setRenaming] = useState<Notebook | null>(null)
  const [renameVal, setRenameVal] = useState('')
  const searchRef = useRef<HTMLInputElement>(null)
  const setSettingsOpen = useUIStore((st) => st.setSettingsOpen)

  useEffect(() => {
    let active = true
    setLoadError('')
    void notebooksApi.list().then((list) => { if (active) setList(list) }).catch((e) => { if (active) setLoadError(reportError(e).message) })
    return () => { active = false }
  }, [retry])

  useEffect(() => {
    try {
      localStorage.setItem('kn-view', view)
    } catch {
      /* ignore */
    }
  }, [view])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        searchRef.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const shown = useMemo(() => {
    if (!list) return []
    const f = list.filter((n) => n.title.toLowerCase().includes(q.trim().toLowerCase()))
    return [...f].sort((a, b) =>
      sort === 'name' ? a.title.localeCompare(b.title, 'zh-CN') : sort === 'created' ? b.createdAt.localeCompare(a.createdAt) : b.updatedAt.localeCompare(a.updatedAt),
    )
  }, [list, q, sort])

  const create = async () => {
    if (creating) return
    setCreating(true)
    try { const nb = await notebooksApi.create(); navigate(`/notebook/${nb.id}?new=1`) }
    catch (e) { reportError(e) } finally { setCreating(false) }
  }

  const remove = async (nb: Notebook) => {
    const ok = await confirmDialog({ title: `删除「${nb.title}」？`, description: '笔记本中的所有来源、对话和笔记都将被永久删除，此操作无法撤销。', confirmText: '删除', danger: true })
    if (!ok) return
    try {
      await notebooksApi.remove(nb.id)
      setList((l) => l?.filter((x) => x.id !== nb.id) ?? null)
      toast('笔记本已删除')
    } catch (e) { reportError(e) }
  }

  const submitRename = async () => {
    if (!renaming || !renameVal.trim() || renamingBusy) return
    setRenamingBusy(true)
    try {
      const nb = await notebooksApi.update(renaming.id, { title: renameVal.trim() })
      setList((l) => l?.map((x) => x.id === nb.id ? nb : x) ?? null)
      setRenaming(null)
    } catch (e) { reportError(e) } finally { setRenamingBusy(false) }
  }

  const empty = list !== null && list.length === 0

  return (
    <div className="min-h-full bg-surface dark:bg-bg">
      {/* 顶栏 */}
      <header className="sticky top-0 z-30 bg-surface/90 backdrop-blur-xl dark:bg-bg/90">
        <div className="mx-auto flex h-16 max-w-[1240px] items-center gap-2 px-4 sm:px-8">
          <Logo />
          <div className="relative ml-auto hidden w-full max-w-[360px] md:block">
            <Search className="pointer-events-none absolute left-4 top-1/2 h-[18px] w-[18px] -translate-y-1/2 text-subtle" />
            <input
              ref={searchRef}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="搜索笔记本"
              className="h-11 w-full rounded-full bg-surface-3 pl-11 pr-16 text-sm outline-none transition-colors placeholder:text-subtle focus:bg-surface focus:shadow-[0_1px_3px_rgba(0,0,0,.12),0_4px_8px_3px_rgba(0,0,0,.06)] dark:focus:bg-surface-2"
            />
            <span className="absolute right-3.5 top-1/2 -translate-y-1/2">
              {q ? (
                <IconButton label="清除" size="sm" className="h-7 w-7" onClick={() => setQ('')}><X /></IconButton>
              ) : (
                <Kbd>⌘K</Kbd>
              )}
            </span>
          </div>
          <div className="ml-auto flex items-center gap-0.5 md:ml-3">
            <IconButton label="设置" onClick={() => setSettingsOpen(true)}><Settings /></IconButton>
            <ThemeToggle />
            <UserMenu />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-[1240px] px-4 pb-20 sm:px-8">
        <h1 className="pb-8 pt-8 text-[32px] font-normal leading-tight tracking-tight sm:pb-10 sm:pt-12 sm:text-[44px]">
          欢迎使用<span className="font-medium">知页</span>
        </h1>

        {/* 工具条 */}
        <div className="mb-6 flex flex-wrap items-center gap-2.5">
          <button
            onClick={create}
            disabled={creating}
            className="focus-ring inline-flex h-11 items-center gap-2 rounded-full bg-fg pl-4 pr-5 text-sm font-medium text-bg transition-opacity hover:opacity-90 disabled:opacity-60 dark:bg-fg dark:text-bg"
          >
            <Plus className="h-5 w-5" /> 新建
          </button>
          <div className="relative w-full order-last md:hidden">
            <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-subtle" />
            <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="搜索笔记本" className="h-11 w-full rounded-full bg-surface-3 pl-11 pr-4 text-sm outline-none focus:ring-2 focus:ring-primary/30" />
          </div>
          <div className="ml-auto flex items-center gap-2">
            <div className="flex h-10 items-center rounded-full border border-border p-0.5">
              {(['grid', 'list'] as const).map((v) => (
                <button
                  key={v}
                  onClick={() => setView(v)}
                  aria-label={v === 'grid' ? '网格视图' : '列表视图'}
                  className={`focus-ring flex h-full w-11 items-center justify-center rounded-full transition-colors ${view === v ? 'bg-primary-soft text-primary dark:text-fg' : 'text-muted hover:bg-surface-3'}`}
                >
                  {v === 'grid' ? <LayoutGrid className="h-[18px] w-[18px]" /> : <List className="h-[18px] w-[18px]" />}
                </button>
              ))}
            </div>
            <Dropdown
              align="right"
              trigger={({ toggle }) => (
                <button onClick={toggle} className="focus-ring inline-flex h-10 items-center gap-1.5 rounded-full border border-border px-4 text-sm text-fg transition-colors hover:bg-surface-3">
                  {SORT_LABEL[sort]} <ChevronDown className="h-4 w-4 text-muted" />
                </button>
              )}
              items={(Object.keys(SORT_LABEL) as Sort[]).map((k) => ({ label: SORT_LABEL[k], onClick: () => setSort(k) }))}
            />
          </div>
        </div>

        <h2 className="mb-4 text-[22px] font-normal tracking-tight">我的笔记本</h2>

        {/* 列表 */}
        {loadError ? <EmptyState icon={<Search />} title="笔记本加载失败" description={loadError} action={<Button onClick={() => setRetry((n) => n + 1)}>重试</Button>} /> : !list ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-[220px] rounded-[20px]" />)}
          </div>
        ) : shown.length === 0 && q ? (
          <EmptyState icon={<Search />} title="没有找到匹配的笔记本" description={`没有标题包含「${q}」的笔记本`} />
        ) : view === 'grid' ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {!q && (
              <button
                onClick={create}
                disabled={creating}
                className="focus-ring group flex h-[220px] flex-col items-center justify-center gap-4 rounded-[20px] border border-border bg-surface transition-shadow hover:shadow-[0_1px_3px_rgba(0,0,0,.12),0_4px_8px_3px_rgba(0,0,0,.06)] dark:bg-surface"
              >
                <span className="flex h-16 w-16 items-center justify-center rounded-full bg-primary-soft text-primary transition-transform group-hover:scale-105 dark:text-fg">
                  <Plus className="h-8 w-8" />
                </span>
                <span className="text-lg">新建笔记本</span>
              </button>
            )}
            {shown.map((nb) => (
              <NotebookCard key={nb.id} nb={nb} view="grid" onRename={() => (setRenaming(nb), setRenameVal(nb.title))} onDelete={() => remove(nb)} />
            ))}
          </div>
        ) : (
          <div className="overflow-hidden rounded-[20px] border border-border">
            <div className="hidden items-center gap-4 border-b border-border bg-surface-2 px-4 py-3 text-[13px] font-medium text-muted sm:flex">
              <div className="w-10" />
              <div className="flex-1">标题</div>
              <div className="w-32">创建时间</div>
              <div className="hidden w-28 md:block">最近更新</div>
              <div className="w-8" />
            </div>
            <div className="p-1.5">
              {shown.map((nb) => (
                <NotebookCard key={nb.id} nb={nb} view="list" onRename={() => (setRenaming(nb), setRenameVal(nb.title))} onDelete={() => remove(nb)} />
              ))}
            </div>
          </div>
        )}
        {empty && !q && <p className="mt-8 text-center text-sm text-muted">还没有笔记本。点击「新建笔记本」，上传你的第一份资料吧。</p>}
      </main>

      <Modal
        open={!!renaming}
        onClose={() => setRenaming(null)}
        title="重命名笔记本"
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setRenaming(null)}>取消</Button>
            <Button onClick={submitRename} loading={renamingBusy} disabled={!renameVal.trim()}>保存</Button>
          </>
        }
      >
        <input
          autoFocus
          value={renameVal}
          onChange={(e) => setRenameVal(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && submitRename()}
          className="mb-2 h-11 w-full rounded-xl border border-border bg-surface-2 px-3.5 text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
        />
      </Modal>
    </div>
  )
}
