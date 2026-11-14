import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, ArrowUpRight, BookOpen, ChevronRight, Download, ExternalLink, List, Quote, Sparkles } from 'lucide-react'
import type { Source } from '@/types'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { useAuthStore } from '@/store/useAuthStore'
import { sourceFileUrl, sourceDownloadName } from '@/api/sources'
import { Button, IconButton } from '@/components/ui/Button'
import { Badge } from '@/components/ui/misc'
import { SourceIcon, sourceTypeLabel, sourcePageCountLabel, sourceLocationLabel } from '@/components/ui/SourceIcon'
import { cn, formatBytes } from '@/lib/utils'

/** 后端引用会折叠空白，定位时兼容原文中的换行与多空格。 */
function findQuote(content: string, quote: string) {
  if (!quote) return null
  const pattern = quote.split(/\s+/).map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('\\s+')
  return new RegExp(pattern).exec(content)
}

export function SourceDetail({ source: s, onBack }: { source: Source; onBack: () => void }) {
  const { highlight, citationLocation, preview, openPreview } = useWorkspace()
  const token = useAuthStore((st) => st.token)
  const [sectionPage, setSectionPage] = useState<number | null>(null)
  const quoteRef = useRef<HTMLDivElement>(null)
  const originalRef = useRef<HTMLDivElement>(null)
  const quote = highlight?.replace(/(?:…|\.{3})$/, '').trim() || ''
  const content = s.content ?? ''
  const match = findQuote(content, quote)
  const currentPage = preview?.sourceId === s.id ? preview.page : sectionPage ?? citationLocation?.page
  const section = s.sections?.find((item) => item.page === currentPage)
  const location = citationLocation?.locationLabel || (citationLocation?.page ? sourceLocationLabel(s, citationLocation.page) : '')
  const hasFile = !!s.originalExt || s.size !== undefined

  useEffect(() => {
    setSectionPage(null)
    if (highlight) quoteRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }, [highlight, citationLocation])

  return (
    <div className="flex min-h-0 flex-1 flex-col animate-fade-in">
      <div className="flex shrink-0 items-center gap-1 px-3 py-2">
        <IconButton label="返回来源列表" size="sm" onClick={onBack}><ArrowLeft /></IconButton>
        <span className="text-xs text-muted">所有来源</span>
      </div>
      <div className="scroll-thin min-h-0 flex-1 overflow-y-auto px-4 pb-6">
        <div className="rounded-3xl border border-border/70 bg-surface-2 p-4">
          <SourceIcon type={s.type} size="lg" />
          <h3 className="mt-3 break-words text-base font-medium leading-6 [overflow-wrap:anywhere]">{s.title}</h3>
          <div className="mt-2.5 flex flex-wrap gap-1.5">
            <Badge>{sourceTypeLabel(s.type)}</Badge>
            {s.originalExt && s.originalExt !== s.type && <Badge>{s.originalExt.toUpperCase()}</Badge>}
            {s.size !== undefined && <Badge>{formatBytes(s.size)}</Badge>}
            {!!s.pageCount && <Badge>{sourcePageCountLabel(s)}</Badge>}
          </div>
          {(s.previewable || hasFile) && <div className="mt-4 flex flex-wrap gap-2">
            {s.previewable && <Button size="sm" icon={<BookOpen className="h-3.5 w-3.5" />} onClick={() => openPreview(s.id, currentPage || 1)}>查看原件</Button>}
            {hasFile && <a href={sourceFileUrl(s, 'file', token)} download={sourceDownloadName(s)} referrerPolicy="no-referrer" className="focus-ring inline-flex h-8 items-center gap-1.5 rounded-full border border-border bg-surface px-3 text-[13px] font-medium hover:bg-surface-3"><Download className="h-3.5 w-3.5" />下载</a>}
          </div>}
          {s.url && <a href={s.url} target="_blank" rel="noreferrer" className="focus-ring mt-3 inline-flex items-center gap-1.5 rounded-lg text-xs text-primary hover:underline">打开原链接 <ExternalLink className="h-3 w-3" /></a>}
        </div>

        <section className="mt-4 rounded-3xl border border-primary/10 bg-primary-soft/35 p-4">
          <h4 className="mb-2 flex items-center gap-2 text-[13px] font-medium text-primary"><Sparkles className="h-4 w-4" />来源概览</h4>
          <p className="break-words text-sm leading-6">{s.summary || '暂未生成来源概览。'}</p>
          {!!s.topics?.length && <div className="mt-3 flex flex-wrap gap-1.5">{s.topics.map((topic) => <span key={topic} className="max-w-full break-words rounded-full border border-primary/15 bg-surface/70 px-2.5 py-1 text-xs">{topic}</span>)}</div>}
        </section>

        {!!s.sections?.length && <section className="mt-5" aria-label="来源目录">
          <h4 className="mb-2.5 flex items-center gap-2 px-1 text-[13px] font-medium"><List className="h-4 w-4 text-muted" />目录<span className="ml-auto text-xs font-normal text-subtle">{s.sections.length} 项</span></h4>
          <ol className="space-y-1.5">
            {s.sections.map((item) => <li key={item.page}>
              <button
                onClick={() => { setSectionPage(item.page); if (s.previewable) openPreview(s.id, item.page); else originalRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }) }}
                aria-current={currentPage === item.page ? 'page' : undefined}
                className={cn('focus-ring flex w-full items-start gap-2 rounded-2xl border p-3 text-left transition-colors', currentPage === item.page ? 'border-primary/30 bg-primary-soft/40' : 'border-border/70 bg-surface hover:bg-surface-2')}
              >
                <span className="min-w-0 flex-1">
                  <span className="block break-words text-[13px] font-medium leading-5">{item.label}</span>
                  {item.excerpt && <span className="mt-1 line-clamp-2 block break-words text-xs leading-5 text-muted">{item.excerpt}</span>}
                </span>
                <ChevronRight className="mt-0.5 h-3.5 w-3.5 shrink-0 text-subtle" />
              </button>
            </li>)}
          </ol>
        </section>}

        {quote && <div ref={quoteRef} className="mt-5 scroll-mt-3 rounded-2xl border border-warning/30 bg-warning/10 p-3.5 text-sm leading-6">
          <div className="mb-2 flex flex-wrap items-center gap-1.5 text-xs font-medium text-warning"><Quote className="h-3.5 w-3.5" />引用片段{location && <span>· {location}</span>}</div>
          <mark className="break-words bg-transparent text-fg">{quote}</mark>
          {s.previewable && <button onClick={() => openPreview(s.id, citationLocation?.page || 1)} className="focus-ring mt-3 flex items-center gap-1 rounded-lg text-left text-xs font-medium text-primary">
            {citationLocation?.page ? `在原件中查看第 ${citationLocation.page} 页` : '在原件中查看'}<ArrowUpRight className="h-3.5 w-3.5 shrink-0" />
          </button>}
        </div>}

        <section ref={originalRef} className="mt-6 scroll-mt-3">
          <h4 className="mb-2 flex flex-wrap items-center gap-2 text-[13px] font-medium">原文预览{!!s.wordCount && <span className="text-xs font-normal text-subtle">{s.wordCount.toLocaleString()} 字</span>}</h4>
          {section && <div className="mb-3 rounded-2xl border border-primary/20 bg-primary-soft/25 p-3 text-sm leading-6"><p className="mb-1 text-xs font-medium text-primary">{section.label}</p><mark className="bg-primary-soft/70 text-fg">{section.excerpt}</mark></div>}
          <div className="whitespace-pre-wrap break-words text-sm leading-7 text-fg/90 [overflow-wrap:anywhere]">
            {match ? <>{content.slice(0, match.index)}<mark className="rounded bg-warning/25 px-0.5 text-fg">{match[0]}</mark>{content.slice(match.index + match[0].length)}</> : content || '暂无可显示的原文。'}
          </div>
        </section>
      </div>
    </div>
  )
}
