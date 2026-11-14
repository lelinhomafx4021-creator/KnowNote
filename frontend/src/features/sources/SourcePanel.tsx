import { useState } from 'react'
import { Plus, MoreVertical, Pencil, Trash2, Loader2, AlertCircle, ArrowLeft, Sparkles, PanelLeftClose, FileSearch, ExternalLink } from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { Button, IconButton } from '@/components/ui/Button'
import { Checkbox, EmptyState, Progress, Skeleton, Badge } from '@/components/ui/misc'
import { Dropdown } from '@/components/ui/Dropdown'
import { SourceIcon, sourceTypeLabel } from '@/components/ui/SourceIcon'
import { Modal } from '@/components/ui/Modal'
import { AddSourceModal } from './AddSourceModal'
import { confirmDialog } from '@/store/useUIStore'
import type { Source } from '@/types'
import { cn, formatBytes } from '@/lib/utils'

export function SourcePanel({ onCollapse }: { onCollapse?: () => void }) {
  const { sources, selectedIds, toggleSource, selectAll, openSourceId, openSource, loading } = useWorkspace()
  const [adding, setAdding] = useState(false)
  const ready = sources.filter((s) => s.status === 'ready')
  const allChecked = ready.length > 0 && ready.every((s) => selectedIds.includes(s.id))
  const someChecked = selectedIds.length > 0 && !allChecked
  const current = sources.find((s) => s.id === openSourceId)

  return (
    <div className="flex h-full flex-col">
      <PanelHeader title="来源" onCollapse={onCollapse} collapseIcon={<PanelLeftClose />} />
      {current ? (
        <SourceDetail source={current} onBack={() => openSource(null)} />
      ) : (
        <>
          <div className="px-4 pb-3">
            <Button variant="outline" className="w-full" icon={<Plus className="h-4 w-4" />} onClick={() => setAdding(true)}>
              添加来源
            </Button>
          </div>
          {loading ? (
            <div className="space-y-2 px-4">
              {[0, 1, 2].map((i) => <Skeleton key={i} className="h-12" />)}
            </div>
          ) : sources.length === 0 ? (
            <EmptyState
              className="flex-1"
              icon={<FileSearch />}
              title="还没有来源"
              description="添加 PDF、网页、文本或音视频，AI 将只基于这些资料回答问题。"
              action={<Button size="sm" onClick={() => setAdding(true)} icon={<Plus className="h-4 w-4" />}>添加第一个来源</Button>}
            />
          ) : (
            <>
              <label className="flex cursor-pointer items-center justify-between px-5 py-2 text-[13px] text-muted">
                <span>选择所有来源</span>
                <Checkbox checked={allChecked} indeterminate={someChecked} onChange={(v) => selectAll(v)} label="选择所有来源" />
              </label>
              <ul className="scroll-thin flex-1 space-y-0.5 overflow-y-auto px-2 pb-4">
                {sources.map((s) => (
                  <SourceItem key={s.id} s={s} checked={selectedIds.includes(s.id)} onCheck={() => toggleSource(s.id)} onOpen={() => openSource(s.id)} />
                ))}
              </ul>
            </>
          )}
        </>
      )}
      <AddSourceModal open={adding} onClose={() => setAdding(false)} />
    </div>
  )
}

export function PanelHeader({ title, onCollapse, collapseIcon, extra }: { title: string; onCollapse?: () => void; collapseIcon?: React.ReactNode; extra?: React.ReactNode }) {
  return (
    <div className="flex h-14 shrink-0 items-center gap-1 border-b border-border/60 px-4">
      <h2 className="flex-1 text-[15px] font-medium">{title}</h2>
      {extra}
      {onCollapse && (
        <IconButton label={`收起${title}`} size="sm" onClick={onCollapse} className="hidden lg:inline-flex">
          {collapseIcon}
        </IconButton>
      )}
    </div>
  )
}

function SourceItem({ s, checked, onCheck, onOpen }: { s: Source; checked: boolean; onCheck: () => void; onOpen: () => void }) {
  const { renameSource, removeSource } = useWorkspace()
  const [renaming, setRenaming] = useState(false)
  const [val, setVal] = useState(s.title)
  const busy = s.status === 'uploading' || s.status === 'processing'

  return (
    <li>
      <div
        onClick={() => s.status === 'ready' && onOpen()}
        className={cn(
          'group flex items-center gap-2.5 rounded-2xl px-3 py-2.5 transition-colors',
          s.status === 'ready' ? 'cursor-pointer hover:bg-surface-3/70' : 'cursor-default',
        )}
      >
        <SourceIcon type={s.type} size="sm" />
        <div className="min-w-0 flex-1">
          <div className={cn('truncate text-sm', busy && 'text-muted')} title={s.title}>{s.title}</div>
          {busy && (
            <div className="mt-1.5 flex items-center gap-2">
              <Progress value={s.status === 'processing' ? 100 : s.progress ?? 0} className={cn('h-1', s.status === 'processing' && 'animate-pulse')} />
              <span className="shrink-0 text-[11px] text-subtle">{s.status === 'uploading' ? `${s.progress ?? 0}%` : '解析中'}</span>
            </div>
          )}
        </div>
        <div className="flex items-center">
          {s.status === 'ready' && (
            <Dropdown
              className="opacity-100 transition-opacity lg:opacity-0 lg:group-hover:opacity-100"
              trigger={({ toggle }) => (
                <IconButton label="更多" size="sm" className="h-7 w-7" onClick={toggle}><MoreVertical /></IconButton>
              )}
              items={[
                { label: '重命名', icon: <Pencil />, onClick: () => (setVal(s.title), setRenaming(true)) },
                {
                  label: '删除来源', icon: <Trash2 />, danger: true,
                  onClick: async () => (await confirmDialog({ title: '删除来源？', description: `「${s.title}」将从笔记本中移除，引用它的回答将无法跳转。`, confirmText: '删除', danger: true })) && removeSource(s.id),
                },
              ]}
            />
          )}
          {busy ? <Loader2 className="mx-1.5 h-4 w-4 animate-spin text-primary" /> : s.status === 'error' ? <AlertCircle className="mx-1.5 h-4 w-4 text-danger" /> : <Checkbox checked={checked} onChange={onCheck} label={`选择 ${s.title}`} className="mx-1" />}
        </div>
      </div>
      <Modal
        open={renaming}
        onClose={() => setRenaming(false)}
        title="重命名来源"
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setRenaming(false)}>取消</Button>
            <Button disabled={!val.trim()} onClick={() => (renameSource(s.id, val.trim()), setRenaming(false))}>保存</Button>
          </>
        }
      >
        <input autoFocus value={val} onChange={(e) => setVal(e.target.value)} className="mb-2 h-11 w-full rounded-xl border border-border bg-surface-2 px-3.5 text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/20" />
      </Modal>
    </li>
  )
}

function SourceDetail({ source: s, onBack }: { source: Source; onBack: () => void }) {
  const highlight = useWorkspace((st) => st.highlight)
  const quote = highlight?.replace(/…$/, '').trim()
  const content = s.content ?? ''
  const idx = quote ? content.indexOf(quote.slice(0, 40)) : -1

  return (
    <div className="flex min-h-0 flex-1 flex-col animate-fade-in">
      <div className="flex items-center gap-2 px-3 pb-2 pt-1">
        <IconButton label="返回来源列表" size="sm" onClick={onBack}><ArrowLeft /></IconButton>
        <span className="text-xs text-muted">返回列表</span>
      </div>
      <div className="scroll-thin flex-1 overflow-y-auto px-5 pb-6">
        <div className="flex items-start gap-3">
          <SourceIcon type={s.type} size="lg" />
          <div className="min-w-0">
            <h3 className="font-semibold leading-snug">{s.title}</h3>
            <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
              <Badge>{sourceTypeLabel(s.type)}</Badge>
              {s.size && <Badge>{formatBytes(s.size)}</Badge>}
              {s.wordCount && <Badge>{s.wordCount.toLocaleString()} 字</Badge>}
              {s.url && (
                <a href={s.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-xs text-primary hover:underline">
                  打开原链接 <ExternalLink className="h-3 w-3" />
                </a>
              )}
            </div>
          </div>
        </div>

        <div className="mt-5 rounded-2xl bg-gradient-to-br from-primary-soft to-primary-soft/30 p-4">
          <div className="mb-2 flex items-center gap-1.5 text-[13px] font-medium text-primary">
            <Sparkles className="h-4 w-4" /> 来源概览
          </div>
          <p className="text-sm leading-6">{s.summary}</p>
          {!!s.topics?.length && (
            <div className="mt-3 flex flex-wrap gap-1.5">
              {s.topics.map((t) => (
                <span key={t} className="rounded-full border border-primary/20 bg-surface/70 px-2.5 py-1 text-xs text-fg">{t}</span>
              ))}
            </div>
          )}
        </div>

        {quote && (
          <div className="mt-4 rounded-2xl border border-warning/30 bg-warning/5 p-3.5 text-sm leading-6">
            <div className="mb-1 text-xs font-medium text-warning">引用片段</div>
            {quote}…
          </div>
        )}

        <h4 className="mb-2 mt-6 text-xs font-medium uppercase tracking-wider text-subtle">原文预览</h4>
        <div className="whitespace-pre-wrap text-sm leading-7 text-fg/90">
          {idx >= 0 && quote ? (
            <>
              {content.slice(0, idx)}
              <mark className="rounded bg-warning/25 px-0.5 text-fg">{content.slice(idx, idx + quote.length)}</mark>
              {content.slice(idx + quote.length)}
            </>
          ) : (
            content
          )}
        </div>
      </div>
    </div>
  )
}
