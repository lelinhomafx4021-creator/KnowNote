/**
 * 模拟后端：内存数据 + 随机延迟。
 * 由 USE_MOCK 选择执行，与真实接口保持相同的领域类型。
 */
import type {
  Tag, Notebook, SampleNotebook, Source, Note, StudioArtifact, StudioKind, Message, Citation,
  ChatStreamEvent, NotebookGuide, AudioOverview, UserSettings, MindMapNode, SlideDeck, ExportFormat, SourceType,
} from '@/types'
import { sleep, uid } from '@/lib/utils'
import * as db from './data'
import { mockSourceStructure, sectionContent } from './sourceFixtures'

const now = () => new Date().toISOString()
const delay = (min = 150, max = 450) => sleep(min + Math.random() * (max - min))
const clone = <T,>(v: T): T => JSON.parse(JSON.stringify(v))

function touch(notebookId: string) {
  const nb = db.notebooks.find((n) => n.id === notebookId)
  if (nb) {
    nb.updatedAt = now()
    nb.sourceCount = db.sources.filter((s) => s.notebookId === notebookId).length
  }
}

/* ---------------- 笔记本 ---------------- */
export async function listNotebooks(tag?: string): Promise<Notebook[]> {
  await delay()
  return clone(db.notebooks.filter((n) => !tag || n.tags.some((t) => t.id === tag)))
}
export async function getNotebook(id: string): Promise<Notebook> {
  await delay(80, 200)
  const nb = db.notebooks.find((n) => n.id === id)
  if (!nb) throw new Error('笔记本不存在')
  return clone(nb)
}
const EMOJIS = ['📘', '🧪', '🎯', '🌱', '🧩', '🚀', '📝', '🔭']
export async function createNotebook(title = '未命名笔记本'): Promise<Notebook> {
  await delay()
  const nb: Notebook = {
    id: uid('nb'), title, emoji: EMOJIS[Math.floor(Math.random() * EMOJIS.length)],
    color: Math.floor(Math.random() * 8), tags: [], sourceCount: 0, createdAt: now(), updatedAt: now(),
  }
  db.notebooks.unshift(nb)
  db.messages[nb.id] = []
  return clone(nb)
}
export async function updateNotebook(id: string, patch: Partial<Notebook>): Promise<Notebook> {
  await delay(80, 200)
  const nb = db.notebooks.find((n) => n.id === id)
  if (!nb) throw new Error('笔记本不存在')
  Object.assign(nb, patch, { updatedAt: now() })
  return clone(nb)
}
export async function deleteNotebook(id: string): Promise<void> {
  await delay()
  const i = db.notebooks.findIndex((n) => n.id === id)
  if (i >= 0) db.notebooks.splice(i, 1)
}

/* ---------------- 来源 ---------------- */
export async function listSources(notebookId: string): Promise<Source[]> {
  await delay()
  return clone(db.sources.filter((s) => s.notebookId === notebookId))
}

const originalFiles = new Map<string, { url: string; name: string }>()

export function sourceDownloadUrl(source: Source): string {
  return originalFiles.get(source.id)?.url || `data:text/plain;charset=utf-8,${encodeURIComponent(source.content || source.summary || source.title)}`
}

export function sourceDownloadName(source: Source): string {
  return originalFiles.get(source.id)?.name || `${source.title}.txt`
}

export interface AddSourceInput {
  type: SourceType
  title: string
  size?: number
  url?: string
  text?: string
}

export async function getSource(id: string): Promise<Source> {
  const source = db.sources.find((s) => s.id === id)
  if (!source) throw new Error('来源不存在')
  return clone(source)
}

/** 模拟上传 + 解析：通过 onProgress 回调推送状态变化 */
export async function addSource(
  notebookId: string,
  input: AddSourceInput & { file?: File },
  onProgress?: (s: Source) => void,
  signal?: AbortSignal,
): Promise<Source> {
  const src: Source = {
    id: uid('src'), notebookId, type: input.type, title: input.title, size: input.size, url: input.url,
    status: input.type === 'web' || input.type === 'youtube' || input.type === 'text' ? 'processing' : 'uploading',
    progress: 0, createdAt: now(), originalExt: input.file?.name.split('.').pop()?.toLowerCase(),
  }
  if (input.file) originalFiles.set(src.id, { url: URL.createObjectURL(input.file), name: input.file.name })
  db.sources.push(src)
  touch(notebookId)
  if (!signal?.aborted) onProgress?.(clone(src))

  if (src.status === 'uploading') {
    for (let p = 10; p <= 100; p += 15 + Math.random() * 15) {
      await sleep(180)
      src.progress = Math.min(100, Math.round(p))
      if (!signal?.aborted) onProgress?.(clone(src))
    }
    src.status = 'processing'
    src.progress = 100
    if (!signal?.aborted) onProgress?.(clone(src))
  }
  await sleep(1200 + Math.random() * 800)
  const text = input.text?.trim()
  const structure = mockSourceStructure(input.type)
  Object.assign(src, {
    ...structure,
    status: 'ready',
    wordCount: text ? text.length : 1500 + Math.floor(Math.random() * 6000),
    summary: text
      ? `这段文本主要讨论了：${text.slice(0, 60)}${text.length > 60 ? '……' : ''}。系统已完成分段与向量化，可在对话中引用。`
      : `系统已完成对「${input.title}」的解析：抽取正文、切分为语义片段并生成向量索引。该资料涉及的核心概念已整理为下方关键主题，可在对话中直接提问。`,
    topics: ['核心概念', '方法论', '实践案例', '结论'],
    content: text || (structure.sections ? sectionContent(structure.sections) : '（原文预览）离线演示正文，可在对话中查看引用。'),
  })
  touch(notebookId)
  if (!signal?.aborted) onProgress?.(clone(src))
  return clone(src)
}

export async function renameSource(id: string, title: string): Promise<void> {
  await delay(60, 120)
  const s = db.sources.find((x) => x.id === id)
  if (s) s.title = title
}
export async function deleteSource(id: string): Promise<void> {
  await delay(80, 200)
  const i = db.sources.findIndex((s) => s.id === id)
  if (i >= 0) {
    const nbId = db.sources[i].notebookId
    const file = originalFiles.get(id)
    if (file) URL.revokeObjectURL(file.url)
    originalFiles.delete(id)
    db.sources.splice(i, 1)
    touch(nbId)
  }
}

/* ---------------- 笔记本概要（Notebook Guide） ---------------- */
export async function getGuide(notebookId: string): Promise<NotebookGuide> {
  await delay(300, 700)
  const srcs = db.sources.filter((s) => s.notebookId === notebookId && s.status === 'ready')
  if (!srcs.length) return { summary: '', suggestedQuestions: [] }
  if (notebookId === 'nb_1') {
    return {
      summary:
        '这些资料围绕**检索增强生成（RAG）与个人资料管理**展开：阅读指南介绍从资料处理到可追溯回答的流程；六张答辩幻灯片梳理知页的系统架构、结构化解析和引用定位；实验表格记录切分参数与来源类型。工程文章、开题报告与 LLM 讲座补充实践和研究背景。你可以通过目录逐页阅读，也可以从回答中的引用回到原件。',
      suggestedQuestions: ['如何从引用回到原件？', '我的系统应该如何设计文档切分策略？', '总结一下答辩演示文稿的技术路线'],
    }
  }
  return {
    summary: `本笔记本包含 ${srcs.length} 份资料，主要涉及 ${[...new Set(srcs.flatMap((s) => s.topics ?? []))].slice(0, 5).join('、')} 等主题。`,
    suggestedQuestions: ['帮我总结这些资料的核心观点', '这些资料之间有什么联系？', '列出需要重点掌握的概念'],
  }
}

/* ---------------- 对话（流式） ---------------- */
export async function listMessages(notebookId: string): Promise<Message[]> {
  await delay(100, 250)
  return clone(db.messages[notebookId] ?? [])
}
export async function clearMessages(notebookId: string): Promise<void> {
  await delay(80, 150)
  db.messages[notebookId] = []
}

function cite(index: number, s: Source): Citation {
  const section = s.sections?.find((item) => item.page === (s.type === 'pptx' ? 4 : s.type === 'pdf' ? 3 : 1)) || s.sections?.[0]
  const quote = section?.excerpt || (s.content ?? s.summary ?? '').replace(/\n+/g, ' ').slice(0, 120) + '…'
  return { index, sourceId: s.id, sourceTitle: s.title, quote, page: section?.page, locationLabel: section?.label }
}

function composeAnswer(question: string, srcs: Source[]): { text: string; citations: Citation[] } {
  if (!srcs.length) return { text: '请在来源面板中勾选至少一个已就绪来源，我会根据你的资料回答并标注出处。', citations: [] }
  const chosen = [...new Set([srcs.find((s) => s.type === 'pdf'), srcs.find((s) => s.type === 'pptx'), srcs.find((s) => s.type === 'xlsx' || s.type === 'csv'), ...srcs].filter((s): s is Source => !!s))].slice(0, 3)
  const citations = chosen.map((s, i) => cite(i + 1, s))
  return {
    citations,
    text: `针对「${question}」，从所选资料中整理出以下内容：

${citations.map((c) => `**${c.sourceTitle}**${c.locationLabel ? `（${c.locationLabel}）` : ''}\n\n${c.quote} [${c.index}]`).join('\n\n')}

你可以点击引用角标核对原文，也可以从来源目录打开对应页面，确认上下文后再保存到笔记。`,
  }
}

/**
 * 流式问答。真实实现应调用 POST /api/notebooks/:id/chat（SSE），
 * 逐条产出 delta / citations / done 事件。
 */
export async function* streamChat(
  notebookId: string,
  question: string,
  sourceIds: string[],
  signal?: AbortSignal,
): AsyncGenerator<ChatStreamEvent> {
  const list = (db.messages[notebookId] ??= [])
  list.push({ id: uid('msg'), role: 'user', content: question, createdAt: now() })
  const srcs = db.sources.filter((s) => sourceIds.includes(s.id) && s.status === 'ready')
  const { text, citations } = composeAnswer(question, srcs)

  await sleep(600)
  if (signal?.aborted) return
  yield { type: 'citations', citations }
  let out = ''
  const chunks = text.match(/[\s\S]{1,3}/g) ?? []
  for (const ch of chunks) {
    if (signal?.aborted) break
    await sleep(12 + Math.random() * 18)
    if (signal?.aborted) break
    out += ch
    yield { type: 'delta', text: ch }
  }
  const id = uid('msg')
  list.push({ id, role: 'assistant', content: out, citations, createdAt: now() })
  yield { type: 'done', messageId: id }
}

export async function feedbackMessage(id: string, value: 'up' | 'down' | null): Promise<void> {
  for (const list of Object.values(db.messages)) {
    const message = list.find((m) => m.id === id)
    if (message) message.feedback = value
  }
}

/* ---------------- 笔记 ---------------- */
export async function listNotes(notebookId: string): Promise<Note[]> {
  await delay()
  return clone(db.notes.filter((n) => n.notebookId === notebookId).sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)))
}
export async function createNote(notebookId: string, data: Pick<Note, 'title' | 'content' | 'origin'>): Promise<Note> {
  await delay(80, 200)
  const n: Note = { id: uid('note'), notebookId, ...data, createdAt: now(), updatedAt: now() }
  db.notes.push(n)
  return clone(n)
}
export async function updateNote(id: string, patch: Partial<Pick<Note, 'title' | 'content'>>): Promise<Note> {
  await delay(60, 150)
  const n = db.notes.find((x) => x.id === id)
  if (!n) throw new Error('笔记不存在')
  Object.assign(n, patch, { updatedAt: now() })
  return clone(n)
}
export async function deleteNote(id: string): Promise<void> {
  await delay(60, 150)
  const i = db.notes.findIndex((n) => n.id === id)
  if (i >= 0) db.notes.splice(i, 1)
}

/* ---------------- 工作室 ---------------- */
export const STUDIO_TITLES: Record<StudioKind, string> = {
  ppt: 'PPT 课件',
  word: 'Word 文档',
  pdf: 'PDF 讲义',
  'study-guide': '学习指南',
  briefing: '简报文档',
  faq: '常见问题',
  timeline: '时间线',
  mindmap: '思维导图',
}

export async function listArtifacts(notebookId: string): Promise<StudioArtifact[]> {
  await delay()
  return clone(db.artifacts.filter((a) => a.notebookId === notebookId))
}

function buildMindmap(notebookId: string): MindMapNode {
  const nb = db.notebooks.find((n) => n.id === notebookId)
  const srcs = db.sources.filter((s) => s.notebookId === notebookId && s.status === 'ready')
  if (notebookId === 'nb_1') {
    return {
      id: 'root', label: 'RAG 个人资料管理系统',
      children: [
        { id: 'a', label: 'RAG 原理', children: [
          { id: 'a1', label: '参数化记忆 (seq2seq)' }, { id: 'a2', label: '非参数化记忆 (向量索引)' },
          { id: 'a3', label: 'RAG-Sequence / RAG-Token' } ] },
        { id: 'b', label: '工程流水线', children: [
          { id: 'b1', label: '文档切分' }, { id: 'b2', label: 'Embedding' }, { id: 'b3', label: '向量数据库' },
          { id: 'b4', label: '检索与重排序' } ] },
        { id: 'c', label: '系统功能', children: [
          { id: 'c1', label: '多格式资料导入' }, { id: 'c2', label: '带引用的智能问答' },
          { id: 'c3', label: '摘要 / FAQ / 思维导图' } ] },
        { id: 'd', label: 'LLM 背景', children: [
          { id: 'd1', label: '预训练与 RLHF' }, { id: 'd2', label: '提示注入与安全' } ] },
      ],
    }
  }
  return {
    id: 'root', label: nb?.title ?? '主题',
    children: srcs.map((s) => ({
      id: s.id, label: s.title.replace(/\.(pdf|docx|md)$/, '').slice(0, 18),
      children: (s.topics ?? []).slice(0, 4).map((t, i) => ({ id: `${s.id}_${i}`, label: t })),
    })),
  }
}

function buildSlideDeck(notebookId: string, srcs: Source[], topic?: string): SlideDeck {
  const nb = db.notebooks.find((n) => n.id === notebookId)
  const topics = [...new Set(srcs.flatMap((s) => s.topics ?? []))]
  const deckTitle = topic?.trim() || `${nb?.title ?? '专题'} · 课堂精讲课件`
  return {
    title: deckTitle,
    subtitle: `基于 ${srcs.length} 份来源${topic ? '（RAG 向量检索重点）' : ''}生成 · 教学演示文稿`,
    slides: [
      {
        title: '课程引入与学习目标',
        subtitle: `覆盖主题：${topics.slice(0, 3).join('、') || '核心知识脉络'}`,
        bullets: [
          '明确本讲要解决的核心问题与教学重点',
          '结合来源原文把握关键概念与适用前提',
          '通过课堂对比与思考题完成迁移应用',
        ],
        notes: '开场引导学生回顾已有知识，指出本节依据的教材/资料来源范围。',
      },
      ...srcs.slice(0, 4).map((s, idx) => ({
        title: s.title.replace(/\.(pdf|docx|pptx|md|txt)$/i, ''),
        subtitle: s.topics?.slice(0, 2).join(' · ') || `来源 ${idx + 1}`,
        bullets: [
          s.summary || '提炼该来源核心观点与实验依据。',
          ...(s.topics ?? []).slice(0, 2).map((t) => `重点关注：**${t}** 的定义与论证细节`),
        ],
        notes: `讲解《${s.title}》时提示学生对照原文关键页码或表格数据核验前提条件。`,
      })),
      {
        title: '课堂小结与思考题',
        subtitle: '知识巩固与课后延展',
        bullets: [
          `核心概念回顾：${topics.slice(0, 4).join(' → ') || '定义 · 原理 · 对比 · 应用'}`,
          '思考：不同来源的方法在边界条件下有哪些权衡？',
        ],
        notes: '预留 5 分钟组织学生提问或填写随堂反馈。',
      },
    ],
  }
}

function buildContent(kind: StudioKind, srcs: Source[], topic?: string): string {
  const topics = [...new Set(srcs.flatMap((s) => s.topics ?? []))]
  const focusNote = topic?.trim() ? `（RAG 定向主题：${topic.trim()}）` : ''
  switch (kind) {
    case 'word':
      return `> **文档定位**：Word 结构化教案与讲义 ${focusNote}\n\n# ${topic?.trim() || '课程教案与核心讲义'}\n\n## 教学目标\n${topics.slice(0, 4).map((t) => `- 掌握 **${t}** 的核心概念与原文依据`).join('\n')}\n\n## 核心知识精讲\n${srcs.map((s) => `### ${s.title.replace(/\.(pdf|docx|pptx|md|txt)$/i, '')}\n${s.summary ?? ''}`).join('\n\n')}\n\n## 重点对比表\n| 来源 | 核心主题 | 关键要点 |\n|---|---|---|\n${srcs.map((s) => `| ${s.title} | ${(s.topics ?? []).slice(0, 2).join('、')} | ${(s.summary ?? '').slice(0, 32)} |`).join('\n')}\n\n## 随堂练习与参考要点\n1. 结合资料说明「${topics[0] ?? '核心概念'}」在实际问题中的作用与限制。`
    case 'pdf':
      return `# ${topic?.trim() || '学生导学讲义（打印版）'}\n\n> **导读提示**：本讲义整理自 ${srcs.length} 份来源 ${focusNote}，适合导出 PDF 排版发给学生。\n\n## 核心要点速览\n${topics.slice(0, 5).map((t) => `- **${t}**：见对应课程来源原文论证与案例。`).join('\n')}\n\n## 来源要点对照\n| 资料标题 | 重点主题 |\n|---|---|\n${srcs.map((s) => `| ${s.title} | ${(s.topics ?? []).slice(0, 3).join('、')} |`).join('\n')}\n\n## 课后自测题\n1. 如何区分各来源中的明确事实与推论？`
    case 'study-guide':
      return `## 学习目标\n${topics.slice(0, 4).map((t) => `- 理解并能解释 **${t}**`).join('\n')}\n\n## 简答题\n${topics.slice(0, 5).map((t, i) => `${i + 1}. 请简述「${t}」的含义及其作用。`).join('\n')}\n\n## 论述题\n1. 结合资料，论述各核心概念之间的关系。\n2. 如果要将这些方法应用到实际系统中，需要注意哪些问题？\n\n## 术语表\n${topics.map((t) => `- **${t}**：见相关来源中的定义。`).join('\n')}`
    case 'briefing':
      return `## 执行摘要\n本简报基于 ${srcs.length} 份资料生成，概括了其中的核心主题与关键结论。\n\n## 核心主题\n${srcs.map((s) => `### ${s.title.replace(/\.(pdf|docx|md)$/, '')}\n${s.summary ?? ''}`).join('\n\n')}\n\n## 结论与建议\n- 各资料在核心观点上相互印证；\n- 建议优先关注：${topics.slice(0, 3).join('、')}。`
    case 'faq':
      return topics.slice(0, 6).map((t) => `### ${t}是什么？\n根据资料，${t}是本笔记本中反复出现的关键概念，相关内容可参见对应来源的摘要与原文。`).join('\n\n')
    case 'timeline':
      return `## 时间线\n| 时间 | 事件 |\n|---|---|\n| 2017 | Transformer 架构提出 |\n| 2019 | DPR 稠密段落检索被提出 |\n| 2020 | RAG 论文发表，提出检索增强生成框架 |\n| 2022 | ChatGPT 发布，LLM 进入大众视野 |\n| 2023 | 向量数据库与 RAG 工程化快速普及 |\n| 2026 | 本毕业设计：基于 LLM 的个人资料管理系统 |\n\n## 人物表\n- **Patrick Lewis**：RAG 论文第一作者\n- **Andrej Karpathy**：《Intro to LLMs》讲者`
    default:
      return ''
  }
}

export async function generateArtifact(
  notebookId: string,
  kind: StudioKind,
  options?: { topic?: string; preset?: string; sourceIds?: string[] },
): Promise<StudioArtifact> {
  await sleep(1000 + Math.random() * 800)
  const srcs = db.sources.filter((s) => s.notebookId === notebookId && s.status === 'ready' && (!options?.sourceIds?.length || options.sourceIds.includes(s.id)))
  const baseTitle = STUDIO_TITLES[kind]
  const topic = options?.topic?.trim()
  const title = topic ? `${baseTitle} · ${topic.slice(0, 24)}` : baseTitle
  const format: ExportFormat | undefined = kind === 'ppt' ? 'pptx' : kind === 'pdf' ? 'pdf' : kind === 'word' ? 'docx' : undefined
  const slides = kind === 'ppt' ? buildSlideDeck(notebookId, srcs, topic) : undefined
  const content = slides
    ? slides.slides.map((s, i) => `## 第${i + 1}页 · ${s.title}\n${s.bullets.map((b) => `- ${b}`).join('\n')}${s.notes ? `\n> **教师讲稿**：${s.notes}` : ''}`).join('\n\n')
    : kind === 'mindmap' ? undefined : buildContent(kind, srcs, topic)
  const a: StudioArtifact = {
    id: uid('art'),
    notebookId,
    kind,
    title,
    format,
    preset: options?.preset,
    prompt: topic,
    retrievalMode: topic ? 'rag' : 'sources',
    createdAt: now(),
    ...(kind === 'mindmap' ? { mindmap: buildMindmap(notebookId) } : { content, slides }),
  }
  db.artifacts.unshift(a)
  return clone(a)
}

export async function exportArtifactFile(artifact: StudioArtifact, ext: ExportFormat): Promise<{ blob: Blob; filename: string }> {
  await delay(120, 250)
  const text = artifact.content || JSON.stringify(artifact.mindmap ?? {}, null, 2)
  return { blob: new Blob([`# ${artifact.title}\n\n（离线演示模式导出文本，启动后端服务后可导出真实 .${ext} 二进制文件）\n\n${text}`], { type: 'text/markdown;charset=utf-8' }), filename: `${artifact.title || '知页产物'}.md` }
}

export async function exportMarkdownFallback(title: string, content: string, ext: ExportFormat): Promise<{ blob: Blob; filename: string }> {
  await delay(120, 250)
  return { blob: new Blob([`# ${title}\n\n（离线演示模式导出文本，启动后端服务后可导出真实 .${ext} 二进制文件）\n\n${content}`], { type: 'text/markdown;charset=utf-8' }), filename: `${title || '学习笔记'}.md` }
}
export async function deleteArtifact(id: string): Promise<void> {
  await delay(60, 120)
  const i = db.artifacts.findIndex((a) => a.id === id)
  if (i >= 0) db.artifacts.splice(i, 1)
}

/* ---------------- 音频概览 ---------------- */
const audios: Record<string, AudioOverview> = {}
export async function getAudio(notebookId: string): Promise<AudioOverview> {
  await delay(80, 160)
  return clone(audios[notebookId] ?? { notebookId, status: 'idle' })
}
export async function generateAudio(notebookId: string): Promise<AudioOverview> {
  audios[notebookId] = { notebookId, status: 'generating' }
  await sleep(3500)
  const nb = db.notebooks.find((n) => n.id === notebookId)
  audios[notebookId] = { notebookId, status: 'ready', title: `深度对谈：${nb?.title ?? ''}`, transcript: `主持人甲：欢迎来到知页。今天我们一起聊聊「${nb?.title ?? '学习资料'}」。\n\n主持人乙：我们可以先梳理核心概念，再结合来源中的例子理解它们。\n\n主持人甲：带着问题回到原文，能够帮助我们建立自己的知识结构。` }
  return clone(audios[notebookId])
}

/* ---------------- 用户设置 ---------------- */
export async function getSettings(): Promise<UserSettings> {
  await delay(80, 160)
  return clone(db.settings)
}
export async function updateSettings(patch: Partial<UserSettings>): Promise<UserSettings> {
  await delay(80, 160)
  Object.assign(db.settings, patch)
  return clone(db.settings)
}

/* ---------------- 标签 ---------------- */
export async function listTags(): Promise<Tag[]> { await delay(60, 100); return clone(db.tags) }
export async function createTag(name: string, color: string): Promise<Tag> {
  name = name.trim()
  if (!name || name.length > 50) throw new Error('标签名称需要 1 到 50 个字符')
  if (db.tags.some((t) => t.name === name)) throw new Error('该标签名称已存在')
  const tag = { id: uid('tag'), name, color, createdAt: now() }; db.tags.push(tag); return clone(tag)
}
export async function updateTag(id: string, patch: Partial<Pick<Tag, 'name' | 'color'>>): Promise<Tag> {
  const tag = db.tags.find((t) => t.id === id)
  if (!tag) throw new Error('标签不存在')
  if (patch.name !== undefined && (!patch.name.trim() || patch.name.trim().length > 50)) throw new Error('标签名称需要 1 到 50 个字符')
  if (patch.name && db.tags.some((t) => t.id !== id && t.name === patch.name!.trim())) throw new Error('该标签名称已存在')
  Object.assign(tag, patch, patch.name ? { name: patch.name.trim() } : {})
  for (const nb of db.notebooks) nb.tags = nb.tags.map((t) => t.id === id ? clone(tag) : t)
  return clone(tag)
}
export async function deleteTag(id: string): Promise<void> {
  const i = db.tags.findIndex((t) => t.id === id); if (i >= 0) db.tags.splice(i, 1)
  for (const nb of db.notebooks) nb.tags = nb.tags.filter((t) => t.id !== id)
}
export async function setNotebookTags(id: string, tagIds: string[]): Promise<Notebook> {
  const nb = db.notebooks.find((n) => n.id === id)
  if (!nb || tagIds.some((id) => !db.tags.some((t) => t.id === id))) throw new Error('资源不存在')
  nb.tags = clone(db.tags.filter((t) => tagIds.includes(t.id))); touch(id); return clone(nb)
}

/* ---------------- 精选示例 ---------------- */
const SAMPLE_MAP: Record<string, { slug: string; notebookId: string; description: string; tone: number }> = {
  'rag-guide': {
    slug: 'rag-guide',
    notebookId: 'nb_1',
    description: '包含 RAG 阅读指南、答辩幻灯片与实验记录，体验引用定位与思维导图。',
    tone: 0,
  },
  'db-review': {
    slug: 'db-review',
    notebookId: 'nb_2',
    description: '数据库系统原理章节讲义、事务并发笔记与历年考题梳理。',
    tone: 3,
  },
  'pm-prep': {
    slug: 'pm-prep',
    notebookId: 'nb_3',
    description: '产品经理高频面试题库与 PRD 撰写规范，适合提问演练。',
    tone: 5,
  },
  'ml-notes': {
    slug: 'ml-notes',
    notebookId: 'nb_4',
    description: '机器学习讲义、反向传播推导与可视化课程笔记，一站复习。',
    tone: 2,
  },
}

export async function listSamples(): Promise<SampleNotebook[]> {
  await delay(80, 180)
  return Object.values(SAMPLE_MAP).map((item) => {
    const nb = db.notebooks.find((n) => n.id === item.notebookId)
    return {
      slug: item.slug,
      title: nb?.title ?? item.slug,
      emoji: nb?.emoji ?? '📘',
      description: item.description,
      tone: item.tone,
      sourceCount: nb?.sourceCount ?? 0,
      ready: true,
      addedNotebookId: nb?.id,
    }
  })
}

export async function useSample(slug: string): Promise<Notebook> {
  await delay(100, 220)
  const item = SAMPLE_MAP[slug]
  const nb = item ? db.notebooks.find((n) => n.id === item.notebookId) : db.notebooks[0]
  if (!nb) throw new Error('示例不存在')
  return clone(nb)
}
