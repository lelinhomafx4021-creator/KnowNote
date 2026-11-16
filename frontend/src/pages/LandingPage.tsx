import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  ArrowRight, Sparkles, Lock, ShieldCheck,
  GraduationCap, Microscope, Briefcase, ChevronRight, Check,
} from 'lucide-react'
import { authApi } from '@/api/auth'
import { USE_MOCK } from '@/api/client'
import { useAuthStore } from '@/store/useAuthStore'
import { Logo } from '@/components/layout/Logo'
import { ThemeToggle } from '@/components/layout/UserMenu'
import { Button } from '@/components/ui/Button'
import { SourceIcon } from '@/components/ui/SourceIcon'
import { StudioToolIcon } from '@/components/ui/FeatureIcons'
import type { SourceType, StudioKind } from '@/types'
import { cn } from '@/lib/utils'

const SHOWCASE_SOURCES: { id: string; type: SourceType; title: string; meta: string; excerpt: string }[] = [
  {
    id: 'pdf',
    type: 'pdf',
    title: 'RAG 阅读与引用指南.pdf',
    meta: 'PDF · 12 页',
    excerpt: '文本按约 500 字切分并保留 80 字重叠窗口，结合 pgvector HNSW 索引实现低延迟余弦 Top-K 召回，避免语义割裂。',
  },
  {
    id: 'pptx',
    type: 'pptx',
    title: '知页系统设计与答辩.pptx',
    meta: '演示文稿 · 6 张幻灯片',
    excerpt: '系统按资料结构保存页码与幻灯片编号，点击回答中的角标可直接回到原件定位上下文。',
  },
  {
    id: 'xlsx',
    type: 'xlsx',
    title: '检索消融实验记录.xlsx',
    meta: '表格 · 2 个工作表',
    excerpt: '在 500 字块长与 Top-5 召回配置下，回答引用定位准确率达到 94.2%，兼顾上下文完整度。',
  },
  {
    id: 'web',
    type: 'web',
    title: 'PostgreSQL pgvector 工程实践',
    meta: '网页正文 · 自动提取',
    excerpt: '结构化业务数据与向量索引共存在同一 PostgreSQL 实例中，保障数据一致性与用户隔离。',
  },
]

const ALL_FORMATS: { type: SourceType; label: string }[] = [
  { type: 'pdf', label: 'PDF' },
  { type: 'docx', label: 'Word' },
  { type: 'pptx', label: 'PPT' },
  { type: 'xlsx', label: 'Excel' },
  { type: 'csv', label: 'CSV' },
  { type: 'web', label: '网页' },
  { type: 'markdown', label: 'Markdown' },
  { type: 'text', label: '文本' },
  { type: 'audio', label: '音频' },
  { type: 'youtube', label: '视频' },
]

const STUDIO_TILES: { kind: StudioKind; label: string; desc: string; tint: string }[] = [
  { kind: 'mindmap', label: '思维导图', desc: '交互式折叠与拖拽全局脉络', tint: 'bg-[#f3e8fd] text-[#6b21a8] dark:bg-[#30283c] dark:text-[#d8b4fe]' },
  { kind: 'study-guide', label: '学习指南', desc: '提炼考纲、简答题与核心术语', tint: 'bg-[#e4f7fb] text-[#0369a1] dark:bg-[#1f3236] dark:text-[#7dd3fc]' },
  { kind: 'briefing', label: '简报文档', desc: '结构化汇总多份来源结论', tint: 'bg-[#e6f4ea] text-[#146c2e] dark:bg-[#223229] dark:text-[#6dd58c]' },
  { kind: 'faq', label: '常见问题', desc: '自动归纳高频疑问与解答', tint: 'bg-[#fef7e0] text-[#b06000] dark:bg-[#36321f] dark:text-[#fcd34d]' },
  { kind: 'timeline', label: '时间线', desc: '按时间轴梳理关键事件脉络', tint: 'bg-[#fce8e6] text-[#b3261e] dark:bg-[#3a2826] dark:text-[#fca5a5]' },
]

export default function LandingPage() {
  const navigate = useNavigate()
  const { user, accept } = useAuthStore()
  const [activeCite, setActiveCite] = useState<'pdf' | 'pptx'>('pdf')
  const [demoBusy, setDemoBusy] = useState(false)

  const highlighted = SHOWCASE_SOURCES.find((s) => s.id === activeCite) ?? SHOWCASE_SOURCES[0]

  const quickDemo = async () => {
    if (demoBusy) return
    setDemoBusy(true)
    try {
      const result = await authApi.login({ email: 'demo@knownote.local', password: 'demo-preview' })
      accept(result)
      navigate('/')
    } finally {
      setDemoBusy(false)
    }
  }

  return (
    <div className="min-h-full bg-surface text-fg dark:bg-bg">
      {/* NotebookLM 风格极简顶栏 */}
      <header className="sticky top-0 z-40 bg-surface/85 backdrop-blur-xl dark:bg-bg/85">
        <div className="mx-auto flex h-16 max-w-[1200px] items-center justify-between gap-4 px-5 sm:px-8">
          <Logo to={user ? '/welcome' : '/'} />
          <nav aria-label="页面导航" className="hidden items-center gap-1 md:flex">
            {[
              ['#overview', '概览'],
              ['#how-it-works', '核心体验'],
              ['#use-cases', '适用场景'],
              ['#privacy', '数据隐私'],
            ].map(([href, label]) => (
              <a
                key={href}
                href={href}
                className="focus-ring rounded-full px-4 py-2 text-sm text-muted transition-colors hover:bg-surface-3 hover:text-fg"
              >
                {label}
              </a>
            ))}
          </nav>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            {user ? (
              <Link to="/">
                <Button className="h-10 bg-fg px-5 text-bg hover:opacity-90 dark:bg-fg dark:text-bg">
                  进入工作台 <ArrowRight className="h-4 w-4" />
                </Button>
              </Link>
            ) : (
              <>
                <Link to="/login">
                  <Button variant="ghost" className="h-10 px-4">登录</Button>
                </Link>
                {USE_MOCK ? (
                  <Button
                    loading={demoBusy}
                    onClick={() => void quickDemo()}
                    className="h-10 bg-fg px-5 text-bg hover:opacity-90 dark:bg-fg dark:text-bg"
                  >
                    体验知页
                  </Button>
                ) : (
                  <Link to="/register">
                    <Button className="h-10 bg-fg px-5 text-bg hover:opacity-90 dark:bg-fg dark:text-bg">
                      立即开始
                    </Button>
                  </Link>
                )}
              </>
            )}
          </div>
        </div>
      </header>

      {/* Hero：NotebookLM 风格居中超大标题 + 蓝青柔和渐变高光 */}
      <section id="overview" className="relative overflow-hidden pb-16 pt-12 sm:pb-24 sm:pt-20">
        <div className="pointer-events-none absolute left-1/2 top-0 h-[420px] w-[820px] -translate-x-1/2 rounded-full bg-gradient-to-tr from-[#0b57d0]/12 via-[#10b981]/10 to-[#6366f1]/12 blur-[100px]" />

        <div className="relative mx-auto max-w-[1040px] px-5 text-center sm:px-8">
          <h1 className="text-[40px] font-medium leading-[1.12] tracking-tight sm:text-[64px]">
            理解一切，
            <span className="bg-gradient-to-r from-[#0b57d0] via-[#0ea5e9] to-[#10b981] bg-clip-text text-transparent">
              更有深度
            </span>
          </h1>
          <p className="mx-auto mt-6 max-w-2xl text-base leading-7 text-muted sm:text-[19px] sm:leading-8">
            知页 KnowNote 是你的 AI 研究与笔记伙伴。导入论文、课件、表格与网页，在有据可查的对话、思维导图与学习指南中快速掌握复杂知识。
          </p>

          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            {user ? (
              <Link to="/">
                <Button size="lg" className="h-12 bg-fg px-8 text-[15px] text-bg shadow-lift hover:opacity-90 dark:bg-fg dark:text-bg">
                  打开我的笔记本 <ArrowRight className="h-4 w-4" />
                </Button>
              </Link>
            ) : (
              <>
                {USE_MOCK ? (
                  <Button
                    size="lg"
                    loading={demoBusy}
                    onClick={() => void quickDemo()}
                    className="h-12 bg-fg px-8 text-[15px] text-bg shadow-lift hover:opacity-90 dark:bg-fg dark:text-bg"
                  >
                    立即体验知页
                  </Button>
                ) : (
                  <Link to="/register">
                    <Button size="lg" className="h-12 bg-fg px-8 text-[15px] text-bg shadow-lift hover:opacity-90 dark:bg-fg dark:text-bg">
                      立即体验知页
                    </Button>
                  </Link>
                )}
                <Link to="/login">
                  <Button variant="outline" size="lg" className="h-12 px-7 text-[15px]">
                    已有账号登录
                  </Button>
                </Link>
              </>
            )}
          </div>

          {/* 迷你 NotebookLM 工作台预览窗 */}
          <div className="mt-14 overflow-hidden rounded-[28px] border border-border bg-bg p-3 text-left shadow-pop sm:p-4 dark:bg-surface-2/60">
            <div className="mb-3 flex items-center justify-between rounded-2xl bg-surface px-4 py-2.5 shadow-card">
              <div className="flex items-center gap-2.5">
                <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-[#e9eef6] text-base dark:bg-[#28303d]">🧠</span>
                <span className="truncate text-sm font-medium">大语言模型与 RAG 技术综述</span>
              </div>
              <span className="hidden rounded-full bg-primary-soft px-3 py-1 text-xs font-medium text-primary sm:inline-flex">
                点击 [1] [2] 预览引用溯源
              </span>
            </div>

            <div className="grid gap-3 lg:grid-cols-[260px_1fr_270px]">
              {/* 左栏：来源 */}
              <div className="rounded-2xl border border-border/70 bg-surface p-3.5">
                <div className="mb-3 flex items-center justify-between text-xs font-semibold text-muted">
                  <span>来源 · {SHOWCASE_SOURCES.length}</span>
                  <span className="text-primary">全选</span>
                </div>
                <div className="space-y-1.5">
                  {SHOWCASE_SOURCES.map((s) => {
                    const active = s.id === highlighted.id
                    return (
                      <button
                        key={s.id}
                        type="button"
                        onClick={() => (s.id === 'pdf' || s.id === 'pptx') && setActiveCite(s.id)}
                        className={cn(
                          'flex w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-left transition-colors',
                          active ? 'bg-primary-soft/60 ring-1 ring-primary/30' : 'hover:bg-surface-2',
                        )}
                      >
                        <SourceIcon type={s.type} size="sm" />
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-xs font-medium">{s.title}</p>
                          <p className="text-[11px] text-subtle">{s.meta}</p>
                        </div>
                        <span className="flex h-4 w-4 shrink-0 items-center justify-center rounded bg-primary text-primary-fg">
                          <Check className="h-3 w-3" strokeWidth={3} />
                        </span>
                      </button>
                    )
                  })}
                </div>
                <div className="mt-3 rounded-xl border border-warning/30 bg-warning/10 p-2.5 text-[11px] leading-5">
                  <p className="font-semibold text-warning">引用片段 · {highlighted.meta}</p>
                  <p className="mt-1 text-fg/85">“{highlighted.excerpt}”</p>
                </div>
              </div>

              {/* 中栏：对话 */}
              <div className="flex flex-col justify-between rounded-2xl border border-border/70 bg-surface p-4 sm:p-5">
                <div className="space-y-4">
                  <div className="flex items-center gap-2.5">
                    <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-surface-2 text-xl ring-1 ring-border/70">🧠</span>
                    <div>
                      <h2 className="text-base font-semibold">笔记本概要与循证对话</h2>
                      <p className="text-xs text-muted">4 个已选来源 · 严格基于原文作答</p>
                    </div>
                  </div>
                  <div className="flex justify-end">
                    <div className="rounded-2xl rounded-br-sm bg-primary px-4 py-2 text-sm text-primary-fg">
                      系统如何保证回答中的每个引用都能回到原件？
                    </div>
                  </div>
                  <div className="flex items-start gap-2.5">
                    <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary-soft text-primary">
                      <Sparkles className="h-3.5 w-3.5" />
                    </span>
                    <p className="text-sm leading-7 text-fg/90">
                      在资料解析入库时，系统会保留约 80 字上下文重叠并建立 pgvector 向量索引
                      <button
                        type="button"
                        onClick={() => setActiveCite('pdf')}
                        className={cn(
                          'mx-1 inline-flex h-[18px] min-w-[18px] -translate-y-px items-center justify-center rounded-full px-1 text-[11px] font-semibold transition-all',
                          activeCite === 'pdf' ? 'bg-primary text-primary-fg ring-2 ring-primary/30' : 'bg-primary-soft text-primary',
                        )}
                      >
                        1
                      </button>
                      ，同时记录 PDF 页码、幻灯片或工作表序号，点击角标即可跳转原件定位段落
                      <button
                        type="button"
                        onClick={() => setActiveCite('pptx')}
                        className={cn(
                          'mx-1 inline-flex h-[18px] min-w-[18px] -translate-y-px items-center justify-center rounded-full px-1 text-[11px] font-semibold transition-all',
                          activeCite === 'pptx' ? 'bg-primary text-primary-fg ring-2 ring-primary/30' : 'bg-primary-soft text-primary',
                        )}
                      >
                        2
                      </button>
                      。
                    </p>
                  </div>
                </div>
                <div className="mt-5 flex items-center justify-between rounded-full border border-border bg-surface-2 px-4 py-2 text-xs text-subtle">
                  <span>基于所选来源继续提问…</span>
                  <span className="rounded-full bg-surface-3 px-2.5 py-0.5 text-[11px] text-muted">4 个来源</span>
                </div>
              </div>

              {/* 右栏：工作室 */}
              <div className="flex flex-col gap-3 rounded-2xl border border-border/70 bg-surface p-3.5">
                <div className="grid grid-cols-2 gap-2">
                  {STUDIO_TILES.map((tile, idx) => (
                    <div
                      key={tile.kind}
                      className={cn(
                        'flex items-center gap-2 rounded-xl p-2.5 text-xs font-medium',
                        tile.tint,
                        idx === 0 && 'col-span-2',
                      )}
                    >
                      <StudioToolIcon kind={tile.kind} className="h-4 w-4 shrink-0" />
                      <span className="truncate">{tile.label}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* NotebookLM 标志性交替图文 Showcase */}
      <section id="how-it-works" className="scroll-mt-20 border-t border-border/60 py-16 sm:py-24">
        <div className="mx-auto max-w-[1140px] space-y-20 px-5 sm:space-y-28 sm:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-[28px] font-medium tracking-tight sm:text-[40px]">你的个性化 AI 研究助手</h2>
            <p className="mt-3 text-base text-muted">从散落各处的文档到可查证、可听、可视化的知识体系</p>
          </div>

          {/* Showcase 1: 上传资料 */}
          <div className="grid items-center gap-10 lg:grid-cols-2 lg:gap-16">
            <div>
              <span className="inline-flex rounded-full bg-[#e9eef6] px-3 py-1 text-xs font-medium text-[#0b57d0] dark:bg-[#28303d] dark:text-[#a8c7fa]">
                01 · 汇集来源
              </span>
              <h3 className="mt-4 text-2xl font-medium tracking-tight sm:text-3xl">上传多种资料，一键建立私有知识底座</h3>
              <p className="mt-4 text-base leading-7 text-muted">
                无论是学术 PDF、课程 PPT、实验 Excel 表格、Word 文档、网页文章还是会议录音，知页都会自动提取全文、识别目录层级，并生成结构化概要与主题标签。
              </p>
            </div>
            <div className="rounded-[28px] bg-[#e9eef6]/70 p-6 sm:p-8 dark:bg-surface-2">
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
                {ALL_FORMATS.map((f) => (
                  <div
                    key={f.type}
                    className="flex flex-col items-center gap-2 rounded-2xl border border-border/60 bg-surface p-3.5 text-center shadow-card"
                  >
                    <SourceIcon type={f.type} size="md" />
                    <span className="text-xs font-medium">{f.label}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* 展示 2：工作室 */}
          <div className="grid items-center gap-10 lg:grid-cols-2 lg:gap-16">
            <div className="order-2 rounded-[28px] bg-[#f3e8fd]/65 p-6 sm:p-8 lg:order-1 dark:bg-surface-2">
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-2.5">
                  {STUDIO_TILES.slice(0, 4).map((t) => (
                    <div key={t.kind} className="rounded-2xl border border-border/60 bg-surface p-3.5 shadow-card">
                      <span className={cn('inline-flex h-8 w-8 items-center justify-center rounded-xl', t.tint)}>
                        <StudioToolIcon kind={t.kind} className="h-4 w-4" />
                      </span>
                      <p className="mt-2.5 text-sm font-semibold">{t.label}</p>
                      <p className="mt-0.5 text-xs text-muted">{t.desc}</p>
                    </div>
                  ))}
                </div>
              </div>
            </div>
            <div className="order-1 lg:order-2">
              <span className="inline-flex rounded-full bg-[#f3e8fd] px-3 py-1 text-xs font-medium text-[#6b21a8] dark:bg-[#30283c] dark:text-[#d8b4fe]">
                02 · 多维沉淀
              </span>
              <h3 className="mt-4 text-2xl font-medium tracking-tight sm:text-3xl">即时生成思维导图与学习指南</h3>
              <p className="mt-4 text-base leading-7 text-muted">
                在右侧「工作室」面板，把零散材料一键转化为可缩放折叠的思维导图、考前复习指南、简报、FAQ 与时间线，笔记还可反向转为新来源。
              </p>
            </div>
          </div>

          {/* Showcase 3: 溯源引用 */}
          <div className="grid items-center gap-10 lg:grid-cols-2 lg:gap-16">
            <div>
              <span className="inline-flex rounded-full bg-[#e6f4ea] px-3 py-1 text-xs font-medium text-[#146c2e] dark:bg-[#223229] dark:text-[#6dd58c]">
                03 · 精准溯源
              </span>
              <h3 className="mt-4 text-2xl font-medium tracking-tight sm:text-3xl">每个结论都有依据，点击直达原件页码</h3>
              <p className="mt-4 text-base leading-7 text-muted">
                基于 pgvector 向量相似度检索与分节位置索引，知页的每次回答都附带清晰的内联角标。悬停即可预览引文片段，点击直接跳转 PDF 页码或 PPT 幻灯片核对原文。
              </p>
            </div>
            <div className="rounded-[28px] bg-[#e6f4ea]/70 p-6 sm:p-8 dark:bg-surface-2">
              <div className="space-y-3 rounded-2xl border border-border/70 bg-surface p-5 shadow-card">
                <p className="text-xs font-medium text-muted">AI 循证回答</p>
                <p className="text-sm leading-7">
                  实验记录显示，当切片长度设为 500 字并保留 80 字重叠时，跨段落信息完整性最高，综合召回准确率达到 94.2%
                  <span className="mx-1 inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-primary px-1.5 text-[11px] font-semibold text-primary-fg">1</span>
                  。
                </p>
                <div className="rounded-xl border border-border/70 bg-surface-2 p-3 text-xs">
                  <div className="flex items-center justify-between font-medium text-primary">
                    <span>检索消融实验记录.xlsx</span>
                    <span>第 1 个工作表</span>
                  </div>
                  <p className="mt-1 leading-5 text-muted">“……chunk=500, overlap=80, Top-5 召回下引用命中率 94.2%……”</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* NotebookLM 适用场景三卡 */}
      <section id="use-cases" className="scroll-mt-20 border-t border-border/60 bg-surface-2/60 py-16 sm:py-20 dark:bg-surface/30">
        <div className="mx-auto max-w-[1140px] px-5 sm:px-8">
          <div className="mb-12 text-center">
            <h2 className="text-[28px] font-medium tracking-tight sm:text-[36px]">大家都如何用知页</h2>
            <p className="mt-2 text-base text-muted">无论科研、备考还是行业调研，都能快速上手</p>
          </div>
          <div className="grid gap-5 md:grid-cols-3">
            {[
              {
                icon: Microscope,
                tone: 'bg-[#e9eef6] text-[#0b57d0] dark:bg-[#28303d] dark:text-[#a8c7fa]',
                title: '论文精读与毕业设计',
                desc: '导入数十篇 PDF 文献与开题报告，快速对比研究方法、追溯实验数据出处，生成文献综述框架。',
              },
              {
                icon: GraduationCap,
                tone: 'bg-[#e6f4ea] text-[#146c2e] dark:bg-[#223229] dark:text-[#6dd58c]',
                title: '课程学习与考前冲刺',
                desc: '把老师的 PPT 课件、电子书章节与复习录音放进同一本笔记本，一键生成学习指南、术语表与问答测验。',
              },
              {
                icon: Briefcase,
                tone: 'bg-[#fef7e0] text-[#b06000] dark:bg-[#36321f] dark:text-[#fcd34d]',
                title: '产品调研与方案整理',
                desc: '抓取多篇行业网页与访谈纪要，用「简报文档」和「思维导图」梳理脉络，并随时导出 Markdown 归档。',
              },
            ].map((item) => (
              <article key={item.title} className="rounded-3xl border border-border/70 bg-surface p-7 shadow-card">
                <span className={cn('inline-flex h-12 w-12 items-center justify-center rounded-2xl', item.tone)}>
                  <item.icon className="h-6 w-6" />
                </span>
                <h3 className="mt-5 text-lg font-semibold">{item.title}</h3>
                <p className="mt-2.5 text-sm leading-6 text-muted">{item.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      {/* NotebookLM 招牌隐私承诺条 */}
      <section id="privacy" className="scroll-mt-20 py-16 sm:py-20">
        <div className="mx-auto max-w-[1040px] px-5 sm:px-8">
          <div className="flex flex-col items-start justify-between gap-6 rounded-[28px] bg-[#e9eef6] p-8 sm:flex-row sm:items-center sm:p-10 dark:bg-surface-2">
            <div className="max-w-xl">
              <div className="inline-flex items-center gap-2 text-sm font-semibold text-primary">
                <ShieldCheck className="h-5 w-5" /> 数据隐私与安全隔离
              </div>
              <h2 className="mt-3 text-2xl font-medium tracking-tight">我们同样重视你的个人资料隐私</h2>
              <p className="mt-2.5 text-sm leading-6 text-muted">
                你的个人笔记、上传文档、向量分块与对话记录均按用户 ID 严格隔离保存，仅用于你自己的检索与回答生成，绝不会用你的私人资料训练模型。
              </p>
            </div>
            <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-3xl bg-surface text-primary shadow-card">
              <Lock className="h-9 w-9" />
            </div>
          </div>

          <div className="mt-14 text-center">
            <h2 className="text-2xl font-medium tracking-tight sm:text-3xl">准备好开启你的第一本知页了吗？</h2>
            <div className="mt-6 flex flex-wrap justify-center gap-3">
              {user ? (
                <Link to="/">
                  <Button size="lg" className="h-12 bg-fg px-8 text-bg hover:opacity-90 dark:bg-fg dark:text-bg">
                    进入工作台 <ChevronRight className="h-4 w-4" />
                  </Button>
                </Link>
              ) : (
                <>
                  <Link to="/register">
                    <Button size="lg" className="h-12 bg-fg px-8 text-bg hover:opacity-90 dark:bg-fg dark:text-bg">
                      免费注册使用 <ChevronRight className="h-4 w-4" />
                    </Button>
                  </Link>
                  <Link to="/login">
                    <Button size="lg" variant="outline" className="h-12 px-7">
                      登录账户
                    </Button>
                  </Link>
                </>
              )}
            </div>
          </div>
        </div>
      </section>

      <footer className="border-t border-border/60 py-8 text-xs text-muted">
        <div className="mx-auto flex max-w-[1200px] flex-col items-center justify-between gap-3 px-5 sm:flex-row sm:px-8">
          <div className="flex items-center gap-2">
            <Logo compact to={user ? '/welcome' : '/'} />
            <span>知页 KnowNote · 基于大语言模型的个人资料管理系统</span>
          </div>
          <div className="flex items-center gap-5">
            <Link to="/login" className="hover:text-fg">登录</Link>
            <Link to="/register" className="hover:text-fg">注册</Link>
            <span>简体中文</span>
          </div>
        </div>
      </footer>
    </div>
  )
}
