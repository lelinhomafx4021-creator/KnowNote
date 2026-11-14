import { useNavigate } from 'react-router-dom'
import { MoreHorizontal, Pencil, Trash2, FileText, Clock } from 'lucide-react'
import type { Notebook } from '@/types'
import { Dropdown } from '@/components/ui/Dropdown'
import { IconButton } from '@/components/ui/Button'
import { cn, formatDate, formatRelative } from '@/lib/utils'

/** 笔记本图标底色（柔和色块，深浅主题都可读） */
export const TINTS = [
  'bg-indigo-50 ring-indigo-100 dark:bg-indigo-400/10 dark:ring-indigo-400/20',
  'bg-sky-50 ring-sky-100 dark:bg-sky-400/10 dark:ring-sky-400/20',
  'bg-emerald-50 ring-emerald-100 dark:bg-emerald-400/10 dark:ring-emerald-400/20',
  'bg-amber-50 ring-amber-100 dark:bg-amber-400/10 dark:ring-amber-400/20',
  'bg-rose-50 ring-rose-100 dark:bg-rose-400/10 dark:ring-rose-400/20',
  'bg-violet-50 ring-violet-100 dark:bg-violet-400/10 dark:ring-violet-400/20',
  'bg-teal-50 ring-teal-100 dark:bg-teal-400/10 dark:ring-teal-400/20',
  'bg-slate-100 ring-slate-200 dark:bg-slate-400/10 dark:ring-slate-400/20',
]
/** 卡片顶部细色条 */
const ACCENTS = ['bg-indigo-400', 'bg-sky-400', 'bg-emerald-400', 'bg-amber-400', 'bg-rose-400', 'bg-violet-400', 'bg-teal-400', 'bg-slate-400']

interface Props {
  nb: Notebook
  view: 'grid' | 'list'
  onRename: () => void
  onDelete: () => void
}

export function NotebookCard({ nb, view, onRename, onDelete }: Props) {
  const navigate = useNavigate()
  const open = () => navigate(`/notebook/${nb.id}`)
  const tint = TINTS[nb.color % 8]
  const menu = (
    <Dropdown
      trigger={({ toggle, open: o }) => (
        <IconButton label="更多操作" size="sm" onClick={toggle} active={o}>
          <MoreHorizontal />
        </IconButton>
      )}
      items={[
        { label: '重命名', icon: <Pencil />, onClick: onRename },
        { label: '删除', icon: <Trash2 />, onClick: onDelete, danger: true },
      ]}
    />
  )

  if (view === 'list') {
    return (
      <div
        role="link"
        tabIndex={0}
        onClick={open}
        onKeyDown={(e) => e.key === 'Enter' && open()}
        className="focus-ring group flex cursor-pointer items-center gap-4 rounded-xl px-3 py-2.5 transition-colors hover:bg-surface-2"
      >
        <div className={cn('flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-lg ring-1 ring-inset', tint)}>{nb.emoji}</div>
        <div className="min-w-0 flex-1">
          <div className="truncate text-[15px] font-medium">{nb.title}</div>
          <div className="text-xs text-subtle">{nb.sourceCount} 个来源</div>
        </div>
        <div className="hidden w-32 text-sm text-muted sm:block">{formatDate(nb.createdAt)}</div>
        <div className="hidden w-28 text-sm text-muted md:block">{formatRelative(nb.updatedAt)}</div>
        {menu}
      </div>
    )
  }

  return (
    <div
      role="link"
      tabIndex={0}
      onClick={open}
      onKeyDown={(e) => e.key === 'Enter' && open()}
      className="focus-ring group relative flex h-[188px] cursor-pointer flex-col overflow-hidden rounded-2xl border border-border/80 bg-surface p-5 shadow-card transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/25 hover:shadow-lift"
    >
      <span className={cn('absolute inset-x-0 top-0 h-[3px] opacity-0 transition-opacity group-hover:opacity-100', ACCENTS[nb.color % 8])} />
      <div className="flex items-start justify-between">
        <span className={cn('flex h-11 w-11 items-center justify-center rounded-xl text-[22px] ring-1 ring-inset', tint)}>{nb.emoji}</span>
        <div className="-mr-1.5 -mt-1 opacity-100 transition-opacity md:opacity-0 md:group-hover:opacity-100 [&:has([aria-expanded=true])]:opacity-100">{menu}</div>
      </div>
      <h3 className="mt-auto line-clamp-2 text-base font-semibold leading-snug tracking-tight">{nb.title}</h3>
      <div className="mt-3 flex items-center gap-3 border-t border-border/60 pt-3 text-xs text-subtle">
        <span className="inline-flex items-center gap-1"><FileText className="h-3.5 w-3.5" />{nb.sourceCount} 个来源</span>
        <span className="inline-flex items-center gap-1"><Clock className="h-3.5 w-3.5" />{formatRelative(nb.updatedAt)}</span>
      </div>
    </div>
  )
}
