import { useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Citation } from '@/types'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { useUIStore } from '@/store/useUIStore'
import { sourceLocationLabel } from '@/components/ui/SourceIcon'
import { cn } from '@/lib/utils'

export function CitationChip({ c }: { c?: Citation; n: number }) {
  const [position, setPosition] = useState<{ top: number; left: number; below: boolean } | null>(null)
  const ref = useRef<HTMLButtonElement>(null)
  const tooltipId = useId()
  const sources = useWorkspace((s) => s.sources)
  const openSource = useWorkspace((s) => s.openSource)
  const setMobileTab = useUIStore((s) => s.setMobileTab)
  if (!c) return null
  const source = sources.find((s) => s.id === c.sourceId)
  const location = c.locationLabel || (c.page && source ? sourceLocationLabel(source, c.page) : '')

  const enter = () => {
    const rect = ref.current?.getBoundingClientRect()
    if (!rect) return
    const width = Math.min(288, window.innerWidth - 24)
    const below = rect.top < 230
    setPosition({ left: Math.max(12, Math.min(window.innerWidth - width - 12, rect.left + rect.width / 2 - width / 2)), top: below ? rect.bottom + 8 : rect.top - 8, below })
  }

  return (
    <span className="inline-block align-baseline" onMouseEnter={enter} onMouseLeave={() => setPosition(null)}>
      <button
        ref={ref}
        type="button"
        onClick={() => {
          if (!source) return
          openSource(c.sourceId, c.quote, { page: c.page, locationLabel: c.locationLabel })
          setMobileTab('sources')
          setPosition(null)
        }}
        onFocus={enter}
        onBlur={() => setPosition(null)}
        aria-describedby={position ? tooltipId : undefined}
        aria-disabled={!source}
        className={cn('focus-ring mx-0.5 inline-flex h-[18px] min-w-[18px] -translate-y-px items-center justify-center rounded-full px-1 text-[11px] font-semibold leading-none transition-colors', source ? 'bg-primary-soft text-primary hover:bg-primary hover:text-primary-fg' : 'bg-surface-3 text-subtle line-through')}
        aria-label={`引用 ${c.index}：${c.sourceTitle}${location ? `，${location}` : ''}`}
      >{c.index}</button>
      {position && createPortal(
        <div id={tooltipId} role="tooltip" className="pointer-events-none fixed z-[60] w-72 max-w-[calc(100vw-24px)] rounded-2xl border border-border/70 bg-surface p-3.5 text-left shadow-pop" style={{ left: position.left, top: position.top, transform: position.below ? undefined : 'translateY(-100%)' }}>
          <p className="truncate text-xs font-medium text-primary">{c.sourceTitle}</p>
          {location && <p className="mt-1 text-xs font-medium text-muted">{location}</p>}
          <p className="mt-1.5 line-clamp-5 break-words text-[13px] leading-6 text-fg/90">“{c.quote}”</p>
          <p className="mt-2 text-[11px] text-subtle">{source ? '点击在来源中查看' : '该来源已被删除'}</p>
        </div>, document.body,
      )}
    </span>
  )
}
