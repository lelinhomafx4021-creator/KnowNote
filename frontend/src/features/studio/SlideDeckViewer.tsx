import { Presentation, StickyNote } from 'lucide-react'
import type { SlideDeck } from '@/types'
import { cn } from '@/lib/utils'

function renderInlineBold(text: string) {
  const parts = text.split(/(\*\*[^*]+\*\*)/g)
  return parts.map((part, idx) =>
    part.startsWith('**') && part.endsWith('**') ? (
      <strong key={idx} className="font-semibold text-fg">
        {part.slice(2, -2)}
      </strong>
    ) : (
      <span key={idx}>{part}</span>
    ),
  )
}

interface SlideDeckViewerProps {
  deck: SlideDeck
  expanded?: boolean
  className?: string
}

/** 直接纵向展示每一页的内容（封面 + 各页），不做翻页与多页概览。 */
export function SlideDeckViewer({ deck, expanded = false, className }: SlideDeckViewerProps) {
  const total = 1 + deck.slides.length
  const card = cn(
    'relative overflow-hidden rounded-2xl border border-border/80 bg-gradient-to-br from-surface via-surface to-surface-2/80 shadow-card',
    expanded ? 'p-7 sm:p-9' : 'p-4 sm:p-5',
  )
  const pageTag = (n: number) => (
    <span className="rounded-full bg-surface-3/80 px-2 py-0.5 text-[11px] font-medium tabular-nums text-subtle">
      {n} / {total}
    </span>
  )

  return (
    <div className={cn('flex flex-col gap-4', className)}>
      {/* 封面 */}
      <section className={card} aria-label="封面">
        <div className="relative pl-3.5">
          <span className="absolute inset-y-0 left-0 w-1.5 rounded-full bg-gradient-to-b from-fg to-primary" />
          <div className="flex items-center justify-between gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-full bg-primary-soft px-2.5 py-0.5 text-[11px] font-medium text-primary">
              <Presentation className="h-3 w-3" /> KnowNote · 教学课件
            </span>
            {pageTag(1)}
          </div>
          <h3 className={cn('mt-4 font-bold tracking-tight text-fg', expanded ? 'text-2xl leading-snug sm:text-[28px]' : 'text-lg leading-snug')}>
            {deck.title}
          </h3>
          {deck.subtitle && (
            <p className={cn('mt-2.5 text-muted', expanded ? 'text-[15px] leading-7' : 'text-xs leading-relaxed')}>
              {deck.subtitle}
            </p>
          )}
        </div>
      </section>

      {/* 各页内容 */}
      {deck.slides.map((slide, i) => (
        <section key={i} className={card} aria-label={`第 ${i + 2} 页`}>
          <div className="flex items-center justify-between gap-2">
            <div className="h-1 w-14 rounded-full bg-gradient-to-r from-primary to-primary/40" />
            {pageTag(i + 2)}
          </div>
          <h4 className={cn('mt-3 font-bold text-fg', expanded ? 'text-xl leading-snug sm:text-2xl' : 'text-base leading-snug')}>
            {slide.title}
          </h4>
          {slide.subtitle && (
            <p className={cn('mt-1 font-medium text-primary', expanded ? 'text-sm' : 'text-xs')}>{slide.subtitle}</p>
          )}
          <ul className={cn('mt-4 rounded-xl border border-border/60 bg-surface-2/85', expanded ? 'space-y-3 p-5 text-[15px]' : 'space-y-2 p-3.5 text-[13px]')}>
            {slide.bullets.map((bullet, j) => (
              <li key={j} className="flex items-start gap-2.5 leading-relaxed text-fg/90">
                <span className="mt-2.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                <span>{renderInlineBold(bullet)}</span>
              </li>
            ))}
          </ul>
          {slide.notes && (
            <div className="mt-3 flex items-start gap-1.5 text-xs leading-5 text-muted">
              <StickyNote className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-500" />
              <p>{slide.notes}</p>
            </div>
          )}
        </section>
      ))}
    </div>
  )
}
