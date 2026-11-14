import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LayoutGrid, List, Plus, Search, ArrowDownUp, X, BookOpen, FileText, Clock } from 'lucide-react'
import type { Notebook } from '@/types'
import { reportError } from '@/api/client'
import { notebooksApi } from '@/api'
import { useAuthStore } from '@/store/useAuthStore'
import { Logo } from '@/components/layout/Logo'
import { ThemeToggle, UserMenu } from '@/components/layout/UserMenu'
import { Button, IconButton } from '@/components/ui/Button'
import { Segmented, Skeleton, Kbd, EmptyState } from '@/components/ui/misc'
import { Modal } from '@/components/ui/Modal'
import { Dropdown } from '@/components/ui/Dropdown'
import { NotebookCard } from '@/features/home/NotebookCard'
import { confirmDialog, toast } from '@/store/useUIStore'

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
  const user = useAuthStore((st) => st.user)

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

  const totalSources = list?.reduce((s, n) => s + n.sourceCount, 0) ?? 0
  const recentCount = list?.filter((n) => Date.now() - new Date(n.updatedAt).getTime() < 7 * 86400_000).length ?? 0

  return (
    <div className="min-h-full bg-bg">
      {/* 顶栏 */}
      <header className="sticky top-0 z-30 border-b border-border/50 bg-bg/80 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-7xl items-center gap-3 px-4 sm:px-6">
          <Logo />
          <div className="relative ml-auto hidden w-full max-w-sm md:block">
            <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-subtle" />
            <input
              ref={searchRef}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="搜索笔记本"
              className="h-10 w-full rounded-full border border-transparent bg-surface-3 pl-10 pr-16 text-sm outline-none transition-colors placeholder:text-subtle focus:border-primary/40 focus:bg-surface"
            />
            <span className="absolute right-3 top-1/2 -translate-y-1/2">
              {q ? (
                <IconButton label="清除" size="sm" className="h-6 w-6" onClick={() => setQ('')}><X /></IconButton>
              ) : (
                <Kbd>⌘K</Kbd>
              )}
            </span>
          </div>
          <div className="ml-auto flex items-center gap-1 md:ml-2">
            <ThemeToggle />
            <UserMenu />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 pb-16 sm:px-6">
        {/* 欢迎区 */}
        <section className="relative flex flex-col gap-6 py-8 sm:py-10 lg:flex-row lg:items-end lg:justify-between">
          <div className="pointer-events-none absolute -inset-x-6 -top-4 bottom-0 dot-grid [mask-image:radial-gradient(ellipse_at_top_left,black_10%,transparent_65%)]" />
          <div className="relative">
            <p className="text-sm font-medium text-primary">{greeting()}{user?.nickname ? `，${user.nickname}` : ''}</p>
            <h1 className="mt-2 text-[28px] font-semibold tracking-tight sm:text-[34px] sm:leading-tight">你的个人知识库</h1>
            <p className="mt-2 max-w-xl text-[15px] leading-7 text-muted">上传论文、网页与笔记，AI 会基于你的资料回答问题、整理要点——每一句回答都能追溯到原文。</p>
          </div>
          {list && (
            <div className="relative grid grid-cols-3 gap-3">
              <Stat icon={<BookOpen />} n={list.length} label="笔记本" />
              <Stat icon={<FileText />} n={totalSources} label="份资料" />
              <Stat icon={<Clock />} n={recentCount} label="本周更新" />
            </div>
          )}
        </section>

        {/* 工具条 */}
        <div className="mb-5 flex flex-wrap items-center gap-3">
          <h2 className="mr-auto flex items-baseline gap-2 text-lg font-semibold tracking-tight">我的笔记本{list && <span className="text-sm font-normal text-subtle">{shown.length}</span>}</h2>
          <div className="relative w-full md:hidden">
            <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-subtle" />
            <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="搜索笔记本" className="h-10 w-full rounded-full bg-surface-3 pl-10 pr-4 text-sm outline-none focus:bg-surface focus:ring-2 focus:ring-primary/30" />
          </div>
          <Dropdown
            align="right"
            trigger={({ toggle }) => (
              <Button variant="ghost" size="sm" icon={<ArrowDownUp className="h-4 w-4" />} onClick={toggle}>
                {SORT_LABEL[sort]}
              </Button>
            )}
            items={(Object.keys(SORT_LABEL) as Sort[]).map((k) => ({ label: SORT_LABEL[k], onClick: () => setSort(k) }))}
          />
          <Segmented
            value={view}
            onChange={setView}
            options={[
              { value: 'grid', label: <span className="sr-only">网格</span>, icon: <LayoutGrid /> },
              { value: 'list', label: <span className="sr-only">列表</span>, icon: <List /> },
            ]}
          />
          <Button icon={<Plus className="h-4 w-4" />} onClick={create} loading={creating}>
            新建笔记本
          </Button>
        </div>

        {/* 列表 */}
        {loadError ? <EmptyState icon={<Search />} title="笔记本加载失败" description={loadError} action={<Button onClick={() => setRetry((n) => n + 1)}>重试</Button>} /> : !list ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-[188px] rounded-2xl" />)}
          </div>
        ) : shown.length === 0 && q ? (
          <EmptyState icon={<Search />} title="没有找到匹配的笔记本" description={`没有标题包含「${q}」的笔记本`} />
        ) : view === 'grid' ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {!q && (
              <button
                onClick={create}
                disabled={creating}
                className="focus-ring group hidden h-[188px] flex-col items-center justify-center gap-3 rounded-2xl border border-dashed sm:flex border-border bg-surface/60 text-muted transition-colors hover:border-primary/40 hover:bg-primary-soft/40 hover:text-primary"
              >
                <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-primary text-primary-fg shadow-sm transition-transform group-hover:scale-105">
                  <Plus className="h-5 w-5" />
                </span>
                <span className="font-medium">新建笔记本</span>
              </button>
            )}
            {shown.map((nb) => (
              <NotebookCard key={nb.id} nb={nb} view="grid" onRename={() => (setRenaming(nb), setRenameVal(nb.title))} onDelete={() => remove(nb)} />
            ))}
          </div>
        ) : (
          <div className="panel p-2">
            <div className="hidden items-center gap-4 px-4 py-2 text-xs font-medium text-subtle sm:flex">
              <div className="w-11" />
              <div className="flex-1">标题</div>
              <div className="w-32">创建时间</div>
              <div className="hidden w-28 md:block">最近更新</div>
              <div className="w-8" />
            </div>
            {shown.map((nb) => (
              <NotebookCard key={nb.id} nb={nb} view="list" onRename={() => (setRenaming(nb), setRenameVal(nb.title))} onDelete={() => remove(nb)} />
            ))}
          </div>
        )}
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

function greeting() {
  const h = new Date().getHours()
  return h < 6 ? '夜深了' : h < 12 ? '早上好' : h < 14 ? '中午好' : h < 18 ? '下午好' : '晚上好'
}

function Stat({ icon, n, label }: { icon: React.ReactNode; n: number | string; label: string }) {
  return (
    <div className="min-w-[104px] rounded-xl border border-border/80 bg-surface px-4 py-3 shadow-card">
      <div className="flex items-center gap-1.5 text-xs text-subtle [&_svg]:h-3.5 [&_svg]:w-3.5">{icon}{label}</div>
      <div className="mt-1 text-xl font-semibold tabular-nums tracking-tight">{n}</div>
    </div>
  )
}
