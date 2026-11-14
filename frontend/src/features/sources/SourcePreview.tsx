import { useEffect, useRef, useState } from 'react'
import { ChevronLeft, ChevronRight, Download, ExternalLink, X } from 'lucide-react'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { useAuthStore } from '@/store/useAuthStore'
import { USE_MOCK } from '@/api/client'
import { sourceFileUrl, sourceDownloadName } from '@/api/sources'
import { SourceIcon, sourceLocationLabel } from '@/components/ui/SourceIcon'
import { IconButton } from '@/components/ui/Button'

/** 桌面覆盖中间与右侧区域，保留左侧目录；手机全屏展示。 */
export function SourcePreview() {
  const { preview, sources, openPreview, closePreview } = useWorkspace()
  const token = useAuthStore((s) => s.token)
  const source = sources.find((s) => s.id === preview?.sourceId)
  const [pageInput, setPageInput] = useState('1')
  const closeRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  useEffect(() => { setPageInput(String(preview?.page || 1)) }, [preview?.sourceId, preview?.page])
  useEffect(() => {
    if (!preview) return
    const previous = document.activeElement
    closeRef.current?.focus()
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closePreview()
      // 手机全屏时让焦点停留在预览操作区，桌面仍可访问来源目录。
      if (event.key === 'Tab' && !matchMedia('(min-width: 1024px)').matches) {
        const controls = panelRef.current?.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input')
        if (!controls?.length) return
        const first = controls[0], last = controls[controls.length - 1]
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
      }
    }
    document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('keydown', onKey); if (previous instanceof HTMLElement && previous.isConnected) previous.focus() }
  }, [!!preview, closePreview])

  if (!preview || !source) return null
  const url = sourceFileUrl(source, 'preview', token, preview.page)
  const jump = (page: number) => openPreview(source.id, page)
  const location = sourceLocationLabel(source, preview.page)

  return (
    <div ref={panelRef} role="region" aria-label="原件预览" className="fixed inset-0 z-50 flex min-h-0 min-w-0 flex-col overflow-hidden bg-surface lg:absolute lg:z-30 lg:rounded-3xl lg:border lg:border-border lg:shadow-pop">
      <header className="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-2 border-b border-border/70 px-3 py-3 sm:px-4">
        <SourceIcon type={source.type} size="sm" />
        <div className="min-w-0 flex-1"><h2 className="truncate text-sm font-medium" title={source.title}>{source.title}</h2><p className="text-xs text-subtle">原件预览{USE_MOCK ? ' · 示例页面' : ''}</p></div>
        <a href={url} target="_blank" rel="noreferrer" aria-label="在新标签打开" title="在新标签打开" className="focus-ring flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-muted hover:bg-surface-3"><ExternalLink className="h-4 w-4" /></a>
        <a href={sourceFileUrl(source, 'file', token)} download={sourceDownloadName(source)} referrerPolicy="no-referrer" aria-label="下载原件" title="下载原件" className="focus-ring flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-muted hover:bg-surface-3"><Download className="h-4 w-4" /></a>
        <IconButton ref={closeRef} label="关闭原件预览" size="sm" onClick={closePreview}><X /></IconButton>
        <div className="flex w-full items-center justify-center gap-2 pt-1 text-xs text-muted">
          <IconButton label="上一页" size="sm" disabled={preview.page <= 1} onClick={() => jump(preview.page - 1)}><ChevronLeft /></IconButton>
          <form onSubmit={(event) => { event.preventDefault(); const page = Number(pageInput); if (Number.isFinite(page) && page >= 1) jump(page); setPageInput(String(Number.isFinite(page) && page >= 1 ? Math.min(source.pageCount || Infinity, Math.floor(page)) : preview.page)) }} className="flex items-center gap-2">
            <label htmlFor="source-preview-page">当前页</label>
            <input id="source-preview-page" aria-label="跳转页码" type="number" min={1} max={source.pageCount} value={pageInput} onChange={(event) => setPageInput(event.target.value)} className="h-8 w-14 rounded-xl border border-border bg-surface-2 px-2 text-center tabular-nums text-fg outline-none focus:border-primary" />
            {!!source.pageCount && <span>/ {source.pageCount}</span>}
            <button type="submit" className="focus-ring rounded-full px-2 py-1 text-primary hover:bg-primary-soft">跳转</button>
          </form>
          <IconButton label="下一页" size="sm" disabled={!!source.pageCount && preview.page >= source.pageCount} onClick={() => jump(preview.page + 1)}><ChevronRight /></IconButton>
        </div>
      </header>
      <iframe key={url} src={url} title={`${source.title} · ${location}`} referrerPolicy="no-referrer" className="min-h-0 w-full flex-1 border-0 bg-surface-3" />
      <p className="shrink-0 border-t border-border/70 px-4 pb-[max(0.625rem,env(safe-area-inset-bottom))] pt-2.5 text-center text-[11px] text-subtle">{USE_MOCK ? '离线示例页面，可通过目录、引用与页码切换。' : '若浏览器无法显示 PDF，请在新标签打开或下载原件。'}</p>
    </div>
  )
}
