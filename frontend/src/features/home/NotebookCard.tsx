import { useNavigate } from 'react-router-dom'
import { MoreVertical, Pencil, Trash2, FileStack } from 'lucide-react'
import type { Notebook } from '@/types'
import { Dropdown } from '@/components/ui/Dropdown'
import { IconButton } from '@/components/ui/Button'
import { cn, formatDate, formatRelative } from '@/lib/utils'

export const COVERS = [
  'from-indigo-100 to-violet-100 dark:from-indigo-500/15 dark:to-violet-500/10',
  'from-sky-100 to-cyan-100 dark:from-sky-500/15 dark:to-cyan-500/10',
  'from-emerald-100 to-teal-100 dark:from-emerald-500/15 dark:to-teal-500/10',
  'from-amber-100 to-orange-100 dark:from-amber-500/15 dark:to-orange-500/10',
  'from-rose-100 to-pink-100 dark:from-rose-500/15 dark:to-pink-500/10',
  'from-fuchsia-100 to-purple-100 dark:from-fuchsia-500/15 dark:to-purple-500/10',
  'from-lime-100 to-green-100 dark:from-lime-500/15 dark:to-green-500/10',
  'from-slate-100 to-zinc-100 dark:from-slate-500/15 dark:to-zinc-500/10',
]

interface Props {
  nb: Notebook
  view: 'grid' | 'list'
  onRename: () => void
  onDelete: () => void
}

export function NotebookCard({ nb, view, onRename, onDelete }: Props) {
  const navigate = useNavigate()
  const open = () => navigate(`/notebook/${nb.id}`)
  const menu = (
    <Dropdown
      trigger={({ toggle, open: o }) => (
        <IconButton label="更多操作" size="sm" onClick={toggle} active={o} className="bg-surface/0 hover:bg-surface/80">
          <MoreVertical />
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
        className="focus-ring group flex cursor-pointer items-center gap-4 rounded-2xl px-4 py-3 transition-colors hover:bg-surface"
      >
        <div className={cn('flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br text-xl', COVERS[nb.color % 8])}>{nb.emoji}</div>
        <div className="min-w-0 flex-1">
          <div className="truncate font-medium">{nb.title}</div>
          <div className="text-xs text-muted">{nb.sourceCount} 个来源</div>
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
      className={cn(
        'focus-ring group relative flex aspect-[5/4] cursor-pointer flex-col rounded-3xl bg-gradient-to-br p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-pop',
        COVERS[nb.color % 8],
      )}
    >
      <div className="flex items-start justify-between">
        <span className="text-4xl drop-shadow-sm">{nb.emoji}</span>
        <div className="opacity-100 transition-opacity md:opacity-0 md:group-hover:opacity-100 [&:has([aria-expanded])]:opacity-100">{menu}</div>
      </div>
      <h3 className="mt-auto line-clamp-2 text-[17px] font-semibold leading-snug">{nb.title}</h3>
      <div className="mt-2 flex items-center gap-1.5 text-xs text-muted">
        <span>{formatDate(nb.createdAt)}</span>
        <span>·</span>
        <FileStack className="h-3.5 w-3.5" />
        <span>{nb.sourceCount} 个来源</span>
      </div>
    </div>
  )
}
