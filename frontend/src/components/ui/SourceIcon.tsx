import type { Source, SourceType } from '@/types'
import { cn } from '@/lib/utils'

interface Meta {
  label: string
  bg: string
  glyph: () => JSX.Element
}

function PdfGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <path d="M6 3.5A2.5 2.5 0 0 1 8.5 1H14l5.5 5.5V20.5A2.5 2.5 0 0 1 17 23H8.5A2.5 2.5 0 0 1 6 20.5v-17Z" fill="currentColor" fillOpacity="0.16" stroke="currentColor" strokeWidth="1.6" />
      <path d="M14 1v4a1.5 1.5 0 0 0 1.5 1.5h4" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
      <rect x="3.2" y="11" width="13.6" height="6.8" rx="2" fill="currentColor" />
      <path d="M5.7 16v-3.2h1.35a1.05 1.05 0 0 1 0 2.1H5.7m3.7 1.1v-3.2h1c1.1 0 1.7.65 1.7 1.6s-.6 1.6-1.7 1.6h-1Zm4.1 0v-3.2h1.9m-1.9 1.55h1.5" stroke="#fff" strokeWidth="1.15" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function DocxGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <path d="M5.5 3.5A2.5 2.5 0 0 1 8 1h6l5.5 5.5v14A2.5 2.5 0 0 1 17 23H8a2.5 2.5 0 0 1-2.5-2.5v-17Z" fill="currentColor" fillOpacity="0.14" stroke="currentColor" strokeWidth="1.6" />
      <path d="M14 1v4a1.5 1.5 0 0 0 1.5 1.5h4" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
      <path d="M8.5 10.5h7M8.5 14h7M8.5 17.5h4.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      <circle cx="17.5" cy="17.5" r="3.5" fill="currentColor" />
      <path d="m16 16.4.8 2.3.7-1.6.7 1.6.8-2.3" stroke="#fff" strokeWidth="1.1" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function PptxGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <rect x="3" y="4" width="18" height="13.5" rx="2.5" fill="currentColor" fillOpacity="0.16" stroke="currentColor" strokeWidth="1.6" />
      <path d="M7.5 13.5v-3M11.5 13.5V8.5M15.5 13.5v-4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M12 17.5V21M8.5 21h7" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  )
}

function SheetGlyph({ isCsv }: { isCsv?: boolean }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <rect x="3.5" y="3.5" width="17" height="17" rx="3" fill="currentColor" fillOpacity="0.14" stroke="currentColor" strokeWidth="1.6" />
      <path d="M3.5 9h17" stroke="currentColor" strokeWidth="1.6" />
      <path d="M10 9v11.5M15.5 9v11.5M3.5 14.5h17" stroke="currentColor" strokeOpacity="0.65" strokeWidth="1.4" />
      {isCsv && <circle cx="18" cy="6.2" r="1.2" fill="currentColor" />}
    </svg>
  )
}

function WebGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <circle cx="12" cy="12" r="9" fill="currentColor" fillOpacity="0.14" stroke="currentColor" strokeWidth="1.6" />
      <ellipse cx="12" cy="12" rx="4.2" ry="9" stroke="currentColor" strokeWidth="1.5" />
      <path d="M3.6 9.2h16.8M3.6 14.8h16.8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="18.5" cy="5.5" r="2.6" fill="currentColor" />
    </svg>
  )
}

function MarkdownGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <rect x="2.5" y="5" width="19" height="14" rx="3" fill="currentColor" fillOpacity="0.15" stroke="currentColor" strokeWidth="1.6" />
      <path d="M6.2 15V9l2.6 3.1L11.4 9v6M15.8 9v5.5m0 0-2-2m2 2 2-2" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function TextGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <rect x="4" y="3" width="16" height="18" rx="3" fill="currentColor" fillOpacity="0.14" stroke="currentColor" strokeWidth="1.6" />
      <path d="M8 8.5h8M8 12.2h8M8 15.9h5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      <circle cx="16.5" cy="16.2" r="1.4" fill="currentColor" />
    </svg>
  )
}

function AudioGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <rect x="3" y="4" width="18" height="16" rx="4" fill="currentColor" fillOpacity="0.14" stroke="currentColor" strokeWidth="1.6" />
      <path d="M7.5 10.5v3M10.5 8v8M13.5 9.5v5M16.5 7.5v9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}

function VideoGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-full w-full" aria-hidden>
      <rect x="2.5" y="4.5" width="19" height="15" rx="4" fill="currentColor" fillOpacity="0.15" stroke="currentColor" strokeWidth="1.6" />
      <path d="M10.2 9.1a.6.6 0 0 1 .9-.52l4.8 2.9a.6.6 0 0 1 0 1.04l-4.8 2.9a.6.6 0 0 1-.9-.52V9.1Z" fill="currentColor" />
    </svg>
  )
}

const map: Record<SourceType, Meta> = {
  pdf: { label: 'PDF', bg: 'bg-rose-500/12 text-rose-600 ring-rose-500/20 dark:text-rose-400', glyph: PdfGlyph },
  docx: { label: 'Word', bg: 'bg-blue-500/12 text-blue-600 ring-blue-500/20 dark:text-blue-400', glyph: DocxGlyph },
  pptx: { label: '演示文稿', bg: 'bg-orange-500/12 text-orange-600 ring-orange-500/20 dark:text-orange-300', glyph: PptxGlyph },
  xlsx: { label: '表格', bg: 'bg-emerald-500/12 text-emerald-600 ring-emerald-500/20 dark:text-emerald-400', glyph: () => <SheetGlyph /> },
  csv: { label: '表格', bg: 'bg-teal-500/12 text-teal-600 ring-teal-500/20 dark:text-teal-300', glyph: () => <SheetGlyph isCsv /> },
  web: { label: '网页', bg: 'bg-cyan-500/12 text-cyan-600 ring-cyan-500/20 dark:text-cyan-300', glyph: WebGlyph },
  text: { label: '文本', bg: 'bg-amber-500/12 text-amber-600 ring-amber-500/20 dark:text-amber-300', glyph: TextGlyph },
  markdown: { label: 'Markdown', bg: 'bg-indigo-500/12 text-indigo-600 ring-indigo-500/20 dark:text-indigo-300', glyph: MarkdownGlyph },
  audio: { label: '音频', bg: 'bg-violet-500/12 text-violet-600 ring-violet-500/20 dark:text-violet-300', glyph: AudioGlyph },
  youtube: { label: '视频', bg: 'bg-pink-500/12 text-pink-600 ring-pink-500/20 dark:text-pink-300', glyph: VideoGlyph },
}

export const sourceTypeLabel = (t: SourceType) => map[t].label

export const sourcePageCountLabel = (s: Source) => s.pageCount
  ? `${s.pageCount} ${s.type === 'pptx' ? '张' : s.type === 'xlsx' || s.type === 'csv' ? '个工作表' : '页'}`
  : ''

export const sourceLocationLabel = (s: Source, page: number) =>
  s.type === 'pptx' ? `第 ${page} 张幻灯片` : s.type === 'xlsx' || s.type === 'csv' ? `第 ${page} 个工作表` : `第 ${page} 页`

export function SourceIcon({ type, size = 'md', className }: { type: SourceType; size?: 'sm' | 'md' | 'lg'; className?: string }) {
  const { glyph: Glyph, bg } = map[type]
  const s = {
    sm: 'h-7 w-7 rounded-lg p-1.5',
    md: 'h-9 w-9 rounded-xl p-[7px]',
    lg: 'h-12 w-12 rounded-2xl p-2.5',
  }[size]
  return (
    <span className={cn('flex shrink-0 items-center justify-center ring-1 ring-inset', bg, s, className)}>
      <Glyph />
    </span>
  )
}
