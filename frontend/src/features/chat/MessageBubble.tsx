import { memo, useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Copy, Check, ThumbsUp, ThumbsDown, Pin, Sparkles } from 'lucide-react'
import type { Message } from '@/types'
import { useWorkspace } from '@/store/useWorkspaceStore'
import { toast } from '@/store/useUIStore'
import { IconButton } from '@/components/ui/Button'
import { CitationChip } from './CitationChip'
import { cn } from '@/lib/utils'

/** 把正文中的 [1] 转成 markdown 链接，交给自定义渲染器变成引用角标 */
const linkifyCitations = (md: string) => md.replace(/\[(\d{1,2})\](?!\()/g, '[$1](#cite-$1)')

export const MessageBubble = memo(function MessageBubble({ m }: { m: Message }) {
  const { feedback, createNote } = useWorkspace()
  const [copied, setCopied] = useState(false)

  const components = useMemo(
    () => ({
      a: ({ href, children }: { href?: string; children?: React.ReactNode }) => {
        if (href?.startsWith('#cite-')) {
          const n = Number(href.slice(6))
          return <CitationChip n={n} c={m.citations?.find((c) => c.index === n)} />
        }
        return <a href={href} target="_blank" rel="noreferrer" className="text-primary underline-offset-2 hover:underline">{children}</a>
      },
    }),
    [m.citations],
  )

  if (m.role === 'user') {
    return (
      <div className="flex justify-end animate-slide-up">
        <div className="max-w-[85%] whitespace-pre-wrap rounded-3xl rounded-br-lg bg-primary px-4 py-2.5 text-[15px] leading-7 text-primary-fg">{m.content}</div>
      </div>
    )
  }

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(m.content.replace(/\[(\d+)\]/g, ''))
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      toast('复制失败', 'error')
    }
  }

  const save = async () => {
    const first = m.content.split('\n').find((l) => l.trim() && !l.startsWith('|')) ?? 'AI 回答'
    const note = await createNote({ title: first.replace(/[#*>\-[\]\d]/g, '').trim().slice(0, 24) || 'AI 回答', content: m.content.replace(/\[(\d+)\]/g, ''), origin: 'chat' }, false)
    if (note) toast('已保存到笔记', 'success')
  }

  const thinking = m.streaming && !m.content

  return (
    <div className="group flex gap-3 animate-slide-up">
      <div className="mt-1 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-primary to-accent text-white">
        <Sparkles className="h-3.5 w-3.5" />
      </div>
      <div className="min-w-0 flex-1">
        {thinking ? (
          <div className="flex items-center gap-2 py-1.5 text-sm text-muted">
            <span className="flex gap-1">
              {[0, 1, 2].map((i) => (
                <span key={i} className="h-1.5 w-1.5 animate-bounce rounded-full bg-primary" style={{ animationDelay: `${i * 0.15}s` }} />
              ))}
            </span>
            正在检索来源并组织回答…
          </div>
        ) : (
          <div className={cn('prose-ai', m.streaming && '[&>*:last-child]:after:ml-0.5 [&>*:last-child]:after:inline-block [&>*:last-child]:after:h-4 [&>*:last-child]:after:w-[3px] [&>*:last-child]:after:translate-y-0.5 [&>*:last-child]:after:animate-blink [&>*:last-child]:after:bg-primary [&>*:last-child]:after:content-[""]')}>
            <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
              {linkifyCitations(m.content)}
            </ReactMarkdown>
          </div>
        )}
        {!m.streaming && m.content && (
          <div className="-ml-2 mt-1 flex items-center gap-0.5 text-muted">
            <button onClick={save} className="focus-ring mr-1 inline-flex h-8 items-center gap-1.5 rounded-full border border-border px-3 text-[13px] transition-colors hover:bg-surface-3 hover:text-fg">
              <Pin className="h-3.5 w-3.5" /> 保存到笔记
            </button>
            <IconButton label="复制" size="sm" onClick={copy}>{copied ? <Check className="text-success" /> : <Copy />}</IconButton>
            <IconButton label="有帮助" size="sm" active={m.feedback === 'up'} onClick={() => feedback(m.id, 'up')}><ThumbsUp /></IconButton>
            <IconButton label="没帮助" size="sm" active={m.feedback === 'down'} onClick={() => feedback(m.id, 'down')}><ThumbsDown /></IconButton>
            {!!m.citations?.length && <span className="ml-2 text-xs text-subtle">引用了 {new Set(m.citations.map((c) => c.sourceId)).size} 个来源</span>}
          </div>
        )}
      </div>
    </div>
  )
})
