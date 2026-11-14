import { FileText, Globe, Type, FileType2, Music, Youtube, FileCode2, Presentation, Table2 } from 'lucide-react'
import type { Source, SourceType } from '@/types'
import { cn } from '@/lib/utils'

const map: Record<SourceType, { icon: typeof FileText; cls: string; label: string }> = {
  pdf: { icon: FileText, cls: 'bg-red-500/10 text-red-500', label: 'PDF' },
  docx: { icon: FileType2, cls: 'bg-blue-500/10 text-blue-500', label: 'Word' },
  pptx: { icon: Presentation, cls: 'bg-orange-500/10 text-orange-700 dark:text-orange-300', label: '演示文稿' },
  xlsx: { icon: Table2, cls: 'bg-green-500/10 text-green-700 dark:text-green-300', label: '表格' },
  csv: { icon: Table2, cls: 'bg-green-500/10 text-green-700 dark:text-green-300', label: '表格' },
  web: { icon: Globe, cls: 'bg-emerald-500/10 text-emerald-500', label: '网页' },
  text: { icon: Type, cls: 'bg-amber-500/10 text-amber-600', label: '文本' },
  markdown: { icon: FileCode2, cls: 'bg-slate-500/10 text-slate-500', label: 'Markdown' },
  audio: { icon: Music, cls: 'bg-purple-500/10 text-purple-500', label: '音频' },
  youtube: { icon: Youtube, cls: 'bg-rose-500/10 text-rose-500', label: '视频' },
}

export const sourceTypeLabel = (t: SourceType) => map[t].label

export const sourcePageCountLabel = (s: Source) => s.pageCount
  ? `${s.pageCount} ${s.type === 'pptx' ? '张' : s.type === 'xlsx' || s.type === 'csv' ? '个工作表' : '页'}`
  : ''

export const sourceLocationLabel = (s: Source, page: number) =>
  s.type === 'pptx' ? `第 ${page} 张幻灯片` : s.type === 'xlsx' || s.type === 'csv' ? `第 ${page} 个工作表` : `第 ${page} 页`

export function SourceIcon({ type, size = 'md', className }: { type: SourceType; size?: 'sm' | 'md' | 'lg'; className?: string }) {
  const { icon: Icon, cls } = map[type]
  const s = { sm: 'h-7 w-7 rounded-lg [&_svg]:h-3.5 [&_svg]:w-3.5', md: 'h-9 w-9 rounded-xl [&_svg]:h-[18px] [&_svg]:w-[18px]', lg: 'h-12 w-12 rounded-2xl [&_svg]:h-6 [&_svg]:w-6' }[size]
  return (
    <span className={cn('flex shrink-0 items-center justify-center', cls, s, className)}>
      <Icon />
    </span>
  )
}
