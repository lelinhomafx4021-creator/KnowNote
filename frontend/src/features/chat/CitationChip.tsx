import { useRef, useState } from 'react'
import type { Citation } from '@/types'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { useUIStore } from '@/store/useUIStore'
import { cn } from '@/lib/utils'

export function CitationChip({ c }: { c?: Citation; n: number }) {
  const [show, setShow] = useState(false)
  const [below, setBelow] = useState(false)
  const ref = useRef<HTMLSpanElement>(null)
  const sources = useWorkspace((s) => s.sources)
  const openSource = useWorkspace((s) => s.openSource)
  const setMobileTab = useUIStore((s) => s.setMobileTab)
  if (!c) return null
  const exists = sources.some((s) => s.id === c.sourceId)

  const enter = () => {
    const r = ref.current?.getBoundingClientRect()
    setBelow(!!r && r.top < 220)
    setShow(true)
  }

  return (
    <span ref={ref} className="relative inline-block align-baseline" onMouseEnter={enter} onMouseLeave={() => setShow(false)}>
      <button
        type="button"
        onClick={() => {
          if (!exists) return
          openSource(c.sourceId, c.quote)
          setMobileTab('sources')
        }}
        onFocus={enter}
        onBlur={() => setShow(false)}
        className={cn(
          'focus-ring mx-0.5 inline-flex h-[18px] min-w-[18px] -translate-y-px items-center justify-center rounded-full px-1 text-[11px] font-semibold leading-none transition-colors',
          exists ? 'bg-primary-soft text-primary hover:bg-primary hover:text-primary-fg' : 'bg-surface-3 text-subtle line-through',
        )}
        aria-label={`引用 ${c.index}：${c.sourceTitle}`}
      >
        {c.index}
      </button>
      {show && (
        <span
          role="tooltip"
          className={cn(
            'absolute left-1/2 z-30 w-72 -translate-x-1/2 animate-fade-in rounded-2xl border border-border/70 bg-surface p-3.5 text-left shadow-pop',
            below ? 'top-full mt-2' : 'bottom-full mb-2',
          )}
        >
          <span className="mb-1.5 block truncate text-xs font-medium text-primary">{c.sourceTitle}</span>
          <span className="line-clamp-5 block text-[13px] font-normal leading-6 text-fg/90">“{c.quote}”</span>
          <span className="mt-2 block text-[11px] text-subtle">{exists ? '点击在来源中查看' : '该来源已被删除'}</span>
        </span>
      )}
    </span>
  )
}
