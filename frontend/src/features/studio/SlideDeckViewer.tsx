import { useState, type KeyboardEvent } from 'react'
import { ChevronLeft, ChevronRight, Presentation, StickyNote } from 'lucide-react'
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

export function SlideDeckViewer({ deck, expanded = false, className }: SlideDeckViewerProps) {
  const [index, setIndex] = useState(0)
  const totalPages = 1 + deck.slides.length
  const safeIndex = Math.min(index, totalPages - 1)
  const currentSlide = safeIndex === 0 ? null : deck.slides[safeIndex - 1]

  const go = (next: number) => setIndex(Math.max(0, Math.min(totalPages - 1, next)))
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'ArrowLeft') { e.preventDefault(); go(safeIndex - 1) }
    else if (e.key === 'ArrowRight') { e.preventDefault(); go(safeIndex + 1) }
  }

  const presenterNotes = safeIndex === 0
    ? `课程封面：${deck.title}。共 ${totalPages} 页（含封面），可点击下方「下载 .pptx」在 PowerPoint 或 WPS 中直接编辑讲授。`
    : currentSlide?.notes

  return (
    <div
      tabIndex={0}
      onKeyDown={onKeyDown}
      className={cn('flex flex-col gap-3 outline-none', className)}
    >
      {/* 16:9 幻灯片画布预览 */}
      <div
        className={cn(
          'relative overflow-hidden rounded-2xl border border-border/80 bg-gradient-to-br from-surface via-surface to-surface-2/80 shadow-card',
          expanded ? 'flex min-h-[340px] flex-1 flex-col justify-between p-7 sm:p-9' : 'p-4 sm:p-5',
        )}
      >
        {safeIndex === 0 ? (
          <div className="relative flex flex-1 flex-col justify-between gap-5 pl-3.5">
            <span className="absolute inset-y-0 left-0 w-1.5 rounded-full bg-gradient-to-b from-fg to-primary" />
            <div>
              <div className="flex items-center justify-between gap-2">
                <span className="inline-flex items-center gap-1.5 rounded-full bg-primary-soft px-2.5 py-0.5 text-[11px] font-medium text-primary">
                  <Presentation className="h-3 w-3" /> KnowNote · 教学课件
                </span>
                <span className="rounded-full bg-surface-3/80 px-2 py-0.5 text-[11px] font-medium tabular-nums text-subtle">
                  封面 · {totalPages} 页
                </span>
              </div>
              <h3 className={cn('mt-4 font-bold tracking-tight text-fg', expanded ? 'text-2xl sm:text-[28px] leading-snug' : 'text-lg leading-snug')}>
                {deck.title}
              </h3>
              {deck.subtitle && (
                <p className={cn('mt-2.5 text-muted', expanded ? 'text-[15px] leading-7' : 'text-xs leading-relaxed')}>
                  {deck.subtitle}
                </p>
              )}
            </div>
            <div className="flex items-center justify-between border-t border-border/60 pt-3 text-[11px] text-subtle">
              <span>共 {totalPages} 页幻灯片</span>
              <span>支持导出可编辑 .pptx 与讲义 PDF</span>
            </div>
          </div>
        ) : currentSlide && (
          <div className="flex h-full flex-col justify-between gap-4">
            <div>
              <div className="flex items-center justify-between gap-2">
                <div className="h-1 w-14 rounded-full bg-gradient-to-r from-primary to-primary/40" />
                <span className="rounded-full bg-surface-3/80 px-2 py-0.5 text-[11px] font-medium tabular-nums text-subtle">
                  第 {safeIndex + 1} / {totalPages} 页
                </span>
              </div>
              <h4 className={cn('mt-3 font-bold text-fg', expanded ? 'text-xl sm:text-2xl leading-snug' : 'text-base leading-snug')}>
                {currentSlide.title}
              </h4>
              {currentSlide.subtitle && (
                <p className={cn('mt-1 font-medium text-primary', expanded ? 'text-sm' : 'text-xs')}>
                  {currentSlide.subtitle}
                </p>
              )}
              <ul className={cn('mt-4 rounded-xl border border-border/60 bg-surface-2/85 p-3.5', expanded ? 'space-y-3 p-5 text-[15px]' : 'space-y-2 text-[13px]')}>
                {currentSlide.bullets.map((bullet, i) => (
                  <li key={i} className="flex items-start gap-2.5 leading-relaxed text-fg/90">
                    <span className="mt-2.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                    <span>{renderInlineBold(bullet)}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className="flex items-center justify-between border-t border-border/50 pt-2.5 text-[11px] text-subtle">
              <span>知页 KnowNote · 智能备课课件</span>
              <span>使用 ← → 方向键快速翻页</span>
            </div>
          </div>
        )}
      </div>

      {/* 翻页控制器 */}
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          onClick={() => go(safeIndex - 1)}
          disabled={safeIndex === 0}
          className="focus-ring inline-flex items-center gap-1 rounded-xl border border-border/80 bg-surface px-2.5 py-1.5 text-xs font-medium text-fg transition hover:bg-surface-2 disabled:pointer-events-none disabled:opacity-40"
        >
          <ChevronLeft className="h-3.5 w-3.5" /> 上一页
        </button>

        <div className="scroll-thin flex flex-1 items-center justify-center gap-1 overflow-x-auto px-1 py-0.5">
          {Array.from({ length: totalPages }, (_, idx) => (
            <button
              key={idx}
              type="button"
              onClick={() => go(idx)}
              aria-label={idx === 0 ? '封面页' : `第 ${idx + 1} 页`}
              className={cn(
                'focus-ring h-6 min-w-6 rounded-lg px-1.5 text-[11px] font-medium tabular-nums transition',
                idx === safeIndex ? 'bg-primary text-primary-fg shadow-sm' : 'bg-surface-3/80 text-muted hover:text-fg',
              )}
            >
              {idx === 0 ? '封面' : idx + 1}
            </button>
          ))}
        </div>

        <button
          type="button"
          onClick={() => go(safeIndex + 1)}
          disabled={safeIndex === totalPages - 1}
          className="focus-ring inline-flex items-center gap-1 rounded-xl border border-border/80 bg-surface px-2.5 py-1.5 text-xs font-medium text-fg transition hover:bg-surface-2 disabled:pointer-events-none disabled:opacity-40"
        >
          下一页 <ChevronRight className="h-3.5 w-3.5" />
        </button>
      </div>

      {/* 教师讲稿备注 */}
      {presenterNotes && (
        <div className="rounded-2xl border border-border/70 bg-surface-2 p-3 text-xs leading-5 text-muted">
          <div className="mb-1 flex items-center gap-1.5 font-semibold text-fg">
            <StickyNote className="h-3.5 w-3.5 text-amber-500" /> 教师讲稿 / 演讲备注
          </div>
          <p>{presenterNotes}</p>
        </div>
      )}
    </div>
  )
}
