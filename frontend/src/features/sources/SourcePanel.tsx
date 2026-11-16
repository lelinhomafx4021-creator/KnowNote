import { useState } from 'react'
import { Library, Plus, MoreVertical, Pencil, Trash2, Loader2, AlertCircle, PanelLeftClose } from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { Button, IconButton } from '@/components/ui/Button'
import { Checkbox, EmptyState, Progress, Skeleton } from '@/components/ui/misc'
import { Dropdown } from '@/components/ui/Dropdown'
import { SourceIcon, sourceTypeLabel, sourcePageCountLabel } from '@/components/ui/SourceIcon'
import { UploadIllustration } from '@/components/ui/FeatureIcons'
import { Modal } from '@/components/ui/Modal'
import { AddSourceModal } from './AddSourceModal'
import { SourceDetail } from './SourceDetail'
import { confirmDialog, toast } from '@/store/useUIStore'
import type { Source } from '@/types'
import { cn } from '@/lib/utils'
import { ARTIFACT_DRAG_TYPE, isArtifactDrag } from '@/lib/dragArtifact'

export function SourcePanel({ onCollapse }: { onCollapse?: () => void }) {
  const { sources, selectedIds, toggleSource, selectAll, openSourceId, openSource, loading, artifacts, addSource } = useWorkspace()
  const [adding, setAdding] = useState(false)
  const [dropActive, setDropActive] = useState(false)
  const ready = sources.filter((s) => s.status === 'ready')
  const allChecked = ready.length > 0 && ready.every((s) => selectedIds.includes(s.id))
  const someChecked = selectedIds.length > 0 && !allChecked
  const current = sources.find((s) => s.id === openSourceId)

  // 把工作室产物拖到来源面板上，作为文本来源加入笔记本（思维导图为树状结构，暂不支持）
  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    if (!isArtifactDrag(e)) return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'copy'
    if (!dropActive) setDropActive(true)
  }
  const handleDragLeave = (e: React.DragEvent<HTMLDivElement>) => {
    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDropActive(false)
  }
  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    if (!isArtifactDrag(e)) return
    e.preventDefault()
    setDropActive(false)
    const art = artifacts.find((a) => a.id === e.dataTransfer.getData(ARTIFACT_DRAG_TYPE))
    if (!art || art.mindmap || !art.content?.trim()) {
      toast('这个产物暂不能加入来源', 'error')
      return
    }
    void addSource({ type: 'text', title: art.title, text: art.content })
    toast(`已将「${art.title}」加入来源`, 'success')
  }

  return (
    <div
      className={cn('flex h-full flex-col transition-colors', dropActive && 'bg-primary-soft/30 ring-2 ring-inset ring-primary/40')}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      <PanelHeader title="来源" icon={<Library />} count={sources.length} onCollapse={onCollapse} collapseIcon={<PanelLeftClose />} />
      {current ? (
        <SourceDetail key={current.id} source={current} onBack={() => openSource(null)} />
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
              icon={<UploadIllustration className="h-12 w-12" />}
              title="还没有来源"
              description="添加 PDF、演示文稿、表格、网页或文本，开始基于资料的对话。"
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

export function PanelHeader({ title, icon, count, onCollapse, collapseIcon, extra }: { title: string; icon?: React.ReactNode; count?: number; onCollapse?: () => void; collapseIcon?: React.ReactNode; extra?: React.ReactNode }) {
  return (
    <div className="flex h-[52px] shrink-0 items-center gap-1 border-b border-border/60 pl-4 pr-2.5">
      <h2 className="flex flex-1 items-center gap-2 text-sm font-semibold tracking-tight">
        {icon && <span className="flex h-6 w-6 items-center justify-center rounded-md bg-surface-3 text-muted [&_svg]:h-3.5 [&_svg]:w-3.5">{icon}</span>}
        {title}
        {count !== undefined && count > 0 && <span className="rounded-full bg-surface-3 px-1.5 py-px text-[11px] font-medium tabular-nums text-muted">{count}</span>}
      </h2>
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
          {s.status === 'ready' && <p className="mt-0.5 truncate text-xs text-subtle">{[sourceTypeLabel(s.type), sourcePageCountLabel(s)].filter(Boolean).join(' · ')}</p>}
          {s.status === 'error' && <p className="mt-1 truncate text-xs text-danger" title={s.error}>{s.error || '处理失败，请删除后重试'}</p>}
          {busy && (
            <div className="mt-1.5 flex items-center gap-2">
              <Progress value={s.status === 'processing' ? 100 : s.progress ?? 0} className={cn('h-1', s.status === 'processing' && 'animate-pulse')} />
              <span className="shrink-0 text-[11px] text-subtle">{s.status === 'uploading' ? `${s.progress ?? 0}%` : '解析中'}</span>
            </div>
          )}
        </div>
        <div className="flex items-center">
          {!busy && (
            <Dropdown
              className="opacity-100 transition-opacity lg:opacity-0 lg:group-hover:opacity-100"
              trigger={({ toggle }) => (
                <IconButton label="更多" size="sm" className="h-7 w-7" onClick={toggle}><MoreVertical /></IconButton>
              )}
              items={[
                ...(s.id.startsWith('pending_') ? [] : [{ label: '重命名', icon: <Pencil />, onClick: () => (setVal(s.title), setRenaming(true)) }]),
                {
                  label: '删除来源', icon: <Trash2 />, danger: true,
                  onClick: async () => (await confirmDialog({ title: '删除来源？', description: `「${s.title}」将从笔记本中移除，引用它的回答将无法跳转。`, confirmText: '删除', danger: true })) && removeSource(s.id),
                },
              ]}
            />
          )}
          {busy ? <Loader2 className="mx-1.5 h-4 w-4 animate-spin text-primary" /> : s.status === 'error' ? <span tabIndex={0} title={s.error || '来源处理失败，请删除后重试'} aria-label={s.error || '来源处理失败'}><AlertCircle className="mx-1.5 h-4 w-4 text-danger" /></span> : <Checkbox checked={checked} onChange={onCheck} label={`选择 ${s.title}`} className="mx-1" />}
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
