import { useNavigate } from 'react-router-dom'
import { MoreVertical, Pencil, Trash2 } from 'lucide-react'
import type { Notebook } from '@/types'
import { Dropdown } from '@/components/ui/Dropdown'
import { IconButton } from '@/components/ui/Button'
import { cn, formatDate, formatRelative } from '@/lib/utils'

/** Material 3 风格的柔和卡片底色（浅色 / 深色） */
export const TONES = [
  'bg-[#e9eef6] dark:bg-[#28303d]',
  'bg-[#e6f4ea] dark:bg-[#223229]',
  'bg-[#fef7e0] dark:bg-[#36321f]',
  'bg-[#fce8e6] dark:bg-[#3a2826]',
  'bg-[#f3e8fd] dark:bg-[#30283c]',
  'bg-[#e4f7fb] dark:bg-[#1f3236]',
  'bg-[#fde7f3] dark:bg-[#3a2733]',
  'bg-[#f1f3f4] dark:bg-[#2b2c2e]',
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
  const tone = TONES[nb.color % 8]
  const menu = (
    <Dropdown
      trigger={({ toggle, open: o }) => (
        <IconButton label="更多操作" size="sm" onClick={toggle} active={o} className="hover:bg-black/5 dark:hover:bg-white/10">
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
        className="focus-ring group flex cursor-pointer items-center gap-4 rounded-2xl px-3 py-2.5 transition-colors hover:bg-surface-3/70"
      >
        <div className={cn('flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-xl', tone)}>{nb.emoji}</div>
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
      className={cn(
        'focus-ring group relative flex h-[220px] cursor-pointer flex-col rounded-[20px] p-5 transition-shadow duration-200 hover:shadow-[0_1px_3px_rgba(0,0,0,.12),0_4px_8px_3px_rgba(0,0,0,.06)]',
        tone,
      )}
    >
      <div className="flex items-start justify-between">
        <span className="text-[44px] leading-none">{nb.emoji}</span>
        <div className="-mr-2 -mt-1">{menu}</div>
      </div>
      <h3 className="mt-auto line-clamp-3 text-[20px] font-normal leading-7 tracking-tight">{nb.title}</h3>
      <p className="mt-2 text-[13px] text-muted">{formatDate(nb.createdAt)} · {nb.sourceCount} 个来源</p>
    </div>
  )
}
