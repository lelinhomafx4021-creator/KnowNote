import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ChevronDown, ChevronUp, ArrowUpRight } from 'lucide-react'
import type { SampleNotebook } from '@/types'
import { samplesApi } from '@/api'
import { reportError } from '@/api/client'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/misc'
import { cn } from '@/lib/utils'
import { TONES } from './NotebookCard'

export function FeaturedSamples({ collapsed, onToggle }: { collapsed: boolean; onToggle: () => void }) {
  const navigate = useNavigate()
  const [samples, setSamples] = useState<SampleNotebook[] | null>(null)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [retry, setRetry] = useState(0)

  useEffect(() => {
    let active = true
    let timer: ReturnType<typeof setTimeout>
    const load = async () => {
      try {
        const data = await samplesApi.list()
        if (!active) return
        setSamples(data)
        setError('')
        if (data.some((item) => !item.ready)) timer = setTimeout(load, 2000)
      } catch (e) { if (active) setError(reportError(e).message) }
    }
    void load()
    return () => { active = false; clearTimeout(timer) }
  }, [retry])

  const open = async (sample: SampleNotebook) => {
    if (busy) return
    setBusy(sample.slug)
    try {
      const notebook = await samplesApi.use(sample.slug)
      navigate(`/notebook/${notebook.id}`)
    } catch (e) { reportError(e) } finally { setBusy('') }
  }

  return (
    <section id="featured-samples" aria-labelledby="samples-title" className="mb-10 scroll-mt-20">
      <div className="mb-4 flex items-center justify-between gap-4">
        <div>
          <h2 id="samples-title" className="text-[22px] font-medium tracking-tight">精选示例</h2>
          {!collapsed && <p className="mt-1 text-sm text-muted">精选专业笔记本，选一本即可参考示例资料、提问溯源或一键生成 PPT / Word / PDF</p>}
        </div>
        <button onClick={onToggle} aria-expanded={!collapsed} aria-controls="samples-cards" className="focus-ring inline-flex shrink-0 items-center gap-1 rounded-full px-3 py-2 text-sm text-muted transition-colors hover:bg-surface-3 hover:text-fg">
          {collapsed ? '展开' : '收起'}{collapsed ? <ChevronDown className="h-4 w-4" /> : <ChevronUp className="h-4 w-4" />}
        </button>
      </div>
      {!collapsed && (
        <div id="samples-cards">
          {error ? (
            <div className="flex flex-wrap items-center gap-3 text-sm text-muted">
              <span>示例加载失败：{error}</span>
              <Button variant="outline" size="sm" onClick={() => setRetry((v) => v + 1)}>重试</Button>
            </div>
          ) : (
            <div className="flex snap-x snap-mandatory gap-4 overflow-x-auto pb-2 lg:grid lg:grid-cols-4 lg:overflow-visible">
              {!samples
                ? Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-[264px] w-[270px] shrink-0 rounded-[20px] lg:w-auto" />)
                : samples.map((sample) => (
                  <article
                    key={sample.slug}
                    className={cn(
                      'group flex min-h-[264px] w-[270px] shrink-0 snap-start flex-col rounded-[20px] border border-black/5 p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lift lg:w-auto dark:border-white/5',
                      TONES[sample.tone % TONES.length],
                    )}
                  >
                    <span aria-hidden="true" className="flex h-14 w-14 items-center justify-center rounded-2xl bg-surface/80 text-[30px] leading-none shadow-card ring-1 ring-black/5 backdrop-blur-sm transition-transform duration-200 group-hover:scale-105 dark:bg-surface/55 dark:ring-white/10">
                      {sample.emoji}
                    </span>
                    <h3 className="mt-4 text-lg font-semibold leading-7 tracking-tight">{sample.title}</h3>
                    <p className="mt-1.5 line-clamp-3 text-sm leading-6 text-muted">{sample.description}</p>
                    <div className="mt-auto flex items-center justify-between gap-2 pt-4">
                      <span className="text-[13px] text-muted">{sample.sourceCount} 个来源</span>
                      <Button
                        variant="outline"
                        size="sm"
                        loading={busy === sample.slug}
                        disabled={!!busy || (!sample.ready && !sample.addedNotebookId)}
                        onClick={() => open(sample)}
                        aria-label={`${sample.addedNotebookId ? '打开' : '体验'}${sample.title}`}
                        className="bg-surface/90 backdrop-blur"
                      >
                        {busy === sample.slug ? '正在打开' : sample.addedNotebookId ? '打开' : sample.ready ? <>立即体验 <ArrowUpRight className="h-3.5 w-3.5" /></> : '准备中'}
                      </Button>
                    </div>
                  </article>
                ))}
            </div>
          )}
        </div>
      )}
    </section>
  )
}
